package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/prehub/prehub/backend/internal/clickhouse"
	"github.com/prehub/prehub/backend/internal/config"
	"github.com/prehub/prehub/backend/internal/db"
	"github.com/prehub/prehub/backend/internal/domain"
	"github.com/prehub/prehub/backend/internal/editorial"
	"github.com/prehub/prehub/backend/internal/github"
	"github.com/prehub/prehub/backend/internal/openai"
	"github.com/prehub/prehub/backend/internal/scoring"
)

type Server struct {
	config       config.Config
	logger       *slog.Logger
	store        *db.Store
	mux          *http.ServeMux
	openaiClient *openai.Client
	wg           sync.WaitGroup
}

func New(cfg config.Config, logger *slog.Logger, store *db.Store) *Server {
	server := &Server{
		config: cfg,
		logger: logger,
		store:  store,
		mux:    http.NewServeMux(),
	}
	if cfg.OpenAIAPIKey != "" {
		server.openaiClient = openai.New(cfg.OpenAIAPIKey)
	}
	server.routes()
	return server
}

func (s *Server) Handler() http.Handler {
	return s.withLogging(s.mux)
}

// Drain waits for all background goroutines (e.g. search query logging) to finish.
// Call this during graceful shutdown after httpServer.Shutdown returns.
func (s *Server) Drain() {
	s.wg.Wait()
}

func (s *Server) productNow() time.Time {
	return time.Now().In(productLocation(s.config.TimeZone))
}

func productLocation(name string) *time.Location {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(name)
	if err == nil {
		return location
	}
	if name == "Asia/Shanghai" || name == "PRC" {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return time.UTC
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /v1/daily-picks/today", s.withInternalAuth(s.handleTodayPick))
	s.mux.HandleFunc("GET /v1/daily-picks/recent", s.withInternalAuth(s.handleRecentDailyPicks))
	s.mux.HandleFunc("GET /v1/search", s.withInternalAuth(s.handleSearch))
	s.mux.HandleFunc("GET /v1/repositories/{owner}/{repo}", s.withInternalAuth(s.handleRepository))
	s.mux.HandleFunc("GET /v1/radar/overview", s.withInternalAuth(s.handleRadarOverview))
	s.mux.HandleFunc("GET /v1/radar/trending", s.withInternalAuth(s.handleRadarTrending))
	s.mux.HandleFunc("GET /v1/radar/repositories/{owner}/{repo}/metrics", s.withInternalAuth(s.handleRadarRepositoryMetrics))
	s.mux.HandleFunc("POST /v1/feedback", s.withInternalAuth(s.handleFeedback))
	s.mux.HandleFunc("GET /v1/admin/overview", s.withInternalAuth(s.handleAdminOverview))
	s.mux.HandleFunc("GET /v1/admin/candidates", s.withInternalAuth(s.handleCandidates))
	s.mux.HandleFunc("POST /v1/admin/candidates/{candidateId}/approve", s.withInternalAuth(s.handleApproveCandidate))
	s.mux.HandleFunc("POST /v1/admin/candidates/{candidateId}/publish", s.withInternalAuth(s.handlePublishCandidate))
	s.mux.HandleFunc("POST /v1/admin/repositories/submit", s.withInternalAuth(s.handleSubmitRepository))
	s.mux.HandleFunc("POST /v1/admin/recrawl", s.withInternalAuth(s.handleRecrawl))
	s.mux.HandleFunc("POST /v1/admin/radar/watchlist", s.withInternalAuth(s.handleAddRadarWatchlist))
	s.mux.HandleFunc("POST /v1/admin/radar/backfill", s.withInternalAuth(s.handleRadarBackfill))
	s.mux.HandleFunc("POST /v1/admin/daily-picks/generate", s.withInternalAuth(s.handleGenerateDailyPicks))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"service": "prehub-api",
		"status":  "ok",
		"version": "0.1.0",
	})
}

func (s *Server) handleTodayPick(w http.ResponseWriter, r *http.Request) {
	category := domain.NormalizeCategory(r.URL.Query().Get("category"))
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database is unavailable"})
		return
	}
	pick, ok, err := s.store.GetTodayPick(r.Context(), s.productNow(), category)
	if err != nil {
		s.logger.Warn("daily pick db read failed", "error", err)
		writeDatabaseError(w)
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "daily pick not found"})
		return
	}
	writeJSON(w, http.StatusOK, pick)
}

