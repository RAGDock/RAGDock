package parser

import (
	"os"
	"strings"
)

type Chunk struct {
	Content string // 块内容
	Path    string // 文件路径
	Heading string // 标题路径，例如 "项目概述 > 技术架构"
}

// ParseMarkdown 核心逻辑：支持标准标题切片，并对非标准文档进行段落兜底
func ParseMarkdown(filePath string) ([]Chunk, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	content := string(data)
	// 1. 统一换行符，防止 Windows \r\n 干扰匹配逻辑
	content = strings.ReplaceAll(content, "\r\n", "\n")

	var chunks []Chunk

	// 2. 检查是否包含标准 Markdown 标题 (#)
	// 检查 "\n#" 是为了确保 # 出现在行首
	hasHeaders := strings.HasPrefix(content, "#") || strings.Contains(content, "\n#")

	if hasHeaders {
		// --- 方案 A: 标准标题切分 ---
		lines := strings.Split(content, "\n")
		var currentHeading []string
		var currentBody strings.Builder

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {

				// 如果当前缓冲区有内容，先存为一个已有的 Chunk
				trimmedBody := strings.TrimSpace(currentBody.String())
				if len(trimmedBody) > 2 { // 过滤掉少于 2 字符的琐碎内容或空行
					chunks = append(chunks, buildChunk(currentBody.String(), filePath, currentHeading))
					currentBody.Reset()
				}

				// 解析标题层级 (支持 #, ##, ### 等)
				level := 0
				for i := 0; i < len(trimmed) && trimmed[i] == '#'; i++ {
					level++
				}
				title := strings.TrimSpace(trimmed[level:])

				// 维护标题栈：level 1 对应索引 0
				if level <= len(currentHeading) {
					currentHeading = currentHeading[:level-1]
				}

				currentHeading = append(currentHeading, title)
			} else {
				currentBody.WriteString(line + "\n")
			}
		}
		// 处理文件结尾的最后一个块
		if currentBody.Len() > 0 {
			chunks = append(chunks, buildChunk(currentBody.String(), filePath, currentHeading))
		}
	} else {
		// --- 方案 B: 兜底切分 (针对你的 Flutter 手册) ---
		// 既然文档没有使用 # 标题，我们按照双换行（段落）进行切分
		// 这样可以避免整个文件被当成一条数据，从而提高检索精度
		paragraphs := strings.Split(content, "\n\n")
		for _, p := range paragraphs {
			p = strings.TrimSpace(p)
			if len(p) < 20 { // 过滤掉太短的无意义行（如只有几个字的零碎行）
				continue
			}
			chunks = append(chunks, Chunk{
				Content: p,
				Path:    filePath,
				Heading: "段落内容",
			})
		}
	}

	return chunks, nil
}

// 辅助函数：构建带层级标题路径的 Chunk
// internal/parser/md_parser.go

func buildChunk(body string, path string, headings []string) Chunk {
	trimmedBody := strings.TrimSpace(body)

	// 如果内容为空，标记为特殊字符串
	if trimmedBody == "" {
		return Chunk{Content: "EMPTY_IGNORE"}
	}

	fullHeading := strings.Join(headings, " > ")
	if fullHeading == "" {
		fullHeading = "正文"
	}
	return Chunk{
		Content: trimmedBody,
		Path:    path,
		Heading: fullHeading,
	}
}
