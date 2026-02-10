package rag

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nikhilbhutani/backendwithai/internal/embedding"
	"github.com/nikhilbhutani/backendwithai/internal/llm"
	"github.com/nikhilbhutani/backendwithai/internal/vectorstore"
	"github.com/nikhilbhutani/backendwithai/pkg/chunker"
)

type Pipeline interface {
	Ingest(ctx context.Context, req IngestRequest) error
	Query(ctx context.Context, req QueryRequest) (*QueryResponse, error)
	Search(ctx context.Context, req SearchRequest) ([]vectorstore.SearchResult, error)
}

type IngestRequest struct {
	DocumentID uuid.UUID
	TenantID   uuid.UUID
	Content    string
	ChunkOpts  chunker.ChunkOptions
}

type QueryRequest struct {
	Query    string `json:"query"`
	TopK     int    `json:"top_k,omitempty"`
	MinScore float64 `json:"min_score,omitempty"`
	Hybrid   bool   `json:"hybrid,omitempty"`
	Model    string `json:"model,omitempty"`
	Provider string `json:"provider,omitempty"`
}

type QueryResponse struct {
	Answer    string     `json:"answer"`
	Citations []Citation `json:"citations"`
	Model     string     `json:"model"`
	Tokens    int        `json:"tokens"`
}

type SearchRequest struct {
	Query    string  `json:"query"`
	TopK     int     `json:"top_k,omitempty"`
	MinScore float64 `json:"min_score,omitempty"`
	Hybrid   bool    `json:"hybrid,omitempty"`
}

type pipeline struct {
	store     vectorstore.VectorStore
	embedSvc  *embedding.Service
	retriever *Retriever
	generator *Generator
}

func NewPipeline(store vectorstore.VectorStore, embedSvc *embedding.Service, gw llm.Gateway) Pipeline {
	return &pipeline{
		store:     store,
		embedSvc:  embedSvc,
		retriever: NewRetriever(store, embedSvc),
		generator: NewGenerator(gw),
	}
}

func (p *pipeline) Ingest(ctx context.Context, req IngestRequest) error {
	opts := req.ChunkOpts
	if opts.ChunkSize == 0 {
		opts = chunker.DefaultOptions()
	}

	chunkResults := ChunkText(req.Content, opts)
	if len(chunkResults) == 0 {
		return fmt.Errorf("no chunks generated from content")
	}

	// Extract texts for batch embedding
	texts := make([]string, len(chunkResults))
	for i, c := range chunkResults {
		texts[i] = c.Content
	}

	embeddings, err := p.embedSvc.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("generate embeddings: %w", err)
	}

	chunks := make([]vectorstore.Chunk, len(chunkResults))
	for i, cr := range chunkResults {
		chunks[i] = vectorstore.Chunk{
			ID:         uuid.New(),
			DocumentID: req.DocumentID,
			TenantID:   req.TenantID,
			ChunkIndex: cr.Index,
			Content:    cr.Content,
			Embedding:  embeddings[i],
			TokenCount: cr.TokenCount,
		}
	}

	if err := p.store.Upsert(ctx, chunks); err != nil {
		return fmt.Errorf("store chunks: %w", err)
	}

	return nil
}

func (p *pipeline) Query(ctx context.Context, req QueryRequest) (*QueryResponse, error) {
	if req.TopK <= 0 {
		req.TopK = 5
	}

	tenantID := getTenantIDFromContext(ctx)

	results, err := p.retriever.Retrieve(ctx, req.Query, RetrieveOptions{
		TenantID: tenantID,
		TopK:     req.TopK,
		MinScore: req.MinScore,
		Hybrid:   req.Hybrid,
	})
	if err != nil {
		return nil, fmt.Errorf("retrieve: %w", err)
	}

	genResp, err := p.generator.Generate(ctx, GenerateRequest{
		Query:    req.Query,
		Context:  results,
		Model:    req.Model,
		Provider: req.Provider,
	})
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}

	return &QueryResponse{
		Answer:    genResp.Answer,
		Citations: genResp.Citations,
		Model:     genResp.Usage.Model,
		Tokens:    genResp.Usage.TotalTokens,
	}, nil
}

func (p *pipeline) Search(ctx context.Context, req SearchRequest) ([]vectorstore.SearchResult, error) {
	if req.TopK <= 0 {
		req.TopK = 10
	}

	tenantID := getTenantIDFromContext(ctx)

	return p.retriever.Retrieve(ctx, req.Query, RetrieveOptions{
		TenantID: tenantID,
		TopK:     req.TopK,
		MinScore: req.MinScore,
		Hybrid:   req.Hybrid,
	})
}

// getTenantIDFromContext extracts tenant ID — uses the tenant package indirectly
// to avoid circular imports. Expects the ID to be set in context.
func getTenantIDFromContext(ctx context.Context) uuid.UUID {
	type tenantIDKey string
	if id, ok := ctx.Value(tenantIDKey("tenant_id")).(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}