func (s *Server) handleRecentDailyPicks(w http.ResponseWriter, r *http.Request) {
	days, err := parseDaysQuery(r.URL.Query().Get("days"), 7, 31)
	if err != nil {
		writeRequestError(w, err)
		return
	}

	category := domain.NormalizeCategory(r.URL.Query().Get("category"))
	now := s.productNow()
	from := now.AddDate(0, 0, -(days - 1))
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database is unavailable"})
		return
	}
	picks, err := s.store.ListDailyPicks(r.Context(), from, now, category)
	if err != nil {
		s.logger.Warn("recent daily picks db read failed", "error", err)
		writeDatabaseError(w)
		return
	}
	if !dailyPicksIncludeDate(picks, now.Format("2006-01-02")) {
		todayPick, ok, todayErr := s.store.GetTodayPick(r.Context(), now, category)
		if todayErr == nil && ok {
			picks = append([]domain.DailyPick{todayPick}, picks...)
		} else if todayErr != nil {
			s.logger.Warn("today daily pick db read failed", "error", todayErr)
			writeDatabaseError(w)
			return
		}
	}
	writeJSON(w, http.StatusOK, domain.DailyPickHistory{
		FromDate: from.Format("2006-01-02"),
		ToDate:   now.Format("2006-01-02"),
		Days:     days,
		Category: category,
		Picks:    picks,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database is unavailable"})
		return
	}

	page := 1
	if pageRaw := r.URL.Query().Get("page"); pageRaw != "" {
		if p, err := strconv.Atoi(pageRaw); err == nil && p > 0 {
			page = p
		}
	}

	limit := 20
	if limitRaw := r.URL.Query().Get("limit"); limitRaw != "" {
		if l, err := parseLimitQuery(limitRaw, 20, 100); err == nil {
			limit = l
		}
	}

	offset := (page - 1) * limit

	var repositories []domain.Repository
	var total int
	if owner, repoName, ok := parseSearchRepositoryRef(query); ok {
		repos, err := s.searchExactRepository(r.Context(), owner, repoName)
		if err != nil {
			s.logger.Warn("exact github repository search failed", "owner", owner, "repo", repoName, "error", err)
			writeUpstreamError(w)
			return
		}
		repositories = repos
		total = len(repos)
	} else {
		// Try to generate query embedding for hybrid search
		var queryEmbedding []float32
		if s.openaiClient != nil && query != "" {
			emb, err := s.openaiClient.GenerateEmbedding(r.Context(), query)
			if err != nil {
				s.logger.Warn("query embedding generation failed, falling back to full-text search", "error", err)
			} else {
				queryEmbedding = emb
			}
		}

		repos, count, err := s.store.SearchRepositoriesHybrid(r.Context(), query, queryEmbedding, limit, offset)
		if err != nil {
			s.logger.Warn("search db read failed", "error", err)
			writeDatabaseError(w)
			return
		}
		repositories = repos
		total = count
	}

	intent := searchIntent(query)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		logCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.store.SaveSearchQuery(logCtx, query, intent, len(repositories)); err != nil {
			s.logger.Warn("save search query failed", "query", query, "error", err)
		}
	}()

	hasMore := offset+len(repositories) < total
	writeJSON(w, http.StatusOK, domain.SearchResponse{
		Query:    query,
		Intent:   intent,
		Results:  repositories,
		Total:    total,
		HasMore:  hasMore,
		Page:     page,
		PageSize: limit,
	})
}

func (s *Server) searchExactRepository(ctx context.Context, owner string, repoName string) ([]domain.Repository, error) {
	local, ok, err := s.store.GetRepository(ctx, owner, repoName)
	if err != nil {
		return nil, err
	}

	candidate, _, err := s.buildCandidateFromGitHub(ctx, owner, repoName, "search_exact")
	if err != nil {
		if ok {
			return []domain.Repository{local}, nil
		}
		return nil, err
	}
	repo := candidate.Repository
	if candidate.Score != nil {
		result, err := s.store.SaveCandidate(ctx, candidate.Repository, *candidate.Score, "search_exact", "pending_review")
		if err != nil {
			s.logger.Warn("exact github repository persist failed", "repo", candidate.Repository.FullName, "error", err)
		} else {
			repo = result.Candidate.Repository
		}
	}
	return []domain.Repository{repo}, nil
}

