# RAGDock Performance Optimization Roadmap

This document outlines strategic optimization opportunities for RAGDock to achieve enterprise-grade speed, accuracy, and scalability on local hardware.

---

## 1. Indexing Pipeline: From Linear to Concurrent (High Impact)

Currently, document processing happens sequentially. On multi-core systems (like the Apple M4), this leaves significant performance on the table.

- **Goroutine Worker Pool**: Implement a concurrent indexing scheduler in `app.go`. Instead of processing files one by one, use a pool of 4-8 workers to parse and embed multiple files simultaneously.
- **Batch Embedding**: Update `internal/model/embedder.go` to support batch inference. ONNX Runtime is significantly more efficient when processing a batch of 8 or 16 text chunks in a single call compared to multiple individual calls.
- **Incremental Indexing**: Implement a hashing mechanism (MD5/SHA256) or track `ModTime` in the `documents` table. Skip re-parsing and re-embedding for files that haven't changed since the last sync.

## 2. Search Enhancement: Hybrid & Precision (High Quality)

Vector search is great for semantics but can miss exact keywords or technical terms.

- **Hybrid Search (Vector + BM25)**: Integrate SQLite's **FTS5** extension alongside `vec0`. Use FTS5 for keyword matching and combine the results with vector similarity using the **Reciprocal Rank Fusion (RRF)** algorithm.
- **Cross-Encoder Re-ranking**: After the initial retrieval (Top 50), use a lightweight ONNX-based Cross-Encoder model to re-score the top candidates. This drastically improves retrieval precision by focusing on the actual relationship between the query and the context.

## 3. Storage Layer: SQLite Tuning (High Stability)

Optimizing database I/O ensures the UI remains responsive even during heavy indexing.

- **WAL Mode**: Enable Write-Ahead Logging (`PRAGMA journal_mode=WAL;`). This allows concurrent reads (searching) and writes (indexing) without database locking issues.
- **Memory Mapping & Cache**: Increase the SQLite page cache size and enable memory-mapped I/O (`PRAGMA mmap_size = 268435456;`) to leverage the fast Unified Memory in Mac Mini M4.
- **Vector Quantization**: For massive knowledge bases, explore scalar quantization to store vectors as `int8` instead of `float32`, reducing storage footprint and I/O pressure by 4x.

## 4. Hardware Acceleration: Tapping into NPU/GPU (Ultra High Impact)

- **ONNX Execution Providers**: Currently, the embedding model likely runs on the CPU. Configure `onnxruntime_go` to use the **CoreML** or **Metal** Execution Provider on macOS. This offloads embedding tasks to the Apple Neural Engine (ANE) or GPU.
- **VLM Image Pre-processing**: Optimize image indexing by downsampling high-resolution images or converting them to grayscale before sending them to Ollama. This reduces the `load_duration` for VLM models significantly.

## 5. UX & Logic Optimizations (Low Effort, High Gain)

- **Semantic Caching**: Store recent query-response pairs in a local cache. If a new query is semantically identical to a previous one, return the cached answer instantly (ms instead of seconds).
- **Stream Throttling**: In the frontend, throttle the rendering of `llm_token` events. Instead of updating the DOM for every single character, batch updates every 50-100ms to reduce UI thread load.

---

## Implementation Priority (Phase-based)

### Phase 1: Core Stability (Next 1-2 Weeks)
1.  Enable **SQLite WAL Mode** to fix read/write conflicts.
2.  Implement **Incremental Indexing** (Hash-based) to save CPU/Time.

### Phase 2: Throughput Optimization
1.  Build the **Goroutine Worker Pool** for parallel file processing.
2.  Enable **Batching** in the ONNX embedder.

### Phase 3: Accuracy & Intelligence
1.  Integrate **FTS5 Hybrid Search**.
2.  Enable **CoreML/Metal Acceleration** for ONNX.

---
*Documented on: 2026-04-19*
*"Performance is a feature, not an afterthought."*
