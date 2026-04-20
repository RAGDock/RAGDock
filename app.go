package main

import (
	"bytes"
	"context"
	"encoding/binary"
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
	ctx          context.Context
	cfg          *config.AppConfig
	cancelFunc   context.CancelFunc
	mgr          *db.Manager
	embedder     *model.Embedder
	watcher      *fsnotify.Watcher
	vlmSemaphore chan struct{} // Global semaphore for rate-limiting heavy VLM/Parsing tasks
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

	// 2. Initialize the global semaphore for concurrency control
	a.vlmSemaphore = make(chan struct{}, a.cfg.VlmConcurrency)

	var err error
	// 3. Initialize the Database Manager
	a.mgr, err = db.NewManager(a.cfg)
	if err != nil {
		fmt.Printf("Database initialization failed: %v\n", err)
	}

	// 4. Initialize the Embedding Model (ONNX)
	a.embedder, err = model.NewEmbedder(a.cfg)
	if err != nil {
		fmt.Printf("Embedding model initialization failed: %v\n", err)
	}
}

// GetLanguage returns the current application language ("zh" or "en")
func (a *App) GetLanguage() string {
	return a.cfg.Language
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
					info, err := os.Stat(event.Name)
					sizeStr := "unknown"
					if err == nil {
						sizeStr = fmt.Sprintf("%.1fkb", float64(info.Size())/1024.0)
					}
					op := "added"
					if event.Op&fsnotify.Write == fsnotify.Write {
						op = "modified"
					}
					fmt.Printf("%s | [WATCH] | file %s [%s] size: %s\n", time.Now().Format("15:04:05:000"), op, filepath.Base(event.Name), sizeStr)

					// Run indexing in background, throttled by the semaphore
					go a.indexSingleFile(event.Name)
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
	ext := strings.ToLower(filepath.Ext(path))
	isImage := ext == ".jpg" || ext == ".jpeg" || ext == ".png"

	if isImage {
		fmt.Printf("%s [WAIT] Image in queue: [%s]\n", time.Now().Format("3:04:05 PM"), filepath.Base(path))
	}

	// Acquire semaphore (blocks if full)
	a.vlmSemaphore <- struct{}{}
	defer func() { <-a.vlmSemaphore }() // Release when done

	if isImage {
		fmt.Printf("%s [START] Processing image: [%s]\n", time.Now().Format("3:04:05 PM"), filepath.Base(path))
	}

	startTime := time.Now()
	metrics := model.PerfMetrics{Action: "index"}

	// Ensure the file is fully written and accessible
	if err := a.waitUntilReady(path, 5*time.Second); err != nil {
		fmt.Printf("File is busy: %v\n", err)
		return
	}

	var chunks []parser.Chunk
	var err error

	// 1. Parse content based on file type
	parseStart := time.Now()
	if ext == ".md" {
		chunks, err = parser.ParseMarkdown(path)
	} else {
		// Handle images using the Vision Language Model (VLM)
		var chunk parser.Chunk
		chunk, err = parser.ProcessImage(a.cfg, path)
		if err == nil {
			chunks = []parser.Chunk{chunk}
		}
	}
	metrics.ParseMs = time.Since(parseStart).Milliseconds()

	if err != nil {
		fmt.Printf("Parsing failed [%s]: %v\n", path, err)
		runtime.EventsEmit(a.ctx, "file_synced", "Parsing failed: "+filepath.Base(path))
		return
	}

	// 2. Clean up existing indices
	a.mgr.Conn.Exec("DELETE FROM vec_idx WHERE rowid IN (SELECT id FROM documents WHERE file_path = ?)", path)
	a.mgr.Conn.Exec("DELETE FROM documents WHERE file_path = ?", path)

	// 3. Vectorize chunks and insert
	var totalEmbedMs int64
	for _, c := range chunks {
		if c.Content == "EMPTY_IGNORE" {
			continue
		}

		embedStart := time.Now()
		vec, err := a.embedder.Generate(c.Content)
		totalEmbedMs += time.Since(embedStart).Milliseconds()

		if err != nil {
			continue
		}

		res, err := a.mgr.Conn.Exec("INSERT INTO documents(heading, content, file_path) VALUES(?, ?, ?)",
			c.Heading, c.Content, path)
		if err != nil {
			continue
		}
		docID, _ := res.LastInsertId()
		a.mgr.Conn.Exec("INSERT INTO vec_idx(rowid, embedding) VALUES(?, ?)", docID, Float32ToByte(vec))
	}

	metrics.EmbedMs = totalEmbedMs
	metrics.TotalMs = time.Since(startTime).Milliseconds()
	runtime.EventsEmit(a.ctx, "perf_metrics", metrics)

	if isImage {
		fmt.Printf("%s | [FINISH] | Image processed: [%s] total: %dms\n", time.Now().Format("15:04:05:000"), filepath.Base(path), metrics.TotalMs)
	}

	// Notify the frontend of success
	runtime.EventsEmit(a.ctx, "file_synced", "Synced: "+filepath.Base(path))
}

