package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/nikhilbhutani/backendwithai/internal/api"
	"github.com/nikhilbhutani/backendwithai/internal/config"
	"github.com/nikhilbhutani/backendwithai/internal/database"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Database connection (optional — gracefully handle missing DATABASE_URL)
	var dbPool interface{ Ping(context.Context) error }
	db, err := database.NewPool(ctx, cfg.Database)
	if err != nil {
		slog.Warn("database unavailable, running without DB", "error", err)
	} else {
		dbPool = db
		defer db.Close()

		if err := database.RunMigrations(ctx, db, cfg.Database.MigrationsPath); err != nil {
			slog.Warn("migrations failed", "error", err)
		}
	}
	_ = dbPool

	// Redis connection (optional)
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		slog.Warn("redis unavailable, running without cache", "error", err)
	}
	defer rdb.Close()

	// Setup router
	router := api.NewRouter(db, rdb, cfg)
	handler := router.Setup()

	srv := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("starting API server", "addr", cfg.Addr())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced shutdown", "error", err)
	}
	slog.Info("server stopped")
}
