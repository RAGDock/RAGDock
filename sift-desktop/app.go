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

	"sift.local/internal/config"
	"sift.local/internal/db"
	"sift.local/internal/llm"
	"sift.local/internal/model"
	"sift.local/internal/parser"

	"github.com/fsnotify/fsnotify"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx        context.Context
	cfg        *config.AppConfig
	cancelFunc context.CancelFunc
	mgr        *db.Manager
	embedder   *model.Embedder
	watcher    *fsnotify.Watcher
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
	// 1. 加载配置
	a.cfg = config.LoadConfig()

	var err error
	// 2. 初始化数据库（传入配置对象）
	a.mgr, err = db.NewManager(a.cfg)
	if err != nil {
		fmt.Printf("❌ 数据库加载失败: %v\n", err)
	}

	// 3. 初始化嵌入模型（传入配置对象）
	a.embedder, err = model.NewEmbedder(a.cfg)
	if err != nil {
		fmt.Printf("❌ 模型加载失败: %v\n", err)
	}
}

// startWatcher 实时监听新文件
func (a *App) startWatcher(path string) {
	if a.watcher != nil {
		a.watcher.Close()
	}
	var err error
	a.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("❌ 无法启动监听器: %v\n", err)
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
				// 扩展：支持图片和 Markdown
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
				fmt.Printf("❌ 监听错误: %v\n", err)
			}
		}
	}()
	a.watcher.Add(path)
}

func (a *App) indexSingleFile(path string) {
	if err := a.waitUntilReady(path, 5*time.Second); err != nil {
		fmt.Printf("⚠️ 文件占用中: %v\n", err)
		return
	}

	var chunks []parser.Chunk
	var err error
	ext := strings.ToLower(filepath.Ext(path))

	// 1. 根据文件类型分发任务
	if ext == ".md" {
		chunks, err = parser.ParseMarkdown(path)
	} else {
		// 图片语意化处理
		// 注意：此处假设你已按前文建议在 internal/parser 下创建了 ProcessImage
		var chunk parser.Chunk
		chunk, err = parser.ProcessImage(a.cfg, path)
		if err == nil {
			chunks = []parser.Chunk{chunk}
		}
	}

	if err != nil {
		fmt.Printf("❌ 解析失败 [%s]: %v\n", path, err)
		runtime.EventsEmit(a.ctx, "file_synced", "❌ 解析失败: "+filepath.Base(path))
		return
	}

	// 2. 清理旧索引
	a.mgr.Conn.Exec("DELETE FROM vec_idx WHERE rowid IN (SELECT id FROM documents WHERE file_path = ?)", path)
	a.mgr.Conn.Exec("DELETE FROM documents WHERE file_path = ?", path)

	// 3. 统一入库
	for _, c := range chunks {
		if c.Content == "EMPTY_IGNORE" {
			continue
		}

		// 生成 384 维向量（无论文字还是图片描述都统一维度）
		vec, err := a.embedder.Generate(c.Content)
		if err != nil {
			continue
		}

		res, err := a.mgr.Conn.Exec("INSERT INTO documents(heading, content, file_path) VALUES(?, ?, ?)",
			c.Heading, c.Content, path)
		if err != nil {
			fmt.Printf("❌ docs 写入失败: %v\n", err)
			continue
		}
		docID, _ := res.LastInsertId()

		// 存入向量索引表
		_, err = a.mgr.Conn.Exec("INSERT INTO vec_idx(rowid, embedding) VALUES(?, ?)",
			docID, Float32ToByte(vec))
		if err != nil {
			fmt.Printf("❌ vec_idx 写入失败: %v\n", err)
		}

		// 在此处统一触发同步成功的事件
		msg := "✅ 文档已同步: "
		if strings.HasSuffix(strings.ToLower(path), ".md") {
			msg = "📄 文档已同步: "
		} else {
			msg = "🖼️ 图片语意索引完成: " // 针对 MiniCPM-V 处理后的提示
		}

		runtime.EventsEmit(a.ctx, "file_synced", msg+filepath.Base(path))
	}
}

