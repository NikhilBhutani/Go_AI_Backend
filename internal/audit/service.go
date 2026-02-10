package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"time"

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

type LogEntry struct {
	Action       string
	ResourceType string
	ResourceID   *uuid.UUID
	Details      map[string]interface{}
	IPAddress    string
}

func (s *Service) Log(ctx context.Context, entry LogEntry) error {
	tenantID := tenant.IDFromContext(ctx)
	user := tenant.UserFromContext(ctx)

	var userID *uuid.UUID
	if user != nil {
		userID = &user.ID
	}

	details, _ := json.Marshal(entry.Details)

	var ip *netip.Addr
	if entry.IPAddress != "" {
		parsed, err := netip.ParseAddr(entry.IPAddress)
		if err == nil {
			ip = &parsed
		}
	}

	_, err := s.db.Exec(ctx,
		`INSERT INTO audit_logs (tenant_id, user_id, action, resource_type, resource_id, details, ip_address)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		tenantID, userID, entry.Action, entry.ResourceType, entry.ResourceID, details, ip,
	)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}

	return nil
}

func (s *Service) LogLLMUsage(ctx context.Context, record models.LLMUsageLog) error {
	tenantID := tenant.IDFromContext(ctx)
	user := tenant.UserFromContext(ctx)

	var userID *uuid.UUID
	if user != nil {
		userID = &user.ID
	}

	metadata, _ := json.Marshal(record.Metadata)

	_, err := s.db.Exec(ctx,
		`INSERT INTO llm_usage_logs (tenant_id, user_id, provider, model, input_tokens, output_tokens, total_tokens, cost_usd, latency_ms, endpoint, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		tenantID, userID, record.Provider, record.Model, record.InputTokens, record.OutputTokens,
		record.TotalTokens, record.CostUSD, record.LatencyMs, record.Endpoint, metadata,
	)
	if err != nil {
		return fmt.Errorf("insert LLM usage log: %w", err)
	}

	return nil
}

type AuditQuery struct {
	StartDate *time.Time
	EndDate   *time.Time
	Action    string
	Limit     int
	Offset    int
}

func (s *Service) GetAuditLogs(ctx context.Context, q AuditQuery) ([]models.AuditLog, error) {
	tenantID := tenant.IDFromContext(ctx)
	if q.Limit <= 0 {
		q.Limit = 50
	}

	query := `SELECT id, tenant_id, user_id, action, resource_type, resource_id, details, ip_address, created_at
			  FROM audit_logs WHERE tenant_id = $1`
	args := []interface{}{tenantID}
	argIdx := 2

	if q.Action != "" {
		query += fmt.Sprintf(" AND action = $%d", argIdx)
		args = append(args, q.Action)
		argIdx++
	}
	if q.StartDate != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, *q.StartDate)
		argIdx++
	}
	if q.EndDate != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, *q.EndDate)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, q.Limit, q.Offset)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var l models.AuditLog
		if err := rows.Scan(&l.ID, &l.TenantID, &l.UserID, &l.Action, &l.ResourceType, &l.ResourceID, &l.Details, &l.IPAddress, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, nil
}

type UsageSummary struct {
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	TotalCalls   int     `json:"total_calls"`
	TotalTokens  int     `json:"total_tokens"`
	TotalCostUSD float64 `json:"total_cost_usd"`
}

func (s *Service) GetUsageSummary(ctx context.Context, startDate, endDate *time.Time) ([]UsageSummary, error) {
	tenantID := tenant.IDFromContext(ctx)

	query := `SELECT provider, model, COUNT(*) as total_calls,
			         COALESCE(SUM(total_tokens), 0) as total_tokens,
			         COALESCE(SUM(cost_usd), 0) as total_cost_usd
			  FROM llm_usage_logs WHERE tenant_id = $1`
	args := []interface{}{tenantID}
	argIdx := 2

	if startDate != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, *startDate)
		argIdx++
	}
	if endDate != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, *endDate)
		argIdx++
	}

	query += " GROUP BY provider, model ORDER BY total_cost_usd DESC"

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query usage summary: %w", err)
	}
	defer rows.Close()

	var summaries []UsageSummary
	for rows.Next() {
		var us UsageSummary
		if err := rows.Scan(&us.Provider, &us.Model, &us.TotalCalls, &us.TotalTokens, &us.TotalCostUSD); err != nil {
			return nil, fmt.Errorf("scan usage summary: %w", err)
		}
		summaries = append(summaries, us)
	}
	return summaries, nil
}
