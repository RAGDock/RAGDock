package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/joho/godotenv"
)

type AppConfig struct {
	// Ollama 相关配置
	OllamaURL string

	// VLM 模型配置
	VlmModel         string
	VlmTemp          float32
	VlmRepeatPenalty float32

	// RAG 模型配置
	RagModel         string
	RagTemp          float32
	RagRepeatPenalty float32
	RagK             int

	// 本地模型与向量配置
	ModelDir      string
	ModelFilename string
	TokenizerName string
	ModelDim      int
	EmbedderModel string

	// 数据库配置
	DbDir  string
	DbName string
}

func LoadConfig() *AppConfig {
	_ = godotenv.Load()

	return &AppConfig{
		// 环境变量键名与 .env 保持全大写一致
		OllamaURL: getEnv("OLLAMA_URL", "http://localhost:11434"),

		// VLM 配置
		VlmModel:         getEnv("VLM_MODEL", "minicpm-v:8b-2.6-q4_K_M"),
		VlmTemp:          getEnvFloat("VLM_TEMP", 0.1),
		VlmRepeatPenalty: getEnvFloat("VLM_REPEAT_PENALTY", 1.1),

		// RAG 配置
		RagModel:         getEnv("RAG_MODEL", "gemma2:9b"),
		RagTemp:          getEnvFloat("RAG_TEMP", 0.7),
		RagRepeatPenalty: getEnvFloat("RAG_REPEAT_PENALTY", 1.2),
		RagK:             getEnvInt("RAG_K", 10),

		// 向量化index模型
		EmbedderModel: getEnv("EMBEDDER_MODEL", "bge-m3"),

		// 本地路径配置
		ModelDir:      getEnv("MODEL_DIR", "resources/models"),
		ModelFilename: getEnv("MODEL_FILENAME", "model.onnx"),
		TokenizerName: getEnv("TOKENIZER_NAME", "tokenizer.json"),
		ModelDim:      getEnvInt("MODEL_DIM", 384),
		DbDir:         getEnv("DB_DIR", "."),
		DbName:        getEnv("DB_NAME", "sift_local.db"),
	}
}

// GetFullLibPath 根据平台自动补全库文件后缀
// baseName: 不带后缀的文件名，如 "vec0" 或 "libonnxruntime"
func (c *AppConfig) GetFullLibPath(baseName string) string {
	ext := ".so" // 默认 Linux
	switch runtime.GOOS {
	case "windows":
		ext = ".dll"
	case "darwin":
		ext = ".dylib"
	}

	fullName := baseName + ext
	return filepath.Join("resources", "lib", fullName)
}

func (c *AppConfig) GetTokenizerPath() string {
	return filepath.Join(c.ModelDir, c.TokenizerName)
}

func (c *AppConfig) GetModelPath() string {
	return filepath.Join(c.ModelDir, c.ModelFilename)
}

func (c *AppConfig) GetDbPath() string {
	return filepath.Join(c.DbDir, c.DbName)
}

// --- 辅助转换函数 ---

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