func (s *Server) handleRepository(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database is unavailable"})
		return
	}
	repo, ok, err := s.store.GetRepository(r.Context(), r.PathValue("owner"), r.PathValue("repo"))
	if err != nil {
		s.logger.Warn("repository db read failed", "error", err)
		writeDatabaseError(w)
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "repository not found"})
		return
	}
	writeJSON(w, http.StatusOK, repo)
}

func (s *Server) handleRadarOverview(w http.ResponseWriter, r *http.Request) {
	category := domain.NormalizeCategory(r.URL.Query().Get("category"))
	window := r.URL.Query().Get("window")
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database is unavailable"})
		return
	}
	overview, err := s.store.RadarOverview(r.Context(), category, window)
	if err != nil {
		s.logger.Warn("radar overview db read failed", "error", err)
		writeDatabaseError(w)
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (s *Server) handleRadarTrending(w http.ResponseWriter, r *http.Request) {
	category := domain.NormalizeCategory(r.URL.Query().Get("category"))
	window := r.URL.Query().Get("window")
	limit, err := parseLimitQuery(r.URL.Query().Get("limit"), 50, 100)
	if err != nil {
		writeRequestError(w, err)
		return
	}
	potentialOnly := r.URL.Query().Get("potential") == "true"
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database is unavailable"})
		return
	}
	items, err := s.store.ListRadarTrending(r.Context(), category, window, limit, potentialOnly)
	if err != nil {
		s.logger.Warn("radar trending db read failed", "error", err)
		writeDatabaseError(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleRadarRepositoryMetrics(w http.ResponseWriter, r *http.Request) {
	window := r.URL.Query().Get("window")
	owner := r.PathValue("owner")
	repoName := r.PathValue("repo")
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database is unavailable"})
		return
	}
	metrics, ok, err := s.store.RadarMetrics(r.Context(), owner, repoName, window)
	if err != nil {
		s.logger.Warn("radar metrics db read failed", "error", err)
	}
	if err == nil && ok {
		writeJSON(w, http.StatusOK, metrics)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "repository not found"})
}

func (s *Server) handleFeedback(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database is unavailable"})
		return
	}

	var input struct {
		Action             string `json:"action"`
		RepositoryFullName string `json:"repositoryFullName"`
		RepositoryID       string `json:"repositoryId,omitempty"`
		Context            string `json:"context,omitempty"`
	}

	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if input.Action != "like" && input.Action != "dislike" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "action must be 'like' or 'dislike'"})
		return
	}

	repositoryFullName := strings.TrimSpace(input.RepositoryFullName)
	if repositoryFullName == "" {
		repositoryFullName = strings.TrimSpace(input.RepositoryID)
	}
	if repositoryFullName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "repositoryFullName is required"})
		return
	}

	if err := s.store.SaveFeedback(r.Context(), input.Action, repositoryFullName, input.Context); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "repository not found"})
			return
		}
		s.logger.Warn("save feedback failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save feedback"})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (s *Server) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
	if s.store != nil {
		overview, err := s.store.AdminOverview(r.Context())
		if err == nil {
			writeJSON(w, http.StatusOK, overview)
			return
		}
		s.logger.Warn("admin overview db read failed", "error", err)
		writeDatabaseError(w)
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database is unavailable"})
}

func (s *Server) handleCandidates(w http.ResponseWriter, r *http.Request) {
	if s.store != nil {
		candidates, err := s.store.ListCandidates(r.Context(), 50)
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"candidates": candidates})
			return
		}
		s.logger.Warn("candidate db read failed", "error", err)
		writeDatabaseError(w)
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database is unavailable"})
}

