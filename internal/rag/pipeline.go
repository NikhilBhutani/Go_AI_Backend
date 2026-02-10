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
	Query        string `json:"query"`
	TopK         int    `json:"top_k,omitempty"`
	MinScore     float64 `json:"min_score,omitempty"`
	Hybrid       bool   `json:"hybrid,omitempty"`
	Model        string `json:"model,omitempty"`
	Provider     string `json:"provider,omitempty"`
	Rerank       bool   `json:"rerank,omitempty"`        // enable LLM reranking
	QueryRewrite bool   `json:"query_rewrite,omitempty"` // enable multi-query rewriting
	UseHyDE      bool   `json:"use_hyde,omitempty"`      // enable HyDE
}

type QueryResponse struct {
	Answer    string     `json:"answer"`
	Citations []Citation `json:"citations"`
	Model     string     `json:"model"`
	Tokens    int        `json:"tokens"`
}

type SearchRequest struct {
	Query        string  `json:"query"`
	TopK         int     `json:"top_k,omitempty"`
	MinScore     float64 `json:"min_score,omitempty"`
	Hybrid       bool    `json:"hybrid,omitempty"`
	Rerank       bool    `json:"rerank,omitempty"`
	QueryRewrite bool    `json:"query_rewrite,omitempty"`
	UseHyDE      bool    `json:"use_hyde,omitempty"`
}

type pipeline struct {
	store         vectorstore.VectorStore
	embedSvc      *embedding.Service
	retriever     *Retriever
	generator     *Generator
	reranker      Reranker
	queryRewriter QueryRewriter
	hyde          *HyDE
}

func NewPipeline(store vectorstore.VectorStore, embedSvc *embedding.Service, gw llm.Gateway) Pipeline {
	return &pipeline{
		store:         store,
		embedSvc:      embedSvc,
		retriever:     NewRetriever(store, embedSvc),
		generator:     NewGenerator(gw),
		reranker:      NewLLMReranker(gw, ""),
		queryRewriter: NewLLMQueryRewriter(gw, ""),
		hyde:          NewHyDE(gw, ""),
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

	results, err := p.retrieve(ctx, req.Query, RetrieveOptions{
		TenantID: tenantID,
		TopK:     req.TopK,
		MinScore: req.MinScore,
		Hybrid:   req.Hybrid,
	}, req.QueryRewrite, req.UseHyDE)
	if err != nil {
		return nil, fmt.Errorf("retrieve: %w", err)
	}

	// Rerank if requested
	if req.Rerank && len(results) > 0 {
		results, err = p.reranker.Rerank(ctx, req.Query, results)
		if err != nil {
			return nil, fmt.Errorf("rerank: %w", err)
		}
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

	results, err := p.retrieve(ctx, req.Query, RetrieveOptions{
		TenantID: tenantID,
		TopK:     req.TopK,
		MinScore: req.MinScore,
		Hybrid:   req.Hybrid,
	}, req.QueryRewrite, req.UseHyDE)
	if err != nil {
		return nil, err
	}

	if req.Rerank && len(results) > 0 {
		results, _ = p.reranker.Rerank(ctx, req.Query, results)
	}

	return results, nil
}

// retrieve handles query rewriting, HyDE, and multi-query retrieval.
func (p *pipeline) retrieve(ctx context.Context, query string, opts RetrieveOptions, rewrite, useHyDE bool) ([]vectorstore.SearchResult, error) {
	// HyDE: generate hypothetical document and use its embedding
	if useHyDE {
		hypoDoc, err := p.hyde.GenerateHypothetical(ctx, query)
		if err == nil && hypoDoc != "" {
			// Retrieve using the hypothetical document's embedding
			results, err := p.retriever.Retrieve(ctx, hypoDoc, opts)
			if err == nil && len(results) > 0 {
				return results, nil
			}
		}
	}

	// Multi-query rewriting: expand query into multiple versions, retrieve from all
	if rewrite {
		queries, err := p.queryRewriter.Rewrite(ctx, query)
		if err == nil && len(queries) > 1 {
			return p.multiQueryRetrieve(ctx, queries, opts)
		}
	}

	// Standard retrieval
	return p.retriever.Retrieve(ctx, query, opts)
}

// multiQueryRetrieve runs retrieval for multiple query variants and deduplicates.
func (p *pipeline) multiQueryRetrieve(ctx context.Context, queries []string, opts RetrieveOptions) ([]vectorstore.SearchResult, error) {
	seen := make(map[uuid.UUID]bool)
	var allResults []vectorstore.SearchResult

	for _, q := range queries {
		results, err := p.retriever.Retrieve(ctx, q, opts)
		if err != nil {
			continue
		}
		for _, r := range results {
			if !seen[r.ChunkID] {
				seen[r.ChunkID] = true
				allResults = append(allResults, r)
			}
		}
	}

	// Cap to TopK
	if len(allResults) > opts.TopK {
		allResults = allResults[:opts.TopK]
	}

	return allResults, nil
}

func getTenantIDFromContext(ctx context.Context) uuid.UUID {
	type tenantIDKey string
	if id, ok := ctx.Value(tenantIDKey("tenant_id")).(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}
