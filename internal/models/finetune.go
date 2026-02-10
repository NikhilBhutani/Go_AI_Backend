package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type FinetuneDataset struct {
	ID             uuid.UUID `json:"id" db:"id"`
	TenantID       uuid.UUID `json:"tenant_id" db:"tenant_id"`
	Name           string    `json:"name" db:"name"`
	FilePath       string    `json:"file_path,omitempty" db:"file_path"`
	RecordCount    int       `json:"record_count" db:"record_count"`
	Status         string    `json:"status" db:"status"`
	Provider       string    `json:"provider,omitempty" db:"provider"`
	ProviderFileID string    `json:"provider_file_id,omitempty" db:"provider_file_id"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

type FinetuneJob struct {
	ID            uuid.UUID       `json:"id" db:"id"`
	TenantID      uuid.UUID       `json:"tenant_id" db:"tenant_id"`
	DatasetID     *uuid.UUID      `json:"dataset_id,omitempty" db:"dataset_id"`
	Provider      string          `json:"provider" db:"provider"`
	ProviderJobID string          `json:"provider_job_id,omitempty" db:"provider_job_id"`
	BaseModel     string          `json:"base_model" db:"base_model"`
	Status        string          `json:"status" db:"status"`
	Hyperparams   json.RawMessage `json:"hyperparams" db:"hyperparams"`
	ResultModel   string          `json:"result_model,omitempty" db:"result_model"`
	Error         string          `json:"error,omitempty" db:"error"`
	StartedAt     *time.Time      `json:"started_at,omitempty" db:"started_at"`
	CompletedAt   *time.Time      `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
}

type ModelRegistryEntry struct {
	ID            uuid.UUID       `json:"id" db:"id"`
	TenantID      uuid.UUID       `json:"tenant_id" db:"tenant_id"`
	Name          string          `json:"name" db:"name"`
	Provider      string          `json:"provider" db:"provider"`
	ModelID       string          `json:"model_id" db:"model_id"`
	BaseModel     string          `json:"base_model,omitempty" db:"base_model"`
	FinetuneJobID *uuid.UUID      `json:"finetune_job_id,omitempty" db:"finetune_job_id"`
	IsActive      bool            `json:"is_active" db:"is_active"`
	Metadata      json.RawMessage `json:"metadata" db:"metadata"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
}

const (
	DatasetStatusDraft     = "draft"
	DatasetStatusValidated = "validated"
	DatasetStatusUploaded  = "uploaded"

	JobStatusPending   = "pending"
	JobStatusRunning   = "running"
	JobStatusSucceeded = "succeeded"
	JobStatusFailed    = "failed"
)
