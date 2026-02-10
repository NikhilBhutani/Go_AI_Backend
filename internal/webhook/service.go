package webhook

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhilbhutani/backendwithai/internal/models"
	"github.com/nikhilbhutani/backendwithai/internal/tenant"
)

type Service struct {
	db         *pgxpool.Pool
	dispatcher *Dispatcher
}

func NewService(db *pgxpool.Pool, dispatcher *Dispatcher) *Service {
	return &Service{db: db, dispatcher: dispatcher}
}

type CreateRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*models.Webhook, error) {
	tenantID := tenant.IDFromContext(ctx)

	secret, err := generateSecret()
	if err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}

	eventsJSON, _ := json.Marshal(req.Events)

	var wh models.Webhook
	err = s.db.QueryRow(ctx,
		`INSERT INTO webhooks (tenant_id, url, events, secret, is_active)
		 VALUES ($1, $2, $3, $4, true)
		 RETURNING id, tenant_id, url, events, is_active, created_at`,
		tenantID, req.URL, eventsJSON, secret,
	).Scan(&wh.ID, &wh.TenantID, &wh.URL, &wh.Events, &wh.IsActive, &wh.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert webhook: %w", err)
	}

	// Return secret only on creation
	wh.Secret = secret

	return &wh, nil
}

func (s *Service) List(ctx context.Context) ([]models.Webhook, error) {
	tenantID := tenant.IDFromContext(ctx)

	rows, err := s.db.Query(ctx,
		`SELECT id, tenant_id, url, events, is_active, created_at
		 FROM webhooks WHERE tenant_id = $1 ORDER BY created_at DESC`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []models.Webhook
	for rows.Next() {
		var wh models.Webhook
		if err := rows.Scan(&wh.ID, &wh.TenantID, &wh.URL, &wh.Events, &wh.IsActive, &wh.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan webhook: %w", err)
		}
		webhooks = append(webhooks, wh)
	}
	return webhooks, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID := tenant.IDFromContext(ctx)
	_, err := s.db.Exec(ctx, "DELETE FROM webhooks WHERE id = $1 AND tenant_id = $2", id, tenantID)
	return err
}

// Dispatch sends an event to all matching webhooks for the tenant.
func (s *Service) Dispatch(ctx context.Context, event string, payload interface{}) error {
	tenantID := tenant.IDFromContext(ctx)

	rows, err := s.db.Query(ctx,
		`SELECT id, url, secret FROM webhooks
		 WHERE tenant_id = $1 AND is_active = true AND events @> $2::jsonb`,
		tenantID, fmt.Sprintf(`["%s"]`, event),
	)
	if err != nil {
		return fmt.Errorf("find matching webhooks: %w", err)
	}
	defer rows.Close()

	payloadJSON, _ := json.Marshal(payload)

	for rows.Next() {
		var id uuid.UUID
		var url, secret string
		if err := rows.Scan(&id, &url, &secret); err != nil {
			continue
		}

		if s.dispatcher != nil {
			s.dispatcher.Enqueue(DeliveryRequest{
				WebhookID: id,
				URL:       url,
				Secret:    secret,
				Event:     event,
				Payload:   payloadJSON,
			})
		}
	}
	return nil
}

func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "whsec_" + hex.EncodeToString(b), nil
}
