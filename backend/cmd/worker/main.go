package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/prehub/prehub/backend/internal/config"
	"github.com/prehub/prehub/backend/internal/db"
	"github.com/prehub/prehub/backend/internal/domain"
	"github.com/prehub/prehub/backend/internal/editorial"
	"github.com/prehub/prehub/backend/internal/github"
	"github.com/prehub/prehub/backend/internal/openai"
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

	// Initialize OpenAI client if API key is configured
	var openaiClient *openai.Client
	if cfg.OpenAIAPIKey != "" {
		openaiClient = openai.New(cfg.OpenAIAPIKey)
		logger.Info("OpenAI embedding client initialized")
	}

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
		runDiscoveryCycle(context.Background(), logger, cfg, store, openaiClient)
		refreshDueRadarRepositories(context.Background(), logger, cfg, store)
	}

	if workerRunOnce() {
		logger.Info("worker run once completed")
		return
	}

	radarTicker := time.NewTicker(30 * time.Second)
	defer radarTicker.Stop()

	var discoveryC <-chan time.Time
	var discoveryTicker *time.Ticker
	if interval := discoveryInterval(); interval > 0 {
		discoveryTicker = time.NewTicker(interval)
		discoveryC = discoveryTicker.C
		logger.Info("candidate discovery scheduled", "interval", interval.String())
	} else {
		logger.Info("candidate discovery schedule disabled")
	}
	if discoveryTicker != nil {
		defer discoveryTicker.Stop()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sigCh:
			logger.Info("shutdown signal received, worker stopping")
			return
		case <-radarTicker.C:
			if store != nil {
				refreshDueRadarRepositories(context.Background(), logger, cfg, store)
			}
			logger.Info("worker heartbeat", "status", "idle")
		case <-discoveryC:
			if store != nil {
				runDiscoveryCycle(context.Background(), logger, cfg, store, openaiClient)
			}
		}
	}
}

func runDiscoveryCycle(ctx context.Context, logger *slog.Logger, cfg config.Config, store *db.Store, openaiClient *openai.Client) {
	runCandidateDiscovery(ctx, logger, cfg, store, openaiClient)
	backfillEmbeddings(ctx, logger, store, openaiClient)
	seedRadarWatchlist(ctx, logger, store)
}

func seedRadarWatchlist(ctx context.Context, logger *slog.Logger, store *db.Store) {
	limit := seedRadarLimit()
	if limit <= 0 {
		logger.Info("radar seed skipped", "reason", "PREHUB_RADAR_SEED_LIMIT disabled")
		return
	}
	for _, category := range seedCategories() {
		seeded, err := store.SeedRadarFromCandidates(ctx, category, limit)
		if err != nil {
			logger.Warn("radar seed failed", "category", category, "error", err)
			continue
		}
		logger.Info("radar seed finished", "category", category, "seeded", seeded)
	}
}

func refreshDueRadarRepositories(ctx context.Context, logger *slog.Logger, cfg config.Config, store *db.Store) {
	runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	monitored, err := store.ListDueMonitoredRepositories(runCtx, 5)
	if err != nil {
		logger.Warn("radar due list failed", "error", err)
		return
	}
	if len(monitored) == 0 {
		return
	}

	client := github.New(cfg.GitHubToken, cfg.GitHubAPIVersion)
	for _, item := range monitored {
		response, rate, err := client.GetRepository(runCtx, item.Repository.Owner, item.Repository.Name)
		if err != nil {
			logger.Warn("radar refresh failed", "repo", item.Repository.FullName, "error", err, "remaining", rate.Remaining)
			if rateLimitDepleted(rate) {
				logger.Warn("radar refresh paused", "reason", "github rate limit depleted", "remaining", rate.Remaining)
				return
			}
			continue
		}
		repo := github.ToDomainRepository(response, item.Repository.Topics, item.Repository.Summary)
		if item.Repository.AvatarURL != "" && repo.AvatarURL == "" {
			repo.AvatarURL = item.Repository.AvatarURL
		}
		if _, err := store.SaveMonitoredRepository(runCtx, repo, item.Category, item.Tier); err != nil {
			logger.Warn("radar refresh persist failed", "repo", item.Repository.FullName, "error", err)
			continue
		}
		starEvents, starRate, err := client.ListRecentStargazers(runCtx, item.Repository.Owner, item.Repository.Name, time.Now().UTC().AddDate(0, 0, -30), radarStargazerMaxPages())
		if err != nil {
			logger.Warn("radar stargazers fetch failed", "repo", item.Repository.FullName, "error", err, "remaining", starRate.Remaining)
		} else {
			rate = starRate
			inserted, err := store.SaveRepositoryStarEvents(runCtx, item.Repository.Owner, item.Repository.Name, starEvents)
			if err != nil {
				logger.Warn("radar stargazers persist failed", "repo", item.Repository.FullName, "error", err)
			} else {
				logger.Info("radar stargazers persisted", "repo", item.Repository.FullName, "fetched", len(starEvents), "inserted", inserted, "remaining", rate.Remaining)
			}
		}
		logger.Info("radar refreshed", "repo", repo.FullName, "category", item.Category, "tier", item.Tier, "remaining", rate.Remaining)
	}
}

