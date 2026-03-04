package chunker

import (
	"context"
	"math"
)

// EmbedFunc is the embedding function signature used by SemanticChunker.
// It accepts a slice of texts and returns a slice of float32 embedding vectors.
type EmbedFunc func(ctx context.Context, texts []string) ([][]float32, error)

// SemanticChunker splits text at topic boundaries detected by a drop in cosine
// similarity between adjacent sentence embeddings.
type SemanticChunker struct {
	embedFn    EmbedFunc
	threshold  float64 // similarity drop threshold; default 0.8
	bufferSize int     // sentences to group per window; default 3
}

// NewSemanticChunker creates a SemanticChunker.
// threshold: cosine-similarity threshold below which a split is inserted (0–1, default 0.8).
// bufferSize: number of adjacent sentences to combine into one window before embedding (default 3).
func NewSemanticChunker(embedFn EmbedFunc, threshold float64, bufferSize int) *SemanticChunker {
	if threshold <= 0 {
		threshold = 0.8
	}
	if bufferSize <= 0 {
		bufferSize = 3
	}
	return &SemanticChunker{embedFn: embedFn, threshold: threshold, bufferSize: bufferSize}
}

// ChunkWithContext splits text semantically, returning TextChunks.
// It requires a context because it makes embedding calls.
func (sc *SemanticChunker) ChunkWithContext(ctx context.Context, text string) ([]TextChunk, error) {
	sentences := splitSentences(text)
	if len(sentences) == 0 {
		return nil, nil
	}
	if len(sentences) == 1 {
		return []TextChunk{{Content: sentences[0], Index: 0}}, nil
	}

	// Build windows: each window is a group of bufferSize adjacent sentences.
	windows := buildWindows(sentences, sc.bufferSize)

	// Embed all windows in one batch.
	embeddings, err := sc.embedFn(ctx, windows)
	if err != nil {
		return nil, err
	}

	// Find split points where cosine similarity drops below threshold.
	splits := []int{0} // start of first chunk
	for i := 0; i < len(embeddings)-1; i++ {
		sim := cosineSimilarity(embeddings[i], embeddings[i+1])
		if sim < sc.threshold {
			splits = append(splits, i+1)
		}
	}
	splits = append(splits, len(sentences)) // end sentinel

	// Build output chunks.
	chunks := make([]TextChunk, 0, len(splits)-1)
	for i := 0; i < len(splits)-1; i++ {
		start := splits[i]
		end := splits[i+1]
		content := joinSentences(sentences[start:end])
		if content == "" {
			continue
		}
		chunks = append(chunks, TextChunk{
			Content: content,
			Index:   i,
		})
	}
	return chunks, nil
}

// buildWindows creates one string per sentence where each string is the
// concatenation of up to bufferSize sentences centered on that sentence.
func buildWindows(sentences []string, bufferSize int) []string {
	n := len(sentences)
	windows := make([]string, n)
	half := bufferSize / 2
	for i := range sentences {
		start := i - half
		if start < 0 {
			start = 0
		}
		end := i + half + 1
		if end > n {
			end = n
		}
		windows[i] = joinSentences(sentences[start:end])
	}
	return windows
}

func joinSentences(ss []string) string {
	result := ""
	for _, s := range ss {
		result += s
	}
	return result
}

func cosineSimilarity(a, b []float32) float64 {
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
