package db

import (
	"database/sql"
	"fmt"

	"github.com/RAGDock/RAGDock/internal/config"

	"github.com/mattn/go-sqlite3"
)

// Manager handles all interactions with the local SQLite database
type Manager struct {
	Conn   *sql.DB
	Config *config.AppConfig
}

// Global flag to ensure the custom SQLite driver is registered only once
var isDriverRegistered = false

// NewManager initializes a new database manager and registers the vector extension
func NewManager(cfg *config.AppConfig) (*Manager, error) {
	// Retrieve the platform-specific path for the vec0 SQLite extension
	vecPath := cfg.GetFullLibPath("vec0")

	if !isDriverRegistered {
		// Register a custom SQLite driver with the vector extension enabled
		sql.Register("sqlite3_RAGDock", &sqlite3.SQLiteDriver{
			Extensions: []string{vecPath},
		})
		isDriverRegistered = true
	}

	// Open the database connection using the custom driver
	db, err := sql.Open("sqlite3_RAGDock", cfg.GetDbPath())
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	m := &Manager{Conn: db, Config: cfg}
	
	// Ensure necessary tables exist
	if err := m.bootstrap(); err != nil {
		db.Close()
		return nil, err
	}
	return m, nil
}

// bootstrap creates the required tables (metadata and vector index) if they don't exist
func (m *Manager) bootstrap() error {
	// Table for document metadata and content
	const createDocsTable = `
	CREATE TABLE IF NOT EXISTS documents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		heading TEXT,
		content TEXT NOT NULL,
		file_path TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	// Virtual table for high-performance vector similarity search (vec0)
	// Dimensions are injected dynamically based on the configuration
	createVecTable := fmt.Sprintf(`
	CREATE VIRTUAL TABLE IF NOT EXISTS vec_idx USING vec0(
		embedding FLOAT[%d]
	);`, m.Config.ModelDim)

	if _, err := m.Conn.Exec(createDocsTable); err != nil {
		return fmt.Errorf("failed to create documents table: %v", err)
	}
	
	if _, err := m.Conn.Exec(createVecTable); err != nil {
		return fmt.Errorf("failed to create vector index table: %v", err)
	}
	
	return nil
}