func radarStargazerMaxPages() int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv("PREHUB_STARGAZER_PAGES")))
	if err != nil || value <= 0 {
		return 2
	}
	if value > 10 {
		return 10
	}
	return value
}

func runCandidateDiscovery(ctx context.Context, logger *slog.Logger, cfg config.Config, store *db.Store, openaiClient *openai.Client) {
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(discoveryTimeoutSeconds())*time.Second)
	defer cancel()

	client := github.New(cfg.GitHubToken, cfg.GitHubAPIVersion)
	var rate github.RateLimitSnapshot
	seen := map[string]bool{}
	searches := 0
	persisted := 0
	monitored := 0
	readmesFetched := 0
	stargazersFetched := 0
	enrichmentRateLimited := false
	stargazerRateLimited := false
	maxSearches := discoveryMaxSearches(cfg)
	maxReadmes := discoveryReadmeLimit(cfg)
	maxStargazerRepos := discoveryStargazerRepoLimit(cfg)
	perPage := discoveryPerPage()
	pages := discoveryPages()
	categories := discoveryCategories()
	initialQuery := strings.TrimSpace(os.Getenv("PREHUB_INITIAL_QUERY"))
	if initialQuery != "" {
		categories = []string{domain.NormalizeCategory(os.Getenv("PREHUB_INITIAL_CATEGORY"))}
	}

	for _, category := range categories {
		queries := domain.DiscoverySearchQueries(category, time.Now().UTC())
		if initialQuery != "" {
			queries = []string{initialQuery}
		}

		for _, query := range queries {
			query = strings.TrimSpace(query)
			if query == "" {
				continue
			}
			for page := 1; page <= pages; page++ {
				if searches >= maxSearches {
					logger.Info("candidate discovery search budget reached", "max_searches", maxSearches, "persisted", persisted, "monitored", monitored)
					logger.Info("candidate discovery finished", "remaining", rate.Remaining)
					return
				}

				repositories, searchRate, err := client.SearchRepositoriesPageWithSort(runCtx, query, perPage, page, "updated", "desc")
				searches++
				if err != nil {
					logger.Warn("candidate discovery failed", "category", category, "query", query, "page", page, "error", err)
					if rateLimitDepleted(searchRate) {
						logger.Warn("candidate discovery paused", "reason", "github search rate limit depleted", "remaining", searchRate.Remaining)
						logger.Info("candidate discovery finished", "remaining", searchRate.Remaining, "searches", searches, "persisted", persisted, "monitored", monitored, "readmes", readmesFetched, "stargazer_repos", stargazersFetched)
						return
					}
					break
				}
				rate = searchRate
				logger.Info("candidate discovery fetched", "category", category, "query", query, "page", page, "count", len(repositories), "remaining", rate.Remaining)
				if len(repositories) == 0 {
					break
				}

				for _, item := range repositories {
					seenKey := category + ":" + strings.ToLower(item.FullName)
					if seen[seenKey] {
						continue
					}
					seen[seenKey] = true

					repo := github.ToDomainRepository(item, item.Topics, item.Description)
					score := scoring.ScoreRepository(repo, time.Now().UTC())
					if !scoring.IsCandidateQualityAcceptable(score) {
						logger.Info("candidate skipped by quality guard", "repo", repo.FullName, "category", category, "score", score.Quality)
						continue
					}

					if !enrichmentRateLimited && readmesFetched < maxReadmes {
						_, readme, readmeRate, err := client.GetReadme(runCtx, item.Owner.Login, item.Name)
						if err != nil {
							logger.Warn("candidate readme fetch failed", "repo", item.FullName, "error", err)
							if rateLimitDepleted(readmeRate) {
								enrichmentRateLimited = true
								logger.Warn("candidate readme enrichment paused", "reason", "github rate limit depleted", "remaining", readmeRate.Remaining)
							}
						} else {
							rate = readmeRate
							readmesFetched++
							repo.Summary = editorial.SummarizeReadme(readme, item.Description)
							readmeIconURL := github.ResolveReadmeIconURL(readme, item.Owner.Login, item.Name, item.DefaultBranch)
							repo = github.ToDomainRepository(item, item.Topics, repo.Summary)
							if readmeIconURL != "" {
								repo.AvatarURL = readmeIconURL
							}
							score = scoring.ScoreRepository(repo, time.Now().UTC())
						}
					}

					result, err := store.SaveCandidate(runCtx, repo, score, "global_discovery_"+category, "pending_review")
					if err != nil {
						logger.Warn("candidate persist failed", "repo", repo.FullName, "category", category, "error", err)
						continue
					}
					persisted++

					// Generate and store embedding for the repository
					if openaiClient != nil && result.RepositoryID != "" {
						input := openai.BuildEmbeddingInput(repo)
						if embedding, embErr := openaiClient.GenerateEmbedding(runCtx, input); embErr != nil {
							logger.Warn("embedding generation failed", "repo", repo.FullName, "error", embErr)
						} else if err := store.UpsertEmbedding(runCtx, result.RepositoryID, embedding); err != nil {
							logger.Warn("embedding upsert failed", "repo", repo.FullName, "error", err)
						} else {
							logger.Debug("embedding generated", "repo", repo.FullName)
						}
					}

					if _, err := store.SaveMonitoredRepository(runCtx, repo, category, "candidate"); err != nil {
						logger.Warn("radar auto-watch persist failed", "repo", repo.FullName, "category", category, "error", err)
						continue
					}
					monitored++

					if !stargazerRateLimited && stargazersFetched < maxStargazerRepos {
						events, starRate, err := client.ListRecentStargazers(runCtx, repo.Owner, repo.Name, time.Now().UTC().AddDate(0, 0, -30), radarStargazerMaxPages())
						if err != nil {
							logger.Warn("discovery stargazers fetch failed", "repo", repo.FullName, "error", err, "remaining", starRate.Remaining)
							if rateLimitDepleted(starRate) {
								stargazerRateLimited = true
								logger.Warn("discovery stargazer backfill paused", "reason", "github rate limit depleted", "remaining", starRate.Remaining)
							}
						} else {
							rate = starRate
							stargazersFetched++
							inserted, err := store.SaveRepositoryStarEvents(runCtx, repo.Owner, repo.Name, events)
							if err != nil {
								logger.Warn("discovery stargazers persist failed", "repo", repo.FullName, "error", err)
							} else {
								logger.Info("discovery stargazers persisted", "repo", repo.FullName, "fetched", len(events), "inserted", inserted, "remaining", rate.Remaining)
							}
						}
					}

					logger.Info("candidate persisted and monitored", "repo", repo.FullName, "category", category, "score", score.Quality)
				}

				if runCtx.Err() != nil {
					logger.Warn("candidate discovery stopped", "error", runCtx.Err())
					logger.Info("candidate discovery finished", "remaining", rate.Remaining)
					return
				}
			}
		}
	}
	logger.Info("candidate discovery finished", "remaining", rate.Remaining, "searches", searches, "persisted", persisted, "monitored", monitored, "readmes", readmesFetched, "stargazer_repos", stargazersFetched)
}

