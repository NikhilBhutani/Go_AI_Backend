package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
)

// QueryRewriter transforms user queries for better retrieval.
type QueryRewriter interface {
	Rewrite(ctx context.Context, query string) ([]string, error)
}

// LLMQueryRewriter uses an LLM to expand and rephrase queries.
type LLMQueryRewriter struct {
	gateway llm.Gateway
	model   string
}

func NewLLMQueryRewriter(gw llm.Gateway, model string) *LLMQueryRewriter {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &LLMQueryRewriter{gateway: gw, model: model}
}

// Rewrite generates multiple reformulations of the query for multi-query retrieval.
func (r *LLMQueryRewriter) Rewrite(ctx context.Context, query string) ([]string, error) {
	resp, err := r.gateway.Chat(ctx, llm.ChatRequest{
		Model: r.model,
		Messages: []llm.Message{
			{
				Role: "system",
				Content: `You are a search query optimizer. Given a user question, generate 3 alternative
versions of the question that would help retrieve relevant documents from a vector database.
Each alternative should approach the question from a different angle.
Return ONLY the 3 questions, one per line, no numbering or bullets.`,
			},
			{
				Role:    "user",
				Content: query,
			},
		},
		Temperature: 0.7,
	})
	if err != nil {
		return []string{query}, nil
	}

	lines := strings.Split(strings.TrimSpace(resp.Content), "\n")
	queries := []string{query} // always include original
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && line != query {
			queries = append(queries, line)
		}
	}

	return queries, nil
}

// HyDE (Hypothetical Document Embeddings) generates a hypothetical answer
// to the query, then uses that answer's embedding for retrieval.
// This often retrieves better results than embedding the question directly.
type HyDE struct {
	gateway llm.Gateway
	model   string
}

func NewHyDE(gw llm.Gateway, model string) *HyDE {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &HyDE{gateway: gw, model: model}
}

// GenerateHypothetical creates a hypothetical document that would answer the query.
func (h *HyDE) GenerateHypothetical(ctx context.Context, query string) (string, error) {
	resp, err := h.gateway.Chat(ctx, llm.ChatRequest{
		Model: h.model,
		Messages: []llm.Message{
			{
				Role: "system",
				Content: `Write a short, factual paragraph that would perfectly answer the following question.
Write as if you are writing a passage from a reference document. Do not mention the question itself.
Be specific and detailed.`,
			},
			{
				Role:    "user",
				Content: query,
			},
		},
		Temperature: 0,
	})
	if err != nil {
		return "", fmt.Errorf("generate hypothetical document: %w", err)
	}

	return strings.TrimSpace(resp.Content), nil
}
