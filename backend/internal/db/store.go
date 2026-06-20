package db

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/prehub/prehub/backend/internal/domain"
	"github.com/prehub/prehub/backend/internal/editorial"
)

type Store struct {
	pool *pgxpool.Pool
}

type SubmitCandidateResult struct {
	Candidate    domain.Candidate
	RepositoryID string
}

func Connect(ctx context.Context, databaseURL string) (*Store, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	if config.ConnConfig.RuntimeParams == nil {
		config.ConnConfig.RuntimeParams = map[string]string{}
	}
	if strings.TrimSpace(config.ConnConfig.RuntimeParams["search_path"]) == "" {
		config.ConnConfig.RuntimeParams["search_path"] = "public"
	}
	if config.ConnConfig.ConnectTimeout == 0 {
		config.ConnConfig.ConnectTimeout = 10 * time.Second
	}
	if config.MaxConns == 0 || config.MaxConns > 4 {
		config.MaxConns = 4
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) Health(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) SaveCandidate(ctx context.Context, repo domain.Repository, score domain.ScoreBreakdown, source string, status string) (SubmitCandidateResult, error) {
	if source == "" {
		source = "github"
	}
	if status == "" {
		status = "pending_review"
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SubmitCandidateResult{}, err
	}
	defer tx.Rollback(ctx)

	repositoryID, err := upsertRepository(ctx, tx, repo)
	if err != nil {
		return SubmitCandidateResult{}, err
	}
	if err := replaceTopics(ctx, tx, repositoryID, repo.Topics); err != nil {
		return SubmitCandidateResult{}, err
	}
	if err := upsertReadme(ctx, tx, repositoryID, repo.Summary); err != nil {
		return SubmitCandidateResult{}, err
	}
	if err := refreshRepositorySearchVector(ctx, tx, repositoryID); err != nil {
		return SubmitCandidateResult{}, err
	}
	if err := upsertScore(ctx, tx, repositoryID, score); err != nil {
		return SubmitCandidateResult{}, err
	}

	candidateID := ""
	candidateStatus := status
	err = tx.QueryRow(ctx, `
		SELECT id::text, status
		FROM repository_candidates
		WHERE repository_id = $1 AND source = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, repositoryID, source).Scan(&candidateID, &candidateStatus)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return SubmitCandidateResult{}, err
	}
	if err == nil {
		if candidateStatus != "approved" && candidateStatus != "published" {
			candidateStatus = status
		}
		_, err = tx.Exec(ctx, `
			UPDATE repository_candidates
			SET
				status = $2,
				score_snapshot_json = jsonb_build_object(
					'quality', $3::int,
					'popularity', $4::int,
					'freshness', $5::int,
					'momentum', $6::int,
					'documentation', $7::int,
					'maintenance', $8::int,
					'community', $9::int,
					'license', $10::int,
					'novelty', $11::int
				),
				ai_summary = $12,
				ai_tags_json = $13::jsonb
			WHERE id = $1
		`, candidateID, candidateStatus, score.Quality, score.Popularity, score.Freshness, score.Momentum, score.Documentation, score.Maintenance, score.Community, score.License, score.Novelty, repo.Summary, "[]")
		if err != nil {
			return SubmitCandidateResult{}, err
		}
	} else {
		err = tx.QueryRow(ctx, `
			INSERT INTO repository_candidates (
				repository_id,
				source,
				status,
				score_snapshot_json,
				ai_summary,
				ai_tags_json
			)
			VALUES (
				$1,
				$2,
				$3,
				jsonb_build_object(
					'quality', $4::int,
					'popularity', $5::int,
					'freshness', $6::int,
					'momentum', $7::int,
					'documentation', $8::int,
					'maintenance', $9::int,
					'community', $10::int,
					'license', $11::int,
					'novelty', $12::int
				),
				$13,
				$14::jsonb
			)
			RETURNING id::text
		`, repositoryID, source, candidateStatus, score.Quality, score.Popularity, score.Freshness, score.Momentum, score.Documentation, score.Maintenance, score.Community, score.License, score.Novelty, repo.Summary, "[]").Scan(&candidateID)
		if err != nil {
			return SubmitCandidateResult{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return SubmitCandidateResult{}, err
	}

	return SubmitCandidateResult{
		Candidate: domain.Candidate{
			ID:           candidateID,
			Repository:   repo,
			Status:       candidateStatus,
			QualityScore: score.Quality,
			Score:        &score,
			Source:       source,
		},
		RepositoryID: repositoryID,
	}, nil
}

func (s *Store) SaveRepository(ctx context.Context, repo domain.Repository, score domain.ScoreBreakdown) (domain.Repository, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Repository{}, err
	}
	defer tx.Rollback(ctx)

	repositoryID, err := upsertRepository(ctx, tx, repo)
	if err != nil {
		return domain.Repository{}, err
	}
	if err := replaceTopics(ctx, tx, repositoryID, repo.Topics); err != nil {
		return domain.Repository{}, err
	}
	if err := upsertReadme(ctx, tx, repositoryID, repo.Summary); err != nil {
		return domain.Repository{}, err
	}
	if err := refreshRepositorySearchVector(ctx, tx, repositoryID); err != nil {
		return domain.Repository{}, err
	}
	if err := upsertScore(ctx, tx, repositoryID, score); err != nil {
		return domain.Repository{}, err
	}
	if err := recordMetricSnapshot(ctx, tx, repositoryID, repo); err != nil {
		return domain.Repository{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Repository{}, err
	}

	return s.GetRepositoryByFullName(ctx, repo.FullName)
}

func (s *Store) ListCandidates(ctx context.Context, limit int) ([]domain.Candidate, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			c.id::text,
			c.status,
			c.source,
			COALESCE(sc.quality_score, 0),
			r.full_name,
			r.owner,
			r.name,
			r.html_url,
			COALESCE(r.avatar_url, ''),
			r.description,
			COALESCE(r.primary_language, ''),
			r.stars_count,
			r.forks_count,
			COALESCE(r.license_key, 'unknown'),
			COALESCE(r.pushed_at, now()),
			COALESCE(rr.summary, r.description),
			COALESCE(sc.popularity_score, 0),
			COALESCE(sc.freshness_score, 0),
			COALESCE(sc.momentum_score, 0),
			COALESCE(sc.documentation_score, 0),
			COALESCE(sc.maintenance_score, 0),
			COALESCE(sc.community_score, 0),
			COALESCE(sc.license_score, 0),
			COALESCE(sc.novelty_score, 0),
			COALESCE(array_remove(array_agg(t.topic ORDER BY t.topic), NULL), '{}')
		FROM repository_candidates c
		JOIN repositories r ON r.id = c.repository_id
		LEFT JOIN repository_scores sc ON sc.repository_id = r.id
		LEFT JOIN repository_readmes rr ON rr.repository_id = r.id
		LEFT JOIN repository_topics t ON t.repository_id = r.id
		GROUP BY c.id, c.status, c.source, sc.quality_score, r.id, rr.summary,
			sc.popularity_score, sc.freshness_score, sc.momentum_score,
			sc.documentation_score, sc.maintenance_score, sc.community_score,
			sc.license_score, sc.novelty_score
		ORDER BY c.created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanCandidates(rows)
}

func (s *Store) AdminOverview(ctx context.Context) (domain.AdminOverview, error) {
	var overview domain.AdminOverview
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM repository_candidates),
			(SELECT count(*) FROM repository_candidates WHERE status = 'pending_review'),
			(SELECT count(*) FROM daily_picks WHERE status IN ('scheduled', 'published')),
			'db-ok'
	`).Scan(&overview.CandidateCount, &overview.PendingReviewCount, &overview.ScheduledPickCount, &overview.LastRateLimitStatus)
	return overview, err
}

// buildOrTsQuery splits query text into words and builds a tsquery string
// that matches ANY word (OR semantics). Tokens are split on whitespace and
// common punctuation (dots, hyphens, underscores, slashes), lowercased, and
// filtered (≤1 char). Returns empty string when fewer than 2 tokens remain
// (no OR needed for a single token — plainto_tsquery handles it).
// The output is a valid tsquery literal like: 'next' | 'js'
func buildOrTsQuery(query string) string {
	tokens := strings.FieldsFunc(query, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '.' || r == '-' || r == '_' || r == '/'
	})
	var parts []string
	for _, t := range tokens {
		t = strings.ToLower(t)
		if len(t) <= 1 {
			continue
		}
		parts = append(parts, pgQuoteLiteral(t))
	}
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts, " | ")
}

