# RAGDock

> **The Universal Local RAG Hub for Your Private Knowledge Base.**  
> *Privacy-first, Model-agnostic, and Lightweight.*

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/github/go-mod/go-version/RAGDock/RAGDock)](https://go.dev/)
[![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-blue)](https://github.com/RAGDock/RAGDock)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/RAGDock/RAGDock/pulls)

RAGDock is a high-performance, cross-platform desktop application that transforms your local documents into a searchable, intelligent knowledge base. Built with Go, Wails, and SQLite, it provides a "dock" where you can plug in any local LLM (via Ollama) or embedding model (via ONNX) to interact with your data—100% offline.

---

## Key Features

- **Model Agnostic**: Seamlessly switch between different LLMs via [Ollama](https://ollama.com/) or use built-in ONNX models for embeddings. No vendor lock-in.
- **Privacy by Design**: All data stays on your machine. Parsing, vectorization, and inference happen entirely locally.
- **Lightweight & Fast**: Powered by a Go backend and a specialized SQLite vector extension for sub-millisecond retrieval.
- **True Cross-Platform**: Optimized binaries for Windows, macOS (Intel/Apple Silicon), and Linux.
- **Multi-Format Support**: Intelligent parsing for Markdown, PDF, TXT, and images.
- **Modern UI/UX**: A clean, intuitive interface built with Svelte and Wails for a native desktop experience.

---

## Architecture

RAGDock acts as the orchestration layer between your data and your models:

1.  **Ingestion**: Documents are parsed and cleaned locally.
2.  **Vectorization**: Text chunks are converted into embeddings using local ONNX models.
3.  **Storage**: Vectors and metadata are stored in a local SQLite database with `vec0` extensions.
4.  **Retrieval**: Context-aware search finds the most relevant snippets for your query.
5.  **Generation**: Local LLMs (Ollama) generate precise answers based on the retrieved context.

---

## Resource Setup

For RAGDock to function correctly, specific system libraries and model files must be placed in the `resources` directory. These are excluded from the repository due to size and platform-specific requirements.

### 1. System Libraries (`resources/lib/`)
Place the required dynamic libraries for your operating system:

| File Name | Purpose | Location |
| :--- | :--- | :--- |
| `libonnxruntime.dylib` / `.so` / `.dll` | ONNX Runtime engine | `resources/lib/` |
| `vec0.dylib` / `.so` / `.dll` | SQLite vector search extension | `resources/lib/` |

- **ONNX Runtime**: Download from [Microsoft ONNX Runtime Releases](https://github.com/microsoft/onnxruntime/releases).
- **SQLite vec0**: Obtain from the [sqlite-vec](https://github.com/asg017/sqlite-vec) project.

### 2. Embedding Models (`resources/models/`)
Download and place your chosen embedding model and its tokenizer:

| File Name | Description | Location |
| :--- | :--- | :--- |
| `model.onnx` | ONNX-format embedding model (e.g., BGE-Small) | `resources/models/` |
| `tokenizer.json` | JSON configuration for the tokenizer | `resources/models/` |

---

## Quick Start

### Prerequisites
- [Ollama](https://ollama.com/) installed and running.
- Correct libraries and models placed in `resources/` (see **Resource Setup** above).
- Modern OS (Windows 10+, macOS 12+, or mainstream Linux).

### Installation
1.  **Download**: Get the latest release for your platform from the [Releases](https://github.com/RAGDock/RAGDock/releases) page.
2.  **Launch**: Run the executable. RAGDock will automatically initialize the local environment.
3.  **Connect**: Select your preferred model from settings (e.g., `llama3`, `mistral`).
4.  **Ingest**: Drag and drop folders or files into the "Knowledge" tab.
5.  **Chat**: Start asking questions about your data.

---

## Development

To build RAGDock from source:

```bash
# Clone the repository
git clone https://github.com/RAGDock/RAGDock.git
cd RAGDock

# Ensure resources/lib and resources/models are populated (see Resource Setup)

# Install frontend dependencies
cd frontend
npm install

# Build and run with Wails
cd ..
wails dev
```

*Required: Go 1.21+, Node.js 18+, and Wails CLI.*

---

## Roadmap

- [ ] **Hybrid Search**: Combine keyword search with vector search for higher accuracy.
- [ ] **Plugin System**: Support for custom document parsers (Excel, PowerPoint, etc.).
- [ ] **Multi-Agent RAG**: Advanced reasoning steps for complex queries.
- [ ] **Mobile Client**: Companion app for viewing synced local knowledge.

---

## Contributing

Contributions are welcome. Please feel free to:
1. Fork the Project.
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`).
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`).
4. Push to the Branch (`git push origin feature/AmazingFeature`).
5. Open a Pull Request.

---

## License

Distributed under the Apache License 2.0. See `LICENSE` for more information.

---

## Contact

**RAGDock Team** - [GitHub](https://github.com/RAGDock)

Project Link: [https://github.com/RAGDock/RAGDock](https://github.com/RAGDock/RAGDock)

*"Empowering everyone with their own local intelligence hub."*
