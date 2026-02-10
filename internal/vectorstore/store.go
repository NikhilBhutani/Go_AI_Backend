package vectorstore

import (
	"context"

	"github.com/google/uuid"
)

type Chunk struct {
	ID         uuid.UUID
	DocumentID uuid.UUID
	TenantID   uuid.UUID
	ChunkIndex int
	Content    string
	Embedding  []float32
	TokenCount int
	Metadata   map[string]interface{}
}

type SearchOptions struct {
	TenantID uuid.UUID
	TopK     int
	MinScore float64
}

type SearchResult struct {
	ChunkID    uuid.UUID              `json:"chunk_id"`
	DocumentID uuid.UUID              `json:"document_id"`
	Content    string                 `json:"content"`
	Score      float64                `json:"score"`
	ChunkIndex int                    `json:"chunk_index"`
	Metadata   map[string]interface{} `json:"metadata"`
}

type DeleteFilter struct {
	TenantID   uuid.UUID
	DocumentID uuid.UUID
}

type VectorStore interface {
	Upsert(ctx context.Context, chunks []Chunk) error
	SimilaritySearch(ctx context.Context, query []float32, opts SearchOptions) ([]SearchResult, error)
	HybridSearch(ctx context.Context, query string, queryVec []float32, opts SearchOptions) ([]SearchResult, error)
	Delete(ctx context.Context, filter DeleteFilter) error
}
