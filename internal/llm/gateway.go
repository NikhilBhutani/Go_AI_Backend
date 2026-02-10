package llm

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nikhilbhutani/backendwithai/internal/config"
)

type gateway struct {
	providers       map[string]Provider
	defaultProvider string
	fallbackProvider string
	maxRetries      int
}

func NewGateway(cfg config.LLMConfig) Gateway {
	g := &gateway{
		providers:        make(map[string]Provider),
		defaultProvider:  cfg.DefaultProvider,
		fallbackProvider: cfg.FallbackProvider,
		maxRetries:       cfg.MaxRetries,
	}

	if cfg.OpenAIKey != "" {
		g.providers["openai"] = NewOpenAIProvider(cfg.OpenAIKey)
	}
	if cfg.AnthropicKey != "" {
		g.providers["anthropic"] = NewAnthropicProvider(cfg.AnthropicKey)
	}
	if cfg.OllamaURL != "" {
		g.providers["ollama"] = NewOllamaProvider(cfg.OllamaURL)
	}

	return g
}

func (g *gateway) Provider(name string) (Provider, error) {
	p, ok := g.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not configured", name)
	}
	return p, nil
}

func (g *gateway) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	providerName := req.Provider
	if providerName == "" {
		providerName = g.defaultProvider
	}

	resp, err := g.chatWithRetry(ctx, providerName, req)
	if err != nil && g.fallbackProvider != "" && g.fallbackProvider != providerName {
		slog.Warn("primary provider failed, trying fallback",
			"primary", providerName,
			"fallback", g.fallbackProvider,
			"error", err,
		)
		return g.chatWithRetry(ctx, g.fallbackProvider, req)
	}
	return resp, err
}

func (g *gateway) chatWithRetry(ctx context.Context, providerName string, req ChatRequest) (*ChatResponse, error) {
	p, err := g.Provider(providerName)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= g.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * 500 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			slog.Debug("retrying LLM call", "provider", providerName, "attempt", attempt)
		}

		resp, err := p.ChatCompletion(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("all retries exhausted for %s: %w", providerName, lastErr)
}

func (g *gateway) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	providerName := req.Provider
	if providerName == "" {
		providerName = g.defaultProvider
	}

	p, err := g.Provider(providerName)
	if err != nil {
		return nil, err
	}

	return p.ChatCompletionStream(ctx, req)
}

func (g *gateway) Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	providerName := req.Provider
	if providerName == "" {
		providerName = g.defaultProvider
	}

	p, err := g.Provider(providerName)
	if err != nil {
		return nil, err
	}

	return p.GenerateEmbedding(ctx, req)
}

func (g *gateway) ListModels() []ModelInfo {
	var models []ModelInfo
	for _, p := range g.providers {
		for _, m := range p.Models() {
			models = append(models, ModelInfo{
				Provider: p.Name(),
				Model:    m,
				Type:     "chat",
			})
		}
	}
	return models
}
