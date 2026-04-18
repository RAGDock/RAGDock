package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RAGDock/RAGDock/internal/config"
	"github.com/RAGDock/RAGDock/internal/db"
	"github.com/RAGDock/RAGDock/internal/llm"
	"github.com/RAGDock/RAGDock/internal/model"
	"github.com/RAGDock/RAGDock/internal/parser"

	"github.com/fsnotify/fsnotify"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App defines the main application structure and its state
type App struct {
	ctx        context.Context
	cfg        *config.AppConfig
	cancelFunc context.CancelFunc
	mgr        *db.Manager
	embedder   *model.Embedder
	watcher    *fsnotify.Watcher
}

// NewApp creates a new instance of the App
func NewApp() *App {
	return &App{}
}

// Float32ToByte converts a slice of float32 (vector) to a byte stream for SQLite blob storage
func Float32ToByte(slice []float32) []byte {
	buf := new(bytes.Buffer)
	for _, v := range slice {
		binary.Write(buf, binary.LittleEndian, v)
	}
	return buf.Bytes()
}

// startup is called by Wails when the application starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// 1. Load application configuration
	a.cfg = config.LoadConfig()

	var err error
	// 2. Initialize the Database Manager
	a.mgr, err = db.NewManager(a.cfg)
	if err != nil {
		fmt.Printf("Database initialization failed: %v\n", err)
	}

	// 3. Initialize the Embedding Model (ONNX)
	a.embedder, err = model.NewEmbedder(a.cfg)
	if err != nil {
		fmt.Printf("Embedding model initialization failed: %v\n", err)
	}
}

// startWatcher sets up a real-time file system watcher for the specified directory
func (a *App) startWatcher(path string) {
	if a.watcher != nil {
		a.watcher.Close()
	}
	var err error
	a.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("Failed to start file watcher: %v\n", err)
		return
	}

	go func() {
		for {
			select {
			case event, ok := <-a.watcher.Events:
				if !ok {
					return
				}

				fileName := strings.ToLower(event.Name)
				// Supported formats: Markdown and common image types
				isSupported := strings.HasSuffix(fileName, ".md") ||
					strings.HasSuffix(fileName, ".jpg") ||
					strings.HasSuffix(fileName, ".jpeg") ||
					strings.HasSuffix(fileName, ".png")

				if (event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Write == fsnotify.Write) && isSupported {
					a.indexSingleFile(event.Name)
				}
			case err, ok := <-a.watcher.Errors:
				if !ok {
					return
				}
				fmt.Printf("Watcher error: %v\n", err)
			}
		}
	}()
	a.watcher.Add(path)
}

// indexSingleFile processes a single file (Markdown or Image) and updates the database
func (a *App) indexSingleFile(path string) {
	// Ensure the file is fully written and accessible
	if err := a.waitUntilReady(path, 5*time.Second); err != nil {
		fmt.Printf("File is busy: %v\n", err)
		return
	}

	var chunks []parser.Chunk
	var err error
	ext := strings.ToLower(filepath.Ext(path))

	// 1. Parse content based on file type
	if ext == ".md" {
		chunks, err = parser.ParseMarkdown(path)
	} else {
		// Handle images using the Vision Language Model (VLM)
		// ProcessImage is assumed to be defined in internal/parser
		var chunk parser.Chunk
		chunk, err = parser.ProcessImage(a.cfg, path)
		if err == nil {
			chunks = []parser.Chunk{chunk}
		}
	}

	if err != nil {
		fmt.Printf("Parsing failed [%s]: %v\n", path, err)
		runtime.EventsEmit(a.ctx, "file_synced", "Parsing failed: "+filepath.Base(path))
		return
	}

	// 2. Clean up existing indices for this file to prevent duplicates
	a.mgr.Conn.Exec("DELETE FROM vec_idx WHERE rowid IN (SELECT id FROM documents WHERE file_path = ?)", path)
	a.mgr.Conn.Exec("DELETE FROM documents WHERE file_path = ?", path)

	// 3. Vectorize chunks and insert into database
	for _, c := range chunks {
		if c.Content == "EMPTY_IGNORE" {
			continue
		}

		// Generate 384-dimensional vector for both text and image descriptions
		vec, err := a.embedder.Generate(c.Content)
		if err != nil {
			continue
		}

		res, err := a.mgr.Conn.Exec("INSERT INTO documents(heading, content, file_path) VALUES(?, ?, ?)",
			c.Heading, c.Content, path)
		if err != nil {
			fmt.Printf("Document insertion failed: %v\n", err)
			continue
		}
		docID, _ := res.LastInsertId()

		// Store vector embedding into the virtual index table
		_, err = a.mgr.Conn.Exec("INSERT INTO vec_idx(rowid, embedding) VALUES(?, ?)",
			docID, Float32ToByte(vec))
		if err != nil {
			fmt.Printf("Vector index insertion failed: %v\n", err)
		}

		// Notify the frontend of the successful synchronization
		msg := "Document synced: "
		if strings.HasSuffix(strings.ToLower(path), ".md") {
			msg = "Document synced: "
		} else {
			msg = "Image semantic indexing complete: "
		}

		runtime.EventsEmit(a.ctx, "file_synced", msg+filepath.Base(path))
	}
}

