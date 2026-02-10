package main

import (
	"log/slog"
	"os"

	"github.com/hibiken/asynq"
	"github.com/nikhilbhutani/backendwithai/internal/config"
	"github.com/nikhilbhutani/backendwithai/internal/queue"
	"github.com/nikhilbhutani/backendwithai/internal/queue/workers"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
		},
	)

	registry := queue.NewHandlersRegistry()

	// Register workers
	finetuneWorker := workers.NewFinetuneWorker()

	registry.Register(queue.TypeFinetuneRun, asynq.HandlerFunc(finetuneWorker.ProcessTask))

	slog.Info("starting worker", "concurrency", 10)
	if err := srv.Run(registry.Mux()); err != nil {
		slog.Error("worker error", "error", err)
		os.Exit(1)
	}
}
