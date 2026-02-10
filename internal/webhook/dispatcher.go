package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Dispatcher struct {
	db         *pgxpool.Pool
	httpClient *http.Client
	deliveries chan DeliveryRequest
}

type DeliveryRequest struct {
	WebhookID uuid.UUID
	URL       string
	Secret    string
	Event     string
	Payload   []byte
}

func NewDispatcher(db *pgxpool.Pool) *Dispatcher {
	d := &Dispatcher{
		db: db,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		deliveries: make(chan DeliveryRequest, 1000),
	}
	go d.processLoop()
	return d
}

func (d *Dispatcher) Enqueue(req DeliveryRequest) {
	select {
	case d.deliveries <- req:
	default:
		slog.Warn("webhook delivery queue full, dropping", "webhook_id", req.WebhookID, "event", req.Event)
	}
}

func (d *Dispatcher) processLoop() {
	for req := range d.deliveries {
		d.deliver(req)
	}
}

func (d *Dispatcher) deliver(req DeliveryRequest) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	signature := sign(req.Payload, req.Secret)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", req.URL, bytes.NewReader(req.Payload))
	if err != nil {
		slog.Error("webhook request creation failed", "error", err)
		d.recordDelivery(ctx, req, 0, err)
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Webhook-Event", req.Event)
	httpReq.Header.Set("X-Webhook-Signature", signature)
	httpReq.Header.Set("X-Webhook-ID", req.WebhookID.String())

	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		slog.Error("webhook delivery failed", "error", err, "webhook_id", req.WebhookID)
		d.recordDelivery(ctx, req, 0, err)
		return
	}
	defer resp.Body.Close()

	d.recordDelivery(ctx, req, resp.StatusCode, nil)

	if resp.StatusCode >= 400 {
		slog.Warn("webhook received non-success response", "status", resp.StatusCode, "webhook_id", req.WebhookID)
	}
}

func (d *Dispatcher) recordDelivery(ctx context.Context, req DeliveryRequest, status int, deliveryErr error) {
	var deliveredAt *time.Time
	if deliveryErr == nil && status < 400 {
		now := time.Now()
		deliveredAt = &now
	}

	_, err := d.db.Exec(ctx,
		`INSERT INTO webhook_deliveries (webhook_id, event, payload, response_status, attempts, delivered_at)
		 VALUES ($1, $2, $3, $4, 1, $5)`,
		req.WebhookID, req.Event, req.Payload, status, deliveredAt,
	)
	if err != nil {
		slog.Error("failed to record webhook delivery", "error", err)
	}
}

func sign(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return fmt.Sprintf("sha256=%s", hex.EncodeToString(mac.Sum(nil)))
}
