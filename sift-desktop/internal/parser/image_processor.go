package parser

import (
	"fmt"
	"path/filepath"

	"sift.local/internal/config"
	"sift.local/internal/llm"
)

// ProcessImage 图像索引核心逻辑
func ProcessImage(cfg *config.AppConfig, filePath string) (Chunk, error) {
	// 现在的 DescribeImage 已经在内部处理了文件读取
	description, err := llm.DescribeImage(cfg, filePath)
	if err != nil {
		return Chunk{}, err
	}

	if description == "" {
		return Chunk{}, fmt.Errorf("模型未返回任何描述内容")
	}
	// 增加调试日志，看看模型到底“看到了”什么
	//fmt.Printf("🔍 图片语意提取 [%s]: %s\n", filepath.Base(filePath), description)

	return Chunk{
		Content: description,
		Path:    filePath,
		Heading: "图片语意提取: " + filepath.Base(filePath),
	}, nil
}
