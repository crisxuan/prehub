package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prehub/prehub/backend/internal/config"
	"github.com/prehub/prehub/backend/internal/db"
	"github.com/prehub/prehub/backend/internal/domain"
	"github.com/prehub/prehub/backend/internal/editorial"
	"github.com/prehub/prehub/backend/internal/github"
	"github.com/prehub/prehub/backend/internal/sample"
	"github.com/prehub/prehub/backend/internal/scoring"
)

type Server struct {
	config config.Config
	logger *slog.Logger
	store  *db.Store
	mux    *http.ServeMux
}

func New(cfg config.Config, logger *slog.Logger, store *db.Store) *Server {
	server := &Server{
		config: cfg,
		logger: logger,
		store:  store,
		mux:    http.NewServeMux(),
	}
	server.routes()
	return server
}

func (s *Server) Handler() http.Handler {
	return s.withLogging(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /v1/daily-picks/today", s.withInternalAuth(s.handleTodayPick))
	s.mux.HandleFunc("GET /v1/daily-picks/recent", s.withInternalAuth(s.handleRecentDailyPicks))
	s.mux.HandleFunc("GET /v1/search", s.withInternalAuth(s.handleSearch))
	s.mux.HandleFunc("GET /v1/repositories/{owner}/{repo}", s.withInternalAuth(s.handleRepository))
	s.mux.HandleFunc("POST /v1/feedback", s.withInternalAuth(s.handleFeedback))
	s.mux.HandleFunc("GET /v1/admin/overview", s.withInternalAuth(s.handleAdminOverview))
	s.mux.HandleFunc("GET /v1/admin/candidates", s.withInternalAuth(s.handleCandidates))
	s.mux.HandleFunc("POST /v1/admin/candidates/{candidateId}/approve", s.withInternalAuth(s.handleApproveCandidate))
	s.mux.HandleFunc("POST /v1/admin/candidates/{candidateId}/publish", s.withInternalAuth(s.handlePublishCandidate))
	s.mux.HandleFunc("POST /v1/admin/repositories/submit", s.withInternalAuth(s.handleSubmitRepository))
	s.mux.HandleFunc("POST /v1/admin/recrawl", s.withInternalAuth(s.handleRecrawl))
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
	if s.store != nil {
		pick, ok, err := s.store.GetTodayPick(r.Context(), time.Now().UTC(), category)
		if err == nil && ok {
			writeJSON(w, http.StatusOK, pick)
			return
		}
		if err != nil {
			s.logger.Warn("daily pick db read failed", "error", err)
		}
	}
	writeJSON(w, http.StatusOK, sample.TodayPick())
}

func (s *Server) handleRecentDailyPicks(w http.ResponseWriter, r *http.Request) {
	days, err := parseDaysQuery(r.URL.Query().Get("days"), 7, 31)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	category := domain.NormalizeCategory(r.URL.Query().Get("category"))
	now := time.Now().UTC()
	from := now.AddDate(0, 0, -(days - 1))
	if s.store != nil {
		picks, err := s.store.ListDailyPicks(r.Context(), from, now, category)
		if err == nil {
			writeJSON(w, http.StatusOK, domain.DailyPickHistory{
				FromDate: from.Format("2006-01-02"),
				ToDate:   now.Format("2006-01-02"),
				Days:     days,
				Category: category,
				Picks:    picks,
			})
			return
		}
		s.logger.Warn("recent daily picks db read failed", "error", err)
	}
	writeJSON(w, http.StatusOK, sample.RecentDailyPicks(days, now))
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if s.store != nil {
		repositories, err := s.store.SearchRepositories(r.Context(), query, 50)
		if err == nil {
			writeJSON(w, http.StatusOK, domain.SearchResponse{
				Query:   query,
				Intent:  sample.Intent(query),
				Results: repositories,
			})
			return
		}
		s.logger.Warn("search db read failed", "error", err)
	}
	writeJSON(w, http.StatusOK, sample.Search(query))
}

func (s *Server) handleRepository(w http.ResponseWriter, r *http.Request) {
	if s.store != nil {
		repo, ok, err := s.store.GetRepository(r.Context(), r.PathValue("owner"), r.PathValue("repo"))
		if err == nil && ok {
			writeJSON(w, http.StatusOK, repo)
			return
		}
		if err != nil {
			s.logger.Warn("repository db read failed", "error", err)
		}
	}
	repo, ok := sample.Find(r.PathValue("owner"), r.PathValue("repo"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "repository not found"})
		return
	}
	writeJSON(w, http.StatusOK, repo)
}