func (s *Server) handleApproveCandidate(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database is unavailable"})
		return
	}
	candidate, err := s.store.ApproveCandidate(r.Context(), r.PathValue("candidateId"))
	if err != nil {
		s.logger.Warn("candidate approve failed", "candidate_id", r.PathValue("candidateId"), "error", err)
		writeError(w, http.StatusNotFound, "candidate not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "approved", "candidate": candidate})
}

func (s *Server) handlePublishCandidate(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database is unavailable"})
		return
	}
	var input domain.PublishCandidateInput
	if err := decodeJSON(r, &input); err != nil && !errors.Is(err, io.EOF) {
		writeInvalidRequest(w)
		return
	}
	date := s.productNow()
	if input.Date != "" {
		parsed, err := time.Parse("2006-01-02", input.Date)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "date must be YYYY-MM-DD"})
			return
		}
		date = parsed
	}
	pick, err := s.store.PublishCandidateToday(r.Context(), r.PathValue("candidateId"), date, input.Theme, input.Category)
	if err != nil {
		s.logger.Warn("candidate publish failed", "candidate_id", r.PathValue("candidateId"), "error", err)
		writeError(w, http.StatusBadRequest, "candidate cannot be published")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "published", "dailyPick": pick})
}

func (s *Server) handleGenerateDailyPicks(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database is unavailable"})
		return
	}

	date := s.productNow()

	// Optionally parse date from request body
	var input struct {
		Date string `json:"date"`
	}
	if err := decodeJSON(r, &input); err == nil && input.Date != "" {
		parsed, err := time.Parse("2006-01-02", input.Date)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "date must be YYYY-MM-DD"})
			return
		}
		date = parsed
	}

	response, err := s.store.GenerateDailyPicks(r.Context(), date)
	if err != nil {
		s.logger.Warn("generate daily picks failed", "error", err)
		writeDatabaseError(w)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleSubmitRepository(w http.ResponseWriter, r *http.Request) {
	var input domain.SubmitRepositoryInput
	if err := decodeJSON(r, &input); err != nil {
		writeInvalidRequest(w)
		return
	}

	owner, repoName, err := github.ParseRepositoryURL(input.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "expected github.com/{owner}/{repo}")
		return
	}

	candidate, rate, err := s.buildCandidateFromGitHub(r.Context(), owner, repoName, input.Source)
	if err != nil {
		s.logger.Warn("github submit failed", "owner", owner, "repo", repoName, "error", err)
		writeJSON(w, http.StatusBadGateway, domain.SubmitRepositoryResponse{
			Status:  "failed",
			Message: "repository fetch failed",
		})
		return
	}
	if s.store != nil && candidate.Score != nil {
		result, err := s.store.SaveCandidate(r.Context(), candidate.Repository, *candidate.Score, candidate.Source, candidate.Status)
		if err != nil {
			s.logger.Warn("candidate persist failed", "candidate", candidate.Repository.FullName, "error", err)
			writeDatabaseError(w)
			return
		}
		candidate = result.Candidate
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":    "accepted",
		"candidate": candidate,
		"rateLimit": rate,
	})
}