// SearchAndAsk handles RAG queries with support for conversation history and streaming output
func (a *App) SearchAndAsk(query string, history []llm.Message) error {
	overallStart := time.Now()
	metrics := model.PerfMetrics{Action: "search"}

	if a.embedder == nil {
		return fmt.Errorf("embedding model not loaded")
	}

	// 1. Vectorize query
	embedStart := time.Now()
	queryVec, err := a.embedder.Generate(query)
	metrics.EmbedMs = time.Since(embedStart).Milliseconds()
	if err != nil {
		return err
	}

	// 2. Local Search
	searchStart := time.Now()
	rows, err := a.mgr.Conn.Query(fmt.Sprintf(`
        SELECT d.heading, d.content, d.file_path 
        FROM vec_idx v
        JOIN documents d ON v.rowid = d.id
        WHERE embedding MATCH ? AND k = %d
        ORDER BY distance`, a.cfg.RagK), Float32ToByte(queryVec))
	metrics.SearchMs = time.Since(searchStart).Milliseconds()

	if err != nil {
		return err
	}
	defer rows.Close()

	var contextBuilder strings.Builder
	var snippets []llm.DocSnippet
	rowCount := 0
	for rows.Next() {
		var h, c, p string
		if err := rows.Scan(&h, &c, &p); err == nil {
			contextBuilder.WriteString(fmt.Sprintf("\n[Source: %s]\n%s\n", h, c))
			
			// Extract file metadata
			info, err := os.Stat(p)
			sizeStr := "unknown"
			modTime := "unknown"
			isDeleted := false
			
			if err != nil {
				if os.IsNotExist(err) {
					isDeleted = true
				}
			} else {
				sizeStr = fmt.Sprintf("%.1fkb", float64(info.Size())/1024.0)
				modTime = info.ModTime().Format("2006-01-02 15:04")
			}
			
			snippets = append(snippets, llm.DocSnippet{
				FileName: filepath.Base(p),
				Dir:      filepath.Dir(p),
				Size:     sizeStr,
				ModTime:  modTime,
				Content:  c,
				Deleted:  isDeleted,
			})
			rowCount++
		}
	}

	finalContext := contextBuilder.String()
	fmt.Printf("%s | [RAG]  | Query: [%s] | Found: %d snippets\n", time.Now().Format("15:04:05:000"), query, rowCount)
	if rowCount > 0 {
		// Log a preview of the context for verification
		preview := finalContext
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		fmt.Printf("%s | [RAG]  | Context Preview: %s\n", time.Now().Format("15:04:05:000"), preview)
	} else {
		fmt.Printf("%s | [WARN] | NO LOCAL CONTEXT FOUND for this query!\n", time.Now().Format("15:04:05:000"))
	}

	// 3. LLM Streaming
	var searchCtx context.Context
	searchCtx, a.cancelFunc = context.WithCancel(a.ctx)
	defer func() { a.cancelFunc = nil }()

	llmStart := time.Now()
	var ttftOnce bool
	err = llm.StreamOllama(searchCtx, a.cfg, finalContext, history, query, func(token llm.GenerateResponse) {
		if !ttftOnce {
			token.SearchResults = snippets // Include snippets in the very first event
			if token.Response != "" || token.Thinking != "" {
				metrics.TTFTMs = time.Since(llmStart).Milliseconds()
				ttftOnce = true
			}
		}

		if token.Done {
			metrics.InferenceMs = token.TotalDuration / 1e6 // Nano to Milli
			metrics.PromptTokens = token.PromptEvalCount
			metrics.CompletionTokens = token.EvalCount
		}
		runtime.EventsEmit(a.ctx, "llm_token", token)
	})

	metrics.TotalMs = time.Since(overallStart).Milliseconds()
	runtime.EventsEmit(a.ctx, "perf_metrics", metrics)

	return err
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
	// Pre-scan all valid file paths to manage them in our task pool
	var filesToIndex []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			fileName := strings.ToLower(info.Name())
			if strings.HasSuffix(fileName, ".md") ||
				strings.HasSuffix(fileName, ".jpg") ||
				strings.HasSuffix(fileName, ".jpeg") ||
				strings.HasSuffix(fileName, ".png") {
				filesToIndex = append(filesToIndex, path)
			}
		}
		return nil
	})

	// Launch each task in its own goroutine
	// The a.vlmSemaphore inside indexSingleFile will handle the throttling automatically
	for _, path := range filesToIndex {
		go a.indexSingleFile(path)
	}

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
