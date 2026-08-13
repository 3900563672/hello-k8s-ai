package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/app"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Dashboard Backend configuration is invalid", "error", err)
		os.Exit(2)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx, cfg, logger)
	if err != nil {
		logger.Error("Dashboard Backend could not initialize", "error", err)
		os.Exit(1)
	}
	if err := application.Run(ctx); err != nil {
		logger.Error("Dashboard Backend stopped with an error", "error", err)
		os.Exit(1)
	}
	logger.Info("Dashboard Backend stopped")
}
