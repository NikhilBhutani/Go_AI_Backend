package llm

import (
	"context"
	"fmt"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type AnthropicProvider struct {
	client anthropic.Client
}

func NewAnthropicProvider(apiKey string) *AnthropicProvider {
	return &AnthropicProvider{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
	}
}

func (p *AnthropicProvider) Name() string { return "anthropic" }

func (p *AnthropicProvider) Models() []string {
	return []string{
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
		"claude-sonnet-4-20250514",
		"claude-opus-4-20250514",
	}
}

func (p *AnthropicProvider) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	start := time.Now()

	var systemText string
	var msgs []anthropic.MessageParam
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			systemText = m.Content
		case "user":
			msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		case "assistant":
			msgs = append(msgs, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
		}
	}

	maxTokens := int64(req.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 4096
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(req.Model),
		MaxTokens: maxTokens,
		Messages:  msgs,
	}
	if systemText != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemText},
		}
	}
	if req.Temperature > 0 {
		params.Temperature = anthropic.Float(req.Temperature)
	}
	if req.TopP > 0 {
		params.TopP = anthropic.Float(req.TopP)
	}
	if len(req.Stop) > 0 {
		params.StopSequences = req.Stop
	}

	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic chat: %w", err)
	}

	content := ""
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	latency := time.Since(start).Milliseconds()
	inputTokens := int(resp.Usage.InputTokens)
	outputTokens := int(resp.Usage.OutputTokens)
	cost := CalculateCost(req.Model, inputTokens, outputTokens)

	return &ChatResponse{
		ID:           string(resp.ID),
		Provider:     "anthropic",
		Model:        string(resp.Model),
		Content:      content,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
		CostUSD:      cost,
		LatencyMs:    latency,
	}, nil
}

func (p *AnthropicProvider) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	var systemText string
	var msgs []anthropic.MessageParam
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			systemText = m.Content
		case "user":
			msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		case "assistant":
			msgs = append(msgs, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
		}
	}

	maxTokens := int64(req.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 4096
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(req.Model),
		MaxTokens: maxTokens,
		Messages:  msgs,
	}
	if systemText != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemText},
		}
	}
	if req.Temperature > 0 {
		params.Temperature = anthropic.Float(req.Temperature)
	}

	stream := p.client.Messages.NewStreaming(ctx, params)

	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)
		defer stream.Close()

		accum := anthropic.Message{}
		for stream.Next() {
			evt := stream.Current()
			accum.Accumulate(evt)

			switch evt.Type {
			case "content_block_delta":
				if evt.Delta.Type == "text_delta" {
					ch <- StreamChunk{Content: evt.Delta.Text}
				}
			case "message_stop":
				ch <- StreamChunk{
					Done:         true,
					InputTokens:  int(accum.Usage.InputTokens),
					OutputTokens: int(accum.Usage.OutputTokens),
				}
				return
			}
		}
		if err := stream.Err(); err != nil {
			ch <- StreamChunk{Error: err, Done: true}
		}
	}()

	return ch, nil
}

func (p *AnthropicProvider) GenerateEmbedding(_ context.Context, _ EmbeddingRequest) (*EmbeddingResponse, error) {
	return nil, fmt.Errorf("anthropic does not support embeddings natively — use OpenAI or Ollama")
}