// pgQuoteLiteral wraps a string in SQL single quotes with proper escaping.
func pgQuoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func (s *Store) SearchRepositories(ctx context.Context, query string, limit int, offset int) ([]domain.Repository, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	term := strings.TrimSpace(query)
	orQuery := buildOrTsQuery(term)

	var total int
	countQuery := `
		SELECT count(*) FROM repositories r
		WHERE $1 = ''
			OR COALESCE(r.search_vector, ''::tsvector) @@ plainto_tsquery('simple', $1)
			OR ($4 != '' AND COALESCE(r.search_vector, ''::tsvector) @@ $4::tsquery)
	`
	err := s.pool.QueryRow(ctx, countQuery, term, 0, 0, orQuery).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			r.full_name,
			r.owner,
			r.name,
			r.html_url,
			COALESCE(r.avatar_url, ''),
			r.description,
			COALESCE(r.primary_language, ''),
			r.stars_count,
			r.forks_count,
			COALESCE(r.license_key, 'unknown'),
			COALESCE(r.pushed_at, now()),
			COALESCE(rr.summary, r.description),
			COALESCE(array_remove(array_agg(t.topic ORDER BY t.topic), NULL), '{}')
		FROM repositories r
		LEFT JOIN repository_scores sc ON sc.repository_id = r.id
		LEFT JOIN repository_readmes rr ON rr.repository_id = r.id
		LEFT JOIN repository_topics t ON t.repository_id = r.id
		WHERE
			$1 = ''
			OR COALESCE(r.search_vector, ''::tsvector) @@ plainto_tsquery('simple', $1)
			OR ($4 != '' AND COALESCE(r.search_vector, ''::tsvector) @@ $4::tsquery)
		GROUP BY r.id, rr.summary, sc.quality_score
		ORDER BY
			CASE
				WHEN $1 != '' AND COALESCE(r.search_vector, ''::tsvector) @@ plainto_tsquery('simple', $1)
				THEN 1 ELSE 0
			END DESC,
			CASE WHEN $1 = '' THEN 0 ELSE ts_rank_cd(COALESCE(r.search_vector, ''::tsvector), plainto_tsquery('simple', $1)) END DESC,
			COALESCE(sc.quality_score, 0) DESC,
			r.stars_count DESC
		LIMIT $2 OFFSET $3
	`, term, limit, offset, orQuery)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	repositories := []domain.Repository{}
	for rows.Next() {
		repo, err := scanRepositoryRow(rows)
		if err != nil {
			return nil, 0, err
		}
		repositories = append(repositories, repo)
	}
	return repositories, total, rows.Err()
}

func (s *Store) SaveSearchQuery(ctx context.Context, rawQuery string, intent []string, resultCount int) error {
	intentBytes, err := json.Marshal(intent)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO search_queries (raw_query, parsed_intent_json, result_count)
		VALUES ($1, $2::jsonb, $3)
	`, rawQuery, string(intentBytes), resultCount)
	return err
}

func (s *Store) SaveFeedback(ctx context.Context, action string, fullName string, feedbackContext string) error {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO user_feedback (action, repository_id, context)
		SELECT $1, r.id, $3
		FROM repositories r
		WHERE r.full_name = $2
	`, action, fullName, feedbackContext)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) GetRepository(ctx context.Context, owner string, repoName string) (domain.Repository, bool, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			r.full_name,
			r.owner,
			r.name,
			r.html_url,
			COALESCE(r.avatar_url, ''),
			r.description,
			COALESCE(r.primary_language, ''),
			r.stars_count,
			r.forks_count,
			COALESCE(r.license_key, 'unknown'),
			COALESCE(r.pushed_at, now()),
			COALESCE(rr.summary, r.description),
			COALESCE(array_remove(array_agg(t.topic ORDER BY t.topic), NULL), '{}')
		FROM repositories r
		LEFT JOIN repository_readmes rr ON rr.repository_id = r.id
		LEFT JOIN repository_topics t ON t.repository_id = r.id
		WHERE lower(r.owner) = lower($1) AND lower(r.name) = lower($2)
		GROUP BY r.id, rr.summary
	`, owner, repoName)
	if err != nil {
		return domain.Repository{}, false, err
	}
	defer rows.Close()

	if !rows.Next() {
		return domain.Repository{}, false, rows.Err()
	}
	repository, err := scanRepositoryRow(rows)
	if err != nil {
		return domain.Repository{}, false, err
	}
	return repository, true, rows.Err()
}

func (s *Store) GetTodayPick(ctx context.Context, date time.Time, category string) (domain.DailyPick, bool, error) {
	pickDate := date.Format("2006-01-02")
	category = domain.NormalizeCategory(category)
	repositories, err := s.repositoriesForDailyPick(ctx, pickDate, category)
	if err != nil {
		return domain.DailyPick{}, false, err
	}
	if len(repositories) == 0 {
		repositories, err = s.SearchRepositoriesByCategory(ctx, category, 4)
		if err != nil {
			return domain.DailyPick{}, false, err
		}
	}
	if len(repositories) == 0 {
		return domain.DailyPick{}, false, nil
	}

	theme := "自动推荐"
	_ = s.pool.QueryRow(ctx, `SELECT COALESCE(theme, '自动推荐') FROM daily_picks WHERE date = $1 AND ($2 = 'all' OR category = $2) LIMIT 1`, pickDate, category).Scan(&theme)

	alternatives := []domain.Repository{}
	if len(repositories) > 1 {
		alternatives = repositories[1:]
	}
	return domain.DailyPick{
		Date:         pickDate,
		Category:     category,
		Theme:        theme,
		Primary:      repositories[0],
		Alternatives: alternatives,
	}, true, nil
}

