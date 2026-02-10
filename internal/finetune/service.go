package finetune

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhilbhutani/backendwithai/internal/models"
	"github.com/nikhilbhutani/backendwithai/internal/queue"
	"github.com/nikhilbhutani/backendwithai/internal/storage"
	"github.com/nikhilbhutani/backendwithai/internal/tenant"
)

type Service struct {
	db       *pgxpool.Pool
	storage  storage.Storage
	bucket   string
	registry *Registry
	queue    *queue.Client
}

func NewService(db *pgxpool.Pool, store storage.Storage, bucket string, reg *Registry, qc *queue.Client) *Service {
	return &Service{
		db:       db,
		storage:  store,
		bucket:   bucket,
		registry: reg,
		queue:    qc,
	}
}

type UploadDatasetRequest struct {
	Name     string
	Provider string
	Data     io.Reader
}

func (s *Service) UploadDataset(ctx context.Context, req UploadDatasetRequest) (*models.FinetuneDataset, error) {
	tenantID := tenant.IDFromContext(ctx)
	dsID := uuid.New()
	path := fmt.Sprintf("%s/finetune/%s.jsonl", tenantID, dsID)

	if err := s.storage.Upload(ctx, s.bucket, path, req.Data, "application/jsonl"); err != nil {
		return nil, fmt.Errorf("upload dataset: %w", err)
	}

	var ds models.FinetuneDataset
	err := s.db.QueryRow(ctx,
		`INSERT INTO finetune_datasets (id, tenant_id, name, file_path, status, provider)
		 VALUES ($1, $2, $3, $4, 'draft', $5)
		 RETURNING id, tenant_id, name, file_path, record_count, status, provider, provider_file_id, created_at`,
		dsID, tenantID, req.Name, path, req.Provider,
	).Scan(&ds.ID, &ds.TenantID, &ds.Name, &ds.FilePath, &ds.RecordCount, &ds.Status, &ds.Provider, &ds.ProviderFileID, &ds.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert dataset: %w", err)
	}

	return &ds, nil
}

func (s *Service) ListDatasets(ctx context.Context) ([]models.FinetuneDataset, error) {
	tenantID := tenant.IDFromContext(ctx)
	rows, err := s.db.Query(ctx,
		`SELECT id, tenant_id, name, file_path, record_count, status, provider, provider_file_id, created_at
		 FROM finetune_datasets WHERE tenant_id = $1 ORDER BY created_at DESC`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list datasets: %w", err)
	}
	defer rows.Close()

	var datasets []models.FinetuneDataset
	for rows.Next() {
		var ds models.FinetuneDataset
		if err := rows.Scan(&ds.ID, &ds.TenantID, &ds.Name, &ds.FilePath, &ds.RecordCount, &ds.Status, &ds.Provider, &ds.ProviderFileID, &ds.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan dataset: %w", err)
		}
		datasets = append(datasets, ds)
	}
	return datasets, nil
}

type StartJobRequest struct {
	DatasetID   uuid.UUID       `json:"dataset_id"`
	Provider    string          `json:"provider"`
	BaseModel   string          `json:"base_model"`
	Hyperparams json.RawMessage `json:"hyperparams,omitempty"`
}

func (s *Service) StartJob(ctx context.Context, req StartJobRequest) (*models.FinetuneJob, error) {
	tenantID := tenant.IDFromContext(ctx)

	hyperparams := req.Hyperparams
	if hyperparams == nil {
		hyperparams = json.RawMessage(`{}`)
	}

	var job models.FinetuneJob
	err := s.db.QueryRow(ctx,
		`INSERT INTO finetune_jobs (tenant_id, dataset_id, provider, base_model, status, hyperparams)
		 VALUES ($1, $2, $3, $4, 'pending', $5)
		 RETURNING id, tenant_id, dataset_id, provider, provider_job_id, base_model, status, hyperparams, result_model, error, started_at, completed_at, created_at`,
		tenantID, req.DatasetID, req.Provider, req.BaseModel, hyperparams,
	).Scan(&job.ID, &job.TenantID, &job.DatasetID, &job.Provider, &job.ProviderJobID, &job.BaseModel,
		&job.Status, &job.Hyperparams, &job.ResultModel, &job.Error, &job.StartedAt, &job.CompletedAt, &job.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert job: %w", err)
	}

	// Enqueue async job
	if err := s.queue.EnqueueFinetuneRun(queue.FinetuneRunPayload{
		JobID:    job.ID.String(),
		TenantID: tenantID.String(),
	}); err != nil {
		return nil, fmt.Errorf("enqueue finetune job: %w", err)
	}

	return &job, nil
}

func (s *Service) ListJobs(ctx context.Context) ([]models.FinetuneJob, error) {
	tenantID := tenant.IDFromContext(ctx)
	rows, err := s.db.Query(ctx,
		`SELECT id, tenant_id, dataset_id, provider, provider_job_id, base_model, status, hyperparams, result_model, error, started_at, completed_at, created_at
		 FROM finetune_jobs WHERE tenant_id = $1 ORDER BY created_at DESC`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []models.FinetuneJob
	for rows.Next() {
		var j models.FinetuneJob
		if err := rows.Scan(&j.ID, &j.TenantID, &j.DatasetID, &j.Provider, &j.ProviderJobID, &j.BaseModel,
			&j.Status, &j.Hyperparams, &j.ResultModel, &j.Error, &j.StartedAt, &j.CompletedAt, &j.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (s *Service) GetJob(ctx context.Context, id uuid.UUID) (*models.FinetuneJob, error) {
	tenantID := tenant.IDFromContext(ctx)

	var j models.FinetuneJob
	err := s.db.QueryRow(ctx,
		`SELECT id, tenant_id, dataset_id, provider, provider_job_id, base_model, status, hyperparams, result_model, error, started_at, completed_at, created_at
		 FROM finetune_jobs WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	).Scan(&j.ID, &j.TenantID, &j.DatasetID, &j.Provider, &j.ProviderJobID, &j.BaseModel,
		&j.Status, &j.Hyperparams, &j.ResultModel, &j.Error, &j.StartedAt, &j.CompletedAt, &j.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}
	return &j, nil
}

func (s *Service) ListModels(ctx context.Context) ([]models.ModelRegistryEntry, error) {
	return s.registry.List(ctx)
}
