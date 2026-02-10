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
	"github.com/nikhilbhutani/backendwithai/internal/storage"
)

type DocumentWorker struct {
	docSvc    *document.Service
	storage   storage.Storage
	bucket    string
	extractor document.TextExtractor
	queueClient *queue.Client
}

func NewDocumentWorker(docSvc *document.Service, store storage.Storage, bucket string, qc *queue.Client) *DocumentWorker {
	return &DocumentWorker{
		docSvc:      docSvc,
		storage:     store,
		bucket:      bucket,
		extractor:   document.NewTextExtractor(),
		queueClient: qc,
	}
}

func (w *DocumentWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload queue.DocumentProcessPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	docID, err := uuid.Parse(payload.DocumentID)
	if err != nil {
		return fmt.Errorf("parse document ID: %w", err)
	}

	slog.Info("processing document", "document_id", docID)

	if err := w.docSvc.UpdateStatus(ctx, docID, models.DocStatusProcessing); err != nil {
		return fmt.Errorf("update status to processing: %w", err)
	}

	doc, err := w.docSvc.GetByID(ctx, docID)
	if err != nil {
		w.docSvc.UpdateStatus(ctx, docID, models.DocStatusFailed)
		return fmt.Errorf("get document: %w", err)
	}

	// Download file from storage
	reader, err := w.storage.Download(ctx, w.bucket, doc.FilePath)
	if err != nil {
		w.docSvc.UpdateStatus(ctx, docID, models.DocStatusFailed)
		return fmt.Errorf("download file: %w", err)
	}
	defer reader.Close()

	data, err := readAll(reader)
	if err != nil {
		w.docSvc.UpdateStatus(ctx, docID, models.DocStatusFailed)
		return fmt.Errorf("read file: %w", err)
	}

	// Extract text
	readerAt := document.ReaderAtFromBytes(data)
	extracted, err := w.extractor.Extract(ctx, readerAt, int64(len(data)), doc.FileType)
	if err != nil {
		w.docSvc.UpdateStatus(ctx, docID, models.DocStatusFailed)
		return fmt.Errorf("extract text: %w", err)
	}

	_ = extracted

	// Enqueue embedding generation
	if err := w.queueClient.EnqueueEmbeddingGenerate(queue.EmbeddingGeneratePayload{
		DocumentID: docID.String(),
		TenantID:   payload.TenantID,
	}); err != nil {
		slog.Error("failed to enqueue embedding", "error", err)
	}

	slog.Info("document processed", "document_id", docID)
	return nil
}

func readAll(r interface{ Read([]byte) (int, error) }) ([]byte, error) {
	var data []byte
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return data, err
		}
	}
	return data, nil
}
