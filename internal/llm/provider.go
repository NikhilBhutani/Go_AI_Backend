package llm

import (
	"context"
	"time"
)

// Provider abstracts an LLM provider (OpenAI, Anthropic, Ollama, etc.)
type Provider interface {
	ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
	GenerateEmbedding(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error)
	Name() string
	Models() []string
}

// Gateway provides multi-provider routing with fallback and retry.
type Gateway interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
	Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error)
	Provider(name string) (Provider, error)
	ListModels() []ModelInfo
}

// Message represents a single chat message.
type Message struct {
	Role    string `json:"role"`    // system, user, assistant
	Content string `json:"content"`
}

// ChatRequest is the input for chat completions.
type ChatRequest struct {
	Provider    string    `json:"provider,omitempty"`
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// ChatResponse is the output from chat completions.
type ChatResponse struct {
	ID           string  `json:"id"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	Content      string  `json:"content"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	CostUSD      float64 `json:"cost_usd"`
	LatencyMs    int64   `json:"latency_ms"`
}

// StreamChunk is a single chunk from a streaming response.
type StreamChunk struct {
	Content      string `json:"content,omitempty"`
	Done         bool   `json:"done"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
	Error        error  `json:"-"`
}

// EmbeddingRequest is the input for embedding generation.
type EmbeddingRequest struct {
	Provider string   `json:"provider,omitempty"`
	Model    string   `json:"model"`
	Input    []string `json:"input"`
}

// EmbeddingResponse is the output from embedding generation.
type EmbeddingResponse struct {
	Provider   string      `json:"provider"`
	Model      string      `json:"model"`
	Embeddings [][]float32 `json:"embeddings"`
	Tokens     int         `json:"tokens"`
	CostUSD    float64     `json:"cost_usd"`
}

// ModelInfo describes an available model.
type ModelInfo struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Type     string `json:"type"` // chat, embedding
}

// UsageRecord tracks a single LLM API call for cost tracking.
type UsageRecord struct {
	Provider     string
	Model        string
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	CostUSD      float64
	LatencyMs    int64
	Endpoint     string
	Timestamp    time.Time
}
