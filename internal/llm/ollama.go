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
	"time"

	"github.com/RAGDock/RAGDock/internal/config"
)

// Options defines model parameters for the LLM request
type Options struct {
	Temperature     float32 `json:"temperature"`
	RepeatPenalty   float32 `json:"repeat_penalty"`
	PresencePenalty float32 `json:"presence_penalty"`
	NumCtx          int     `json:"num_ctx"`
	TopK            int     `json:"top_k"`
	TopP            float32 `json:"top_p"`
}

// GenerateRequest represents the body of a request to Ollama's /api/generate
type GenerateRequest struct {
	Model   string   `json:"model"`
	Images  []string `json:"images,omitempty"` // Base64 encoded image strings
	System  string   `json:"system"`
	Prompt  string   `json:"prompt"`
	Stream  bool     `json:"stream"`
	Options Options  `json:"options"`
}

// GenerateResponse represents a chunk of the response from Ollama
type GenerateResponse struct {
	Model              string       `json:"model"`
	Response           string       `json:"response"`
	Thinking           string       `json:"thinking,omitempty"` // Captured reasoning chain
	Done               bool         `json:"done"`
	SearchResults      []DocSnippet `json:"search_results,omitempty"` // Metadata for references
	TotalDuration      int64        `json:"total_duration,omitempty"`
	LoadDuration       int64        `json:"load_duration,omitempty"`
	PromptEvalCount    int          `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64        `json:"prompt_eval_duration,omitempty"`
	EvalCount          int          `json:"eval_count,omitempty"`
	EvalDuration       int64        `json:"eval_duration,omitempty"`
}

// DocSnippet stores metadata for a retrieved document chunk
type DocSnippet struct {
	FileName string `json:"file_name"`
	Dir      string `json:"dir"`
	Size     string `json:"size"`
	ModTime  string `json:"mod_time"`
	Content  string `json:"content"`
	Deleted  bool   `json:"deleted"`
}

// Message defines a single turn in a chat conversation
type Message struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// StreamOllama handles streaming responses from Ollama
func StreamOllama(ctx context.Context, cfg *config.AppConfig, context string, history []Message, question string, onToken func(GenerateResponse)) error {
	// 1. Prepare Language-specific System Prompt
	systemPrompt := `You are the RAGDock Knowledge Assistant. 
Use the provided [Reference Documents] to answer. 
If the documents do not contain enough information to answer, simply state that there is no relevant information.
If image descriptions are present, mention them. 
Keep the response professional and concise.`
	userLabel := "User Question"
	docLabel := "Reference Documents"

	if cfg.Language == "zh" {
		systemPrompt = `你是 RAGDock 知识助手。
请仅使用提供的 [参考文档] 来回答问题。
如果参考文档中没有足够的信息来回答，请直接回答“无相关资料”。
如果文档中包含图片描述，请一并提及。
回答应保持专业且简洁。`
		userLabel = "用户问题"
		docLabel = "参考文档"
	}

	// 2. Construct Prompt
	userPrompt := fmt.Sprintf("### %s:\n%s\n\n### %s:\n%s", docLabel, context, userLabel, question)

	jsonData, _ := json.Marshal(GenerateRequest{
		Model:  cfg.RagModel,
		System: systemPrompt,
		Prompt: userPrompt,
		Stream: true,
		Options: Options{
			Temperature:     cfg.RagTemp,
			RepeatPenalty:   cfg.RagRepeatPenalty,
			PresencePenalty: cfg.RagPresencePenalty,
			TopK:            cfg.RagTopK,
			TopP:            cfg.RagTopP,
			NumCtx:          4096,
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
		return err // Errors like context cancellation are returned immediately
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
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

func DescribeImage(cfg *config.AppConfig, filePath string) (string, error) {
	// 1. Read the image file
	imgData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image file: %v", err)
	}

	// 2. Encode binary data to Base64
	base64Img := base64.StdEncoding.EncodeToString(imgData)

	// --- Ollama VLM Logic ---
	vlmPrompt := "Identify all text and describe the content of this image in detail."
	if cfg.Language == "zh" {
		vlmPrompt = "请识别图中所有的文字，并详细描述图片的内容。"
	}

	reqBody := GenerateRequest{
		Model:  cfg.VlmModel,
		Prompt: vlmPrompt,
		Images: []string{base64Img},
		Stream: false,
		Options: Options{
			Temperature:   0.1,
			RepeatPenalty: 1.1,
			NumCtx:        8192, // Increase context for large images/long text
		},
	}

	jsonData, _ := json.Marshal(reqBody)
	apiURL := fmt.Sprintf("%s/api/generate", cfg.OllamaURL)
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned error status %d: %s", resp.StatusCode, string(body))
	}

	// Read full body for robust parsing and debugging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	var genResp GenerateResponse
	if err := json.Unmarshal(bodyBytes, &genResp); err != nil {
		return "", fmt.Errorf("JSON Unmarshal error: %v", err)
	}

	// Capture response or thinking
	fullContent := genResp.Response
	if fullContent == "" {
		fullContent = genResp.Thinking
	}

	if fullContent != "" {
		fmt.Printf("%s | [VLM] | Processed [%s]: (Thinking: %d, Response: %d, Total: %d chars)\n",
			time.Now().Format("15:04:05:000"), filepath.Base(filePath), len(genResp.Thinking), len(genResp.Response), len(fullContent))
	}

	return fullContent, nil
}
