package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prehub/prehub/backend/internal/config"
	"github.com/prehub/prehub/backend/internal/db"
	"github.com/prehub/prehub/backend/internal/httpapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var store *db.Store
	if cfg.DatabaseURL != "" {
		connectedStore, err := db.Connect(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("database unavailable", "error", err)
			os.Exit(1)
		} else {
			store = connectedStore
			defer store.Close()
			logger.Info("database connected")
		}
	} else {
		logger.Warn("database url is not configured; data endpoints will be unavailable")
	}

	server := httpapi.New(cfg, logger, store)
	addr := ":" + cfg.Port

	httpServer := &http.Server{
		Addr:    addr,
		Handler: server.Handler(),
	}

	// Graceful shutdown on SIGINT / SIGTERM
	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("starting api", "addr", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("api stopped", "error", err)
			os.Exit(1)
		}
	}()

	<-shutdownCtx.Done()
	logger.Info("shutdown signal received, draining connections")

	drainCtx, drainCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer drainCancel()
	if err := httpServer.Shutdown(drainCtx); err != nil {
		logger.Error("shutdown failed", "error", err)
	}
	server.Drain()
	logger.Info("api stopped gracefully")
}
