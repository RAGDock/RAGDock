package parser

import (
	"fmt"
	"path/filepath"

	"github.com/RAGDock/RAGDock/internal/config"
	"github.com/RAGDock/RAGDock/internal/llm"
)

// ProcessImage handles image analysis and semantic text extraction via a Vision Language Model (VLM)
func ProcessImage(cfg *config.AppConfig, filePath string) (Chunk, error) {
	// Call the VLM model (Ollama) to extract text or content description
	description, err := llm.DescribeImage(cfg, filePath)
	if err != nil {
		return Chunk{}, fmt.Errorf("vision model processing failed: %v", err)
	}

	if description == "" {
		return Chunk{}, fmt.Errorf("model returned no content for the image")
	}

	// Wrap the VLM output into a standard searchable Chunk
	return Chunk{
		Content: description,
		Path:    filePath,
		Heading: fmt.Sprintf("Image analysis: %s", filepath.Base(filePath)),
	}, nil
}
