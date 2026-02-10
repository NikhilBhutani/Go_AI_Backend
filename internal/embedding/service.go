package embedding

import (
	"context"
	"fmt"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
)

type Service struct {
	gateway llm.Gateway
	model   string
}

func NewService(gw llm.Gateway, model string) *Service {
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &Service{gateway: gw, model: model}
}

func (s *Service) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Batch in groups of 100 for API limits
	const batchSize = 100
	var allEmbeddings [][]float32

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		resp, err := s.gateway.Embed(ctx, llm.EmbeddingRequest{
			Model: s.model,
			Input: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("embed batch %d: %w", i/batchSize, err)
		}

		allEmbeddings = append(allEmbeddings, resp.Embeddings...)
	}

	return allEmbeddings, nil
}

func (s *Service) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := s.Embed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}
