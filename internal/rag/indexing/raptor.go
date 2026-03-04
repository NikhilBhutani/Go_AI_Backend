// Package indexing provides advanced indexing strategies for the RAG pipeline.
package indexing

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
	"github.com/nikhilbhutani/backendwithai/internal/embedding"
	"github.com/nikhilbhutani/backendwithai/internal/llm"
	"github.com/nikhilbhutani/backendwithai/internal/vectorstore"
)

// RaptorIndexer implements Recursive Abstractive Processing for Tree-Organized
// Retrieval (RAPTOR). It builds a hierarchy of summaries over leaf chunks,
// storing every level in the vector store so retrieval can happen at any
// abstraction level.
type RaptorIndexer struct {
	store    vectorstore.VectorStore
	embedSvc *embedding.Service
	gateway  llm.Gateway
	model    string
	// clusterSize is the target number of leaf chunks per cluster (default 5).
	clusterSize int
}

func NewRaptorIndexer(store vectorstore.VectorStore, embedSvc *embedding.Service, gw llm.Gateway, model string) *RaptorIndexer {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &RaptorIndexer{
		store:       store,
		embedSvc:    embedSvc,
		gateway:     gw,
		model:       model,
		clusterSize: 5,
	}
}

// Index stores leaf chunks (level 0) and then iteratively builds summary
// chunks (level 1, 2, …) until only one cluster remains.
func (r *RaptorIndexer) Index(ctx context.Context, chunks []vectorstore.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Tag leaf chunks with level metadata.
	for i := range chunks {
		if chunks[i].Metadata == nil {
			chunks[i].Metadata = make(map[string]interface{})
		}
		chunks[i].Metadata["chunk_level"] = 0
		chunks[i].Metadata["chunk_type"] = "raptor"
	}

	if err := r.store.Upsert(ctx, chunks); err != nil {
		return fmt.Errorf("raptor: store level-0 chunks: %w", err)
	}

	current := chunks
	level := 1

	for len(current) > 1 {
		clusters := clusterChunks(current, r.clusterSize)
		if len(clusters) == 0 {
			break
		}

		summaryChunks, err := r.summariseClusters(ctx, clusters, level)
		if err != nil {
			return fmt.Errorf("raptor: summarise level %d: %w", level, err)
		}

		// Embed summaries.
		texts := make([]string, len(summaryChunks))
		for i, sc := range summaryChunks {
			texts[i] = sc.Content
		}
		embeddings, err := r.embedSvc.Embed(ctx, texts)
		if err != nil {
			return fmt.Errorf("raptor: embed level-%d summaries: %w", level, err)
		}
		for i := range summaryChunks {
			summaryChunks[i].Embedding = embeddings[i]
		}

		if err := r.store.Upsert(ctx, summaryChunks); err != nil {
			return fmt.Errorf("raptor: store level-%d chunks: %w", level, err)
		}

		current = summaryChunks
		level++

		// Safety: avoid infinite loops on degenerate inputs.
		if level > 10 {
			break
		}
	}

	return nil
}

// summariseClusters generates one summary chunk per cluster.
func (r *RaptorIndexer) summariseClusters(ctx context.Context, clusters [][]vectorstore.Chunk, level int) ([]vectorstore.Chunk, error) {
	// Use the first chunk's document/tenant IDs as defaults.
	var docID, tenantID uuid.UUID
	if len(clusters) > 0 && len(clusters[0]) > 0 {
		docID = clusters[0][0].DocumentID
		tenantID = clusters[0][0].TenantID
	}

	result := make([]vectorstore.Chunk, 0, len(clusters))
	for ci, cluster := range clusters {
		if len(cluster) == 0 {
			continue
		}

		// Concatenate cluster contents for summarisation.
		var sb strings.Builder
		for _, c := range cluster {
			sb.WriteString(c.Content)
			sb.WriteString("\n\n")
		}

		resp, err := r.gateway.Chat(ctx, llm.ChatRequest{
			Model: r.model,
			Messages: []llm.Message{
				{
					Role: "system",
					Content: "Summarise the following text passages into a single coherent paragraph. " +
						"Preserve key facts, entities, and relationships. Be concise.",
				},
				{Role: "user", Content: sb.String()},
			},
			Temperature: 0,
		})
		if err != nil {
			return nil, fmt.Errorf("summarise cluster %d: %w", ci, err)
		}

		meta := map[string]interface{}{
			"chunk_level": level,
			"chunk_type":  "raptor",
		}

		result = append(result, vectorstore.Chunk{
			ID:         uuid.New(),
			DocumentID: docID,
			TenantID:   tenantID,
			ChunkIndex: ci,
			Content:    strings.TrimSpace(resp.Content),
			Metadata:   meta,
		})
	}

	return result, nil
}

// clusterChunks groups chunks into sub-slices of approximately clusterSize
// using a simple cosine-similarity greedy approach.
// When embeddings are missing (zero vectors) it falls back to sequential grouping.
func clusterChunks(chunks []vectorstore.Chunk, clusterSize int) [][]vectorstore.Chunk {
	if clusterSize <= 0 {
		clusterSize = 5
	}

	// Try embedding-based clustering if vectors are present.
	if hasEmbeddings(chunks) {
		return embeddingCluster(chunks, clusterSize)
	}

	// Fallback: sequential grouping.
	var clusters [][]vectorstore.Chunk
	for i := 0; i < len(chunks); i += clusterSize {
		end := i + clusterSize
		if end > len(chunks) {
			end = len(chunks)
		}
		clusters = append(clusters, chunks[i:end])
	}
	return clusters
}

func hasEmbeddings(chunks []vectorstore.Chunk) bool {
	return len(chunks) > 0 && len(chunks[0].Embedding) > 0
}

// embeddingCluster is a simple greedy centroid-expansion clustering.
func embeddingCluster(chunks []vectorstore.Chunk, clusterSize int) [][]vectorstore.Chunk {
	assigned := make([]bool, len(chunks))
	var clusters [][]vectorstore.Chunk

	for seed := 0; seed < len(chunks); seed++ {
		if assigned[seed] {
			continue
		}
		cluster := []vectorstore.Chunk{chunks[seed]}
		assigned[seed] = true

		for len(cluster) < clusterSize {
			bestIdx := -1
			bestSim := -1.0
			seedEmb := chunks[seed].Embedding
			for i, c := range chunks {
				if assigned[i] {
					continue
				}
				sim := cosineFloat32(seedEmb, c.Embedding)
				if sim > bestSim {
					bestSim = sim
					bestIdx = i
				}
			}
			if bestIdx == -1 {
				break
			}
			cluster = append(cluster, chunks[bestIdx])
			assigned[bestIdx] = true
		}
		clusters = append(clusters, cluster)
	}
	return clusters
}

func cosineFloat32(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}
