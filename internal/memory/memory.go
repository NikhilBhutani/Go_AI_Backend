package memory

import (
	"context"
	"sync"
	"time"
)

// Entry is a single item in conversation memory.
type Entry struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Memory provides conversation history storage and retrieval.
type Memory interface {
	Add(ctx context.Context, entry Entry)
	Get(ctx context.Context, limit int) []Entry
	Clear(ctx context.Context)
	Size() int
}

// BufferMemory stores the last N messages in-memory.
// Simple sliding window — most recent messages are kept.
type BufferMemory struct {
	mu      sync.RWMutex
	entries []Entry
	maxSize int
}

func NewBufferMemory(maxSize int) *BufferMemory {
	if maxSize <= 0 {
		maxSize = 50
	}
	return &BufferMemory{
		entries: make([]Entry, 0, maxSize),
		maxSize: maxSize,
	}
}

func (m *BufferMemory) Add(_ context.Context, entry Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	m.entries = append(m.entries, entry)

	// Evict oldest if over capacity
	if len(m.entries) > m.maxSize {
		m.entries = m.entries[len(m.entries)-m.maxSize:]
	}
}

func (m *BufferMemory) Get(_ context.Context, limit int) []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.entries) {
		limit = len(m.entries)
	}

	start := len(m.entries) - limit
	result := make([]Entry, limit)
	copy(result, m.entries[start:])
	return result
}

func (m *BufferMemory) Clear(_ context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = m.entries[:0]
}

func (m *BufferMemory) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}

// TokenWindowMemory keeps messages until a token budget is exceeded,
// then drops the oldest messages first.
type TokenWindowMemory struct {
	mu        sync.RWMutex
	entries   []Entry
	maxTokens int
}

func NewTokenWindowMemory(maxTokens int) *TokenWindowMemory {
	if maxTokens <= 0 {
		maxTokens = 4000
	}
	return &TokenWindowMemory{
		entries:   make([]Entry, 0),
		maxTokens: maxTokens,
	}
}

func (m *TokenWindowMemory) Add(_ context.Context, entry Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	m.entries = append(m.entries, entry)
	m.trim()
}

func (m *TokenWindowMemory) Get(_ context.Context, limit int) []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.entries) {
		limit = len(m.entries)
	}

	start := len(m.entries) - limit
	result := make([]Entry, limit)
	copy(result, m.entries[start:])
	return result
}

func (m *TokenWindowMemory) Clear(_ context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = m.entries[:0]
}

func (m *TokenWindowMemory) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}

func (m *TokenWindowMemory) trim() {
	for m.totalTokens() > m.maxTokens && len(m.entries) > 1 {
		m.entries = m.entries[1:]
	}
}

func (m *TokenWindowMemory) totalTokens() int {
	total := 0
	for _, e := range m.entries {
		// Rough estimate: ~4 chars per token
		total += len(e.Content) / 4
	}
	return total
}
