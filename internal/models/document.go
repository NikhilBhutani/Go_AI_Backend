package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Document struct {
	ID            uuid.UUID       `json:"id" db:"id"`
	TenantID      uuid.UUID       `json:"tenant_id" db:"tenant_id"`
	Title         string          `json:"title" db:"title"`
	FilePath      string          `json:"file_path,omitempty" db:"file_path"`
	FileType      string          `json:"file_type,omitempty" db:"file_type"`
	FileSizeBytes int64           `json:"file_size_bytes,omitempty" db:"file_size_bytes"`
	Status        string          `json:"status" db:"status"`
	Metadata      json.RawMessage `json:"metadata" db:"metadata"`
	CreatedBy     *uuid.UUID      `json:"created_by,omitempty" db:"created_by"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
}

type DocumentChunk struct {
	ID         uuid.UUID       `json:"id" db:"id"`
	DocumentID uuid.UUID       `json:"document_id" db:"document_id"`
	TenantID   uuid.UUID       `json:"tenant_id" db:"tenant_id"`
	ChunkIndex int             `json:"chunk_index" db:"chunk_index"`
	Content    string          `json:"content" db:"content"`
	Embedding  []float32       `json:"-" db:"embedding"`
	TokenCount int             `json:"token_count" db:"token_count"`
	Metadata   json.RawMessage `json:"metadata" db:"metadata"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
}

const (
	DocStatusPending    = "pending"
	DocStatusProcessing = "processing"
	DocStatusReady      = "ready"
	DocStatusFailed     = "failed"
)
