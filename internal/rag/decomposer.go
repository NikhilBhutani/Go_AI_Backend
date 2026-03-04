package rag

import (
	"context"
	"strings"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
)

// Decomposer breaks a complex query into simpler sub-questions.
type Decomposer interface {
	Decompose(ctx context.Context, query string) ([]string, error)
}

// LLMDecomposer uses an LLM to decompose complex questions.
type LLMDecomposer struct {
	gateway llm.Gateway
	model   string
}

func NewLLMDecomposer(gw llm.Gateway, model string) *LLMDecomposer {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &LLMDecomposer{gateway: gw, model: model}
}

// Decompose splits a complex query into 2-4 focused sub-questions.
// If decomposition fails, the original query is returned as a single-element slice.
func (d *LLMDecomposer) Decompose(ctx context.Context, query string) ([]string, error) {
	resp, err := d.gateway.Chat(ctx, llm.ChatRequest{
		Model: d.model,
		Messages: []llm.Message{
			{
				Role: "system",
				Content: `You are a question decomposer for a retrieval-augmented generation system.
Break the user's complex question into 2-4 simpler, focused sub-questions that together cover the original question.
Each sub-question should be independently answerable from a knowledge base.
Return ONLY the sub-questions, one per line, no numbering, bullets, or extra text.`,
			},
			{Role: "user", Content: query},
		},
		Temperature: 0.3,
	})
	if err != nil {
		return []string{query}, nil
	}

	var subQuestions []string
	for _, line := range strings.Split(strings.TrimSpace(resp.Content), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			subQuestions = append(subQuestions, line)
		}
	}

	if len(subQuestions) == 0 {
		return []string{query}, nil
	}

	return subQuestions, nil
}

// Ensure LLMDecomposer implements Decomposer.
var _ Decomposer = (*LLMDecomposer)(nil)
