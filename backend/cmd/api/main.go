package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/prehub/prehub/backend/internal/config"
	"github.com/prehub/prehub/backend/internal/db"
	"github.com/prehub/prehub/backend/internal/httpapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var store *db.Store
	if cfg.DatabaseURL != "" {
		connectedStore, err := db.Connect(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Warn("database unavailable, using in-memory fallback", "error", err)
		} else {
			store = connectedStore
			defer store.Close()
			logger.Info("database connected")
		}
	}

	server := httpapi.New(cfg, logger, store)
	addr := ":" + cfg.Port

	logger.Info("starting api", "addr", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}
