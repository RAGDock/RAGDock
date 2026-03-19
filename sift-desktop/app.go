package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sift-desktop/internal/db"
	"sift-desktop/internal/llm"
	"sift-desktop/internal/model"
	"sift-desktop/internal/parser"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx       context.Context
	mgr       *db.Manager
	embedder  *model.Embedder
	watcher   *fsnotify.Watcher
	watchPath string
}

func NewApp() *App {
	return &App{}
}

// Float32ToByte 将向量切片转换为 SQLite 识别的二进制流
func Float32ToByte(slice []float32) []byte {
	buf := new(bytes.Buffer)
	for _, v := range slice {
		binary.Write(buf, binary.LittleEndian, v)
	}
	return buf.Bytes()
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	var err error
	// 建议在 db.NewManager 内部使用绝对路径加载 sqlite-vec.dll
	a.mgr, err = db.NewManager("./sift_pro.db")
	if err != nil {
		fmt.Printf("❌ 数据库加载失败: %v\n", err)
	}

	// 初始化 ONNX 模型
	a.embedder, err = model.NewEmbedder("resources/models/model.onnx")
	if err != nil {
		fmt.Printf("❌ 模型加载失败: %v\n", err)
	}
}

// indexSingleFile 单个文件入库逻辑, 增加了轮询逻辑
// app.go

func (a *App) indexSingleFile(path string) {
	if err := a.waitUntilReady(path, 5*time.Second); err != nil {
		fmt.Printf("⚠️ 文件占用中: %v\n", err)
		return
	}

	chunks, err := parser.ParseMarkdown(path)
	if err != nil {
		fmt.Printf("❌ 解析失败: %v\n", err)
		return
	}

	// 🛠️ 关键修正 1：DELETE 必须放在循环外面！！
	// 在插入新内容前，一次性删掉该文件的所有旧索引
	a.mgr.Conn.Exec("DELETE FROM vec_idx WHERE rowid IN (SELECT id FROM documents WHERE file_path = ?)", path)
	a.mgr.Conn.Exec("DELETE FROM documents WHERE file_path = ?", path)

	for _, c := range chunks {
		// 🛠️ 关键修正 2：在这里增加拦截
		if c.Content == "EMPTY_IGNORE" {
			continue // 真正跳过空块，不计算向量，不入库
		}

		vec, err := a.embedder.Generate(c.Content)
		if err != nil {
			continue
		}

		// 此时直接 INSERT 即可
		res, err := a.mgr.Conn.Exec("INSERT INTO documents(heading, content, file_path) VALUES(?, ?, ?)",
			c.Heading, c.Content, path)
		if err != nil {
			fmt.Printf("❌ docs 写入失败: %v\n", err)
			continue
		}
		docID, _ := res.LastInsertId()

		_, err = a.mgr.Conn.Exec("INSERT INTO vec_idx(rowid, embedding) VALUES(?, ?)",
			docID, Float32ToByte(vec))
		if err != nil {
			fmt.Printf("❌ vec_idx 写入失败: %v\n", err)
		}
	}
}

// startWatcher 实时监听新文件
func (a *App) startWatcher(path string) {
	if a.watcher != nil {
		a.watcher.Close()
	}
	a.watcher, _ = fsnotify.NewWatcher()
	go func() {
		for {
			select {
			case event, ok := <-a.watcher.Events:
				if !ok {
					return
				}
				// 监听创建或写入，不再需要外层 Sleep，由 indexSingleFile 内部轮询处理
				if (event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Write == fsnotify.Write) &&
					strings.HasSuffix(strings.ToLower(event.Name), ".md") {
					a.indexSingleFile(event.Name)
					runtime.EventsEmit(a.ctx, "file_synced", "✅ 已同步文件: "+filepath.Base(event.Name))
				}
			}
		}
	}()
	a.watcher.Add(path)
}

// SearchAndAsk 核心 RAG 查询
func (a *App) SearchAndAsk(query string) (string, error) {
	queryVec, _ := a.embedder.Generate(query)

	// 关键修复：使用 v.rowid = d.id 进行关联查询
	rows, err := a.mgr.Conn.Query(`
        SELECT d.heading, d.content 
        FROM vec_idx v
        JOIN documents d ON v.rowid = d.id
        WHERE embedding MATCH ? AND k = 10
        ORDER BY distance`, Float32ToByte(queryVec))

	if err != nil {
		return "", err
	}
	defer rows.Close()

	var contextBuilder strings.Builder
	for rows.Next() {
		var heading, content string
		rows.Scan(&heading, &content)
		contextBuilder.WriteString(fmt.Sprintf("\n### %s\n%s\n", heading, content))
	}

	// 看看你到底喂给了 Ollama 什么东西
	fmt.Printf("--- 检索到的上下文 ---\n%s\n------------------\n", contextBuilder.String())

	if contextBuilder.Len() == 0 {
		return "未找到相关参考资料。请确保已建立索引且数据库中有向量数据。", nil
	}

	answer, _ := llm.AskOllama(contextBuilder.String(), query)
	re := regexp.MustCompile(`(?s)<think>.*?</think>`)
	return strings.TrimSpace(re.ReplaceAllString(answer, "")), nil
}

// SelectAndIndexFolder 供前端点击“选择目录”调用
func (a *App) SelectAndIndexFolder() (string, error) {
	folderPath, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择知识库目录",
	})
	if err != nil || folderPath == "" {
		return "", err
	}

	go a.indexDirectory(folderPath)
	go a.startWatcher(folderPath)
	return folderPath, nil
}

// indexDirectory 递归扫描并索引目录
func (a *App) indexDirectory(root string) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			a.indexSingleFile(path)
		}
		return nil
	})
	runtime.EventsEmit(a.ctx, "index_complete", "📚 目录索引同步完成")
}

// waitUntilReady 循环探测文件是否可读，直到超时
func (a *App) waitUntilReady(path string, timeout time.Duration) error {
	start := time.Now()
	for {
		// 尝试以只读模式打开文件
		f, err := os.OpenFile(path, os.O_RDONLY, 0)
		if err == nil {
			f.Close()
			return nil // 文件已释放锁，可以读取
		}

		if time.Since(start) > timeout {
			return fmt.Errorf("等待文件释放超时: %s", path)
		}

		// 如果失败，等 100 毫秒后再试
		time.Sleep(100 * time.Millisecond)
	}
}
