package parser

import (
	"os"
	"strings"
)

// Chunk represents a split portion of a document with its metadata
type Chunk struct {
	Content string // The actual text content of the chunk
	Path    string // The source file path
	Heading string // The hierarchical heading path (e.g., "Intro > Architecture")
}

// ParseMarkdown splits a Markdown file into logical chunks based on headings or paragraphs
func ParseMarkdown(filePath string) ([]Chunk, error) {
	// 1. Read the document file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	content := string(data)
	// Normalize line endings to prevent cross-platform matching issues
	content = strings.ReplaceAll(content, "\r\n", "\n")

	var chunks []Chunk

	// 2. Detect if the document contains standard Markdown headers (#)
	hasHeaders := strings.HasPrefix(content, "#") || strings.Contains(content, "\n#")

	if hasHeaders {
		// --- Strategy A: Split by Markdown Headings ---
		lines := strings.Split(content, "\n")
		var currentHeading []string
		var currentBody strings.Builder

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				// Process the previous chunk if it has enough content
				trimmedBody := strings.TrimSpace(currentBody.String())
				if len(trimmedBody) > 2 {
					chunks = append(chunks, buildChunk(currentBody.String(), filePath, currentHeading))
					currentBody.Reset()
				}

				// Parse heading level (#, ##, ###, etc.)
				level := 0
				for i := 0; i < len(trimmed) && trimmed[i] == '#'; i++ {
					level++
				}
				title := strings.TrimSpace(trimmed[level:])

				// Maintain the hierarchical heading stack
				if level <= len(currentHeading) {
					currentHeading = currentHeading[:level-1]
				}
				currentHeading = append(currentHeading, title)
			} else {
				currentBody.WriteString(line + "\n")
			}
		}
		// Capture the final chunk at the end of the file
		if currentBody.Len() > 0 {
			chunks = append(chunks, buildChunk(currentBody.String(), filePath, currentHeading))
		}
	} else {
		// --- Strategy B: Fallback Paragraph Splitting ---
		// Used for documents without headers to improve retrieval granularity
		paragraphs := strings.Split(content, "\n\n")
		for _, p := range paragraphs {
			p = strings.TrimSpace(p)
			if len(p) < 20 { // Skip trivial or empty paragraphs
				continue
			}
			chunks = append(chunks, Chunk{
				Content: p,
				Path:    filePath,
				Heading: "Paragraph Content",
			})
		}
	}

	return chunks, nil
}

// buildChunk is a helper to construct a Chunk with a formatted hierarchical heading path
func buildChunk(body string, path string, headings []string) Chunk {
	trimmedBody := strings.TrimSpace(body)

	// Mark empty content for exclusion by the indexer
	if trimmedBody == "" {
		return Chunk{Content: "EMPTY_IGNORE"}
	}

	fullHeading := strings.Join(headings, " > ")
	if fullHeading == "" {
		fullHeading = "Main Content"
	}
	return Chunk{
		Content: trimmedBody,
		Path:    path,
		Heading: fullHeading,
	}
}
