package vectorstore

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

type PgVectorStore struct {
	db *pgxpool.Pool
}

func NewPgVectorStore(db *pgxpool.Pool) *PgVectorStore {
	return &PgVectorStore{db: db}
}

func (s *PgVectorStore) Upsert(ctx context.Context, chunks []Chunk) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, c := range chunks {
		id := c.ID
		if id == uuid.Nil {
			id = uuid.New()
		}

		embedding := pgvector.NewVector(c.Embedding)

		_, err := tx.Exec(ctx,
			`INSERT INTO document_chunks (id, document_id, tenant_id, chunk_index, content, embedding, token_count, metadata)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 ON CONFLICT (id) DO UPDATE SET content = $5, embedding = $6, token_count = $7, metadata = $8`,
			id, c.DocumentID, c.TenantID, c.ChunkIndex, c.Content, embedding, c.TokenCount, c.Metadata,
		)
		if err != nil {
			return fmt.Errorf("upsert chunk %d: %w", c.ChunkIndex, err)
		}
	}

	return tx.Commit(ctx)
}

func (s *PgVectorStore) SimilaritySearch(ctx context.Context, query []float32, opts SearchOptions) ([]SearchResult, error) {
	if opts.TopK <= 0 {
		opts.TopK = 10
	}

	embedding := pgvector.NewVector(query)

	rows, err := s.db.Query(ctx,
		`SELECT id, document_id, content, chunk_index, metadata,
		        1 - (embedding <=> $1) AS score
		 FROM document_chunks
		 WHERE tenant_id = $2
		 ORDER BY embedding <=> $1
		 LIMIT $3`,
		embedding, opts.TenantID, opts.TopK,
	)
	if err != nil {
		return nil, fmt.Errorf("similarity search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ChunkID, &r.DocumentID, &r.Content, &r.ChunkIndex, &r.Metadata, &r.Score); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		if opts.MinScore > 0 && r.Score < opts.MinScore {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

func (s *PgVectorStore) HybridSearch(ctx context.Context, query string, queryVec []float32, opts SearchOptions) ([]SearchResult, error) {
	if opts.TopK <= 0 {
		opts.TopK = 10
	}

	embedding := pgvector.NewVector(queryVec)

	// Hybrid: combine vector similarity with keyword (FTS) ranking
	rows, err := s.db.Query(ctx,
		`WITH vector_results AS (
			SELECT id, document_id, content, chunk_index, metadata,
			       1 - (embedding <=> $1) AS vector_score
			FROM document_chunks
			WHERE tenant_id = $2
			ORDER BY embedding <=> $1
			LIMIT $3 * 2
		),
		keyword_results AS (
			SELECT id, document_id, content, chunk_index, metadata,
			       ts_rank(tsv, plainto_tsquery('english', $4)) AS keyword_score
			FROM document_chunks
			WHERE tenant_id = $2 AND tsv @@ plainto_tsquery('english', $4)
			LIMIT $3 * 2
		)
		SELECT COALESCE(v.id, k.id) AS id,
		       COALESCE(v.document_id, k.document_id) AS document_id,
		       COALESCE(v.content, k.content) AS content,
		       COALESCE(v.chunk_index, k.chunk_index) AS chunk_index,
		       COALESCE(v.metadata, k.metadata) AS metadata,
		       (COALESCE(v.vector_score, 0) * 0.7 + COALESCE(k.keyword_score, 0) * 0.3) AS score
		FROM vector_results v
		FULL OUTER JOIN keyword_results k ON v.id = k.id
		ORDER BY score DESC
		LIMIT $3`,
		embedding, opts.TenantID, opts.TopK, query,
	)
	if err != nil {
		return nil, fmt.Errorf("hybrid search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ChunkID, &r.DocumentID, &r.Content, &r.ChunkIndex, &r.Metadata, &r.Score); err != nil {
			return nil, fmt.Errorf("scan hybrid result: %w", err)
		}
		if opts.MinScore > 0 && r.Score < opts.MinScore {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

func (s *PgVectorStore) Delete(ctx context.Context, filter DeleteFilter) error {
	if filter.DocumentID != uuid.Nil {
		_, err := s.db.Exec(ctx,
			"DELETE FROM document_chunks WHERE document_id = $1 AND tenant_id = $2",
			filter.DocumentID, filter.TenantID,
		)
		return err
	}

	_, err := s.db.Exec(ctx, "DELETE FROM document_chunks WHERE tenant_id = $1", filter.TenantID)
	return err
}
