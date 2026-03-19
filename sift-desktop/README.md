Sift (EchoVault)

English | 中文

<a name="english"></a>

English

Sift is a lightweight, privacy-first, and local-only desktop RAG (Retrieval-Augmented Generation) knowledge base. It allows you to perform semantic searches and chat with your local Markdown documents without any data leaving your machine.

✨ Key Features

Local-first RAG: Powered by Ollama (LLM) and ONNX (Embeddings) for 100% offline privacy.

Smart Indexing: Automatically splits Markdown files by headers or paragraphs for precise retrieval.

Real-time Monitoring: Uses fsnotify to monitor folder changes and auto-index new or modified files.

High-Performance Vector Search: Leverages sqlite-vec for efficient local vector operations.

Minimalist UI: A clean black-and-white interface built with Wails and Svelte.

🏗️ Tech Stack

Backend: Go, Wails v2

Database: SQLite + sqlite-vec

Embedding Model: BGE-Micro-v2 (via ONNX Runtime)

Frontend: Svelte, Vite, Tailwind CSS

LLM Engine: Ollama

📂 Project Structure

sift-desktop/
├── app.go              # Main Wails application logic (Bridge)
├── main.go             # Application entry point
├── internal/
│   ├── db/             # SQLite initialization & Vector extension loading
│   ├── llm/            # Ollama API client & Streaming logic
│   ├── model/          # ONNX Embedding engine (Mean Pooling & L2 Norm)
│   └── parser/         # Markdown chunking logic (Header & Paragraph)
├── resources/
│   ├── lib/            # External DLLs (onnxruntime.dll, vec0.dll)
│   └── models/         # AI Models (model.onnx, tokenizer.json)
├── frontend/           # Svelte UI source code
└── build/              # Final binary build output


🚀 Getting Started

Prerequisites

Ollama: Install and run Ollama.

Pull your preferred model: ollama pull qwen2.5:3b (or your custom model name).

Go: Install Go 1.21+.

Node.js: Install Node.js 18+ and npm.

Wails: Install Wails CLI: go install github.com/wailsapp/wails/v2/cmd/wails@latest.

Setup

Clone the repository.

Place onnxruntime.dll and vec0.dll into resources/lib/.

Place your BGE ONNX model and tokenizer.json into resources/models/.

Run in development mode:

wails dev


<a name="chinese"></a>

中文

Sift (EchoVault) 是一款轻量级、隐私至上、全本地运行的桌面端 RAG（检索增强生成）知识库。它让你可以通过语义搜索与本地 Markdown 文档进行对话，所有数据均不会离开你的计算机。

✨ 核心特性

本地优先 RAG: 由 Ollama (LLM) 和 ONNX (Embeddings) 驱动，实现 100% 离线隐私保护。

智能索引: 自动按标题或段落切分 Markdown 文件，确保检索精度。

实时监控: 利用 fsnotify 监控文件夹变动，自动索引新增或修改的文件。

高性能向量搜索: 使用 sqlite-vec 扩展实现高效的本地向量运算。

极简 UI: 使用 Wails 和 Svelte 构建的黑白极简风格界面。

🏗️ 技术栈

后端: Go, Wails v2

数据库: SQLite + sqlite-vec

嵌入模型: BGE-Micro-v2 (通过 ONNX Runtime 运行)

前端: Svelte, Vite, Tailwind CSS

LLM 引擎: Ollama

📂 项目文件结构

sift-desktop/
├── app.go              # Wails 应用主逻辑（桥接 Go 与 JS）
├── main.go             # 程序入口
├── internal/
│   ├── db/             # SQLite 初始化与向量扩展加载
│   ├── llm/            # Ollama 接口调用与流式逻辑
│   ├── model/          # ONNX 嵌入引擎（包含 Mean Pooling 与 L2 归一化）
│   └── parser/         # Markdown 切片逻辑（支持标题与段落）
├── resources/
│   ├── lib/            # 外部 DLL 依赖 (onnxruntime.dll, vec0.dll)
│   └── models/         # AI 模型文件 (model.onnx, tokenizer.json)
├── frontend/           # Svelte UI 源码
└── build/              # 编译生成的二进制文件


🚀 快速开始

环境准备

Ollama: 安装并运行 Ollama。

下载模型: ollama pull qwen2.5:3b (或你在代码中指定的自定义模型名)。

Go: 安装 Go 1.21+。

Node.js: 安装 Node.js 18+ 及 npm。

Wails: 安装 Wails CLI: go install github.com/wailsapp/wails/v2/cmd/wails@latest。

安装运行

克隆仓库。

将 onnxruntime.dll 和 vec0.dll 放入 resources/lib/ 目录。

将 BGE ONNX 模型文件和 tokenizer.json 放入 resources/models/ 目录。

启动开发模式:

wails dev


📄 开源协议

本项目采用 MIT License 协议。