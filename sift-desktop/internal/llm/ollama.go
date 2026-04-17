package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"sift.local/internal/config" // 引入配置包
)

type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type GenerateResponse struct {
	Response string `json:"response"`
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