func discoveryCategories() []string {
	raw := strings.TrimSpace(os.Getenv("PREHUB_DISCOVERY_CATEGORIES"))
	if raw == "" {
		return []string{"ai", "ai-image", "ai-prompts", "ai-skills", "devtools", "web", "data", "backend"}
	}
	return parseCategories(raw)
}

func seedCategories() []string {
	raw := strings.TrimSpace(os.Getenv("PREHUB_RADAR_SEED_CATEGORIES"))
	if raw == "" {
		return []string{domain.NormalizeCategory(os.Getenv("PREHUB_INITIAL_CATEGORY"))}
	}
	return parseCategories(raw)
}

func parseCategories(raw string) []string {
	categories := []string{}
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		category := domain.NormalizeCategory(part)
		if seen[category] {
			continue
		}
		seen[category] = true
		categories = append(categories, category)
	}
	if len(categories) == 0 {
		return []string{domain.DefaultCategory}
	}
	return categories
}

func discoveryPerPage() int {
	value := envInt("PREHUB_DISCOVERY_PER_PAGE", 30)
	if value <= 0 {
		return 30
	}
	if value > 100 {
		return 100
	}
	return value
}

func discoveryPages() int {
	value := envInt("PREHUB_DISCOVERY_PAGES", 2)
	if value <= 0 {
		return 1
	}
	if value > 10 {
		return 10
	}
	return value
}

