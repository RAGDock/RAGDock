package db

import (
	"database/sql"
	"fmt"

	"sift.local/internal/config" // 引入配置包

	"github.com/mattn/go-sqlite3"
)

type Manager struct {
	Conn   *sql.DB
	Config *config.AppConfig
}

var isDriverRegistered = false

func NewManager(cfg *config.AppConfig) (*Manager, error) {
	// 跨平台获取 vec0 扩展路径
	vecPath := cfg.GetFullLibPath("vec0")

	if !isDriverRegistered {
		sql.Register("sqlite3_sift", &sqlite3.SQLiteDriver{
			Extensions: []string{vecPath},
		})
		isDriverRegistered = true
	}

	db, err := sql.Open("sqlite3_sift", cfg.GetDbPath())
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %v", err)
	}

	m := &Manager{Conn: db, Config: cfg}
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

	// 从配置动态注入维度
	createVecTable := fmt.Sprintf(`
	CREATE VIRTUAL TABLE IF NOT EXISTS vec_idx USING vec0(
		embedding FLOAT[%d]
	);`, m.Config.ModelDim)

	m.Conn.Exec(createDocsTable)
	m.Conn.Exec(createVecTable)
	return nil
}
