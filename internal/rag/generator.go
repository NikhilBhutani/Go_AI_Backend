package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
	"github.com/nikhilbhutani/backendwithai/internal/vectorstore"
)

type Generator struct {
	gateway llm.Gateway
}

func NewGenerator(gw llm.Gateway) *Generator {
	return &Generator{gateway: gw}
}

type GenerateRequest struct {
	Query    string
	Context  []vectorstore.SearchResult
	Model    string
	Provider string
}

type GenerateResponse struct {
	Answer    string     `json:"answer"`
	Citations []Citation `json:"citations"`
	Usage     llm.ChatResponse
}

type Citation struct {
	DocumentID string `json:"document_id"`
	ChunkID    string `json:"chunk_id"`
	Content    string `json:"content"`
	Score      float64 `json:"score"`
}

func (g *Generator) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	contextStr := buildContext(req.Context)

	messages := []llm.Message{
		{
			Role: "system",
			Content: `You are a helpful AI assistant. Answer the user's question based on the provided context.
If the context doesn't contain enough information, say so. Always cite which sources you used.
Format citations as [Source N] where N corresponds to the context chunk number.`,
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("Context:\n%s\n\nQuestion: %s", contextStr, req.Query),
		},
	}

	chatReq := llm.ChatRequest{
		Provider: req.Provider,
		Model:    req.Model,
		Messages: messages,
	}

	resp, err := g.gateway.Chat(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("generate answer: %w", err)
	}

	citations := make([]Citation, len(req.Context))
	for i, c := range req.Context {
		citations[i] = Citation{
			DocumentID: c.DocumentID.String(),
			ChunkID:    c.ChunkID.String(),
			Content:    truncate(c.Content, 200),
			Score:      c.Score,
		}
	}

	return &GenerateResponse{
		Answer:    resp.Content,
		Citations: citations,
		Usage:     *resp,
	}, nil
}

func buildContext(results []vectorstore.SearchResult) string {
	var sb strings.Builder
	for i, r := range results {
		fmt.Fprintf(&sb, "[Source %d] (score: %.3f)\n%s\n\n", i+1, r.Score, r.Content)
	}
	return sb.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
