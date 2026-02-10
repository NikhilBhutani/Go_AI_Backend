package queue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/nikhilbhutani/backendwithai/internal/config"
)

type Client struct {
	client *asynq.Client
}

func NewClient(cfg config.RedisConfig) *Client {
	return &Client{
		client: asynq.NewClient(asynq.RedisClientOpt{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		}),
	}
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) EnqueueDocumentProcess(payload DocumentProcessPayload) error {
	return c.enqueue(TypeDocumentProcess, payload, asynq.MaxRetry(3), asynq.Timeout(10*time.Minute))
}

func (c *Client) EnqueueEmbeddingGenerate(payload EmbeddingGeneratePayload) error {
	return c.enqueue(TypeEmbeddingGenerate, payload, asynq.MaxRetry(3), asynq.Timeout(5*time.Minute))
}

func (c *Client) EnqueueFinetuneRun(payload FinetuneRunPayload) error {
	return c.enqueue(TypeFinetuneRun, payload, asynq.MaxRetry(2), asynq.Timeout(30*time.Minute))
}

func (c *Client) EnqueueWebhookDeliver(payload WebhookDeliverPayload) error {
	return c.enqueue(TypeWebhookDeliver, payload, asynq.MaxRetry(5), asynq.Timeout(30*time.Second))
}

func (c *Client) enqueue(taskType string, payload interface{}, opts ...asynq.Option) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	task := asynq.NewTask(taskType, data)
	_, err = c.client.Enqueue(task, opts...)
	if err != nil {
		return fmt.Errorf("enqueue %s: %w", taskType, err)
	}
	return nil
}
