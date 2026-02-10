package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/nikhilbhutani/backendwithai/internal/queue"
)

type FinetuneWorker struct{}

func NewFinetuneWorker() *FinetuneWorker {
	return &FinetuneWorker{}
}

func (w *FinetuneWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload queue.FinetuneRunPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	slog.Info("running finetune job", "job_id", payload.JobID, "tenant_id", payload.TenantID)

	// In production, this would:
	// 1. Upload dataset to provider
	// 2. Create fine-tune job via provider API
	// 3. Poll for completion
	// 4. Register resulting model

	slog.Info("finetune job placeholder completed", "job_id", payload.JobID)
	return nil
}
