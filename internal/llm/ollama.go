package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type OllamaProvider struct {
	baseURL    string
	httpClient *http.Client
}

func NewOllamaProvider(baseURL string) *OllamaProvider {
	return &OllamaProvider{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

func (p *OllamaProvider) Name() string { return "ollama" }

func (p *OllamaProvider) Models() []string {
	return []string{"llama3", "mistral", "codellama", "nomic-embed-text"}
}

type ollamaChatReq struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *ollamaOptions  `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
}

type ollamaChatResp struct {
	Message          ollamaMessage `json:"message"`
	Done             bool          `json:"done"`
	TotalDuration    int64         `json:"total_duration"`
	PromptEvalCount  int           `json:"prompt_eval_count"`
	EvalCount        int           `json:"eval_count"`
}

func (p *OllamaProvider) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	start := time.Now()

	msgs := make([]ollamaMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = ollamaMessage{Role: m.Role, Content: m.Content}
	}

	oReq := ollamaChatReq{
		Model:    req.Model,
		Messages: msgs,
		Stream:   false,
	}
	if req.Temperature > 0 || req.MaxTokens > 0 {
		oReq.Options = &ollamaOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
			TopP:        req.TopP,
		}
	}

	body, _ := json.Marshal(oReq)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama chat: %w", err)
	}
	defer resp.Body.Close()

	var oResp ollamaChatResp
	if err := json.NewDecoder(resp.Body).Decode(&oResp); err != nil {
		return nil, fmt.Errorf("ollama decode: %w", err)
	}

	latency := time.Since(start).Milliseconds()

	return &ChatResponse{
		Provider:     "ollama",
		Model:        req.Model,
		Content:      oResp.Message.Content,
		InputTokens:  oResp.PromptEvalCount,
		OutputTokens: oResp.EvalCount,
		TotalTokens:  oResp.PromptEvalCount + oResp.EvalCount,
		CostUSD:      0, // local models are free
		LatencyMs:    latency,
	}, nil
}

func (p *OllamaProvider) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	msgs := make([]ollamaMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = ollamaMessage{Role: m.Role, Content: m.Content}
	}

	oReq := ollamaChatReq{
		Model:    req.Model,
		Messages: msgs,
		Stream:   true,
	}

	body, _ := json.Marshal(oReq)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama stream request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama stream: %w", err)
	}

	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		dec := json.NewDecoder(resp.Body)
		for {
			var chunk ollamaChatResp
			if err := dec.Decode(&chunk); err != nil {
				if err == io.EOF {
					ch <- StreamChunk{Done: true}
				} else {
					ch <- StreamChunk{Error: err, Done: true}
				}
				return
			}
			if chunk.Done {
				ch <- StreamChunk{
					Done:         true,
					InputTokens:  chunk.PromptEvalCount,
					OutputTokens: chunk.EvalCount,
				}
				return
			}
			ch <- StreamChunk{Content: chunk.Message.Content}
		}
	}()

	return ch, nil
}

type ollamaEmbedReq struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

type ollamaEmbedResp struct {
	Embeddings [][]float32 `json:"embeddings"`
}

func (p *OllamaProvider) GenerateEmbedding(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	model := req.Model
	if model == "" {
		model = "nomic-embed-text"
	}

	oReq := ollamaEmbedReq{
		Model: model,
		Input: req.Input,
	}

	body, _ := json.Marshal(oReq)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama embed request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}
	defer resp.Body.Close()

	var oResp ollamaEmbedResp
	if err := json.NewDecoder(resp.Body).Decode(&oResp); err != nil {
		return nil, fmt.Errorf("ollama embed decode: %w", err)
	}

	return &EmbeddingResponse{
		Provider:   "ollama",
		Model:      model,
		Embeddings: oResp.Embeddings,
		CostUSD:    0,
	}, nil
}
