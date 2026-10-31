package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thomas666-beast/reqval"

	"github.com/thomas666-beast/sindri/internal/auth"
	"github.com/thomas666-beast/sindri/internal/config"
	"github.com/thomas666-beast/sindri/internal/crypto"
	"github.com/thomas666-beast/sindri/internal/db"
	"github.com/thomas666-beast/sindri/internal/dbadapter"
	"github.com/thomas666-beast/sindri/internal/httpapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()

	dbq := dbadapter.New(pool)
	defer dbq.Close()
	reqval.RegisterDBWithPlaceholder(dbq, "$")

	chatCipher, err := crypto.NewCipher(cfg.ChatEncryptionKey)
	if err != nil {
		return fmt.Errorf("init chat cipher: %w", err)
	}

	router := httpapi.NewRouter(httpapi.Deps{
		Pool:           pool,
		Auth:           auth.NewManager(cfg.JWTSecret),
		ChatCipher:     chatCipher,
		CookieSecure:   cfg.CookieSecure,
		UploadsDir:     cfg.UploadsDir,
		AllowedOrigins: cfg.AllowedOrigins,
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "port", cfg.HTTPPort, "env", cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	slog.Info("server stopped cleanly")
	return nil
}