func (s *Store) createAutomaticDailyPick(ctx context.Context, pickDate string, category string) (bool, error) {
	category = domain.NormalizeCategory(category)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	repositoryIDs, err := automaticDailyPickRepositoryIDs(ctx, tx, pickDate, category, 4)
	if err != nil {
		return false, err
	}
	if len(repositoryIDs) == 0 {
		return false, nil
	}

	pickID := ""
	theme := "自动推荐"
	editorialTitle := domain.CategoryLabel(category) + " 自动推荐"
	if err := tx.QueryRow(ctx, `
		INSERT INTO daily_picks (date, category, primary_repository_id, theme, editorial_title, status, published_at)
		VALUES ($1, $2, $3, $4, $5, 'published', now())
		ON CONFLICT (date, category) DO UPDATE SET
			primary_repository_id = EXCLUDED.primary_repository_id,
			theme = EXCLUDED.theme,
			editorial_title = EXCLUDED.editorial_title,
			status = 'published',
			published_at = now()
		RETURNING id::text
	`, pickDate, category, repositoryIDs[0], theme, editorialTitle).Scan(&pickID); err != nil {
		return false, err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM daily_pick_items WHERE daily_pick_id = $1`, pickID); err != nil {
		return false, err
	}
	for index, repositoryID := range repositoryIDs {
		repo, err := repositoryByID(ctx, tx, repositoryID)
		if err != nil {
			return false, err
		}
		reason := repo.Reason
		if strings.TrimSpace(reason) == "" {
			reason = "PreHub 根据分类匹配度、质量评分、维护活跃度和近期发现信号自动生成。"
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO daily_pick_items (daily_pick_id, repository_id, position, reason)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT DO NOTHING
		`, pickID, repositoryID, index+1, reason); err != nil {
			return false, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) GenerateDailyPicks(ctx context.Context, date time.Time) (domain.GenerateDailyPicksResponse, error) {
	pickDate := date.Format("2006-01-02")
	categories := []string{"ai", "ai-image", "ai-prompts", "ai-skills", "devtools", "web", "data", "backend"}

	response := domain.GenerateDailyPicksResponse{
		Date:       pickDate,
		Generated:  0,
		Skipped:    0,
		Picks:      []domain.DailyPick{},
		Categories: []string{},
	}

	for _, category := range categories {
		// Check if already published for this date/category
		var status string
		err := s.pool.QueryRow(ctx, `SELECT status FROM daily_picks WHERE date = $1 AND category = $2`, pickDate, category).Scan(&status)
		if err == nil && status == "published" {
			response.Skipped++
			continue
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return response, err
		}

		// Create automatic daily pick for this category
		created, err := s.createAutomaticDailyPick(ctx, pickDate, category)
		if err != nil {
			return response, err
		}

		if created {
			response.Generated++
			response.Categories = append(response.Categories, category)

			// Fetch the created pick to include in response
			pick, ok, err := s.GetTodayPick(ctx, date, category)
			if err == nil && ok {
				response.Picks = append(response.Picks, pick)
			}
		} else {
			response.Skipped++
		}
	}

	return response, nil
}

func (s *Store) SearchRepositoriesByCategory(ctx context.Context, category string, limit int) ([]domain.Repository, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	filter := repositoryCategoryFilterSQL("r", category)
	boost := repositoryCategoryBoostSQL("r", category)
	rows, err := s.pool.Query(ctx, `
		SELECT
			r.full_name,
			r.owner,
			r.name,
			r.html_url,
			COALESCE(r.avatar_url, ''),
			r.description,
			COALESCE(r.primary_language, ''),
			r.stars_count,
			r.forks_count,
			COALESCE(r.license_key, 'unknown'),
			COALESCE(r.pushed_at, now()),
			COALESCE(rr.summary, r.description),
			COALESCE(array_remove(array_agg(t.topic ORDER BY t.topic), NULL), '{}')
		FROM repositories r
		LEFT JOIN repository_scores sc ON sc.repository_id = r.id
		LEFT JOIN repository_readmes rr ON rr.repository_id = r.id
		LEFT JOIN repository_topics t ON t.repository_id = r.id
		WHERE `+filter+`
		GROUP BY r.id, rr.summary, sc.quality_score
		ORDER BY COALESCE(sc.quality_score, 0) + `+boost+` DESC, r.stars_count DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	repositories := []domain.Repository{}
	for rows.Next() {
		repo, err := scanRepositoryRow(rows)
		if err != nil {
			return nil, err
		}
		repositories = append(repositories, repo)
	}
	return repositories, rows.Err()
}

func (s *Store) ListDailyPicks(ctx context.Context, from time.Time, to time.Time, category string) ([]domain.DailyPick, error) {
	category = domain.NormalizeCategory(category)
	rows, err := s.pool.Query(ctx, `
		SELECT
			dp.date,
			dp.category,
			dp.theme,
			dpi.position,
			COALESCE(dpi.reason, ''),
			r.full_name,
			r.owner,
			r.name,
			r.html_url,
			COALESCE(r.avatar_url, ''),
			r.description,
			COALESCE(r.primary_language, ''),
			r.stars_count,
			r.forks_count,
			COALESCE(r.license_key, 'unknown'),
			COALESCE(r.pushed_at, now()),
			COALESCE(rr.summary, r.description),
			COALESCE(array_remove(array_agg(t.topic ORDER BY t.topic), NULL), '{}')
		FROM daily_picks dp
		JOIN daily_pick_items dpi ON dpi.daily_pick_id = dp.id
		JOIN repositories r ON r.id = dpi.repository_id
		LEFT JOIN repository_readmes rr ON rr.repository_id = r.id
		LEFT JOIN repository_topics t ON t.repository_id = r.id
		WHERE dp.date BETWEEN $1 AND $2 AND ($3 = 'all' OR dp.category = $3) AND dp.status IN ('scheduled', 'published')
		GROUP BY dp.id, dp.date, dp.theme, dpi.position, dpi.reason, r.id, rr.summary
		ORDER BY dp.date DESC, dpi.position ASC
	`, from.Format("2006-01-02"), to.Format("2006-01-02"), category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	picks := []domain.DailyPick{}
	pickIndexByDate := map[string]int{}
	for rows.Next() {
		var pickDate time.Time
		var pickCategory string
		var theme string
		var position int
		var storedReason string
		var repo domain.Repository
		var pushedAt time.Time
		var topics []string

		err := rows.Scan(
			&pickDate,
			&pickCategory,
			&theme,
			&position,
			&storedReason,
			&repo.FullName,
			&repo.Owner,
			&repo.Name,
			&repo.HTMLURL,
			&repo.AvatarURL,
			&repo.Description,
			&repo.Language,
			&repo.Stars,
			&repo.Forks,
			&repo.License,
			&pushedAt,
			&repo.Summary,
			&topics,
		)
		if err != nil {
			return nil, err
		}
		repo.PushedAt = pushedAt.UTC().Format(time.RFC3339)
		repo.Topics = topics
		repo = applyRepositoryNarrative(repo, storedReason)

		dateKey := pickDate.Format("2006-01-02")
		index, ok := pickIndexByDate[dateKey]
		if !ok {
			picks = append(picks, domain.DailyPick{
				Date:         dateKey,
				Category:     pickCategory,
				Theme:        theme,
				Alternatives: []domain.Repository{},
			})
			index = len(picks) - 1
			pickIndexByDate[dateKey] = index
		}

		if position == 1 || picks[index].Primary.FullName == "" {
			picks[index].Primary = repo
		} else {
			picks[index].Alternatives = append(picks[index].Alternatives, repo)
		}
	}
	return picks, rows.Err()
}

func (s *Store) ApproveCandidate(ctx context.Context, candidateID string) (domain.Candidate, error) {
	_, err := s.pool.Exec(ctx, `
		UPDATE repository_candidates
		SET status = 'approved', reviewed_at = now()
		WHERE id = $1
	`, candidateID)
	if err != nil {
		return domain.Candidate{}, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			c.id::text,
			c.status,
			c.source,
			COALESCE(sc.quality_score, 0),
			r.full_name,
			r.owner,
			r.name,
			r.html_url,
			COALESCE(r.avatar_url, ''),
			r.description,
			COALESCE(r.primary_language, ''),
			r.stars_count,
			r.forks_count,
			COALESCE(r.license_key, 'unknown'),
			COALESCE(r.pushed_at, now()),
			COALESCE(rr.summary, r.description),
			COALESCE(sc.popularity_score, 0),
			COALESCE(sc.freshness_score, 0),
			COALESCE(sc.momentum_score, 0),
			COALESCE(sc.documentation_score, 0),
			COALESCE(sc.maintenance_score, 0),
			COALESCE(sc.community_score, 0),
			COALESCE(sc.license_score, 0),
			COALESCE(sc.novelty_score, 0),
			COALESCE(array_remove(array_agg(t.topic ORDER BY t.topic), NULL), '{}')
		FROM repository_candidates c
		JOIN repositories r ON r.id = c.repository_id
		LEFT JOIN repository_scores sc ON sc.repository_id = r.id
		LEFT JOIN repository_readmes rr ON rr.repository_id = r.id
		LEFT JOIN repository_topics t ON t.repository_id = r.id
		WHERE c.id = $1
		GROUP BY c.id, c.status, c.source, sc.quality_score, r.id, rr.summary,
			sc.popularity_score, sc.freshness_score, sc.momentum_score,
			sc.documentation_score, sc.maintenance_score, sc.community_score,
			sc.license_score, sc.novelty_score
	`, candidateID)
	if err != nil {
		return domain.Candidate{}, err
	}
	defer rows.Close()

	candidates, err := scanCandidates(rows)
	if err != nil {
		return domain.Candidate{}, err
	}
	if len(candidates) == 0 {
		return domain.Candidate{}, pgx.ErrNoRows
	}
	return candidates[0], nil
}

func (s *Store) PublishCandidateToday(ctx context.Context, candidateID string, date time.Time, theme string, category string) (domain.DailyPick, error) {
	category = domain.NormalizeCategory(category)
	if theme == "" {
		theme = domain.CategoryLabel(category) + " 开源项目"
	}

	var repositoryID string
	err := s.pool.QueryRow(ctx, `SELECT repository_id::text FROM repository_candidates WHERE id = $1`, candidateID).Scan(&repositoryID)
	if err != nil {
		return domain.DailyPick{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.DailyPick{}, err
	}
	defer tx.Rollback(ctx)

	pickID := ""
	pickDate := date.Format("2006-01-02")
	err = tx.QueryRow(ctx, `
		INSERT INTO daily_picks (date, category, primary_repository_id, theme, editorial_title, status, published_at)
		VALUES ($1, $2, $3, $4, $5, 'published', now())
		ON CONFLICT (date, category) DO UPDATE SET
			primary_repository_id = EXCLUDED.primary_repository_id,
			theme = EXCLUDED.theme,
			editorial_title = EXCLUDED.editorial_title,
			status = 'published',
			published_at = now()
		RETURNING id::text
	`, pickDate, category, repositoryID, theme, theme).Scan(&pickID)
	if err != nil {
		return domain.DailyPick{}, err
	}

	_, err = tx.Exec(ctx, `DELETE FROM daily_pick_items WHERE daily_pick_id = $1`, pickID)
	if err != nil {
		return domain.DailyPick{}, err
	}
	primaryRepo, err := repositoryByID(ctx, tx, repositoryID)
	if err != nil {
		return domain.DailyPick{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO daily_pick_items (daily_pick_id, repository_id, position, reason)
		VALUES ($1, $2, 1, $3)
	`, pickID, repositoryID, primaryRepo.Reason)
	if err != nil {
		return domain.DailyPick{}, err
	}

	alternativeIDs, err := alternativeRepositoryIDs(ctx, tx, repositoryID, category, 3)
	if err != nil {
		return domain.DailyPick{}, err
	}
	for index, alternativeID := range alternativeIDs {
		alternativeRepo, err := repositoryByID(ctx, tx, alternativeID)
		if err != nil {
			return domain.DailyPick{}, err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO daily_pick_items (daily_pick_id, repository_id, position, reason)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT DO NOTHING
		`, pickID, alternativeID, index+2, alternativeRepo.Reason)
		if err != nil {
			return domain.DailyPick{}, err
		}
		_, err = tx.Exec(ctx, `
			UPDATE repository_candidates
			SET status = 'published', reviewed_at = now()
			WHERE repository_id = $1 AND status <> 'rejected'
		`, alternativeID)
		if err != nil {
			return domain.DailyPick{}, err
		}
	}

	_, err = tx.Exec(ctx, `UPDATE repository_candidates SET status = 'published', reviewed_at = now() WHERE id = $1`, candidateID)
	if err != nil {
		return domain.DailyPick{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.DailyPick{}, err
	}
	pick, _, err := s.GetTodayPick(ctx, date, category)
	return pick, err
}

func (s *Store) repositoriesForDailyPick(ctx context.Context, date string, category string) ([]domain.Repository, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			r.full_name,
			r.owner,
			r.name,
			r.html_url,
			COALESCE(r.avatar_url, ''),
			r.description,
			COALESCE(r.primary_language, ''),
			r.stars_count,
			r.forks_count,
			COALESCE(r.license_key, 'unknown'),
			COALESCE(r.pushed_at, now()),
			COALESCE(rr.summary, r.description),
			COALESCE(dpi.reason, ''),
			COALESCE(array_remove(array_agg(t.topic ORDER BY t.topic), NULL), '{}')
		FROM daily_picks dp
		JOIN daily_pick_items dpi ON dpi.daily_pick_id = dp.id
		JOIN repositories r ON r.id = dpi.repository_id
		LEFT JOIN repository_readmes rr ON rr.repository_id = r.id
		LEFT JOIN repository_topics t ON t.repository_id = r.id
		WHERE dp.date = $1 AND ($2 = 'all' OR dp.category = $2) AND dp.status IN ('scheduled', 'published')
		GROUP BY r.id, rr.summary, dpi.position, dpi.reason
		ORDER BY dpi.position ASC
	`, date, domain.NormalizeCategory(category))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	repositories := []domain.Repository{}
	for rows.Next() {
		repo, err := scanDailyPickRepositoryRow(rows)
		if err != nil {
			return nil, err
		}
		repositories = append(repositories, repo)
	}
	return repositories, rows.Err()
}

func repositoryByID(ctx context.Context, tx pgx.Tx, repositoryID string) (domain.Repository, error) {
	rows, err := tx.Query(ctx, `
		SELECT
			r.full_name,
			r.owner,
			r.name,
			r.html_url,
			COALESCE(r.avatar_url, ''),
			r.description,
			COALESCE(r.primary_language, ''),
			r.stars_count,
			r.forks_count,
			COALESCE(r.license_key, 'unknown'),
			COALESCE(r.pushed_at, now()),
			COALESCE(rr.summary, r.description),
			COALESCE(array_remove(array_agg(t.topic ORDER BY t.topic), NULL), '{}')
		FROM repositories r
		LEFT JOIN repository_readmes rr ON rr.repository_id = r.id
		LEFT JOIN repository_topics t ON t.repository_id = r.id
		WHERE r.id = $1
		GROUP BY r.id, rr.summary
	`, repositoryID)
	if err != nil {
		return domain.Repository{}, err
	}
	defer rows.Close()

	if rows.Next() {
		return scanRepositoryRow(rows)
	}
	if err := rows.Err(); err != nil {
		return domain.Repository{}, err
	}
	return domain.Repository{}, pgx.ErrNoRows
}

func alternativeRepositoryIDs(ctx context.Context, tx pgx.Tx, primaryRepositoryID string, category string, limit int) ([]string, error) {
	if limit <= 0 {
		return []string{}, nil
	}

	filter := repositoryCategoryFilterSQL("r", category)
	boost := repositoryCategoryBoostSQL("r", category)
	rows, err := tx.Query(ctx, `
		SELECT repository_id::text
		FROM (
			SELECT DISTINCT ON (r.id)
				r.id AS repository_id,
				`+boost+` AS category_fit_score,
				CASE c.status
					WHEN 'approved' THEN 0
					WHEN 'pending_review' THEN 1
					WHEN 'scored' THEN 2
					WHEN 'discovered' THEN 3
					ELSE 4
				END AS status_rank,
				COALESCE(sc.quality_score, 0) AS quality_score,
				r.stars_count,
				c.created_at
			FROM repository_candidates c
			JOIN repositories r ON r.id = c.repository_id
			LEFT JOIN repository_scores sc ON sc.repository_id = r.id
			WHERE c.repository_id <> $1
				AND c.status IN ('approved', 'pending_review', 'scored', 'discovered')
				AND `+filter+`
			ORDER BY r.id, status_rank ASC, category_fit_score DESC, COALESCE(sc.quality_score, 0) DESC, c.created_at DESC
		) ranked
		ORDER BY status_rank ASC, category_fit_score DESC, quality_score DESC, stars_count ASC, created_at DESC
		LIMIT $2
	`, primaryRepositoryID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func automaticDailyPickRepositoryIDs(ctx context.Context, tx pgx.Tx, pickDate string, category string, limit int) ([]string, error) {
	if limit <= 0 {
		return []string{}, nil
	}

	filter := repositoryCategoryFilterSQL("r", category)
	boost := repositoryCategoryBoostSQL("r", category)
	rows, err := tx.Query(ctx, `
		SELECT repository_id::text
		FROM (
			SELECT DISTINCT ON (r.id)
				r.id AS repository_id,
				`+boost+` AS category_fit_score,
				CASE c.status
					WHEN 'approved' THEN 0
					WHEN 'pending_review' THEN 1
					WHEN 'scored' THEN 2
					WHEN 'discovered' THEN 3
					ELSE 4
				END AS status_rank,
				COALESCE(sc.quality_score, 0) AS quality_score,
				COALESCE(sc.novelty_score, 0) AS novelty_score,
				COALESCE(sc.momentum_score, 0) AS momentum_score,
				r.stars_count,
				c.created_at
			FROM repository_candidates c
			JOIN repositories r ON r.id = c.repository_id
			LEFT JOIN repository_scores sc ON sc.repository_id = r.id
			WHERE c.status IN ('approved', 'pending_review', 'scored', 'discovered')
				AND `+filter+`
				AND NOT EXISTS (
					SELECT 1
					FROM daily_pick_items dpi
					JOIN daily_picks dp ON dp.id = dpi.daily_pick_id
					WHERE dpi.repository_id = r.id
						AND dp.status IN ('scheduled', 'published')
						AND dp.date >= $2::date - interval '30 days'
				)
			ORDER BY r.id, status_rank ASC, category_fit_score DESC,
				COALESCE(sc.quality_score, 0) DESC,
				COALESCE(sc.novelty_score, 0) DESC,
				c.created_at DESC
		) ranked
		ORDER BY status_rank ASC, category_fit_score DESC, quality_score DESC,
			novelty_score DESC, momentum_score DESC, stars_count ASC, created_at DESC
		LIMIT $1
	`, limit, pickDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func repositoryCategoryBoostSQL(repositoryAlias string, category string) string {
	id := repositoryAlias + ".id"
	name := repositoryAlias + ".name"
	description := repositoryAlias + ".description"

	topicIn := func(values string) string {
		return "EXISTS (SELECT 1 FROM repository_topics rt WHERE rt.repository_id = " + id + " AND rt.topic IN (" + values + "))"
	}

	switch domain.NormalizeCategory(category) {
	case "all":
		return "0"
	case "ai-image":
		return `(CASE WHEN ` + topicIn("'ai-image-generation','ai-art','inpainting','sdxl','flux','face-swap','video-generation'") + ` THEN 90 ELSE 0 END
			+ CASE WHEN ` + topicIn("'image-generation','image-editing','text-to-image','image-to-image','diffusion','stable-diffusion','comfyui','dalle'") + ` THEN 45 ELSE 0 END
			+ CASE WHEN ` + name + ` ILIKE '%gpt-image%' OR ` + description + ` ILIKE '%gpt-image%' OR ` + description + ` ILIKE '%image generation%' OR ` + description + ` ILIKE '%image editing%' OR ` + description + ` ILIKE '%text-to-image%' OR ` + description + ` ILIKE '%image-to-image%' OR ` + description + ` ILIKE '%ComfyUI%' OR ` + description + ` ILIKE '%diffusion%' THEN 90 ELSE 0 END
			+ CASE WHEN ` + repositoryAlias + `.stars_count BETWEEN 10 AND 5000 THEN 15 ELSE 0 END
			- CASE WHEN ` + name + ` ILIKE 'awesome%' OR ` + description + ` ILIKE '%comprehensive list%' THEN 60 ELSE 0 END)`
	case "ai-prompts":
		return `(CASE WHEN ` + topicIn("'prompt','prompts','prompt-engineering','prompt-management','prompt-library','prompt-template','chatgpt-prompts','system-prompt'") + ` THEN 45 ELSE 0 END
			+ CASE WHEN ` + name + ` ILIKE '%prompt%' OR ` + description + ` ILIKE '%prompt engineering%' OR ` + description + ` ILIKE '%prompt library%' OR ` + description + ` ILIKE '%prompt workflow%' OR ` + description + ` ILIKE '%prompt template%' OR ` + description + ` ILIKE '%system prompt%' OR ` + description + ` ILIKE '%prompt pack%' OR ` + description + ` ILIKE '%prompt driven%' THEN 90 ELSE 0 END
			+ CASE WHEN ` + repositoryAlias + `.stars_count BETWEEN 10 AND 5000 THEN 15 ELSE 0 END
			- CASE WHEN ` + name + ` ILIKE 'awesome%' OR ` + description + ` ILIKE '%jailbreak%' OR ` + description + ` ILIKE '%bypass%' THEN 60 ELSE 0 END)`
	case "ai-skills":
		return `(CASE WHEN ` + topicIn("'skill','skills','agent','agents','ai-agents','mcp','claude-code','codex','coding-agent','agent-client-protocol','workflow','automation','tool-use'") + ` THEN 45 ELSE 0 END
			+ CASE WHEN ` + name + ` ILIKE '%skill%' OR ` + description + ` ILIKE '%codex skill%' OR ` + description + ` ILIKE '%claude code skill%' OR ` + description + ` ILIKE '%agent skill%' OR ` + description + ` ILIKE '%mcp server%' OR ` + description + ` ILIKE '%agent workflow%' OR ` + description + ` ILIKE '%tool use%' OR ` + description + ` ILIKE '%coding agent%' THEN 90 ELSE 0 END
			+ CASE WHEN ` + repositoryAlias + `.stars_count BETWEEN 10 AND 5000 THEN 15 ELSE 0 END
			- CASE WHEN ` + name + ` ILIKE 'awesome%' OR ` + description + ` ILIKE '%comprehensive list%' THEN 60 ELSE 0 END)`
	case "ai":
		return `(CASE WHEN ` + topicIn("'llm','agents','agentic-ai','ai-agents','mcp','rag','inference','local-llm','model-routing','cost-optimization','agent-evaluation','agent-observability','agentops'") + ` THEN 60 ELSE 0 END
			+ CASE WHEN ` + description + ` ILIKE '%LLM%' OR ` + description + ` ILIKE '%agent%' OR ` + description + ` ILIKE '%MCP%' OR ` + description + ` ILIKE '%RAG%' OR ` + description + ` ILIKE '%model routing%' OR ` + description + ` ILIKE '%evaluation%' THEN 30 ELSE 0 END
			+ CASE WHEN ` + repositoryAlias + `.stars_count BETWEEN 100 AND 8000 THEN 10 ELSE 0 END
			- CASE WHEN ` + name + ` ILIKE 'awesome%' OR ` + description + ` ILIKE '%live support%' OR ` + description + ` ILIKE '%customer support%' OR ` + description + ` ILIKE '%comprehensive list%' THEN 80 ELSE 0 END)`
	default:
		return "0"
	}
}

func repositoryCategoryFilterSQL(repositoryAlias string, category string) string {
	id := repositoryAlias + ".id"
	name := repositoryAlias + ".name"
	description := repositoryAlias + ".description"
	language := repositoryAlias + ".primary_language"

	topicIn := func(values string) string {
		return "EXISTS (SELECT 1 FROM repository_topics rt WHERE rt.repository_id = " + id + " AND rt.topic IN (" + values + "))"
	}

	switch domain.NormalizeCategory(category) {
	case "all":
		return "true"
	case "web":
		return "(" + topicIn("'react','nextjs','vue','svelte','angular','web','frontend','css','html','javascript','typescript'") +
			" OR " + language + " IN ('TypeScript', 'JavaScript') OR " + description + " ILIKE '%web%')"
	case "devtools":
		return "(" + topicIn("'cli','tui','terminal','developer-tools','development','tooling','debugger','testing','lint','formatter','devtools'") +
			" OR " + description + " ILIKE '%developer%' OR " + description + " ILIKE '%terminal%' OR " + description + " ILIKE '%cli%')"
	case "data":
		return "(" + topicIn("'database','postgres','postgresql','redis','mysql','sqlite','vector','search','analytics','data','warehouse'") +
			" OR " + description + " ILIKE '%database%' OR " + description + " ILIKE '%postgres%' OR " + description + " ILIKE '%data%')"
	case "backend":
		return "(" + topicIn("'api','server','backend','microservices','distributed-systems','kubernetes','docker','go','rust','java'") +
			" OR " + language + " IN ('Go', 'Rust', 'Java') OR " + description + " ILIKE '%server%')"
	case "ai-image":
		return "(" + repositoryAlias + ".stars_count BETWEEN 10 AND 12000 AND " + name + " NOT ILIKE 'awesome%' AND " + description + " NOT ILIKE '%free%api%key%' AND " +
			description + " NOT ILIKE '%no credit card%' AND " + description + " NOT ILIKE '%comprehensive list%' AND (" +
			topicIn("'ai-image-generation','ai-art','image-generation','image-editing','text-to-image','image-to-image','diffusion','stable-diffusion','comfyui','dalle','inpainting','sdxl','flux','face-swap','video-generation'") +
			" OR " + name + " ILIKE '%gpt-image%' OR " + description + " ILIKE '%gpt-image%' OR " + description + " ILIKE '%image generation%' OR " + description + " ILIKE '%image editing%' OR " +
			description + " ILIKE '%text-to-image%' OR " + description + " ILIKE '%image-to-image%' OR " + description + " ILIKE '%ComfyUI%' OR " + description + " ILIKE '%diffusion%' OR " +
			"(" + description + " ILIKE '%multimodal%' AND " + description + " ILIKE '%image%')))"
	case "ai-prompts":
		return "(" + repositoryAlias + ".stars_count BETWEEN 10 AND 12000 AND " + name + " NOT ILIKE 'awesome%' AND " + description + " NOT ILIKE '%free%api%key%' AND " +
			description + " NOT ILIKE '%no credit card%' AND " + description + " NOT ILIKE '%jailbreak%' AND " + description + " NOT ILIKE '%bypass%' AND (" +
			topicIn("'prompt','prompts','prompt-engineering','prompt-management','prompt-library','prompt-template','chatgpt-prompts','system-prompt'") +
			" OR " + name + " ILIKE '%prompt%' OR " + description + " ILIKE '%prompt engineering%' OR " + description + " ILIKE '%prompt library%' OR " +
			description + " ILIKE '%prompt workflow%' OR " + description + " ILIKE '%prompt template%' OR " + description + " ILIKE '%system prompt%' OR " + description + " ILIKE '%prompt pack%'))"
	case "ai-skills":
		return "(" + repositoryAlias + ".stars_count BETWEEN 10 AND 12000 AND " + name + " NOT ILIKE 'awesome%' AND " + description + " NOT ILIKE '%free%api%key%' AND " +
			description + " NOT ILIKE '%no credit card%' AND " + description + " NOT ILIKE '%comprehensive list%' AND (" +
			topicIn("'skill','skills','agent','agents','ai-agents','mcp','claude-code','codex','coding-agent','agent-client-protocol','workflow','automation','tool-use'") +
			" OR " + name + " ILIKE '%skill%' OR " + description + " ILIKE '%codex skill%' OR " + description + " ILIKE '%claude code skill%' OR " +
			description + " ILIKE '%agent skill%' OR " + description + " ILIKE '%skills/%' OR " + description + " ILIKE '%mcp server%' OR " +
			description + " ILIKE '%agent workflow%' OR " + description + " ILIKE '%tool use%' OR " + description + " ILIKE '%coding agent%'))"
	case "ai":
		fallthrough
	default:
		return "(" + repositoryAlias + ".stars_count BETWEEN 100 AND 12000 AND " + name + " NOT ILIKE 'awesome%' AND " + description + " NOT ILIKE '%free%api%key%' AND " + description + " NOT ILIKE '%no credit card%' AND " +
			description + " NOT ILIKE '%curated list%' AND " + description + " NOT ILIKE '%comprehensive list%' AND " + description + " NOT ILIKE '%comprehensive collection%' AND " + description + " NOT ILIKE 'Feedback for %' AND NOT " +
			topicIn("'free-api-key','free-api-keys','free-gpt','free-llm','awesome','awesome-list'") + " AND (" +
			topicIn("'ai','artificial-intelligence','llm','machine-learning','deep-learning','agents','agentic-ai','openai','gpt','chatgpt','claude','prompt-engineering','mcp','rag','inference','local-llm'") +
			" OR " + description + " ILIKE '%artificial intelligence%' OR " + description + " ILIKE '%LLM%' OR " + description + " ILIKE '%agent%' OR " + description + " ILIKE '%RAG%' OR " + description + " ILIKE '%workflow%'))"
	}
}

func upsertRepository(ctx context.Context, tx pgx.Tx, repo domain.Repository) (string, error) {
	pushedAt, _ := time.Parse(time.RFC3339, repo.PushedAt)
	var pushed any
	if !pushedAt.IsZero() {
		pushed = pushedAt
	}
	htmlURL := repo.HTMLURL
	if strings.TrimSpace(htmlURL) == "" {
		htmlURL = "https://github.com/" + repo.FullName
	}
	avatarURL := strings.TrimSpace(repo.AvatarURL)

	repositoryID := ""
	err := tx.QueryRow(ctx, `
		INSERT INTO repositories (
			full_name,
			owner,
			name,
			html_url,
			avatar_url,
			description,
			primary_language,
			stars_count,
			forks_count,
			watchers_count,
			license_key,
			pushed_at,
			created_at,
			updated_at,
			last_crawled_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $8, $10, $11, now(), now(), now())
		ON CONFLICT (full_name) DO UPDATE SET
			html_url = EXCLUDED.html_url,
			avatar_url = COALESCE(NULLIF(EXCLUDED.avatar_url, ''), repositories.avatar_url),
			description = EXCLUDED.description,
			primary_language = EXCLUDED.primary_language,
			stars_count = EXCLUDED.stars_count,
			forks_count = EXCLUDED.forks_count,
			watchers_count = EXCLUDED.watchers_count,
			license_key = EXCLUDED.license_key,
			pushed_at = EXCLUDED.pushed_at,
			updated_at = now(),
			last_crawled_at = now()
		RETURNING id::text
	`, repo.FullName, repo.Owner, repo.Name, htmlURL, avatarURL, repo.Description, repo.Language, repo.Stars, repo.Forks, repo.License, pushed).Scan(&repositoryID)
	return repositoryID, err
}

func replaceTopics(ctx context.Context, tx pgx.Tx, repositoryID string, topics []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM repository_topics WHERE repository_id = $1`, repositoryID); err != nil {
		return err
	}
	for _, topic := range topics {
		topic = strings.TrimSpace(strings.ToLower(topic))
		if topic == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `INSERT INTO repository_topics (repository_id, topic) VALUES ($1, $2) ON CONFLICT DO NOTHING`, repositoryID, topic); err != nil {
			return err
		}
	}
	return nil
}

func upsertReadme(ctx context.Context, tx pgx.Tx, repositoryID string, summary string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO repository_readmes (repository_id, summary, fetched_at)
		VALUES ($1, $2, now())
		ON CONFLICT (repository_id) DO UPDATE SET
			summary = EXCLUDED.summary,
			fetched_at = now()
	`, repositoryID, summary)
	return err
}

func refreshRepositorySearchVector(ctx context.Context, tx pgx.Tx, repositoryID string) error {
	_, err := tx.Exec(ctx, `
		UPDATE repositories r
		SET search_vector =
			setweight(to_tsvector('simple', COALESCE(r.full_name, '')), 'A') ||
			setweight(to_tsvector('simple', COALESCE(r.description, '')), 'B') ||
			setweight(to_tsvector('simple', COALESCE(r.primary_language, '')), 'C') ||
			setweight(to_tsvector('simple', COALESCE((
				SELECT rr.summary
				FROM repository_readmes rr
				WHERE rr.repository_id = r.id
			), '')), 'B') ||
			setweight(to_tsvector('simple', COALESCE((
				SELECT array_to_string(array_agg(t.topic ORDER BY t.topic), ' ')
				FROM repository_topics t
				WHERE t.repository_id = r.id
			), '')), 'C')
		WHERE r.id = $1
	`, repositoryID)
	return err
}

func upsertScore(ctx context.Context, tx pgx.Tx, repositoryID string, score domain.ScoreBreakdown) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO repository_scores (
			repository_id,
			quality_score,
			popularity_score,
			freshness_score,
			momentum_score,
			documentation_score,
			maintenance_score,
			community_score,
			license_score,
			novelty_score,
			explanation_json,
			scored_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, '{}'::jsonb, now())
		ON CONFLICT (repository_id) DO UPDATE SET
			quality_score = EXCLUDED.quality_score,
			popularity_score = EXCLUDED.popularity_score,
			freshness_score = EXCLUDED.freshness_score,
			momentum_score = EXCLUDED.momentum_score,
			documentation_score = EXCLUDED.documentation_score,
			maintenance_score = EXCLUDED.maintenance_score,
			community_score = EXCLUDED.community_score,
			license_score = EXCLUDED.license_score,
			novelty_score = EXCLUDED.novelty_score,
			scored_at = now()
	`, repositoryID, score.Quality, score.Popularity, score.Freshness, score.Momentum, score.Documentation, score.Maintenance, score.Community, score.License, score.Novelty)
	return err
}

