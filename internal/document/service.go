package document

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhilbhutani/backendwithai/internal/models"
	"github.com/nikhilbhutani/backendwithai/internal/storage"
	"github.com/nikhilbhutani/backendwithai/internal/tenant"
)

type Service struct {
	db        *pgxpool.Pool
	storage   storage.Storage
	extractor TextExtractor
	bucket    string
}

func NewService(db *pgxpool.Pool, store storage.Storage, bucket string) *Service {
	return &Service{
		db:        db,
		storage:   store,
		extractor: NewTextExtractor(),
		bucket:    bucket,
	}
}

type UploadRequest struct {
	Title    string
	FileType string
	FileSize int64
	Data     io.Reader
	Metadata map[string]interface{}
}

func (s *Service) Upload(ctx context.Context, req UploadRequest) (*models.Document, error) {
	tenantID := tenant.IDFromContext(ctx)
	user := tenant.UserFromContext(ctx)

	docID := uuid.New()
	path := fmt.Sprintf("%s/%s/%s%s", tenantID, docID, time.Now().Format("20060102"), req.FileType)

	if err := s.storage.Upload(ctx, s.bucket, path, req.Data, "application/octet-stream"); err != nil {
		return nil, fmt.Errorf("upload to storage: %w", err)
	}

	metadata, _ := json.Marshal(req.Metadata)

	var userID *uuid.UUID
	if user != nil {
		userID = &user.ID
	}

	var doc models.Document
	err := s.db.QueryRow(ctx,
		`INSERT INTO documents (id, tenant_id, title, file_path, file_type, file_size_bytes, status, metadata, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, tenant_id, title, file_path, file_type, file_size_bytes, status, metadata, created_by, created_at`,
		docID, tenantID, req.Title, path, req.FileType, req.FileSize, models.DocStatusPending, metadata, userID,
	).Scan(&doc.ID, &doc.TenantID, &doc.Title, &doc.FilePath, &doc.FileType, &doc.FileSizeBytes, &doc.Status, &doc.Metadata, &doc.CreatedBy, &doc.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert document: %w", err)
	}

	return &doc, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*models.Document, error) {
	tenantID := tenant.IDFromContext(ctx)
	var doc models.Document
	err := s.db.QueryRow(ctx,
		`SELECT id, tenant_id, title, file_path, file_type, file_size_bytes, status, metadata, created_by, created_at
		 FROM documents WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	).Scan(&doc.ID, &doc.TenantID, &doc.Title, &doc.FilePath, &doc.FileType, &doc.FileSizeBytes, &doc.Status, &doc.Metadata, &doc.CreatedBy, &doc.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	return &doc, nil
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]models.Document, error) {
	tenantID := tenant.IDFromContext(ctx)
	rows, err := s.db.Query(ctx,
		`SELECT id, tenant_id, title, file_path, file_type, file_size_bytes, status, metadata, created_by, created_at
		 FROM documents WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		tenantID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	var docs []models.Document
	for rows.Next() {
		var d models.Document
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Title, &d.FilePath, &d.FileType, &d.FileSizeBytes, &d.Status, &d.Metadata, &d.CreatedBy, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		docs = append(docs, d)
	}
	return docs, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID := tenant.IDFromContext(ctx)

	doc, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if doc.FilePath != "" {
		_ = s.storage.Delete(ctx, s.bucket, doc.FilePath)
	}

	_, err = s.db.Exec(ctx, "DELETE FROM documents WHERE id = $1 AND tenant_id = $2", id, tenantID)
	return err
}

func (s *Service) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := s.db.Exec(ctx, "UPDATE documents SET status = $1 WHERE id = $2", status, id)
	return err
}
