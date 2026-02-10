package queue

const (
	TypeDocumentProcess  = "document:process"
	TypeEmbeddingGenerate = "embedding:generate"
	TypeFinetuneRun      = "finetune:run"
	TypeWebhookDeliver   = "webhook:deliver"
)

type DocumentProcessPayload struct {
	DocumentID string `json:"document_id"`
	TenantID   string `json:"tenant_id"`
}

type EmbeddingGeneratePayload struct {
	DocumentID string `json:"document_id"`
	TenantID   string `json:"tenant_id"`
}

type FinetuneRunPayload struct {
	JobID    string `json:"job_id"`
	TenantID string `json:"tenant_id"`
}

type WebhookDeliverPayload struct {
	WebhookID string `json:"webhook_id"`
	Event     string `json:"event"`
	Payload   string `json:"payload"` // JSON string
}
