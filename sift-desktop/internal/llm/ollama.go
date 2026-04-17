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

// AskOllama 增加 cfg 参数
func AskOllama(cfg *config.AppConfig, context string, question string) (string, error) {
	fullPrompt := fmt.Sprintf(`你是 Sift 知识库助手。请严格基于以下【参考资料】回答用户问题。
如果资料中没有相关信息，请直接回答“资料中未提及”，不要尝试编造。

【参考资料】：%s
【用户问题】：%s`, context, question)

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
