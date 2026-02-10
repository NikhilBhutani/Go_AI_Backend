package rag

import (
	"github.com/nikhilbhutani/backendwithai/pkg/chunker"
	"github.com/nikhilbhutani/backendwithai/pkg/tokenizer"
)

type ChunkResult struct {
	Content    string
	Index      int
	TokenCount int
}

func ChunkText(text string, opts chunker.ChunkOptions) []ChunkResult {
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