func discoveryMaxSearches(cfg config.Config) int {
	fallback := 24
	if strings.TrimSpace(cfg.GitHubToken) == "" {
		fallback = 8
	}
	value := envInt("PREHUB_DISCOVERY_MAX_SEARCHES", fallback)
	if value <= 0 {
		return fallback
	}
	if value > 100 {
		return 100
	}
	return value
}

func discoveryReadmeLimit(cfg config.Config) int {
	fallback := 60
	if strings.TrimSpace(cfg.GitHubToken) == "" {
		fallback = 12
	}
	value := envInt("PREHUB_DISCOVERY_README_LIMIT", fallback)
	if value < 0 {
		return 0
	}
	if value > 300 {
		return 300
	}
	return value
}

func discoveryStargazerRepoLimit(cfg config.Config) int {
	fallback := 12
	if strings.TrimSpace(cfg.GitHubToken) == "" {
		fallback = 2
	}
	value := envInt("PREHUB_DISCOVERY_STARGAZER_REPOS", fallback)
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func discoveryTimeoutSeconds() int {
	value := envInt("PREHUB_DISCOVERY_TIMEOUT_SECONDS", 180)
	if value < 30 {
		return 30
	}
	if value > 900 {
		return 900
	}
	return value
}

func discoveryInterval() time.Duration {
	minutes := envInt("PREHUB_DISCOVERY_INTERVAL_MINUTES", 360)
	if minutes <= 0 {
		return 0
	}
	if minutes < 15 {
		minutes = 15
	}
	if minutes > 24*60 {
		minutes = 24 * 60
	}
	return time.Duration(minutes) * time.Minute
}

func seedRadarLimit() int {
	value := envInt("PREHUB_RADAR_SEED_LIMIT", 50)
	if value <= 0 {
		return 0
	}
	if value > 200 {
		return 200
	}
	return value
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil {
		return fallback
	}
	return value
}

func workerRunOnce() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("PREHUB_WORKER_RUN_ONCE")))
	return value == "1" || value == "true" || value == "yes"
}

func rateLimitDepleted(rate github.RateLimitSnapshot) bool {
	remaining, err := strconv.Atoi(strings.TrimSpace(rate.Remaining))
	return err == nil && remaining <= 0
}

func backfillEmbeddings(ctx context.Context, logger *slog.Logger, store *db.Store, openaiClient *openai.Client) {
	if openaiClient == nil {
		return
	}

	limit := embeddingBackfillLimit()
	if limit <= 0 {
		logger.Info("embedding backfill skipped", "reason", "PREHUB_EMBEDDING_BACKFILL_LIMIT disabled")
		return
	}

	repos, err := store.ListRepositoriesWithoutEmbedding(ctx, limit)
	if err != nil {
		logger.Warn("embedding backfill list failed", "error", err)
		return
	}

	if len(repos) == 0 {
		logger.Info("embedding backfill skipped", "reason", "no repositories without embedding")
		return
	}

	logger.Info("embedding backfill started", "count", len(repos))
	processed := 0
	failed := 0

	for _, repo := range repos {
		if ctx.Err() != nil {
			logger.Info("embedding backfill stopped", "reason", "context cancelled")
			break
		}

		// Need to get repository ID from full_name
		repoID, err := getRepositoryIDByFullName(ctx, store, repo.FullName)
		if err != nil {
			logger.Warn("embedding backfill lookup failed", "repo", repo.FullName, "error", err)
			failed++
			continue
		}

		input := openai.BuildEmbeddingInput(repo)
		embedding, embErr := openaiClient.GenerateEmbedding(ctx, input)
		if embErr != nil {
			logger.Warn("embedding generation failed", "repo", repo.FullName, "error", embErr)
			failed++
			continue
		}

		if err := store.UpsertEmbedding(ctx, repoID, embedding); err != nil {
			logger.Warn("embedding upsert failed", "repo", repo.FullName, "error", err)
			failed++
			continue
		}

		processed++
		logger.Debug("embedding backfilled", "repo", repo.FullName)
	}

	logger.Info("embedding backfill finished", "processed", processed, "failed", failed)
}

func embeddingBackfillLimit() int {
	value := envInt("PREHUB_EMBEDDING_BACKFILL_LIMIT", 100)
	if value <= 0 {
		return 0
	}
	if value > 500 {
		return 500
	}
	return value
}

func getRepositoryIDByFullName(ctx context.Context, store *db.Store, fullName string) (string, error) {
	return store.GetRepositoryID(ctx, fullName)
}
