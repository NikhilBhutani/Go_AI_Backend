package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
)

// SummaryMemory maintains a running summary of the conversation.
// When the buffer exceeds a threshold, older messages are summarized
// and replaced with the summary, preserving context while saving tokens.
type SummaryMemory struct {
	mu              sync.RWMutex
	entries         []Entry
	summary         string
	gateway         llm.Gateway
	model           string
	summarizeAfter  int // summarize when entries exceed this count
}

func NewSummaryMemory(gw llm.Gateway, model string, summarizeAfter int) *SummaryMemory {
	if model == "" {
		model = "gpt-4o-mini"
	}
	if summarizeAfter <= 0 {
		summarizeAfter = 10
	}
	return &SummaryMemory{
		entries:        make([]Entry, 0),
		gateway:        gw,
		model:          model,
		summarizeAfter: summarizeAfter,
	}
}

func (m *SummaryMemory) Add(ctx context.Context, entry Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	m.entries = append(m.entries, entry)

	// Trigger summarization when we have too many entries
	if len(m.entries) > m.summarizeAfter {
		m.summarize(ctx)
	}
}

func (m *SummaryMemory) Get(_ context.Context, limit int) []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Entry

	// Prepend summary as a system message if available
	if m.summary != "" {
		result = append(result, Entry{
			Role:    "system",
			Content: fmt.Sprintf("Previous conversation summary: %s", m.summary),
		})
	}

	entries := m.entries
	if limit > 0 && limit < len(entries) {
		entries = entries[len(entries)-limit:]
	}

	result = append(result, entries...)
	return result
}

func (m *SummaryMemory) Clear(_ context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = m.entries[:0]
	m.summary = ""
}

func (m *SummaryMemory) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}

// GetSummary returns the current conversation summary.
func (m *SummaryMemory) GetSummary() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.summary
}

func (m *SummaryMemory) summarize(ctx context.Context) {
	// Take the older half of entries to summarize
	midpoint := len(m.entries) / 2
	toSummarize := m.entries[:midpoint]

	var sb strings.Builder
	if m.summary != "" {
		fmt.Fprintf(&sb, "Existing summary: %s\n\n", m.summary)
	}
	sb.WriteString("New messages to incorporate:\n")
	for _, e := range toSummarize {
		fmt.Fprintf(&sb, "%s: %s\n", e.Role, e.Content)
	}

	resp, err := m.gateway.Chat(ctx, llm.ChatRequest{
		Model: m.model,
		Messages: []llm.Message{
			{
				Role: "system",
				Content: `Summarize the conversation so far into a concise paragraph.
Capture the key topics discussed, decisions made, and any important context.
This summary will be used to maintain context in future messages.`,
			},
			{
				Role:    "user",
				Content: sb.String(),
			},
		},
		Temperature: 0,
		MaxTokens:   300,
	})
	if err != nil {
		return // keep entries as-is on failure
	}

	m.summary = strings.TrimSpace(resp.Content)
	m.entries = m.entries[midpoint:] // keep only recent entries
}