type repositoryScanner interface {
	Scan(dest ...any) error
}

func scanRepositoryRow(row repositoryScanner) (domain.Repository, error) {
	var repo domain.Repository
	var pushedAt time.Time
	var topics []string
	err := row.Scan(
		&repo.FullName,
		&repo.Owner,
		&repo.Name,
		&repo.HTMLURL,
		&repo.AvatarURL,
		&repo.Description,
		&repo.Language,
		&repo.Stars,
		&repo.Forks,
		&repo.License,
		&pushedAt,
		&repo.Summary,
		&topics,
	)
	if err != nil {
		return domain.Repository{}, err
	}
	repo.PushedAt = pushedAt.UTC().Format(time.RFC3339)
	repo.Topics = topics
	return applyRepositoryNarrative(repo, ""), nil
}

func scanDailyPickRepositoryRow(row repositoryScanner) (domain.Repository, error) {
	var repo domain.Repository
	var pushedAt time.Time
	var storedReason string
	var topics []string
	err := row.Scan(
		&repo.FullName,
		&repo.Owner,
		&repo.Name,
		&repo.HTMLURL,
		&repo.AvatarURL,
		&repo.Description,
		&repo.Language,
		&repo.Stars,
		&repo.Forks,
		&repo.License,
		&pushedAt,
		&repo.Summary,
		&storedReason,
		&topics,
	)
	if err != nil {
		return domain.Repository{}, err
	}
	repo.PushedAt = pushedAt.UTC().Format(time.RFC3339)
	repo.Topics = topics
	return applyRepositoryNarrative(repo, storedReason), nil
}