func (s *Server) handleRecrawl(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Query    string `json:"query"`
		PerPage  int    `json:"perPage"`
		Category string `json:"category"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeInvalidRequest(w)
		return
	}
	queries := []string{strings.TrimSpace(input.Query)}
	if queries[0] == "" {
		queries = domain.DefaultSearchQueries(input.Category)
	}

	client := github.New(s.config.GitHubToken, s.config.GitHubAPIVersion)
	var rate github.RateLimitSnapshot
	var lastErr error
	successfulSearches := 0
	seen := map[string]bool{}
	candidates := []domain.Candidate{}

	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		repositories, searchRate, err := client.SearchRepositoriesWithSort(r.Context(), query, input.PerPage, "updated", "desc")
		if err != nil {
			lastErr = err
			s.logger.Warn("github recrawl failed", "query", query, "error", err)
			continue
		}
		rate = searchRate
		successfulSearches++

		for _, item := range repositories {
			if seen[item.FullName] {
				continue
			}
			seen[item.FullName] = true

			summary := item.Description
			readmeIconURL := ""
			_, readme, readmeRate, err := client.GetReadme(r.Context(), item.Owner.Login, item.Name)
			if err != nil {
				s.logger.Warn("github recrawl readme fetch failed", "owner", item.Owner.Login, "repo", item.Name, "error", err)
			} else {
				rate = readmeRate
				summary = editorial.SummarizeReadme(readme, item.Description)
				readmeIconURL = github.ResolveReadmeIconURL(readme, item.Owner.Login, item.Name, item.DefaultBranch)
			}
			repo := github.ToDomainRepository(item, item.Topics, summary)
			if readmeIconURL != "" {
				repo.AvatarURL = readmeIconURL
			}
			score := scoring.ScoreRepository(repo, time.Now().UTC())
			if !scoring.IsCandidateQualityAcceptable(score) {
				s.logger.Info("recrawl candidate skipped by quality guard", "candidate", repo.FullName, "score", score.Quality)
				continue
			}
			candidate := domain.Candidate{
				ID:           "github_" + strings.ReplaceAll(repo.FullName, "/", "_"),
				Repository:   repo,
				Status:       "discovered",
				QualityScore: score.Quality,
				Score:        &score,
				Source:       "github_search",
			}
			if s.store != nil {
				result, err := s.store.SaveCandidate(r.Context(), repo, score, "github_search", "pending_review")
				if err != nil {
					s.logger.Warn("recrawl candidate persist failed", "candidate", repo.FullName, "error", err)
				} else {
					candidate = result.Candidate
				}
			}
			candidates = append(candidates, candidate)
		}
	}

	if successfulSearches == 0 && lastErr != nil {
		writeUpstreamError(w)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":     "accepted",
		"query":      strings.Join(queries, " | "),
		"queries":    queries,
		"candidates": candidates,
		"rateLimit":  rate,
	})
}

func (s *Server) handleAddRadarWatchlist(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database is unavailable"})
		return
	}
	var input domain.AddWatchlistInput
	if err := decodeJSON(r, &input); err != nil {
		writeInvalidRequest(w)
		return
	}
	owner, repoName, err := github.ParseRepositoryURL(input.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "expected github.com/{owner}/{repo}")
		return
	}
	candidate, rate, err := s.buildCandidateFromGitHub(r.Context(), owner, repoName, "radar_watchlist")
	if err != nil {
		s.logger.Warn("radar watchlist github fetch failed", "owner", owner, "repo", repoName, "error", err)
		writeUpstreamError(w)
		return
	}
	repository, err := s.store.SaveMonitoredRepository(r.Context(), candidate.Repository, input.Category, input.Tier)
	if err != nil {
		s.logger.Warn("radar watchlist persist failed", "repo", candidate.Repository.FullName, "error", err)
		writeDatabaseError(w)
		return
	}
	writeJSON(w, http.StatusAccepted, domain.AddWatchlistResponse{
		Status:     "accepted",
		Repository: repository,
		Category:   domain.NormalizeCategory(input.Category),
		Tier:       input.Tier,
		RateLimit:  rate,
	})
}

func (s *Server) handleRadarBackfill(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database is unavailable"})
		return
	}
	var input domain.RadarBackfillInput
	if err := decodeJSON(r, &input); err != nil && !errors.Is(err, io.EOF) {
		writeInvalidRequest(w)
		return
	}
	category := domain.NormalizeCategory(input.Category)
	windows := normalizeBackfillWindows(input.Window, input.Windows)
	shard, shards, err := normalizeBackfillShard(input.Shard, input.Shards)
	if err != nil {
		writeRequestError(w, err)
		return
	}
	refs, err := s.store.ListMonitoredRepositoryRefs(r.Context(), category, input.Limit, shard, shards)
	if err != nil {
		s.logger.Warn("radar backfill repository list failed", "error", err)
		writeDatabaseError(w)
		return
	}
	if len(refs) == 0 {
		writeJSON(w, http.StatusOK, domain.RadarBackfillResponse{
			Status:   "empty",
			Source:   "clickhouse_gharchive",
			Category: category,
			Results:  []domain.RadarBackfillWindowResult{},
		})
		return
	}

	fullNames := make([]string, 0, len(refs))
	for _, ref := range refs {
		fullNames = append(fullNames, ref.FullName)
	}

	client := clickhouse.New(s.config.ClickHouseURL, s.config.ClickHouseUser, s.config.ClickHousePass)
	now := time.Now().UTC()
	windowEndedAt := now
	if latestEventAt, err := client.FetchLatestWatchEventTime(r.Context()); err != nil {
		s.logger.Warn("radar backfill clickhouse watermark fetch failed", "error", err)
	} else if !latestEventAt.IsZero() {
		watermarkEnd := latestEventAt.Add(time.Second)
		if watermarkEnd.Before(now) {
			windowEndedAt = watermarkEnd
		}
	}
	results := []domain.RadarBackfillWindowResult{}
	for _, window := range windows {
		startedAt := windowEndedAt.Add(-radarHTTPWindowDuration(window))
		trends, err := client.FetchRepositoryTrends(r.Context(), fullNames, clickhouse.TrendFetchOptions{
			Window:        window,
			WindowStarted: startedAt,
			WindowEnded:   windowEndedAt,
			BatchSize:     input.BatchSize,
		})
		if err != nil {
			s.logger.Warn("radar backfill clickhouse fetch failed", "window", window, "error", err)
			writeUpstreamError(w)
			return
		}
		result, err := s.store.SaveExternalRepositoryTrends(r.Context(), refs, trends, "clickhouse_gharchive", window, startedAt, windowEndedAt)
		if err != nil {
			s.logger.Warn("radar backfill persist failed", "window", window, "error", err)
			writeDatabaseError(w)
			return
		}
		results = append(results, result)
	}

	writeJSON(w, http.StatusAccepted, domain.RadarBackfillResponse{
		Status:   "accepted",
		Source:   "clickhouse_gharchive",
		Category: category,
		Results:  results,
	})
}

func (s *Server) buildCandidateFromGitHub(ctx context.Context, owner string, repoName string, source string) (domain.Candidate, github.RateLimitSnapshot, error) {
	client := github.New(s.config.GitHubToken, s.config.GitHubAPIVersion)

	repository, rate, err := client.GetRepository(ctx, owner, repoName)
	if err != nil {
		return domain.Candidate{}, rate, err
	}

	topics, topicRate, err := client.GetTopics(ctx, owner, repoName)
	if err != nil {
		s.logger.Warn("github topics fetch failed", "owner", owner, "repo", repoName, "error", err)
		topics = repository.Topics
	} else {
		rate = topicRate
	}

	summary := repository.Description
	readmeIconURL := ""
	_, readme, readmeRate, err := client.GetReadme(ctx, owner, repoName)
	if err != nil {
		s.logger.Warn("github readme fetch failed", "owner", owner, "repo", repoName, "error", err)
	} else {
		rate = readmeRate
		summary = editorial.SummarizeReadme(readme, repository.Description)
		readmeIconURL = github.ResolveReadmeIconURL(readme, owner, repoName, repository.DefaultBranch)
	}

	repo := github.ToDomainRepository(repository, topics, summary)
	if readmeIconURL != "" {
		repo.AvatarURL = readmeIconURL
	}
	score := scoring.ScoreRepository(repo, time.Now().UTC())
	if source == "" {
		source = "admin_submit"
	}

	return domain.Candidate{
		ID:           "submit_" + strings.ReplaceAll(repo.FullName, "/", "_"),
		Repository:   repo,
		Status:       "pending_review",
		QualityScore: score.Quality,
		Score:        &score,
		Source:       source,
	}, rate, nil
}

func (s *Server) withInternalAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expectedToken := strings.TrimSpace(s.config.InternalAPIToken)
		if expectedToken == "" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "internal API token is not configured"})
			return
		}
		actualToken := strings.TrimSpace(r.Header.Get("x-internal-token"))
		if subtle.ConstantTimeCompare([]byte(actualToken), []byte(expectedToken)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		s.logger.Info(
			"request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", time.Since(startedAt).Milliseconds(),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeInvalidRequest(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "invalid request body")
}

func writeRequestError(w http.ResponseWriter, err error) {
	var requestErr requestError
	if errors.As(err, &requestErr) {
		writeError(w, http.StatusBadRequest, string(requestErr))
		return
	}
	writeInvalidRequest(w)
}

func writeDatabaseError(w http.ResponseWriter) {
	writeError(w, http.StatusInternalServerError, "database operation failed")
}

func writeInternalError(w http.ResponseWriter) {
	writeError(w, http.StatusInternalServerError, "internal server error")
}

func writeUpstreamError(w http.ResponseWriter) {
	writeError(w, http.StatusBadGateway, "upstream service failed")
}

type requestError string

func (e requestError) Error() string {
	return string(e)
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func parseDaysQuery(raw string, fallback int, max int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	days, err := strconv.Atoi(raw)
	if err != nil {
		return 0, requestError("days must be a number")
	}
	if days < 1 {
		return 0, requestError("days must be at least 1")
	}
	if days > max {
		return 0, requestError("days must be at most " + strconv.Itoa(max))
	}
	return days, nil
}

func dailyPicksIncludeDate(picks []domain.DailyPick, date string) bool {
	for _, pick := range picks {
		if pick.Date == date {
			return true
		}
	}
	return false
}

func parseLimitQuery(raw string, fallback int, max int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 0, requestError("limit must be a number")
	}
	if limit < 1 {
		return 0, requestError("limit must be at least 1")
	}
	if limit > max {
		return 0, requestError("limit must be at most " + strconv.Itoa(max))
	}
	return limit, nil
}

func parseSearchRepositoryRef(query string) (string, string, bool) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", "", false
	}

	candidates := []string{cleanRepositorySearchToken(query)}
	for _, field := range strings.Fields(query) {
		candidates = append(candidates, cleanRepositorySearchToken(field))
	}
	lower := strings.ToLower(query)
	for _, marker := range []string{"github.com/", "github.com:"} {
		if index := strings.Index(lower, marker); index >= 0 {
			candidate := query[index:]
			if end := strings.IndexAny(candidate, " \t\r\n，。；、()[]{}<>\"'`"); end >= 0 {
				candidate = candidate[:end]
			}
			candidates = append(candidates, cleanRepositorySearchToken(candidate))
		}
	}

	for _, candidate := range candidates {
		owner, repoName, err := github.ParseRepositoryURL(candidate)
		if err != nil || isReservedGitHubPath(owner) {
			continue
		}
		return owner, repoName, true
	}
	return "", "", false
}

