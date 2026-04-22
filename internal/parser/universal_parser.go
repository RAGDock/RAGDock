package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"github.com/gen2brain/go-fitz"
)

// ParseDocumentUniversal handles PDF, DOCX, EPUB, MOBI, FB2, and XPS using MuPDF.
func ParseDocumentUniversal(filePath string) ([]Chunk, error) {
	doc, err := fitz.New(filePath)
	if err != nil {
		return nil, fmt.Errorf("MuPDF failed to open document: %v", err)
	}
	defer doc.Close()

	var chunks []Chunk
	const maxChunkChars = 800
	ext := strings.ToLower(filepath.Ext(filePath))

	for i := 0; i < doc.NumPage(); i++ {
		text, err := doc.Text(i)
		if err != nil {
			continue
		}

		var cleaned strings.Builder
		for _, r := range text {
			if unicode.IsPrint(r) || r == '\n' || r == '\t' {
				cleaned.WriteRune(r)
			}
		}
		
		fullPageText := cleaned.String()
		fullPageText = strings.ReplaceAll(fullPageText, "\r\n", "\n")
		
		lines := strings.Split(fullPageText, "\n")
		var currentChunk strings.Builder
		
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			
			if currentChunk.Len()+len(line) > maxChunkChars && currentChunk.Len() > 0 {
				chunks = append(chunks, Chunk{
					Content: strings.TrimSpace(currentChunk.String()),
					Path:    filePath,
					Heading: fmt.Sprintf("%s Content (Section %d)", strings.ToUpper(ext[1:]), i+1),
				})
				currentChunk.Reset()
			}
			
			if currentChunk.Len() > 0 {
				currentChunk.WriteString("\n")
			}
			currentChunk.WriteString(line)
		}
		
		if currentChunk.Len() > 0 {
			chunks = append(chunks, Chunk{
				Content: strings.TrimSpace(currentChunk.String()),
				Path:    filePath,
				Heading: fmt.Sprintf("%s Content (Section %d)", strings.ToUpper(ext[1:]), i+1),
			})
			currentChunk.Reset()
		}
	}

	if len(chunks) == 0 {
		return nil, fmt.Errorf("no content extracted from %s", ext)
	}

	fmt.Printf("Universal Parser: %s | Chunks: %d\n", filePath, len(chunks))
	
	// Explicitly suggest GC to help release CGO/MuPDF resources on some platforms
	runtime.GC()
	
	return chunks, nil
}

// ParsePlainText handles standard .txt files.
func ParsePlainText(filePath string) ([]Chunk, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	text := string(data)
	// Basic cleaning
	text = strings.ReplaceAll(text, "\r\n", "\n")
	
	lines := strings.Split(text, "\n")
	var chunks []Chunk
	var currentChunk strings.Builder
	const maxChunkChars = 800

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if currentChunk.Len()+len(line) > maxChunkChars && currentChunk.Len() > 0 {
			chunks = append(chunks, Chunk{
				Content: currentChunk.String(),
				Path:    filePath,
				Heading: "Text Content",
			})
			currentChunk.Reset()
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n")
		}
		currentChunk.WriteString(line)
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, Chunk{
			Content: currentChunk.String(),
			Path:    filePath,
			Heading: "Text Content",
		})
	}

	fmt.Printf("Text Parser: %s | Chunks: %d\n", filePath, len(chunks))
	return chunks, nil
}
