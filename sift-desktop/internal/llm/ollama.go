package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"sift.local/internal/config" // 引入配置包
)

type Options struct {
	Temperature   float32 `json:"temperature"`
	RepeatPenalty float32 `json:"repeat_penalty"`
	TopP          float32 `json:"top_p"`
}

type GenerateRequest struct {
	Model   string   `json:"model"`
	Images  []string `json:"images,omitempty"` // Base64 数组
	Prompt  string   `json:"prompt"`
	Stream  bool     `json:"stream"`
	Options Options  `json:"options"`
}

// GenerateResponse 增加 Thinking 字段
type GenerateResponse struct {
	Model    string `json:"model"`
	Response string `json:"response"`
	Thinking string `json:"thinking"` // 捕获模型的思考过程
	Done     bool   `json:"done"`
}

type Message struct {
	Role    string `json:"role"` // "user" 或 "assistant"
	Content string `json:"content"`
}

// AskOllama 增加 cfg 参数
// 修改函数签名，接收历史记录
func AskOllama(cfg *config.AppConfig, context string, history []Message, question string) (string, error) {
	var historyStr strings.Builder
	for _, msg := range history {
		roleName := "用户"
		if msg.Role == "assistant" {
			roleName = "助手"
		}
		historyStr.WriteString(fmt.Sprintf("%s: %s\n", roleName, msg.Content))
	}

	fullPrompt := fmt.Sprintf(`你是 Sift 知识库助手。请严谨的根据提供的资料和对话历史回答用户的问题，绝对不能胡编乱造或回复不想关的内容。
【参考资料】：%s
【对话历史】：
%s
【当前问题】：%s`, context, historyStr.String(), question)

	reqBody := GenerateRequest{
		Model:  cfg.OllamaModel, // 使用配置中的模型名
		Prompt: fullPrompt,
		Stream: true,
	}

	jsonData, _ := json.Marshal(reqBody)
	// 使用配置中的 URL
	apiURL := fmt.Sprintf("%s/api/generate", cfg.OllamaURL)
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("连接 Ollama 失败: %v", err)
	}
	defer resp.Body.Close()

	// 流式读取逻辑保持不变...
	decoder := json.NewDecoder(resp.Body)
	var fullResponse strings.Builder
	for {
		var genResp GenerateResponse
		if err := decoder.Decode(&genResp); err == io.EOF {
			break
		} else if err != nil {
			return "", err
		}
		fullResponse.WriteString(genResp.Response)
	}

	re := regexp.MustCompile(`(?s)<think>.*?</think>`)
	cleanResponse := re.ReplaceAllString(fullResponse.String(), "")
	return strings.TrimSpace(cleanResponse), nil
}

// StreamOllama 实现流式调用
func StreamOllama(ctx context.Context, cfg *config.AppConfig, context string, history []Message, question string, onToken func(GenerateResponse)) error {
	// 使用极简结构，不再使用长句子约束
	fullPrompt := fmt.Sprintf(`<|think|>
你是 Sift 知识库助手。请严格基于以下【参考资料】直接回答用户的问题。
要求：
1. 优先回答与问题最直接相关的信息。
2. 如果资料中包含图片描述，请明确说明。
3. 保持回答简洁。

### 知识库资料:
%s

### 用户当前问题:
%s`, context, question)
	jsonData, _ := json.Marshal(GenerateRequest{
		Model:  cfg.OllamaModel,
		Prompt: fullPrompt,
		Stream: true,
		Options: Options{
			Temperature:   cfg.OllamaTemp,
			RepeatPenalty: cfg.OllamaRepeatPenalty,
			TopP:          0.9,
		},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.OllamaURL+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err // 如果用户中止，这里会立刻返回 context canceled 错误
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		// 每一行都要检查 context 是否已被取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			var genResp GenerateResponse
			if err := json.Unmarshal(scanner.Bytes(), &genResp); err != nil {
				continue
			}
			onToken(genResp)
			if genResp.Done {
				return nil
			}
		}
	}
	return scanner.Err()
}

// 专门用于图像语意化提取的函数
func DescribeImage(cfg *config.AppConfig, filePath string) (string, error) {
	// 1. 读取图片文件
	imgData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("读取图片文件失败: %v", err)
	}

	// 2. 将二进制数据转为 Base64 字符串
	base64Img := base64.StdEncoding.EncodeToString(imgData)

	// 3. 构造请求，提示词建议针对身份证等证件场景稍作加权
	prompt := "请识别并提取图片中的所有文字。如果是证件，请列出关键字段信息；如果是场景，请详细描述内容。"

	reqBody := GenerateRequest{
		Model:  cfg.OllamaModel,
		Prompt: prompt,
		Images: []string{base64Img}, // 传递编码后的数据
		Stream: false,
		Options: Options{
			Temperature:   0.1,
			RepeatPenalty: 1.1,
		},
	}

	jsonData, _ := json.Marshal(reqBody)
	apiURL := fmt.Sprintf("%s/api/generate", cfg.OllamaURL)
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var genResp GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return "", err
	}

	// 在终端打印提取结果，方便调试
	fmt.Printf("✅ VLM 成功解析内容 [%s]: %s\n", filepath.Base(filePath), genResp.Response)

	return genResp.Response, nil
}