func (s *Server) handleFeedback(w http.ResponseWriter, r *http.Request) {
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
	}
	candidates := sample.Candidates()
	pending := 0
	for _, candidate := range candidates {
		if candidate.Status == "pending_review" {
			pending++
		}
	}

	writeJSON(w, http.StatusOK, domain.AdminOverview{
		CandidateCount:      len(candidates),
		PendingReviewCount:  pending,
		ScheduledPickCount:  1,
		LastRateLimitStatus: "not checked",
	})
}

func (s *Server) handleCandidates(w http.ResponseWriter, r *http.Request) {
	if s.store != nil {
		candidates, err := s.store.ListCandidates(r.Context(), 50)
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"candidates": candidates})
			return
		}
		s.logger.Warn("candidate db read failed", "error", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"candidates": sample.Candidates()})
}

func (s *Server) handleApproveCandidate(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database is unavailable"})
		return
	}
	candidate, err := s.store.ApproveCandidate(r.Context(), r.PathValue("candidateId"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
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
	if err := decodeJSON(r, &input); err != nil && err.Error() != "EOF" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	date := time.Now().UTC()
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "published", "dailyPick": pick})
}

func (s *Server) handleSubmitRepository(w http.ResponseWriter, r *http.Request) {
	var input domain.SubmitRepositoryInput
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	owner, repoName, err := github.ParseRepositoryURL(input.URL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	candidate, rate, err := s.buildCandidateFromGitHub(r.Context(), owner, repoName, input.Source)
	if err != nil {
		s.logger.Warn("github submit failed", "owner", owner, "repo", repoName, "error", err)
		writeJSON(w, http.StatusBadGateway, domain.SubmitRepositoryResponse{
			Status:  "failed",
			Message: err.Error(),
		})
		return
	}
	if s.store != nil && candidate.Score != nil {
		result, err := s.store.SaveCandidate(r.Context(), candidate.Repository, *candidate.Score, candidate.Source, candidate.Status)
		if err != nil {
			s.logger.Warn("candidate persist failed", "candidate", candidate.Repository.FullName, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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
		Query   string `json:"query"`
		PerPage int    `json:"perPage"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if strings.TrimSpace(input.Query) == "" {
		input.Query = "topic:ai stars:100..12000 pushed:>2026-02-01 archived:false fork:false"
	}

	client := github.New(s.config.GitHubToken, s.config.GitHubAPIVersion)
	repositories, rate, err := client.SearchRepositoriesWithSort(r.Context(), input.Query, input.PerPage, "updated", "desc")
	if err != nil {
		s.logger.Warn("github recrawl failed", "query", input.Query, "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	candidates := make([]domain.Candidate, 0, len(repositories))
	for _, item := range repositories {
		summary := item.Description
		_, readme, readmeRate, err := client.GetReadme(r.Context(), item.Owner.Login, item.Name)
		if err != nil {
			s.logger.Warn("github recrawl readme fetch failed", "owner", item.Owner.Login, "repo", item.Name, "error", err)
		} else {
			rate = readmeRate
			summary = editorial.SummarizeReadme(readme, item.Description)
		}
		repo := github.ToDomainRepository(item, item.Topics, summary)
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

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":     "accepted",
		"query":      input.Query,
		"candidates": candidates,
		"rateLimit":  rate,
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
	_, readme, readmeRate, err := client.GetReadme(ctx, owner, repoName)
	if err != nil {
		s.logger.Warn("github readme fetch failed", "owner", owner, "repo", repoName, "error", err)
	} else {
		rate = readmeRate
		summary = editorial.SummarizeReadme(readme, repository.Description)
	}

	repo := github.ToDomainRepository(repository, topics, summary)
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
		if s.config.InternalAPIToken != "" && r.Header.Get("x-internal-token") != s.config.InternalAPIToken {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, strings.TrimSpace(err.Error()), http.StatusInternalServerError)
	}
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
		return 0, errors.New("days must be a number")
	}
	if days < 1 {
		return 0, errors.New("days must be at least 1")
	}
	if days > max {
		return 0, errors.New("days must be at most " + strconv.Itoa(max))
	}
	return days, nil
}
