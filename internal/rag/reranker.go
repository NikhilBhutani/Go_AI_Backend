package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
	"github.com/nikhilbhutani/backendwithai/internal/vectorstore"
)

// Reranker re-scores retrieved chunks for better relevance ordering.
type Reranker interface {
	Rerank(ctx context.Context, query string, results []vectorstore.SearchResult) ([]vectorstore.SearchResult, error)
}

// LLMReranker uses an LLM to judge relevance of each chunk to the query.
type LLMReranker struct {
	gateway llm.Gateway
	model   string
}

func NewLLMReranker(gw llm.Gateway, model string) *LLMReranker {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &LLMReranker{gateway: gw, model: model}
}

func (r *LLMReranker) Rerank(ctx context.Context, query string, results []vectorstore.SearchResult) ([]vectorstore.SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}

	// Build a prompt asking the LLM to score each chunk
	var sb strings.Builder
	for i, res := range results {
		fmt.Fprintf(&sb, "[%d] %s\n\n", i, truncate(res.Content, 500))
	}

	resp, err := r.gateway.Chat(ctx, llm.ChatRequest{
		Model: r.model,
		Messages: []llm.Message{
			{
				Role: "system",
				Content: `You are a relevance scoring assistant. Given a query and a list of text chunks,
score each chunk from 0.0 to 1.0 based on how relevant it is to the query.
Return ONLY a JSON array of objects with "index" and "score" fields. Example:
[{"index": 0, "score": 0.95}, {"index": 1, "score": 0.3}]`,
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("Query: %s\n\nChunks:\n%s", query, sb.String()),
			},
		},
		Temperature: 0,
	})
	if err != nil {
		// On failure, return original results unchanged
		return results, nil
	}

	// Parse scores
	var scores []struct {
		Index int     `json:"index"`
		Score float64 `json:"score"`
	}
	content := strings.TrimSpace(resp.Content)
	// Strip markdown code fences if present
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	if err := json.Unmarshal([]byte(content), &scores); err != nil {
		return results, nil
	}

	// Apply new scores
	scoreMap := make(map[int]float64)
	for _, s := range scores {
		scoreMap[s.Index] = s.Score
	}

	reranked := make([]vectorstore.SearchResult, len(results))
	copy(reranked, results)
	for i := range reranked {
		if score, ok := scoreMap[i]; ok {
			reranked[i].Score = score
		}
	}

	sort.Slice(reranked, func(i, j int) bool {
		return reranked[i].Score > reranked[j].Score
	})

	return reranked, nil
}

// CrossEncoderReranker uses a cross-encoder scoring model via the LLM gateway.
// Scores each (query, document) pair independently for higher accuracy.
type CrossEncoderReranker struct {
	gateway llm.Gateway
	model   string
}

func NewCrossEncoderReranker(gw llm.Gateway, model string) *CrossEncoderReranker {
	return &CrossEncoderReranker{gateway: gw, model: model}
}

func (r *CrossEncoderReranker) Rerank(ctx context.Context, query string, results []vectorstore.SearchResult) ([]vectorstore.SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}

	reranked := make([]vectorstore.SearchResult, len(results))
	copy(reranked, results)

	for i, res := range reranked {
		resp, err := r.gateway.Chat(ctx, llm.ChatRequest{
			Model: r.model,
			Messages: []llm.Message{
				{
					Role:    "system",
					Content: "Rate the relevance of the document to the query on a scale of 0.0 to 1.0. Reply with ONLY the number.",
				},
				{
					Role:    "user",
					Content: fmt.Sprintf("Query: %s\n\nDocument: %s", query, truncate(res.Content, 1000)),
				},
			},
			Temperature: 0,
		})
		if err != nil {
			continue
		}

		score, err := strconv.ParseFloat(strings.TrimSpace(resp.Content), 64)
		if err == nil && score >= 0 && score <= 1 {
			reranked[i].Score = score
		}
	}

	sort.Slice(reranked, func(i, j int) bool {
		return reranked[i].Score > reranked[j].Score
	})

	return reranked, nil
}
