package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/RAGDock/RAGDock/internal/config"
)

// Options defines model parameters for the LLM request
type Options struct {
	Temperature   float32 `json:"temperature"`
	RepeatPenalty float32 `json:"repeat_penalty"`
	TopP          float32 `json:"top_p"`
}

// GenerateRequest represents the body of a request to Ollama's /api/generate
type GenerateRequest struct {
	Model   string   `json:"model"`
	Images  []string `json:"images,omitempty"` // Base64 encoded image strings
	Prompt  string   `json:"prompt"`
	Stream  bool     `json:"stream"`
	Options Options  `json:"options"`
}

// GenerateResponse represents a chunk of the response from Ollama
type GenerateResponse struct {
	Model    string `json:"model"`
	Response string `json:"response"`
	Thinking string `json:"thinking"` // Captured reasoning chain from modern models
	Done     bool   `json:"done"`
}

// Message defines a single turn in a chat conversation
type Message struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// StreamOllama handles streaming responses from the Ollama LLM
func StreamOllama(ctx context.Context, cfg *config.AppConfig, context string, history []Message, question string, onToken func(GenerateResponse)) error {
	// Construct the prompt using a clean structure to minimize token waste
	fullPrompt := fmt.Sprintf(`<|think|>
You are the RAGDock Knowledge Assistant. Use ONLY the provided [Reference Documents] to answer the user's question accurately.
Guidelines:
1. Prioritize information most relevant to the query.
2. If image descriptions are present in the documents, mention them clearly.
3. Keep the response professional and concise.

### Reference Documents:
%s

### User Question:
%s`, context, question)

	jsonData, _ := json.Marshal(GenerateRequest{
		Model:  cfg.RagModel,
		Prompt: fullPrompt,
		Stream: true,
		Options: Options{
			Temperature:   cfg.RagTemp,
			RepeatPenalty: cfg.RagRepeatPenalty,
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

// DescribeImage uses a Vision Language Model (VLM) to extract text and semantic meaning from images
func DescribeImage(cfg *config.AppConfig, filePath string) (string, error) {
	// 1. Read the image file
	imgData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image file: %v", err)
	}

	// 2. Encode binary data to Base64
	base64Img := base64.StdEncoding.EncodeToString(imgData)

	reqBody := GenerateRequest{
		Model:  cfg.VlmModel,
		Prompt: "Extract and identify all text and key details from this image. For documents/receipts, list fields; for scenes, describe the content.",
		Images: []string{base64Img},
		Stream: false,
		Options: Options{
			Temperature:   cfg.VlmTemp,
			RepeatPenalty: cfg.VlmRepeatPenalty,
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

	// Log the VLM result for debugging purposes
	fmt.Printf("VLM successfully parsed content [%s]: %s\n", filepath.Base(filePath), genResp.Response)

	return genResp.Response, nil
}
