package finetune

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhilbhutani/backendwithai/internal/models"
	"github.com/nikhilbhutani/backendwithai/internal/tenant"
)

type Registry struct {
	db *pgxpool.Pool
}

func NewRegistry(db *pgxpool.Pool) *Registry {
	return &Registry{db: db}
}

func (r *Registry) Register(ctx context.Context, entry models.ModelRegistryEntry) (*models.ModelRegistryEntry, error) {
	tenantID := tenant.IDFromContext(ctx)

	var m models.ModelRegistryEntry
	err := r.db.QueryRow(ctx,
		`INSERT INTO model_registry (tenant_id, name, provider, model_id, base_model, finetune_job_id, is_active, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, tenant_id, name, provider, model_id, base_model, finetune_job_id, is_active, metadata, created_at`,
		tenantID, entry.Name, entry.Provider, entry.ModelID, entry.BaseModel, entry.FinetuneJobID, true, entry.Metadata,
	).Scan(&m.ID, &m.TenantID, &m.Name, &m.Provider, &m.ModelID, &m.BaseModel, &m.FinetuneJobID, &m.IsActive, &m.Metadata, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("register model: %w", err)
	}

	return &m, nil
}

func (r *Registry) List(ctx context.Context) ([]models.ModelRegistryEntry, error) {
	tenantID := tenant.IDFromContext(ctx)

	rows, err := r.db.Query(ctx,
		`SELECT id, tenant_id, name, provider, model_id, base_model, finetune_job_id, is_active, metadata, created_at
		 FROM model_registry WHERE tenant_id = $1 AND is_active = true ORDER BY created_at DESC`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	defer rows.Close()

	var entries []models.ModelRegistryEntry
	for rows.Next() {
		var m models.ModelRegistryEntry
		if err := rows.Scan(&m.ID, &m.TenantID, &m.Name, &m.Provider, &m.ModelID, &m.BaseModel, &m.FinetuneJobID, &m.IsActive, &m.Metadata, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan model: %w", err)
		}
		entries = append(entries, m)
	}

	return entries, nil
}

func (r *Registry) Deactivate(ctx context.Context, id uuid.UUID) error {
	tenantID := tenant.IDFromContext(ctx)
	_, err := r.db.Exec(ctx,
		"UPDATE model_registry SET is_active = false WHERE id = $1 AND tenant_id = $2",
		id, tenantID,
	)
	return err
}
