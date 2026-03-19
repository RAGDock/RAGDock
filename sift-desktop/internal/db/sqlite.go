package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mattn/go-sqlite3"
)

type Manager struct {
	Conn *sql.DB
}

// 保证驱动只注册一次
var isDriverRegistered = false

func NewManager(dbPath string) (*Manager, error) {
	// 1. 动态获取基础路径
	// 在 wails dev 模式下，工作目录 (Getwd) 始终是项目根目录
	baseDir, _ := os.Getwd()

	// 拼接 vec0.dll 的相对路径
	vecDllPath := filepath.Join(baseDir, "resources", "lib", "vec0.dll")

	// 打印一下，方便调试时确认路径
	fmt.Printf("📂 SQLite 正在尝试加载扩展: %s\n", vecDllPath)

	// 2. 注册驱动 (增加安全检查，防止多次调用 NewManager 导致 panic)
	if !isDriverRegistered {
		sql.Register("sqlite3_sift", &sqlite3.SQLiteDriver{
			Extensions: []string{vecDllPath},
		})
		isDriverRegistered = true
	}

	// 3. 打开数据库
	db, err := sql.Open("sqlite3_sift", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %v", err)
	}

	m := &Manager{Conn: db}

	// 4. 初始化表结构
	if err := m.bootstrap(); err != nil {
		db.Close()
		return nil, err
	}

	return m, nil
}

func (m *Manager) bootstrap() error {
	const createDocsTable = `
	CREATE TABLE IF NOT EXISTS documents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		heading TEXT,
		content TEXT NOT NULL,
		file_path TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	const createVecTable = `
	CREATE VIRTUAL TABLE IF NOT EXISTS vec_idx USING vec0(
		embedding FLOAT[384]
	);`

	m.Conn.Exec(createDocsTable)
	m.Conn.Exec(createVecTable)
	return nil
}
