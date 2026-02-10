package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/nikhilbhutani/backendwithai/internal/document"
	"github.com/nikhilbhutani/backendwithai/internal/models"
	"github.com/nikhilbhutani/backendwithai/internal/queue"
	"github.com/nikhilbhutani/backendwithai/internal/rag"
	"github.com/nikhilbhutani/backendwithai/pkg/chunker"
)

type EmbeddingWorker struct {
	docSvc   *document.Service
	pipeline rag.Pipeline
}

func NewEmbeddingWorker(docSvc *document.Service, pipeline rag.Pipeline) *EmbeddingWorker {
	return &EmbeddingWorker{
		docSvc:   docSvc,
		pipeline: pipeline,
	}
}

func (w *EmbeddingWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload queue.EmbeddingGeneratePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	docID, err := uuid.Parse(payload.DocumentID)
	if err != nil {
		return fmt.Errorf("parse document ID: %w", err)
	}

	tenantID, err := uuid.Parse(payload.TenantID)
	if err != nil {
		return fmt.Errorf("parse tenant ID: %w", err)
	}

	slog.Info("generating embeddings", "document_id", docID)

	// For now, the content would need to come from somewhere (e.g., stored extracted text).
	// In production, you'd store the extracted content or re-extract it.
	// This is the ingestion step into the RAG pipeline.
	err = w.pipeline.Ingest(ctx, rag.IngestRequest{
		DocumentID: docID,
		TenantID:   tenantID,
		Content:    "", // Would come from extracted text storage
		ChunkOpts:  chunker.DefaultOptions(),
	})
	if err != nil {
		w.docSvc.UpdateStatus(ctx, docID, models.DocStatusFailed)
		return fmt.Errorf("ingest into RAG: %w", err)
	}

	if err := w.docSvc.UpdateStatus(ctx, docID, models.DocStatusReady); err != nil {
		return fmt.Errorf("update status to ready: %w", err)
	}

	slog.Info("embeddings generated", "document_id", docID)
	return nil
}
