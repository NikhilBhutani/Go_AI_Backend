package prompt

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhilbhutani/backendwithai/internal/models"
	"github.com/nikhilbhutani/backendwithai/internal/tenant"
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

type CreateRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	SystemTemplate string `json:"system_template"`
	UserTemplate   string `json:"user_template"`
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*models.Prompt, error) {
	tenantID := tenant.IDFromContext(ctx)

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var p models.Prompt
	var tid *uuid.UUID
	if tenantID != uuid.Nil {
		tid = &tenantID
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO prompts (tenant_id, name, description, current_version)
		 VALUES ($1, $2, $3, 1)
		 RETURNING id, tenant_id, name, description, current_version, created_at`,
		tid, req.Name, req.Description,
	).Scan(&p.ID, &p.TenantID, &p.Name, &p.Description, &p.CurrentVersion, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert prompt: %w", err)
	}

	// Detect variables from templates
	vars := ExtractVariables(req.SystemTemplate + " " + req.UserTemplate)
	varsJSON, _ := json.Marshal(vars)

	_, err = tx.Exec(ctx,
		`INSERT INTO prompt_versions (prompt_id, version, system_template, user_template, variables)
		 VALUES ($1, 1, $2, $3, $4)`,
		p.ID, req.SystemTemplate, req.UserTemplate, varsJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("insert prompt version: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &p, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*models.Prompt, []models.PromptVersion, error) {
	tenantID := tenant.IDFromContext(ctx)

	var p models.Prompt
	err := s.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, description, current_version, created_at
		 FROM prompts WHERE id = $1 AND (tenant_id = $2 OR tenant_id IS NULL)`,
		id, tenantID,
	).Scan(&p.ID, &p.TenantID, &p.Name, &p.Description, &p.CurrentVersion, &p.CreatedAt)
	if err != nil {
		return nil, nil, fmt.Errorf("get prompt: %w", err)
	}

	rows, err := s.db.Query(ctx,
		`SELECT id, prompt_id, version, system_template, user_template, variables, created_at
		 FROM prompt_versions WHERE prompt_id = $1 ORDER BY version DESC`,
		id,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("get versions: %w", err)
	}
	defer rows.Close()

	var versions []models.PromptVersion
	for rows.Next() {
		var v models.PromptVersion
		if err := rows.Scan(&v.ID, &v.PromptID, &v.Version, &v.SystemTemplate, &v.UserTemplate, &v.Variables, &v.CreatedAt); err != nil {
			return nil, nil, fmt.Errorf("scan version: %w", err)
		}
		versions = append(versions, v)
	}

	return &p, versions, nil
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]models.Prompt, error) {
	tenantID := tenant.IDFromContext(ctx)
	rows, err := s.db.Query(ctx,
		`SELECT id, tenant_id, name, description, current_version, created_at
		 FROM prompts WHERE tenant_id = $1 OR tenant_id IS NULL
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		tenantID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list prompts: %w", err)
	}
	defer rows.Close()

	var prompts []models.Prompt
	for rows.Next() {
		var p models.Prompt
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.Description, &p.CurrentVersion, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan prompt: %w", err)
		}
		prompts = append(prompts, p)
	}
	return prompts, nil
}

type NewVersionRequest struct {
	SystemTemplate string `json:"system_template"`
	UserTemplate   string `json:"user_template"`
}

func (s *Service) CreateVersion(ctx context.Context, promptID uuid.UUID, req NewVersionRequest) (*models.PromptVersion, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get current version
	var currentVersion int
	err = tx.QueryRow(ctx, "SELECT current_version FROM prompts WHERE id = $1 FOR UPDATE", promptID).Scan(&currentVersion)
	if err != nil {
		return nil, fmt.Errorf("get current version: %w", err)
	}

	newVersion := currentVersion + 1
	vars := ExtractVariables(req.SystemTemplate + " " + req.UserTemplate)
	varsJSON, _ := json.Marshal(vars)

	var v models.PromptVersion
	err = tx.QueryRow(ctx,
		`INSERT INTO prompt_versions (prompt_id, version, system_template, user_template, variables)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, prompt_id, version, system_template, user_template, variables, created_at`,
		promptID, newVersion, req.SystemTemplate, req.UserTemplate, varsJSON,
	).Scan(&v.ID, &v.PromptID, &v.Version, &v.SystemTemplate, &v.UserTemplate, &v.Variables, &v.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert version: %w", err)
	}

	_, err = tx.Exec(ctx, "UPDATE prompts SET current_version = $1 WHERE id = $2", newVersion, promptID)
	if err != nil {
		return nil, fmt.Errorf("update current version: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &v, nil
}

type RenderRequest struct {
	Version   int               `json:"version,omitempty"` // 0 = current
	Variables map[string]string `json:"variables"`
}

type RenderResponse struct {
	System string `json:"system"`
	User   string `json:"user"`
}

func (s *Service) RenderPrompt(ctx context.Context, promptID uuid.UUID, req RenderRequest) (*RenderResponse, error) {
	version := req.Version

	if version == 0 {
		err := s.db.QueryRow(ctx, "SELECT current_version FROM prompts WHERE id = $1", promptID).Scan(&version)
		if err != nil {
			return nil, fmt.Errorf("get current version: %w", err)
		}
	}

	var v models.PromptVersion
	err := s.db.QueryRow(ctx,
		`SELECT id, prompt_id, version, system_template, user_template, variables, created_at
		 FROM prompt_versions WHERE prompt_id = $1 AND version = $2`,
		promptID, version,
	).Scan(&v.ID, &v.PromptID, &v.Version, &v.SystemTemplate, &v.UserTemplate, &v.Variables, &v.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get version %d: %w", version, err)
	}

	systemRendered, err := Render(v.SystemTemplate, req.Variables)
	if err != nil {
		return nil, fmt.Errorf("render system template: %w", err)
	}

	userRendered, err := Render(v.UserTemplate, req.Variables)
	if err != nil {
		return nil, fmt.Errorf("render user template: %w", err)
	}

	return &RenderResponse{
		System: systemRendered,
		User:   userRendered,
	}, nil
}
