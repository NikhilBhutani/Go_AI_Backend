package rag

import (
	"context"
	"fmt"

	"github.com/nikhilbhutani/backendwithai/internal/embedding"
	"github.com/nikhilbhutani/backendwithai/pkg/chunker"
	"github.com/nikhilbhutani/backendwithai/pkg/tokenizer"
)

type ChunkResult struct {
	Content    string
	Index      int
	TokenCount int
}

// ChunkText splits text using the configured strategy.
// For the "semantic" strategy an embedding service is required; pass nil to
// fall back to recursive chunking.
func ChunkText(text string, opts chunker.ChunkOptions) []ChunkResult {
	return ChunkTextWithEmbeddings(context.Background(), text, opts, nil)
}

// ChunkTextWithEmbeddings splits text, using embedSvc for semantic chunking.
func ChunkTextWithEmbeddings(ctx context.Context, text string, opts chunker.ChunkOptions, embedSvc *embedding.Service) []ChunkResult {
	if opts.ChunkSize == 0 {
		opts = chunker.DefaultOptions()
	}

	if opts.Strategy == "semantic" {
		if embedSvc == nil {
			// Fallback: semantic requires embeddings; use recursive instead.
			opts.Strategy = "recursive"
		} else {
			return semanticChunk(ctx, text, opts, embedSvc)
		}
	}

	c := chunker.New()
	chunks := c.Chunk(text, opts)

	results := make([]ChunkResult, len(chunks))
	for i, ch := range chunks {
		results[i] = ChunkResult{
			Content:    ch.Content,
			Index:      ch.Index,
			TokenCount: tokenizer.CountTokens(ch.Content),
		}
	}
	return results
}

func semanticChunk(ctx context.Context, text string, opts chunker.ChunkOptions, embedSvc *embedding.Service) []ChunkResult {
	embedFn := func(ctx context.Context, texts []string) ([][]float32, error) {
		return embedSvc.Embed(ctx, texts)
	}

	// threshold: 0.8, bufferSize: 3 (defaults inside SemanticChunker)
	sc := chunker.NewSemanticChunker(embedFn, 0, 0)
	chunks, err := sc.ChunkWithContext(ctx, text)
	if err != nil {
		// Fallback to recursive on error.
		opts.Strategy = "recursive"
		return ChunkText(text, opts)
	}

	if len(chunks) == 0 {
		return nil
	}

	results := make([]ChunkResult, len(chunks))
	for i, ch := range chunks {
		results[i] = ChunkResult{
			Content:    ch.Content,
			Index:      ch.Index,
			TokenCount: tokenizer.CountTokens(ch.Content),
		}
	}
	return results
}

// chunkTextError is a sentinel used in tests.
var chunkTextError = fmt.Errorf("chunking error")
