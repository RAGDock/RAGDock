package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/joho/godotenv"
)

// AppConfig stores all configuration parameters for the application
type AppConfig struct {
	// Ollama connection settings
	OllamaURL string

	// VLM (Vision Language Model) settings for image indexing
	VlmModel         string
	VlmTemp          float32
	VlmRepeatPenalty float32

	// RAG (Retrieval-Augmented Generation) chat model settings
	RagModel           string
	RagTemp            float32
	RagRepeatPenalty   float32
	RagPresencePenalty float32
	RagTopK            int
	RagTopP            float32
	RagK               int // Number of context snippets to retrieve

	// Local embedding model (ONNX) and vector settings
	ModelDir      string
	ModelFilename string
	TokenizerName string
	ModelDim      int
	EmbedderModel string

	// Database settings
	DbDir  string
	DbName string
}

// LoadConfig reads settings from .env file or uses default values
func LoadConfig() *AppConfig {
	// Load environment variables from .env if it exists
	_ = godotenv.Load()

	return &AppConfig{
		// Map environment variables (uppercase) to config struct
		OllamaURL: getEnv("OLLAMA_URL", "http://localhost:11434"),

		// VLM configuration (e.g., for OCR and image description)
		VlmModel:         getEnv("VLM_MODEL", "minicpm-v:8b-2.6-q4_K_M"),
		VlmTemp:          getEnvFloat("VLM_TEMP", 0.1),
		VlmRepeatPenalty: getEnvFloat("VLM_REPEAT_PENALTY", 1.1),

		// RAG configuration (chat and knowledge retrieval)
		RagModel:           getEnv("RAG_MODEL", "qwen2.5:1.5b"),
		RagTemp:            getEnvFloat("RAG_TEMP", 1.0),
		RagRepeatPenalty:   getEnvFloat("RAG_REPEAT_PENALTY", 1.2),
		RagPresencePenalty: getEnvFloat("RAG_PRESENCE_PENALTY", 1.5),
		RagTopK:            getEnvInt("RAG_TOP_K", 20),
		RagTopP:            getEnvFloat("RAG_TOP_P", 0.95),
		RagK:               getEnvInt("RAG_K", 10),

		// Local embedding model name (for reference)
		EmbedderModel: getEnv("EMBEDDER_MODEL", "bge-m3"),

		// Path and technical settings for local models
		ModelDir:      getEnv("MODEL_DIR", "resources/models"),
		ModelFilename: getEnv("MODEL_FILENAME", "model.onnx"),
		TokenizerName: getEnv("TOKENIZER_NAME", "tokenizer.json"),
		ModelDim:      getEnvInt("MODEL_DIM", 384),
		DbDir:         getEnv("DB_DIR", "."),
		DbName:        getEnv("DB_NAME", "ragdock_local.db"),
	}
}

// GetFullLibPath returns the platform-specific path for dynamic libraries (.so, .dll, .dylib)
// baseName: file name without extension, e.g., "vec0" or "libonnxruntime"
func (c *AppConfig) GetFullLibPath(baseName string) string {
	ext := ".so" // Default for Linux
	switch runtime.GOOS {
	case "windows":
		ext = ".dll"
	case "darwin":
		ext = ".dylib"
	}

	fullName := baseName + ext
	return filepath.Join("resources", "lib", fullName)
}

// GetTokenizerPath returns the absolute path to the tokenizer JSON file
func (c *AppConfig) GetTokenizerPath() string {
	return filepath.Join(c.ModelDir, c.TokenizerName)
}

// GetModelPath returns the absolute path to the ONNX model file
func (c *AppConfig) GetModelPath() string {
	return filepath.Join(c.ModelDir, c.ModelFilename)
}

// GetDbPath returns the absolute path to the SQLite database file
func (c *AppConfig) GetDbPath() string {
	return filepath.Join(c.DbDir, c.DbName)
}

// --- Helper Functions for Environment Variable Parsing ---

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvFloat(key string, defaultValue float32) float32 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseFloat(valueStr, 32)
	if err != nil {
		return defaultValue
	}
	return float32(value)
}
