package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/prehub/prehub/backend/internal/config"
	"github.com/prehub/prehub/backend/internal/db"
	"github.com/prehub/prehub/backend/internal/editorial"
	"github.com/prehub/prehub/backend/internal/github"
	"github.com/prehub/prehub/backend/internal/scoring"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	logger.Info("starting worker", "redis", cfg.RedisAddr, "github_api_version", cfg.GitHubAPIVersion)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var store *db.Store
	if cfg.DatabaseURL != "" {
		connectedStore, err := db.Connect(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Warn("database unavailable, worker will idle", "error", err)
		} else {
			store = connectedStore
			defer store.Close()
			logger.Info("database connected")
		}
	}
	cancel()

	jobs := []string{
		"github.search_candidates",
		"github.refresh_repo",
		"github.fetch_readme",
		"ai.summarize_readme",
		"ai.generate_embedding",
		"scoring.score_repository",
		"recommendation.generate_daily_candidates",
	}

	for _, job := range jobs {
		logger.Info("registered job", "type", job)
	}

	if store != nil {
		runCandidateDiscovery(context.Background(), logger, cfg, store)
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C
		logger.Info("worker heartbeat", "status", "idle")
	}
}

func runCandidateDiscovery(ctx context.Context, logger *slog.Logger, cfg config.Config, store *db.Store) {
	query := strings.TrimSpace(os.Getenv("PREHUB_INITIAL_QUERY"))
	if query == "" {
		query = "topic:ai stars:100..12000 pushed:>2026-02-01 archived:false fork:false"
	}

	runCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	client := github.New(cfg.GitHubToken, cfg.GitHubAPIVersion)
	repositories, rate, err := client.SearchRepositoriesWithSort(runCtx, query, 10, "updated", "desc")
	if err != nil {
		logger.Warn("candidate discovery failed", "query", query, "error", err)
		return
	}
	logger.Info("candidate discovery fetched", "query", query, "count", len(repositories), "remaining", rate.Remaining)

	for _, item := range repositories {
		summary := item.Description
		_, readme, readmeRate, err := client.GetReadme(runCtx, item.Owner.Login, item.Name)
		if err != nil {
			logger.Warn("candidate readme fetch failed", "repo", item.FullName, "error", err)
		} else {
			rate = readmeRate
			summary = editorial.SummarizeReadme(readme, item.Description)
		}
		repo := github.ToDomainRepository(item, item.Topics, summary)
		score := scoring.ScoreRepository(repo, time.Now().UTC())
		if !scoring.IsCandidateQualityAcceptable(score) {
			logger.Info("candidate skipped by quality guard", "repo", repo.FullName, "score", score.Quality)
			continue
		}
		if _, err := store.SaveCandidate(runCtx, repo, score, "worker_search", "pending_review"); err != nil {
			logger.Warn("candidate persist failed", "repo", repo.FullName, "error", err)
			continue
		}
		logger.Info("candidate persisted", "repo", repo.FullName, "score", score.Quality)
	}
	logger.Info("candidate discovery finished", "remaining", rate.Remaining)
}
