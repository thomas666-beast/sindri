package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/thomas666-beast/sindri/internal/config"
	"github.com/thomas666-beast/sindri/internal/crypto"
	"github.com/thomas666-beast/sindri/internal/db"
	"github.com/thomas666-beast/sindri/internal/db/dbgen"
	"github.com/thomas666-beast/sindri/internal/notification"
	"github.com/thomas666-beast/sindri/internal/order"
	"github.com/thomas666-beast/sindri/internal/settings"
	"github.com/thomas666-beast/sindri/internal/worker"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Initialize services
	queries := dbgen.New(pool)
	notifSvc := notification.NewService(queries)
	settingsSvc := settings.NewService(queries)
	
	// The order service requires a cipher for deliverables (even though the worker doesn't upload them)
	cipher, err := crypto.NewCipher(cfg.ChatEncryptionKey)
	if err != nil {
		slog.Error("failed to create cipher", "error", err)
		os.Exit(1)
	}

	orderSvc := order.NewService(pool, settingsSvc, notifSvc, cipher, cfg.UploadsDir)

	// Start the worker
	w := worker.NewReleaseWorker(pool, orderSvc)
	w.Run(ctx) // This blocks until SIGINT (Ctrl+C)
}