// SearchAndAsk 处理 RAG 查询，支持多轮对话与流式输出 query: 当前用户提问内容 history: 之前的对话历史记录
func (a *App) SearchAndAsk(query string, history []llm.Message) error {
	if a.embedder == nil {
		return fmt.Errorf("嵌入模型未成功加载，请检查资源文件")
	}
	if a.mgr == nil || a.mgr.Conn == nil {
		return fmt.Errorf("数据库未连接")
	}

	var searchCtx context.Context
	searchCtx, a.cancelFunc = context.WithCancel(a.ctx)

	// 确保方法结束时清理取消函数
	defer func() {
		a.cancelFunc = nil
	}()

	// 2. 生成问题的向量索引
	queryVec, err := a.embedder.Generate(query)
	if err != nil {
		return fmt.Errorf("向量化失败: %v", err)
	}

	// 3. 在本地数据库中检索最相关的资料 (K=5 降低小模型负担)
	// 注意：此处使用 Float32ToByte 转换向量格式
	rows, err := a.mgr.Conn.Query(fmt.Sprintf(`
        SELECT d.heading, d.content 
        FROM vec_idx v
        JOIN documents d ON v.rowid = d.id
        WHERE embedding MATCH ? AND k = %d
        ORDER BY distance`, a.cfg.RagK), Float32ToByte(queryVec))

	if err != nil {
		return fmt.Errorf("数据库检索失败: %v", err)
	}
	defer rows.Close()

	// 4. 构建上下文文本块
	var contextBuilder strings.Builder
	for rows.Next() {
		var heading, content string
		if err := rows.Scan(&heading, &content); err == nil {
			// 将 ### 替换为更清晰的标签，帮助模型区分文本和图片语意
			contextBuilder.WriteString(fmt.Sprintf("\n【资料来源：%s】\n%s\n", heading, content))
		}
	}

	contextText := contextBuilder.String()
	if contextText == "" {
		contextText = "未找到相关的本地文档参考。"
	}

	// 5. 启动流式请求
	// 将 searchCtx 传入，以便支持用户手动中止
	err = llm.StreamOllama(searchCtx, a.cfg, contextText, history, query, func(token llm.GenerateResponse) {
		// 通过 Wails 事件系统将实时 Token 发送至前端
		// 包含 token.Thinking (思考内容) 和 token.Response (正式回答)
		runtime.EventsEmit(a.ctx, "llm_token", token)
	})

	if err != nil {
		// 如果是因为用户手动取消导致的错误，不视为系统异常
		if errors.Is(err, context.Canceled) {
			fmt.Println("ℹ️ 用户中止了搜索请求")
			return nil
		}
		return err
	}

	return nil
}

// StopSearch 供前端调用的中止方法
func (a *App) StopSearch() {
	if a.cancelFunc != nil {
		a.cancelFunc() // 触发 context 取消信号
		fmt.Println("🛑 用户手动中止了模型输出")
	}
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

func (a *App) indexDirectory(root string) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			fileName := strings.ToLower(info.Name())
			// 扩展：扫描时同时处理文档和图片
			if strings.HasSuffix(fileName, ".md") ||
				strings.HasSuffix(fileName, ".jpg") ||
				strings.HasSuffix(fileName, ".jpeg") ||
				strings.HasSuffix(fileName, ".png") {
				a.indexSingleFile(path)
			}
		}
		return nil
	})
	runtime.EventsEmit(a.ctx, "index_complete", "📚 目录索引同步完成")
}

func (a *App) waitUntilReady(path string, timeout time.Duration) error {
	start := time.Now()
	for {
		f, err := os.OpenFile(path, os.O_RDONLY, 0)
		if err == nil {
			f.Close()
			return nil
		}
		if time.Since(start) > timeout {
			return fmt.Errorf("等待文件释放超时: %s", path)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
