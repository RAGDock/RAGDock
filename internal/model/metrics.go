package model

// PerfMetrics represents a snapshot of the system's performance for a single action
type PerfMetrics struct {
	Action           string  `json:"action"`             // "index" or "search"
	TotalMs          int64   `json:"total_ms"`          // Total end-to-end latency
	ParseMs          int64   `json:"parse_ms"`          // Parsing/VLM duration
	EmbedMs          int64   `json:"embed_ms"`          // Embedding generation duration
	SearchMs         int64   `json:"search_ms"`         // Vector DB search duration
	TTFTMs           int64   `json:"ttft_ms"`           // Time To First Token (Search only)
	InferenceMs      int64   `json:"inference_ms"`      // Total LLM duration
	PromptTokens     int     `json:"prompt_tokens"`     // Input token count
	CompletionTokens int     `json:"completion_tokens"` // Output token count
}
