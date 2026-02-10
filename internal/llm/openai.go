package llm

import (
	"context"
	"fmt"
	"io"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

type OpenAIProvider struct {
	client *openai.Client
}

func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	return &OpenAIProvider{
		client: openai.NewClient(apiKey),
	}
}

func (p *OpenAIProvider) Name() string { return "openai" }

func (p *OpenAIProvider) Models() []string {
	return []string{
		"gpt-4", "gpt-4-turbo", "gpt-4o", "gpt-4o-mini", "gpt-3.5-turbo",
	}
}

func (p *OpenAIProvider) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	start := time.Now()

	msgs := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = openai.ChatCompletionMessage{Role: m.Role, Content: m.Content}
	}

	oReq := openai.ChatCompletionRequest{
		Model:    req.Model,
		Messages: msgs,
	}
	if req.Temperature > 0 {
		oReq.Temperature = float32(req.Temperature)
	}
	if req.MaxTokens > 0 {
		oReq.MaxTokens = req.MaxTokens
	}
	if req.TopP > 0 {
		oReq.TopP = float32(req.TopP)
	}
	if len(req.Stop) > 0 {
		oReq.Stop = req.Stop
	}

	resp, err := p.client.CreateChatCompletion(ctx, oReq)
	if err != nil {
		return nil, fmt.Errorf("openai chat: %w", err)
	}

	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	latency := time.Since(start).Milliseconds()
	cost := CalculateCost(req.Model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)

	return &ChatResponse{
		ID:           resp.ID,
		Provider:     "openai",
		Model:        resp.Model,
		Content:      content,
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
		TotalTokens:  resp.Usage.TotalTokens,
		CostUSD:      cost,
		LatencyMs:    latency,
	}, nil
}

func (p *OpenAIProvider) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	msgs := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = openai.ChatCompletionMessage{Role: m.Role, Content: m.Content}
	}

	oReq := openai.ChatCompletionRequest{
		Model:    req.Model,
		Messages: msgs,
		Stream:   true,
	}
	if req.Temperature > 0 {
		oReq.Temperature = float32(req.Temperature)
	}
	if req.MaxTokens > 0 {
		oReq.MaxTokens = req.MaxTokens
	}

	stream, err := p.client.CreateChatCompletionStream(ctx, oReq)
	if err != nil {
		return nil, fmt.Errorf("openai stream: %w", err)
	}

	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)
		defer stream.Close()
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				ch <- StreamChunk{Done: true}
				return
			}
			if err != nil {
				ch <- StreamChunk{Error: err, Done: true}
				return
			}
			if len(resp.Choices) > 0 {
				ch <- StreamChunk{Content: resp.Choices[0].Delta.Content}
			}
		}
	}()

	return ch, nil
}

func (p *OpenAIProvider) GenerateEmbedding(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	model := req.Model
	if model == "" {
		model = "text-embedding-3-small"
	}

	oReq := openai.EmbeddingRequest{
		Input: req.Input,
		Model: openai.EmbeddingModel(model),
	}

	resp, err := p.client.CreateEmbeddings(ctx, oReq)
	if err != nil {
		return nil, fmt.Errorf("openai embedding: %w", err)
	}

	embeddings := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		embeddings[i] = d.Embedding
	}

	cost := CalculateCost(model, resp.Usage.PromptTokens, 0)

	return &EmbeddingResponse{
		Provider:   "openai",
		Model:      model,
		Embeddings: embeddings,
		Tokens:     resp.Usage.TotalTokens,
		CostUSD:    cost,
	}, nil
}
