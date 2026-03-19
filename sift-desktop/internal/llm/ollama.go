package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type GenerateResponse struct {
	Response string `json:"response"`
}

// AskOllama 结合本地搜索到的 context 回答用户的问题
func AskOllama(context string, question string) (string, error) {
	// 构建 RAG 专用 Prompt
	fullPrompt := fmt.Sprintf(`你是 Sift 知识库助手。请严格基于以下【参考资料】回答用户问题。
如果资料中没有相关信息，请直接回答“资料中未提及”，不要尝试编造。

【参考资料】：
%s

【用户问题】：
%s`, context, question)

	reqBody := GenerateRequest{
		Model:  "qwen3.5-my", // 必须对应你 ollama create 时的名字
		Prompt: fullPrompt,
		Stream: true,
	}

	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("无法连接到 Ollama (请确保服务已启动): %v", err)
	}
	defer resp.Body.Close()

	// 如果 Stream 是 true，应该这样写：
	decoder := json.NewDecoder(resp.Body)
	var fullResponse strings.Builder

	for {
		var genResp GenerateResponse
		if err := decoder.Decode(&genResp); err == io.EOF {
			break // 读取完毕
		} else if err != nil {
			return "", err
		}
		// 实时处理每一个 token
		fullResponse.WriteString(genResp.Response)
		// 这里可以打印 token 或者通过 channel 发给 UI
	}

	// option 1：使用正则表达式剔除 <think>...</think> 及其包含的内容
	re := regexp.MustCompile(`(?s)<think>.*?</think>`)
	cleanResponse := re.ReplaceAllString(fullResponse.String(), "")
	// 去掉可能残留在开头或结尾的空白换行
	return strings.TrimSpace(cleanResponse), nil

	// option 2：完整输出回答，包含thinking内容
	//return strings.TrimSpace(fullResponse.String()), nil
}
