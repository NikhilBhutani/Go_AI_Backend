package chunker

import (
	"strings"
	"unicode/utf8"
)

type Chunker interface {
	Chunk(text string, opts ChunkOptions) []TextChunk
}

type ChunkOptions struct {
	ChunkSize    int    // target chunk size in characters
	ChunkOverlap int    // overlap between chunks
	Strategy     string // "fixed", "recursive", "sentence"
}

type TextChunk struct {
	Content string
	Index   int
	Start   int // character offset
	End     int
}

func DefaultOptions() ChunkOptions {
	return ChunkOptions{
		ChunkSize:    1000,
		ChunkOverlap: 200,
		Strategy:     "recursive",
	}
}

type defaultChunker struct{}

func New() Chunker {
	return &defaultChunker{}
}

func (c *defaultChunker) Chunk(text string, opts ChunkOptions) []TextChunk {
	if opts.ChunkSize <= 0 {
		opts.ChunkSize = 1000
	}
	if opts.ChunkOverlap < 0 {
		opts.ChunkOverlap = 0
	}

	switch opts.Strategy {
	case "sentence":
		return chunkBySentence(text, opts)
	case "fixed":
		return chunkFixed(text, opts)
	default:
		return chunkRecursive(text, opts)
	}
}

func chunkFixed(text string, opts ChunkOptions) []TextChunk {
	var chunks []TextChunk
	runes := []rune(text)
	idx := 0

	for start := 0; start < len(runes); {
		end := start + opts.ChunkSize
		if end > len(runes) {
			end = len(runes)
		}

		content := string(runes[start:end])
		if strings.TrimSpace(content) != "" {
			chunks = append(chunks, TextChunk{
				Content: content,
				Index:   idx,
				Start:   start,
				End:     end,
			})
			idx++
		}

		step := opts.ChunkSize - opts.ChunkOverlap
		if step <= 0 {
			step = opts.ChunkSize
		}
		start += step
	}

	return chunks
}

func chunkRecursive(text string, opts ChunkOptions) []TextChunk {
	separators := []string{"\n\n", "\n", ". ", " "}

	var chunks []TextChunk
	idx := 0

	parts := splitRecursive(text, separators, opts.ChunkSize)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		byteOffset := strings.Index(text, part)
		chunks = append(chunks, TextChunk{
			Content: part,
			Index:   idx,
			Start:   byteOffset,
			End:     byteOffset + utf8.RuneCountInString(part),
		})
		idx++
	}

	return chunks
}

func splitRecursive(text string, separators []string, chunkSize int) []string {
	if utf8.RuneCountInString(text) <= chunkSize {
		return []string{text}
	}

	if len(separators) == 0 {
		// Fall back to fixed splitting
		var result []string
		runes := []rune(text)
		for i := 0; i < len(runes); i += chunkSize {
			end := i + chunkSize
			if end > len(runes) {
				end = len(runes)
			}
			result = append(result, string(runes[i:end]))
		}
		return result
	}

	sep := separators[0]
	parts := strings.Split(text, sep)
	var result []string
	var current strings.Builder

	for _, part := range parts {
		if current.Len() > 0 && utf8.RuneCountInString(current.String()+sep+part) > chunkSize {
			result = append(result, splitRecursive(current.String(), separators[1:], chunkSize)...)
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteString(sep)
		}
		current.WriteString(part)
	}

	if current.Len() > 0 {
		result = append(result, splitRecursive(current.String(), separators[1:], chunkSize)...)
	}

	return result
}

func chunkBySentence(text string, opts ChunkOptions) []TextChunk {
	// Split by sentence-ending punctuation
	sentences := splitSentences(text)

	var chunks []TextChunk
	var current strings.Builder
	idx := 0

	for _, s := range sentences {
		if current.Len() > 0 && utf8.RuneCountInString(current.String()+s) > opts.ChunkSize {
			chunks = append(chunks, TextChunk{
				Content: strings.TrimSpace(current.String()),
				Index:   idx,
			})
			idx++
			current.Reset()
		}
		current.WriteString(s)
	}

	if current.Len() > 0 {
		chunks = append(chunks, TextChunk{
			Content: strings.TrimSpace(current.String()),
			Index:   idx,
		})
	}

	return chunks
}

func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	for i, r := range text {
		current.WriteRune(r)
		if (r == '.' || r == '!' || r == '?') && i+1 < len(text) && text[i+1] == ' ' {
			sentences = append(sentences, current.String())
			current.Reset()
		}
	}

	if current.Len() > 0 {
		sentences = append(sentences, current.String())
	}

	return sentences
}
