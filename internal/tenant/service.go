package tenant

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhilbhutani/backendwithai/internal/models"
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*models.Tenant, error) {
	var t models.Tenant
	err := s.db.QueryRow(ctx,
		"SELECT id, name, slug, settings, created_at, updated_at FROM tenants WHERE id = $1", id,
	).Scan(&t.ID, &t.Name, &t.Slug, &t.Settings, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get tenant: %w", err)
	}
	return &t, nil
}

func (s *Service) GetBySlug(ctx context.Context, slug string) (*models.Tenant, error) {
	var t models.Tenant
	err := s.db.QueryRow(ctx,
		"SELECT id, name, slug, settings, created_at, updated_at FROM tenants WHERE slug = $1", slug,
	).Scan(&t.ID, &t.Name, &t.Slug, &t.Settings, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get tenant by slug: %w", err)
	}
	return &t, nil
}

func (s *Service) Create(ctx context.Context, name, slug string) (*models.Tenant, error) {
	var t models.Tenant
	err := s.db.QueryRow(ctx,
		`INSERT INTO tenants (name, slug) VALUES ($1, $2)
		 RETURNING id, name, slug, settings, created_at, updated_at`,
		name, slug,
	).Scan(&t.ID, &t.Name, &t.Slug, &t.Settings, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create tenant: %w", err)
	}
	return &t, nil
}

func (s *Service) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var u models.User
	err := s.db.QueryRow(ctx,
		"SELECT id, tenant_id, role_id, email, full_name, created_at FROM users WHERE id = $1", id,
	).Scan(&u.ID, &u.TenantID, &u.RoleID, &u.Email, &u.FullName, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}