func scanCandidates(rows pgx.Rows) ([]domain.Candidate, error) {
	candidates := []domain.Candidate{}
	for rows.Next() {
		var candidate domain.Candidate
		var repo domain.Repository
		var pushedAt time.Time
		var topics []string
		score := domain.ScoreBreakdown{}

		err := rows.Scan(
			&candidate.ID,
			&candidate.Status,
			&candidate.Source,
			&candidate.QualityScore,
			&repo.FullName,
			&repo.Owner,
			&repo.Name,
			&repo.HTMLURL,
			&repo.AvatarURL,
			&repo.Description,
			&repo.Language,
			&repo.Stars,
			&repo.Forks,
			&repo.License,
			&pushedAt,
			&repo.Summary,
			&score.Popularity,
			&score.Freshness,
			&score.Momentum,
			&score.Documentation,
			&score.Maintenance,
			&score.Community,
			&score.License,
			&score.Novelty,
			&topics,
		)
		if err != nil {
			return nil, err
		}
		score.Quality = candidate.QualityScore
		repo.PushedAt = pushedAt.UTC().Format(time.RFC3339)
		repo.Topics = topics
		repo = applyRepositoryNarrative(repo, "")
		candidate.Repository = repo
		candidate.Score = &score
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

func applyRepositoryNarrative(repo domain.Repository, storedReason string) domain.Repository {
	repo.AvatarURL = repositoryAvatarURL(repo)
	repo = editorial.WriteRepositoryNarrative(repo)
	if isEditorialReason(storedReason) {
		repo.Reason = strings.TrimSpace(storedReason)
	}
	return repo
}

func repositoryAvatarURL(repo domain.Repository) string {
	if strings.TrimSpace(repo.AvatarURL) != "" {
		return strings.TrimSpace(repo.AvatarURL)
	}
	owner := strings.TrimSpace(repo.Owner)
	if owner == "" {
		return ""
	}
	return "https://github.com/" + url.PathEscape(owner) + ".png?size=128"
}

func isEditorialReason(reason string) bool {
	trimmed := strings.TrimSpace(reason)
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "<!--") ||
		strings.Contains(lower, "</picture>") ||
		strings.Contains(lower, "marketing notes") ||
		strings.Contains(lower, "option 1: cloud-hosted") {
		return false
	}
	switch trimmed {
	case "", "Admin approved primary pick", "High-scoring alternative candidate":
		return false
	default:
		return true
	}
}

// Embedding methods for semantic search

func (s *Store) UpsertEmbedding(ctx context.Context, repositoryID string, embedding []float32) error {
	embeddingStr := formatEmbedding(embedding)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO repository_embeddings (repository_id, embedding, updated_at)
		VALUES ($1::uuid, $2::vector, now())
		ON CONFLICT (repository_id) DO UPDATE SET embedding = EXCLUDED.embedding, updated_at = now()
	`, repositoryID, embeddingStr)
	return err
}

func formatEmbedding(embedding []float32) string {
	parts := make([]string, len(embedding))
	for i, v := range embedding {
		parts[i] = strconv.FormatFloat(float64(v), 'f', -1, 32)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

type EmbeddingSearchResult struct {
	Repository domain.Repository
	Similarity float64
}

func (s *Store) SearchByEmbedding(ctx context.Context, embedding []float32, limit int) ([]EmbeddingSearchResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	embeddingStr := formatEmbedding(embedding)
	rows, err := s.pool.Query(ctx, `
		SELECT
			r.full_name, r.owner, r.name, r.html_url, COALESCE(r.avatar_url, ''),
			r.description, COALESCE(r.primary_language, ''), r.stars_count, r.forks_count,
			COALESCE(r.license_key, 'unknown'), COALESCE(r.pushed_at, now()),
			COALESCE(rr.summary, r.description),
			COALESCE(array_remove(array_agg(t.topic ORDER BY t.topic), NULL), '{}'),
			1 - (re.embedding <=> $1::vector) AS similarity
		FROM repository_embeddings re
		JOIN repositories r ON r.id = re.repository_id
		LEFT JOIN repository_scores sc ON sc.repository_id = r.id
		LEFT JOIN repository_readmes rr ON rr.repository_id = r.id
		LEFT JOIN repository_topics t ON t.repository_id = r.id
		GROUP BY r.id, re.embedding, rr.summary, sc.quality_score
		ORDER BY re.embedding <=> $1::vector
		LIMIT $2
	`, embeddingStr, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []EmbeddingSearchResult
	for rows.Next() {
		var repo domain.Repository
		var similarity float64
		var pushedAt time.Time
		err := rows.Scan(
			&repo.FullName, &repo.Owner, &repo.Name, &repo.HTMLURL, &repo.AvatarURL,
			&repo.Description, &repo.Language, &repo.Stars, &repo.Forks,
			&repo.License, &pushedAt, &repo.Summary, &repo.Topics, &similarity,
		)
		if err != nil {
			return nil, err
		}
		repo.PushedAt = pushedAt.UTC().Format(time.RFC3339)
		repo = applyRepositoryNarrative(repo, "")
		results = append(results, EmbeddingSearchResult{
			Repository: repo,
			Similarity: similarity,
		})
	}
	return results, rows.Err()
}

func (s *Store) SearchRepositoriesHybrid(ctx context.Context, query string, queryEmbedding []float32, limit int, offset int) ([]domain.Repository, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	if queryEmbedding == nil || len(queryEmbedding) == 0 {
		return s.SearchRepositories(ctx, query, limit, offset)
	}

	candidateLimit := offset + limit
	if candidateLimit < limit*2 {
		candidateLimit = limit * 2
	}
	if candidateLimit > 100 {
		candidateLimit = 100
	}

	ftResults, total, err := s.SearchRepositories(ctx, query, candidateLimit, 0)
	if err != nil {
		return nil, 0, err
	}
	if offset >= candidateLimit {
		return s.SearchRepositories(ctx, query, limit, offset)
	}

	embResults, err := s.SearchByEmbedding(ctx, queryEmbedding, candidateLimit)
	if err != nil || len(embResults) == 0 {
		return s.SearchRepositories(ctx, query, limit, offset)
	}

	const k = 60
	const ftWeight = 0.6
	const embWeight = 0.4

	type scoredRepo struct {
		repo  domain.Repository
		score float64
	}
	merged := make(map[string]*scoredRepo)

	for rank, repo := range ftResults {
		score := 1.0 / float64(k+rank+1) * ftWeight
		if existing, ok := merged[repo.FullName]; ok {
			existing.score += score
		} else {
			merged[repo.FullName] = &scoredRepo{repo: repo, score: score}
		}
	}

	for rank, result := range embResults {
		score := 1.0 / float64(k+rank+1) * embWeight
		if existing, ok := merged[result.Repository.FullName]; ok {
			existing.score += score
		} else {
			merged[result.Repository.FullName] = &scoredRepo{repo: result.Repository, score: score}
		}
	}

	sorted := make([]scoredRepo, 0, len(merged))
	for _, sr := range merged {
		sorted = append(sorted, *sr)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].score > sorted[j].score
	})

	if offset >= len(sorted) {
		return s.SearchRepositories(ctx, query, limit, offset)
	}
	end := offset + limit
	if end > len(sorted) {
		end = len(sorted)
	}

	result := make([]domain.Repository, end-offset)
	for i, sr := range sorted[offset:end] {
		result[i] = sr.repo
	}

	reportedTotal := total
	if len(sorted) > reportedTotal {
		reportedTotal = len(sorted)
	}
	return result, reportedTotal, nil
}

func (s *Store) ListRepositoriesWithoutEmbedding(ctx context.Context, limit int) ([]domain.Repository, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			r.id, r.full_name, r.owner, r.name, r.html_url, COALESCE(r.avatar_url, ''),
			r.description, COALESCE(r.primary_language, ''), r.stars_count, r.forks_count,
			COALESCE(r.license_key, 'unknown'), COALESCE(r.pushed_at, now()),
			COALESCE(rr.summary, r.description),
			COALESCE(array_remove(array_agg(t.topic ORDER BY t.topic), NULL), '{}')
		FROM repositories r
		LEFT JOIN repository_readmes rr ON rr.repository_id = r.id
		LEFT JOIN repository_topics t ON t.repository_id = r.id
		LEFT JOIN repository_embeddings re ON re.repository_id = r.id
		WHERE re.repository_id IS NULL
		GROUP BY r.id, rr.summary
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []domain.Repository
	for rows.Next() {
		var repo domain.Repository
		var id string
		var pushedAt time.Time
		err := rows.Scan(
			&id, &repo.FullName, &repo.Owner, &repo.Name, &repo.HTMLURL, &repo.AvatarURL,
			&repo.Description, &repo.Language, &repo.Stars, &repo.Forks,
			&repo.License, &pushedAt, &repo.Summary, &repo.Topics,
		)
		if err != nil {
			return nil, err
		}
		repo.PushedAt = pushedAt.Format(time.RFC3339)
		repos = append(repos, repo)
	}
	return repos, rows.Err()
}

func (s *Store) GetRepositoryID(ctx context.Context, fullName string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT id FROM repositories WHERE full_name = $1`, fullName).Scan(&id)
	return id, err
}