func cleanRepositorySearchToken(value string) string {
	return strings.Trim(value, " \t\r\n<>\"'`，。；、()[]{}")
}

func isReservedGitHubPath(owner string) bool {
	switch strings.ToLower(strings.TrimSpace(owner)) {
	case "about", "collections", "customer-stories", "events", "explore", "features", "issues", "login", "marketplace", "new", "notifications", "orgs", "pricing", "pulls", "readme", "search", "settings", "sponsors", "topics", "trending", "users":
		return true
	default:
		return false
	}
}

func searchIntent(query string) []string {
	normalized := strings.ToLower(strings.TrimSpace(query))
	intent := []string{"repository-discovery"}
	if strings.Contains(normalized, "ai") || strings.Contains(normalized, "llm") || strings.Contains(normalized, "agent") {
		intent = append(intent, "ai")
	}
	if strings.Contains(normalized, "go") || strings.Contains(normalized, "cli") {
		intent = append(intent, "developer-tools")
	}
	if strings.Contains(normalized, "next") || strings.Contains(normalized, "react") {
		intent = append(intent, "web-framework")
	}
	return intent
}

func normalizeBackfillWindows(window string, windows []string) []string {
	if strings.TrimSpace(window) != "" {
		windows = append([]string{window}, windows...)
	}
	if len(windows) == 0 {
		return []string{"1h", "24h", "7d", "30d"}
	}
	normalized := []string{}
	seen := map[string]bool{}
	for _, window := range windows {
		window = strings.ToLower(strings.TrimSpace(window))
		switch window {
		case "1h", "24h", "7d", "30d":
			if !seen[window] {
				seen[window] = true
				normalized = append(normalized, window)
			}
		}
	}
	if len(normalized) == 0 {
		return []string{"1h", "24h", "7d", "30d"}
	}
	return normalized
}

func normalizeBackfillShard(shard int, shards int) (int, int, error) {
	if shards <= 0 {
		shards = 1
	}
	if shards > 128 {
		return 0, 0, requestError("shards must be <= 128")
	}
	if shard < 0 || shard >= shards {
		return 0, 0, requestError("shard must be >= 0 and < shards")
	}
	return shard, shards, nil
}

func radarHTTPWindowDuration(window string) time.Duration {
	switch strings.ToLower(strings.TrimSpace(window)) {
	case "1h":
		return time.Hour
	case "24h":
		return 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	case "7d":
		fallthrough
	default:
		return 7 * 24 * time.Hour
	}
}
