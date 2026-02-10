package memory

import (
	"fmt"
	"strings"
)

// ContextEngine manages what goes into the LLM's context window.
// It assembles system prompt + memory + RAG context + user message
// while respecting token budgets.
type ContextEngine struct {
	maxTokens       int
	systemBudget    float64 // fraction of budget for system prompt
	memoryBudget    float64 // fraction for conversation history
	retrievalBudget float64 // fraction for RAG context
}

func NewContextEngine(maxTokens int) *ContextEngine {
	return &ContextEngine{
		maxTokens:       maxTokens,
		systemBudget:    0.15,
		memoryBudget:    0.25,
		retrievalBudget: 0.40,
		// remaining 0.20 for user message + response
	}
}

// ContextParts holds the components that will be assembled into the final prompt.
type ContextParts struct {
	SystemPrompt   string
	ConversationHistory []Entry
	RetrievedContext    []string
	UserMessage    string
}

// AssembledContext is the final prompt ready for the LLM.
type AssembledContext struct {
	Messages    []Message `json:"messages"`
	TotalTokens int       `json:"total_tokens"`
	Truncated   bool      `json:"truncated"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Assemble builds the final prompt within the token budget.
func (ce *ContextEngine) Assemble(parts ContextParts) AssembledContext {
	var messages []Message
	totalTokens := 0
	truncated := false

	// 1. System prompt (always included, truncated if too long)
	systemBudget := int(float64(ce.maxTokens) * ce.systemBudget)
	systemPrompt := truncateToTokens(parts.SystemPrompt, systemBudget)
	if systemPrompt != parts.SystemPrompt {
		truncated = true
	}
	messages = append(messages, Message{Role: "system", Content: systemPrompt})
	totalTokens += estimateTokens(systemPrompt)

	// 2. Retrieved context (injected into system or as separate message)
	if len(parts.RetrievedContext) > 0 {
		retrievalBudget := int(float64(ce.maxTokens) * ce.retrievalBudget)
		contextStr := ce.assembleRetrievalContext(parts.RetrievedContext, retrievalBudget)
		if contextStr != "" {
			messages = append(messages, Message{
				Role:    "system",
				Content: fmt.Sprintf("Relevant context:\n%s", contextStr),
			})
			totalTokens += estimateTokens(contextStr)
		}
	}

	// 3. Conversation history (most recent first, within budget)
	memoryBudget := int(float64(ce.maxTokens) * ce.memoryBudget)
	historyTokens := 0
	var historyMsgs []Message
	for i := len(parts.ConversationHistory) - 1; i >= 0; i-- {
		entry := parts.ConversationHistory[i]
		entryTokens := estimateTokens(entry.Content)
		if historyTokens+entryTokens > memoryBudget {
			truncated = true
			break
		}
		historyMsgs = append([]Message{{Role: entry.Role, Content: entry.Content}}, historyMsgs...)
		historyTokens += entryTokens
	}
	messages = append(messages, historyMsgs...)
	totalTokens += historyTokens

	// 4. User message (always included)
	messages = append(messages, Message{Role: "user", Content: parts.UserMessage})
	totalTokens += estimateTokens(parts.UserMessage)

	return AssembledContext{
		Messages:    messages,
		TotalTokens: totalTokens,
		Truncated:   truncated,
	}
}

func (ce *ContextEngine) assembleRetrievalContext(chunks []string, budget int) string {
	var sb strings.Builder
	tokens := 0
	for i, chunk := range chunks {
		chunkTokens := estimateTokens(chunk)
		if tokens+chunkTokens > budget {
			break
		}
		fmt.Fprintf(&sb, "[Source %d]\n%s\n\n", i+1, chunk)
		tokens += chunkTokens
	}
	return sb.String()
}

func estimateTokens(text string) int {
	// Rough estimate: ~4 chars per token for English
	return max(len(text)/4, 1)
}

func truncateToTokens(text string, maxTokens int) string {
	maxChars := maxTokens * 4
	if len(text) <= maxChars {
		return text
	}
	return text[:maxChars]
}
