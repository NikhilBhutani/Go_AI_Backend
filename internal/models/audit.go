package models

import (
	"encoding/json"
	"net/netip"
	"time"

	"github.com/google/uuid"
)

type LLMUsageLog struct {
	ID           uuid.UUID       `json:"id" db:"id"`
	TenantID     uuid.UUID       `json:"tenant_id" db:"tenant_id"`
	UserID       *uuid.UUID      `json:"user_id,omitempty" db:"user_id"`
	Provider     string          `json:"provider" db:"provider"`
	Model        string          `json:"model" db:"model"`
	InputTokens  int             `json:"input_tokens" db:"input_tokens"`
	OutputTokens int             `json:"output_tokens" db:"output_tokens"`
	TotalTokens  int             `json:"total_tokens" db:"total_tokens"`
	CostUSD      float64         `json:"cost_usd" db:"cost_usd"`
	LatencyMs    int             `json:"latency_ms" db:"latency_ms"`
	Endpoint     string          `json:"endpoint" db:"endpoint"`
	Metadata     json.RawMessage `json:"metadata" db:"metadata"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
}

type AuditLog struct {
	ID           uuid.UUID       `json:"id" db:"id"`
	TenantID     uuid.UUID       `json:"tenant_id" db:"tenant_id"`
	UserID       *uuid.UUID      `json:"user_id,omitempty" db:"user_id"`
	Action       string          `json:"action" db:"action"`
	ResourceType string          `json:"resource_type,omitempty" db:"resource_type"`
	ResourceID   *uuid.UUID      `json:"resource_id,omitempty" db:"resource_id"`
	Details      json.RawMessage `json:"details" db:"details"`
	IPAddress    *netip.Addr     `json:"ip_address,omitempty" db:"ip_address"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
}

type Webhook struct {
	ID        uuid.UUID `json:"id" db:"id"`
	TenantID  uuid.UUID `json:"tenant_id" db:"tenant_id"`
	URL       string    `json:"url" db:"url"`
	Events    []string  `json:"events" db:"events"`
	Secret    string    `json:"-" db:"secret"`
	IsActive  bool      `json:"is_active" db:"is_active"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type WebhookDelivery struct {
	ID             uuid.UUID       `json:"id" db:"id"`
	WebhookID      uuid.UUID       `json:"webhook_id" db:"webhook_id"`
	Event          string          `json:"event" db:"event"`
	Payload        json.RawMessage `json:"payload" db:"payload"`
	ResponseStatus int             `json:"response_status" db:"response_status"`
	Attempts       int             `json:"attempts" db:"attempts"`
	NextRetryAt    *time.Time      `json:"next_retry_at,omitempty" db:"next_retry_at"`
	DeliveredAt    *time.Time      `json:"delivered_at,omitempty" db:"delivered_at"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
}
