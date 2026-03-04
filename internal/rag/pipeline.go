package rag

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nikhilbhutani/backendwithai/internal/embedding"
	"github.com/nikhilbhutani/backendwithai/internal/llm"
	"github.com/nikhilbhutani/backendwithai/internal/rag/indexing"
	"github.com/nikhilbhutani/backendwithai/internal/vectorstore"
	"github.com/nikhilbhutani/backendwithai/pkg/chunker"
)

type Pipeline interface {
	Ingest(ctx context.Context, req IngestRequest) error
	Query(ctx context.Context, req QueryRequest) (*QueryResponse, error)
	Search(ctx context.Context, req SearchRequest) ([]vectorstore.SearchResult, error)
}

// IndexType controls which indexing strategy to use during ingestion.
const (
	IndexTypeStandard = "standard"
	IndexTypeRaptor   = "raptor"
	IndexTypeMultiRep = "multi_rep"
)

type IngestRequest struct {
	DocumentID uuid.UUID
	TenantID   uuid.UUID
	Content    string
	ChunkOpts  chunker.ChunkOptions
	// IndexType selects the indexing strategy: "standard" (default), "raptor", "multi_rep".
	IndexType string
}

type QueryRequest struct {
	Query        string  `json:"query"`
	TopK         int     `json:"top_k,omitempty"`
	MinScore     float64 `json:"min_score,omitempty"`
	Hybrid       bool    `json:"hybrid,omitempty"`
	Model        string  `json:"model,omitempty"`
	Provider     string  `json:"provider,omitempty"`
	Rerank       bool    `json:"rerank,omitempty"`        // enable LLM reranking
	QueryRewrite bool    `json:"query_rewrite,omitempty"` // enable multi-query rewriting
	UseHyDE      bool    `json:"use_hyde,omitempty"`      // enable HyDE
	// Strategy overrides automatic routing: "simple", "complex", "comparison".
	Strategy string `json:"strategy,omitempty"`
}

type QueryResponse struct {
	Answer      string     `json:"answer"`
	Citations   []Citation `json:"citations"`
	Model       string     `json:"model"`
	Tokens      int        `json:"tokens"`
	RoutingInfo RouteInfo  `json:"routing_info,omitempty"`
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
	store          vectorstore.VectorStore
	embedSvc       *embedding.Service
	retriever      *Retriever
	generator      *Generator
	reranker       Reranker
	queryRewriter  QueryRewriter
	hyde           *HyDE
	router         QueryRouter
	decomposer     Decomposer
	raptorIndexer  *indexing.RaptorIndexer
	multiRepIndexer *indexing.MultiRepIndexer
}

// PipelineOptions allows optional component injection.
type PipelineOptions struct {
	Router          QueryRouter
	Decomposer      Decomposer
	RaptorIndexer   *indexing.RaptorIndexer
	MultiRepIndexer *indexing.MultiRepIndexer
}

func NewPipeline(store vectorstore.VectorStore, embedSvc *embedding.Service, gw llm.Gateway) Pipeline {
	return NewPipelineWithOptions(store, embedSvc, gw, PipelineOptions{})
}

// NewPipelineWithOptions creates a pipeline with optional advanced components.
func NewPipelineWithOptions(store vectorstore.VectorStore, embedSvc *embedding.Service, gw llm.Gateway, opts PipelineOptions) Pipeline {
	router := opts.Router
	if router == nil {
		router = NewLLMQueryRouter(gw, "")
	}
	decomposer := opts.Decomposer
	if decomposer == nil {
		decomposer = NewLLMDecomposer(gw, "")
	}
	raptorIndexer := opts.RaptorIndexer
	if raptorIndexer == nil {
		raptorIndexer = indexing.NewRaptorIndexer(store, embedSvc, gw, "")
	}
	multiRepIndexer := opts.MultiRepIndexer
	if multiRepIndexer == nil {
		multiRepIndexer = indexing.NewMultiRepIndexer(store, embedSvc, gw, "")
	}

	return &pipeline{
		store:           store,
		embedSvc:        embedSvc,
		retriever:       NewRetriever(store, embedSvc),
		generator:       NewGenerator(gw),
		reranker:        NewLLMReranker(gw, ""),
		queryRewriter:   NewLLMQueryRewriter(gw, ""),
		hyde:            NewHyDE(gw, ""),
		router:          router,
		decomposer:      decomposer,
		raptorIndexer:   raptorIndexer,
		multiRepIndexer: multiRepIndexer,
	}
}

func (p *pipeline) Ingest(ctx context.Context, req IngestRequest) error {
	opts := req.ChunkOpts
	if opts.ChunkSize == 0 {
		opts = chunker.DefaultOptions()
	}

	chunkResults := ChunkTextWithEmbeddings(ctx, req.Content, opts, p.embedSvc)
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

	switch req.IndexType {
	case IndexTypeRaptor:
		return p.raptorIndexer.Index(ctx, chunks)
	case IndexTypeMultiRep:
		return p.multiRepIndexer.Index(ctx, chunks)
	default:
		if err := p.store.Upsert(ctx, chunks); err != nil {
			return fmt.Errorf("store chunks: %w", err)
		}
	}

	return nil
}

