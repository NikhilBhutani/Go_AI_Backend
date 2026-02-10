package tokenizer

import (
	"strings"
)

// CountTokens provides a rough token count estimate.
// For production, use tiktoken-go for exact counts.
func CountTokens(text string) int {
	// Rough estimate: ~4 chars per token for English
	words := strings.Fields(text)
	return max(len(words)*4/3, 1)
}

// CountTokensForModel returns estimated token count.
// TODO: integrate tiktoken-go for model-specific exact counts.
func CountTokensForModel(text, model string) int {
	_ = model // model-specific counting would use tiktoken
	return CountTokens(text)
}
