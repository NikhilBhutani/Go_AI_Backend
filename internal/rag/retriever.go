package rag

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nikhilbhutani/backendwithai/internal/embedding"
	"github.com/nikhilbhutani/backendwithai/internal/vectorstore"
)

type Retriever struct {
	store    vectorstore.VectorStore
	embedSvc *embedding.Service
}

func NewRetriever(store vectorstore.VectorStore, embedSvc *embedding.Service) *Retriever {
	return &Retriever{store: store, embedSvc: embedSvc}
}

type RetrieveOptions struct {
	TenantID uuid.UUID
	TopK     int
	MinScore float64
	Hybrid   bool // use hybrid search (vector + keyword)
}

func (r *Retriever) Retrieve(ctx context.Context, query string, opts RetrieveOptions) ([]vectorstore.SearchResult, error) {
	queryVec, err := r.embedSvc.EmbedSingle(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	searchOpts := vectorstore.SearchOptions{
		TenantID: opts.TenantID,
		TopK:     opts.TopK,
		MinScore: opts.MinScore,
	}

	if opts.Hybrid {
		return r.store.HybridSearch(ctx, query, queryVec, searchOpts)
	}
	return r.store.SimilaritySearch(ctx, queryVec, searchOpts)
}
