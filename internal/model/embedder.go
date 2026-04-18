package model

import (
	"fmt"
	"path/filepath"

	"github.com/RAGDock/RAGDock/internal/config"
	"github.com/sugarme/tokenizer"
	"github.com/yalue/onnxruntime_go"
)

// Embedder handles local text vectorization using ONNX models
type Embedder struct {
	session   *onnxruntime_go.DynamicAdvancedSession
	tokenizer *tokenizer.Tokenizer
}

// NewEmbedder initializes the ONNX runtime and loads the specified model and tokenizer
func NewEmbedder(cfg *config.AppConfig) (*Embedder, error) {
	// Initialize ONNX runtime with the platform-specific library
	libPath := cfg.GetFullLibPath("libonnxruntime")
	onnxruntime_go.SetSharedLibraryPath(libPath)
	err := onnxruntime_go.Initialize()
	if err != nil {
		return nil, fmt.Errorf("ONNX initialization failed: %v", err)
	}

	// Load the tokenizer from the local JSON file
	tk, err := tokenizer.FromFile(cfg.GetTokenizerPath())
	if err != nil {
		return nil, fmt.Errorf("failed to load tokenizer: %v", err)
	}

	// Create an advanced ONNX session for the embedding model
	modelPath := cfg.GetModelPath()
	session, err := onnxruntime_go.NewDynamicAdvancedSession(modelPath,
		[]string{"input_ids", "attention_mask", "token_type_ids"},
		[]string{"last_hidden_state"},
		nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create ONNX session: %v", err)
	}

	return &Embedder{
		session:   session,
		tokenizer: tk,
	}, nil
}

// Generate converts input text into a high-dimensional vector (embedding)
func (e *Embedder) Generate(text string) ([]float32, error) {
	// 1. Tokenize the input text
	en, err := e.tokenizer.EncodeSingle(text)
	if err != nil {
		return nil, fmt.Errorf("tokenization failed: %v", err)
	}

	ids := en.GetIds()
	mask := en.GetAttentionMask()
	types := en.GetTypeIds()
	length := int64(len(ids))

	// 2. Prepare ONNX input tensors
	inputIDs := onnxruntime_go.NewTensor([]int64{1, length}, ids)
	attentionMask := onnxruntime_go.NewTensor([]int64{1, length}, mask)
	tokenTypeIDs := onnxruntime_go.NewTensor([]int64{1, length}, types)

	defer inputIDs.Destroy()
	defer attentionMask.Destroy()
	defer tokenTypeIDs.Destroy()

	// 3. Execute model inference
	output, err := e.session.Run([]onnxruntime_go.ArbitraryTensor{inputIDs, attentionMask, tokenTypeIDs})
	if err != nil {
		return nil, fmt.Errorf("inference failed: %v", err)
	}
	defer output[0].Destroy()

	// 4. Post-process: Extract the [CLS] token embedding (first token)
	// This represents the semantic summary of the entire sentence/chunk
	rawOutput := output[0].GetData().([]float32)
	dim := 384 // Feature vector dimension for the BGE-Small model
	clsEmbedding := make([]float32, dim)
	copy(clsEmbedding, rawOutput[0:dim])

	return clsEmbedding, nil
}
