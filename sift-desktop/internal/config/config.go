package config

import (
	"log"
	"os"
	"path/filepath"
	"runtime" // ✅ 引入 runtime 包用于检测系统
	"strconv"

	"github.com/joho/godotenv"
)

type AppConfig struct {
	ModelDir      string
	ModelFileName string
	TokenizerName string
	ModelDim      int
	DbDir         string
	DbName        string
	OllamaURL     string
	OllamaModel   string
	LibDir        string // ✅ 新增：动态库存储目录
}

func LoadConfig() *AppConfig {
	err := godotenv.Load()
	if err != nil {
		log.Println("⚠️ 未找到 .env 文件，将尝试使用系统环境变量")
	}

	wd, _ := os.Getwd()

	return &AppConfig{
		ModelDir:      getEnv("MODEL_DIR", filepath.Join(wd, "resources", "models")),
		ModelFileName: getEnv("MODEL_FILENAME", "model.onnx"),
		TokenizerName: getEnv("TOKENIZER_NAME", "tokenizer.json"),
		ModelDim:      getEnvAsInt("MODEL_DIM", 384),
		DbDir:         getEnv("DB_DIR", wd),
		DbName:        getEnv("DB_NAME", "sift_local.db"),
		OllamaURL:     getEnv("OLLAMA_URL", "http://localhost:11434"),
		OllamaModel:   getEnv("OLLAMA_MODEL", "qwen3.5:0.8b"),
		LibDir:        getEnv("LIB_DIR", filepath.Join(wd, "resources", "lib")), // ✅ 默认 resources/lib
	}
}

// ✅ 新增：根据平台获取正确的动态库文件名
func (c *AppConfig) GetLibFileName(baseName string) string {
	switch runtime.GOOS {
	case "windows":
		return baseName + ".dll"
	case "darwin":
		// macOS 上的库通常有 lib 前缀，如 libonnxruntime.dylib
		if baseName == "onnxruntime" {
			return "lib" + baseName + ".dylib"
		}
		return baseName + ".dylib"
	default: // linux
		return "lib" + baseName + ".so"
	}
}

// ✅ 新增：获取动态库的完整绝对路径
func (c *AppConfig) GetFullLibPath(baseName string) string {
	return filepath.Join(c.LibDir, c.GetLibFileName(baseName))
}

// 路径拼接辅助方法
func (c *AppConfig) GetModelPath() string     { return filepath.Join(c.ModelDir, c.ModelFileName) }
func (c *AppConfig) GetTokenizerPath() string { return filepath.Join(c.ModelDir, c.TokenizerName) }
func (c *AppConfig) GetDbPath() string        { return filepath.Join(c.DbDir, c.DbName) }

// 辅助函数保持不变
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}