// SearchAndAsk handles RAG queries with support for conversation history and streaming output
func (a *App) SearchAndAsk(query string, history []llm.Message) error {
	if a.embedder == nil {
		return fmt.Errorf("embedding model not loaded, please check resource files")
	}
	if a.mgr == nil || a.mgr.Conn == nil {
		return fmt.Errorf("database connection not established")
	}

	var searchCtx context.Context
	searchCtx, a.cancelFunc = context.WithCancel(a.ctx)

	// Ensure the cancel function is cleared when the method exits
	defer func() {
		a.cancelFunc = nil
	}()

	// 1. Vectorize the user's query
	queryVec, err := a.embedder.Generate(query)
	if err != nil {
		return fmt.Errorf("vectorization failed: %v", err)
	}

	// 2. Perform a vector search on the local database (Top-K results)
	rows, err := a.mgr.Conn.Query(fmt.Sprintf(`
        SELECT d.heading, d.content 
        FROM vec_idx v
        JOIN documents d ON v.rowid = d.id
        WHERE embedding MATCH ? AND k = %d
        ORDER BY distance`, a.cfg.RagK), Float32ToByte(queryVec))

	if err != nil {
		return fmt.Errorf("database search failed: %v", err)
	}
	defer rows.Close()

	// 3. Build the context block for the LLM
	var contextBuilder strings.Builder
	for rows.Next() {
		var heading, content string
		if err := rows.Scan(&heading, &content); err == nil {
			contextBuilder.WriteString(fmt.Sprintf("\n[Source: %s]\n%s\n", heading, content))
		}
	}

	contextText := contextBuilder.String()
	if contextText == "" {
		contextText = "No relevant local documents found."
	}

	// 4. Start streaming the LLM request via Ollama
	err = llm.StreamOllama(searchCtx, a.cfg, contextText, history, query, func(token llm.GenerateResponse) {
		// Emit tokens to the frontend in real-time
		runtime.EventsEmit(a.ctx, "llm_token", token)
	})

	if err != nil {
		// If the error was due to manual cancellation, handle it gracefully
		if errors.Is(err, context.Canceled) {
			fmt.Println("Search request cancelled by user")
			return nil
		}
		return err
	}

	return nil
}

// StopSearch allows the frontend to manually abort the LLM generation
func (a *App) StopSearch() {
	if a.cancelFunc != nil {
		a.cancelFunc() // Trigger context cancellation
		fmt.Println("Model generation manually stopped by user")
	}
}

// SelectAndIndexFolder opens a directory dialog and starts the indexing process
func (a *App) SelectAndIndexFolder() (string, error) {
	folderPath, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Knowledge Base Folder",
	})
	if err != nil || folderPath == "" {
		return "", err
	}

	// Start indexing and file watching in background goroutines
	go a.indexDirectory(folderPath)
	go a.startWatcher(folderPath)
	return folderPath, nil
}

// indexDirectory recursively scans a directory for supported files
func (a *App) indexDirectory(root string) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			fileName := strings.ToLower(info.Name())
			if strings.HasSuffix(fileName, ".md") ||
				strings.HasSuffix(fileName, ".jpg") ||
				strings.HasSuffix(fileName, ".jpeg") ||
				strings.HasSuffix(fileName, ".png") {
				a.indexSingleFile(path)
			}
		}
		return nil
	})
	runtime.EventsEmit(a.ctx, "index_complete", "Directory indexing complete")
}

// waitUntilReady waits for a file to be released by other processes
func (a *App) waitUntilReady(path string, timeout time.Duration) error {
	start := time.Now()
	for {
		f, err := os.OpenFile(path, os.O_RDONLY, 0)
		if err == nil {
			f.Close()
			return nil
		}
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for file access: %s", path)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