func (p *pipeline) Query(ctx context.Context, req QueryRequest) (*QueryResponse, error) {
	if req.TopK <= 0 {
		req.TopK = 5
	}

	tenantID := getTenantIDFromContext(ctx)
	retrieveOpts := RetrieveOptions{
		TenantID: tenantID,
		TopK:     req.TopK,
		MinScore: req.MinScore,
		Hybrid:   req.Hybrid,
	}

	// Route the query (or use caller-specified strategy).
	route, err := p.router.Route(ctx, req.Query)
	if err != nil {
		route = defaultRoute()
	}
	if req.Strategy != "" {
		route.Strategy = req.Strategy
		route.UseDecompose = req.Strategy == "complex"
		route.UseRewrite = req.Strategy == "comparison"
	}

	// Propagate per-request flags.
	if req.UseHyDE {
		route.UseHyDE = true
	}
	if req.QueryRewrite {
		route.UseRewrite = true
	}

	var results []vectorstore.SearchResult

	switch {
	case route.UseDecompose:
		results, err = p.decomposeAndRetrieve(ctx, req.Query, retrieveOpts)
	case route.UseRewrite || route.UseHyDE:
		results, err = p.retrieve(ctx, req.Query, retrieveOpts, route.UseRewrite, route.UseHyDE)
	default:
		results, err = p.retriever.Retrieve(ctx, req.Query, retrieveOpts)
	}
	if err != nil {
		return nil, fmt.Errorf("retrieve: %w", err)
	}

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
		Answer:      genResp.Answer,
		Citations:   genResp.Citations,
		Model:       genResp.Usage.Model,
		Tokens:      genResp.Usage.TotalTokens,
		RoutingInfo: routeInfoFromRoute(route),
	}, nil
}

func (p *pipeline) Search(ctx context.Context, req SearchRequest) ([]vectorstore.SearchResult, error) {
	if req.TopK <= 0 {
		req.TopK = 10
	}

	tenantID := getTenantIDFromContext(ctx)
	retrieveOpts := RetrieveOptions{
		TenantID: tenantID,
		TopK:     req.TopK,
		MinScore: req.MinScore,
		Hybrid:   req.Hybrid,
	}

	results, err := p.retrieve(ctx, req.Query, retrieveOpts, req.QueryRewrite, req.UseHyDE)
	if err != nil {
		return nil, err
	}

	if req.Rerank && len(results) > 0 {
		results, _ = p.reranker.Rerank(ctx, req.Query, results)
	}

	return results, nil
}

// retrieve handles HyDE, multi-query rewriting with RRF, and standard retrieval.
func (p *pipeline) retrieve(ctx context.Context, query string, opts RetrieveOptions, rewrite, useHyDE bool) ([]vectorstore.SearchResult, error) {
	if useHyDE {
		hypoDoc, err := p.hyde.GenerateHypothetical(ctx, query)
		if err == nil && hypoDoc != "" {
			results, err := p.retriever.Retrieve(ctx, hypoDoc, opts)
			if err == nil && len(results) > 0 {
				return results, nil
			}
		}
	}

	if rewrite {
		queries, err := p.queryRewriter.Rewrite(ctx, query)
		if err == nil && len(queries) > 1 {
			return p.multiQueryRetrieve(ctx, queries, opts)
		}
	}

	return p.retriever.Retrieve(ctx, query, opts)
}

// decomposeAndRetrieve breaks the query into sub-questions, retrieves for each,
// then merges with RRF fusion.
func (p *pipeline) decomposeAndRetrieve(ctx context.Context, query string, opts RetrieveOptions) ([]vectorstore.SearchResult, error) {
	subQuestions, err := p.decomposer.Decompose(ctx, query)
	if err != nil || len(subQuestions) == 0 {
		return p.retriever.Retrieve(ctx, query, opts)
	}

	resultSets := make([][]vectorstore.SearchResult, 0, len(subQuestions))
	for _, q := range subQuestions {
		results, err := p.retriever.Retrieve(ctx, q, opts)
		if err != nil {
			continue
		}
		resultSets = append(resultSets, results)
	}

	if len(resultSets) == 0 {
		return p.retriever.Retrieve(ctx, query, opts)
	}

	return reciprocalRankFusion(resultSets, 60, opts.TopK), nil
}

// multiQueryRetrieve runs retrieval for multiple query variants and merges with RRF.
func (p *pipeline) multiQueryRetrieve(ctx context.Context, queries []string, opts RetrieveOptions) ([]vectorstore.SearchResult, error) {
	resultSets := make([][]vectorstore.SearchResult, 0, len(queries))
	for _, q := range queries {
		results, err := p.retriever.Retrieve(ctx, q, opts)
		if err != nil {
			continue
		}
		resultSets = append(resultSets, results)
	}

	if len(resultSets) == 0 {
		return nil, nil
	}

	return reciprocalRankFusion(resultSets, 60, opts.TopK), nil
}

func getTenantIDFromContext(ctx context.Context) uuid.UUID {
	type tenantIDKey string
	if id, ok := ctx.Value(tenantIDKey("tenant_id")).(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}
