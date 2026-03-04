package rag

import (
	"sort"

	"github.com/google/uuid"
	"github.com/nikhilbhutani/backendwithai/internal/vectorstore"
)

// reciprocalRankFusion merges multiple ranked result sets using the RRF algorithm.
//
// score(d) = Σ 1 / (k + rank_i(d))   where k = 60 (standard constant)
//
// resultSets — one slice per query variant, each already ranked best-first.
// k          — RRF constant (pass 60 for the standard value).
// topK       — maximum number of results to return.
func reciprocalRankFusion(resultSets [][]vectorstore.SearchResult, k int, topK int) []vectorstore.SearchResult {
	if k <= 0 {
		k = 60
	}

	type entry struct {
		result vectorstore.SearchResult
		score  float64
	}

	scores := make(map[uuid.UUID]*entry)
	order := make([]uuid.UUID, 0) // track insertion order for stable output

	for _, results := range resultSets {
		for rank, r := range results {
			rrfScore := 1.0 / float64(k+rank+1)
			if e, exists := scores[r.ChunkID]; exists {
				e.score += rrfScore
				// Keep the highest original similarity score for tie-breaking / display
				if r.Score > e.result.Score {
					e.result.Score = r.Score
				}
			} else {
				cp := r // copy
				scores[r.ChunkID] = &entry{result: cp, score: rrfScore}
				order = append(order, r.ChunkID)
			}
		}
	}

	// Collect into a slice and sort by descending RRF score.
	merged := make([]vectorstore.SearchResult, 0, len(scores))
	for _, id := range order {
		e := scores[id]
		e.result.Score = e.score // replace score with fused RRF score
		merged = append(merged, e.result)
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})

	if topK > 0 && len(merged) > topK {
		merged = merged[:topK]
	}
	return merged
}
