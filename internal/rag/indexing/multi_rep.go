package indexing

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/nikhilbhutani/backendwithai/internal/embedding"
	"github.com/nikhilbhutani/backendwithai/internal/llm"
	"github.com/nikhilbhutani/backendwithai/internal/vectorstore"
)

// MultiRepIndexer implements multi-representation indexing:
//   - Generates a concise LLM summary for each chunk.
//   - Embeds the summary (for more precise semantic matching during search).
//   - Stores the full original content so the LLM gets rich context at generation time.
type MultiRepIndexer struct {
	store    vectorstore.VectorStore
	embedSvc *embedding.Service
	gateway  llm.Gateway
	model    string
}

func NewMultiRepIndexer(store vectorstore.VectorStore, embedSvc *embedding.Service, gw llm.Gateway, model string) *MultiRepIndexer {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &MultiRepIndexer{
		store:    store,
		embedSvc: embedSvc,
		gateway:  gw,
		model:    model,
	}
}

// Index generates summaries for each chunk, embeds the summaries, but stores
// the original full content for retrieval. Chunks are tagged with
// chunk_type = "multi_rep" in metadata.
func (m *MultiRepIndexer) Index(ctx context.Context, chunks []vectorstore.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	summaries, err := m.generateSummaries(ctx, chunks)
	if err != nil {
		return fmt.Errorf("multi_rep: generate summaries: %w", err)
	}

	embeddings, err := m.embedSvc.Embed(ctx, summaries)
	if err != nil {
		return fmt.Errorf("multi_rep: embed summaries: %w", err)
	}

	indexed := make([]vectorstore.Chunk, len(chunks))
	for i, c := range chunks {
		meta := copyMeta(c.Metadata)
		meta["chunk_type"] = "multi_rep"
		meta["summary"] = summaries[i]

		indexed[i] = vectorstore.Chunk{
			ID:         orNewUUID(c.ID),
			DocumentID: c.DocumentID,
			TenantID:   c.TenantID,
			ChunkIndex: c.ChunkIndex,
			Content:    c.Content,   // full original content
			Embedding:  embeddings[i], // summary embedding
			TokenCount: c.TokenCount,
			Metadata:   meta,
		}
	}

	if err := m.store.Upsert(ctx, indexed); err != nil {
		return fmt.Errorf("multi_rep: store chunks: %w", err)
	}
	return nil
}

// generateSummaries calls the LLM for each chunk in sequence.
// A future optimisation could batch these.
func (m *MultiRepIndexer) generateSummaries(ctx context.Context, chunks []vectorstore.Chunk) ([]string, error) {
	summaries := make([]string, len(chunks))
	for i, c := range chunks {
		resp, err := m.gateway.Chat(ctx, llm.ChatRequest{
			Model: m.model,
			Messages: []llm.Message{
				{
					Role: "system",
					Content: "Summarise the following passage in 1-2 sentences. " +
						"Capture the most important fact or concept. Be specific.",
				},
				{Role: "user", Content: c.Content},
			},
			Temperature: 0,
		})
		if err != nil {
			// Fallback: use first 200 chars as summary.
			content := c.Content
			if len(content) > 200 {
				content = content[:200]
			}
			summaries[i] = strings.TrimSpace(content)
			continue
		}
		summaries[i] = strings.TrimSpace(resp.Content)
	}
	return summaries, nil
}

func copyMeta(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m)+2)
	for k, v := range m {
		out[k] = v
	}
	return out
}

func orNewUUID(id uuid.UUID) uuid.UUID {
	if id == uuid.Nil {
		return uuid.New()
	}
	return id
}
