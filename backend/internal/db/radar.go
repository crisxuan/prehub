package db

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/prehub/prehub/backend/internal/domain"
	"github.com/prehub/prehub/backend/internal/scoring"
)

const defaultRadarWindow = "24h"

func (s *Store) SaveMonitoredRepository(ctx context.Context, repo domain.Repository, category string, tier string) (domain.Repository, error) {
	category = domain.NormalizeCategory(category)
	tier = normalizeRadarTier(tier)
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
	if strings.TrimSpace(repo.Summary) != "" {
		if err := upsertReadme(ctx, tx, repositoryID, repo.Summary); err != nil {
			return domain.Repository{}, err
		}
	}
	if err := upsertScore(ctx, tx, repositoryID, scoring.ScoreRepository(repo, time.Now().UTC())); err != nil {
		return domain.Repository{}, err
	}
	if err := upsertMonitoredRepository(ctx, tx, repositoryID, category, tier); err != nil {
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

func (s *Store) GetRepositoryByFullName(ctx context.Context, fullName string) (domain.Repository, error) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 {
		return domain.Repository{}, pgx.ErrNoRows
	}
	repo, ok, err := s.GetRepository(ctx, parts[0], parts[1])
	if err != nil {
		return domain.Repository{}, err
	}
	if !ok {
		return domain.Repository{}, pgx.ErrNoRows
	}
	return repo, nil
}

func (s *Store) SeedRadarFromCandidates(ctx context.Context, category string, limit int) (int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	category = domain.NormalizeCategory(category)
	rows, err := s.pool.Query(ctx, `
		SELECT
			r.id::text,
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
		JOIN repository_candidates c ON c.repository_id = r.id
		LEFT JOIN repository_readmes rr ON rr.repository_id = r.id
		LEFT JOIN repository_topics t ON t.repository_id = r.id
		LEFT JOIN repository_scores sc ON sc.repository_id = r.id
		WHERE NOT EXISTS (
			SELECT 1 FROM monitored_repositories mr
			WHERE mr.repository_id = r.id AND mr.category = $1
		)
		GROUP BY r.id, rr.summary, sc.quality_score
		ORDER BY COALESCE(sc.quality_score, 0) DESC, r.stars_count DESC
		LIMIT $2
	`, category, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	seeded := 0
	for rows.Next() {
		var repositoryID string
		var repo domain.Repository
		var pushedAt time.Time
		var topics []string
		if err := rows.Scan(
			&repositoryID,
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
		); err != nil {
			return seeded, err
		}
		repo.PushedAt = pushedAt.UTC().Format(time.RFC3339)
		repo.Topics = topics
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return seeded, err
		}
		if err := upsertMonitoredRepository(ctx, tx, repositoryID, category, "candidate"); err != nil {
			tx.Rollback(ctx)
			return seeded, err
		}
		if err := recordMetricSnapshot(ctx, tx, repositoryID, repo); err != nil {
			tx.Rollback(ctx)
			return seeded, err
		}
		if err := tx.Commit(ctx); err != nil {
			return seeded, err
		}
		seeded++
	}
	return seeded, rows.Err()
}

func (s *Store) ListDueMonitoredRepositories(ctx context.Context, limit int) ([]domain.MonitoredRepository, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := s.pool.Query(ctx, `
		SELECT
			mr.category,
			mr.tier,
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
		FROM monitored_repositories mr
		JOIN repositories r ON r.id = mr.repository_id
		LEFT JOIN repository_readmes rr ON rr.repository_id = r.id
		LEFT JOIN repository_topics t ON t.repository_id = r.id
		WHERE mr.status = 'active' AND (mr.next_refresh_at IS NULL OR mr.next_refresh_at <= now())
		GROUP BY mr.id, mr.category, mr.tier, r.id, rr.summary
		ORDER BY mr.next_refresh_at ASC NULLS FIRST
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	monitored := []domain.MonitoredRepository{}
	for rows.Next() {
		item := domain.MonitoredRepository{}
		var pushedAt time.Time
		var topics []string
		if err := rows.Scan(
			&item.Category,
			&item.Tier,
			&item.Repository.FullName,
			&item.Repository.Owner,
			&item.Repository.Name,
			&item.Repository.HTMLURL,
			&item.Repository.AvatarURL,
			&item.Repository.Description,
			&item.Repository.Language,
			&item.Repository.Stars,
			&item.Repository.Forks,
			&item.Repository.License,
			&pushedAt,
			&item.Repository.Summary,
			&topics,
		); err != nil {
			return nil, err
		}
		item.Repository.PushedAt = pushedAt.UTC().Format(time.RFC3339)
		item.Repository.Topics = topics
		item.Repository = applyRepositoryNarrative(item.Repository, "")
		monitored = append(monitored, item)
	}
	return monitored, rows.Err()
}

func (s *Store) RadarOverview(ctx context.Context, category string, window string) (domain.RadarOverview, error) {
	category = domain.NormalizeCategory(category)
	window = normalizeRadarWindow(window)

	trending, err := s.ListRadarTrending(ctx, category, window, 10, false)
	if err != nil {
		return domain.RadarOverview{}, err
	}
	potential := radarPotentialFromTrending(trending, 10)
	events := []domain.RadarEvent{}

	var monitoredCount int
	var candidateCount int
	var starDelta int
	var coverage domain.RadarDataCoverage
	if stats, ok, err := s.radarExternalOverviewStats(ctx, category, window); err != nil {
		return domain.RadarOverview{}, err
	} else if ok {
		monitoredCount = stats.monitoredCount
		candidateCount = stats.candidateCount
		starDelta = stats.starDelta
		coverage = stats.coverage
	} else {
		if err := s.pool.QueryRow(ctx, `SELECT count(DISTINCT repository_id) FROM monitored_repositories WHERE status = 'active' AND ($1 = 'all' OR category = $1)`, category).Scan(&monitoredCount); err != nil {
			return domain.RadarOverview{}, err
		}
		if err := s.pool.QueryRow(ctx, `
			SELECT count(DISTINCT c.repository_id)
			FROM repository_candidates c
			WHERE
				$1 = 'all'
				OR c.source = 'global_discovery_' || $1
				OR EXISTS (
					SELECT 1
					FROM monitored_repositories mr
					WHERE mr.repository_id = c.repository_id
						AND mr.status = 'active'
						AND mr.category = $1
				)
		`, category).Scan(&candidateCount); err != nil {
			return domain.RadarOverview{}, err
		}
		starDelta, coverage, err = s.radarWindowSummary(ctx, category, window)
		if err != nil {
			return domain.RadarOverview{}, err
		}
	}

	return domain.RadarOverview{
		Category:       category,
		Window:         window,
		MonitoredCount: monitoredCount,
		StarDelta:      starDelta,
		CandidateCount: candidateCount,
		APIHealth: domain.RadarAPIHealth{
			Status: "ok",
		},
		DataCoverage: coverage,
		TopTrending:  trending,
		TopPotential: potential,
		RecentEvents: events,
	}, nil
}

type radarOverviewStats struct {
	monitoredCount int
	candidateCount int
	starDelta      int
	coverage       domain.RadarDataCoverage
}

func (s *Store) radarExternalOverviewStats(ctx context.Context, category string, window string) (radarOverviewStats, bool, error) {
	seconds := int(radarWindowDuration(window).Seconds())
	freshnessSeconds := int(externalTrendFreshnessDuration(window).Seconds())

	var stats radarOverviewStats
	var externalCount int
	var observedSince time.Time
	var observedUntil time.Time
	var windowStartedAt time.Time
	if err := s.pool.QueryRow(ctx, `
		WITH scoped AS (
			SELECT DISTINCT repository_id
			FROM monitored_repositories
			WHERE status = 'active' AND ($1 = 'all' OR category = $1)
		),
		external_summary AS (
			SELECT
				COALESCE(sum(external.star_delta), 0)::int AS star_delta,
				count(*)::int AS external_count,
				COALESCE(min(external.window_started_at), now() - ($2::double precision * interval '1 second')) AS observed_since,
				COALESCE(max(external.window_ended_at), now()) AS observed_until
			FROM scoped
			JOIN repository_external_trends external ON external.repository_id = scoped.repository_id
				AND external.source = $3
				AND external.trend_window = $4
				AND external.window_ended_at >= now() - ($5::double precision * interval '1 second')
		)
		SELECT
			(SELECT count(*)::int FROM scoped),
			(
				SELECT count(DISTINCT c.repository_id)::int
				FROM repository_candidates c
				WHERE
					$1 = 'all'
					OR c.source = 'global_discovery_' || $1
					OR c.repository_id IN (SELECT repository_id FROM scoped)
			),
			external_summary.star_delta,
			external_summary.external_count,
			external_summary.observed_since,
			external_summary.observed_until,
			now() - ($2::double precision * interval '1 second')
		FROM external_summary
	`, category, seconds, externalTrendSourceClickHouse, window, freshnessSeconds).Scan(
		&stats.monitoredCount,
		&stats.candidateCount,
		&stats.starDelta,
		&externalCount,
		&observedSince,
		&observedUntil,
		&windowStartedAt,
	); err != nil {
		return radarOverviewStats{}, false, err
	}
	if externalCount == 0 {
		return radarOverviewStats{}, false, nil
	}
	if !sufficientExternalTrendCoverage(externalCount, stats.monitoredCount) {
		return radarOverviewStats{}, false, nil
	}
	stats.coverage = radarCoverage(true, observedSince, observedUntil, windowStartedAt)
	return stats, true, nil
}

func radarPotentialFromTrending(items []domain.RadarTrendItem, limit int) []domain.RadarTrendItem {
	potential := []domain.RadarTrendItem{}
	for _, item := range items {
		stars := item.Repository.Stars
		if stars >= 10 && stars <= 12000 {
			potential = append(potential, item)
		}
		if len(potential) >= limit {
			return potential
		}
	}
	if len(potential) > 0 {
		return potential
	}
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Store) radarWindowSummary(ctx context.Context, category string, window string) (int, domain.RadarDataCoverage, error) {
	category = domain.NormalizeCategory(category)
	window = normalizeRadarWindow(window)
	if starDelta, coverage, ok, err := s.externalRadarWindowSummary(ctx, category, window); err != nil {
		return 0, domain.RadarDataCoverage{}, err
	} else if ok {
		return starDelta, coverage, nil
	}
	seconds := int(radarWindowDuration(window).Seconds())
	toleranceSeconds := int(radarBaselineTolerance(window).Seconds())
	freshnessSeconds := int(externalTrendFreshnessDuration(window).Seconds())

	var starDelta int
	var coverageComplete bool
	var observedSince time.Time
	var observedUntil time.Time
	var windowStartedAt time.Time
	if err := s.pool.QueryRow(ctx, `
		WITH scoped AS (
			SELECT DISTINCT repository_id
			FROM monitored_repositories
			WHERE status = 'active' AND ($1 = 'all' OR category = $1)
		),
		per_repo AS (
			SELECT
				CASE
					WHEN external.repository_id IS NOT NULL THEN COALESCE(external.star_delta, 0)
					ELSE GREATEST(
						COALESCE(star_events.star_delta, 0),
						COALESCE(latest.stars_count, r.stars_count) - COALESCE(base.stars_count, earliest.stars_count, r.stars_count)
					)
				END AS star_delta,
				COALESCE(external.window_started_at, base.captured_at, earliest.captured_at, star_events.first_starred_at, latest.captured_at, now()) AS observed_since,
				COALESCE(external.window_ended_at, latest.captured_at, now()) AS observed_until,
				(
					external.repository_id IS NOT NULL
					OR (
						base.captured_at IS NOT NULL
						AND base.captured_at >= now() - (($2::double precision + $3::double precision) * interval '1 second')
					)
				) AS coverage_complete
			FROM scoped mr
			JOIN repositories r ON r.id = mr.repository_id
			LEFT JOIN repository_external_trends external ON external.repository_id = r.id
				AND external.source = 'clickhouse_gharchive'
				AND external.trend_window = $4
				AND external.window_ended_at >= now() - ($5::double precision * interval '1 second')
			LEFT JOIN LATERAL (
				SELECT stars_count, captured_at
				FROM repository_metric_snapshots
				WHERE repository_id = r.id
				ORDER BY captured_at DESC
				LIMIT 1
			) latest ON true
			LEFT JOIN LATERAL (
				SELECT stars_count, captured_at
				FROM repository_metric_snapshots
				WHERE repository_id = r.id AND captured_at <= now() - ($2::double precision * interval '1 second')
				ORDER BY captured_at DESC
				LIMIT 1
			) base ON true
			LEFT JOIN LATERAL (
				SELECT stars_count, captured_at
				FROM repository_metric_snapshots
				WHERE repository_id = r.id
				ORDER BY captured_at ASC
				LIMIT 1
			) earliest ON true
			LEFT JOIN LATERAL (
				SELECT count(*)::int AS star_delta, min(se.starred_at) AS first_starred_at
				FROM repository_star_events se
				WHERE se.repository_id = r.id AND se.starred_at >= now() - ($2::double precision * interval '1 second')
			) star_events ON true
		)
		SELECT
			COALESCE(sum(GREATEST(star_delta, 0)), 0)::int,
			COALESCE(bool_and(coverage_complete), true),
			COALESCE(min(observed_since), now() - ($2::double precision * interval '1 second')),
			COALESCE(max(observed_until), now()),
			now() - ($2::double precision * interval '1 second')
		FROM per_repo
	`, category, seconds, toleranceSeconds, window, freshnessSeconds).Scan(
		&starDelta,
		&coverageComplete,
		&observedSince,
		&observedUntil,
		&windowStartedAt,
	); err != nil {
		return 0, domain.RadarDataCoverage{}, err
	}

	return starDelta, radarCoverage(coverageComplete, observedSince, observedUntil, windowStartedAt), nil
}

func (s *Store) ListRadarTrending(ctx context.Context, category string, window string, limit int, potentialOnly bool) ([]domain.RadarTrendItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	category = domain.NormalizeCategory(category)
	window = normalizeRadarWindow(window)
	if items, ok, err := s.listExternalRadarTrending(ctx, category, window, limit, potentialOnly); err != nil {
		return nil, err
	} else if ok {
		return items, nil
	}
	seconds := int(radarWindowDuration(window).Seconds())
	toleranceSeconds := int(radarBaselineTolerance(window).Seconds())
	freshnessSeconds := int(externalTrendFreshnessDuration(window).Seconds())
	potentialFilter := ""
	if potentialOnly {
		potentialFilter = "AND r.stars_count BETWEEN 10 AND 12000"
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
			topic_list.topics,
			CASE
				WHEN external.repository_id IS NOT NULL THEN COALESCE(external.star_delta, 0)
				ELSE GREATEST(
					COALESCE(star_events.star_delta, 0),
					COALESCE(latest.stars_count, r.stars_count) - COALESCE(base.stars_count, earliest.stars_count, r.stars_count)
				)
			END,
			COALESCE(latest.forks_count, r.forks_count) - COALESCE(base.forks_count, earliest.forks_count, r.forks_count),
			COALESCE(latest.open_issues_count, r.open_issues_count) - COALESCE(base.open_issues_count, earliest.open_issues_count, r.open_issues_count),
			CASE
				WHEN external.repository_id IS NOT NULL THEN COALESCE(external.activity_events, 0)
				ELSE COALESCE(activity.activity_events, 0) + COALESCE(star_events.star_delta, 0)
			END,
			COALESCE(sc.quality_score, 0),
			COALESCE(external.window_started_at, base.captured_at, earliest.captured_at, star_events.first_starred_at, latest.captured_at, now()),
			COALESCE(external.window_ended_at, latest.captured_at, now()),
			now() - ($2::double precision * interval '1 second'),
			(
				external.repository_id IS NOT NULL
				OR (
					base.captured_at IS NOT NULL
					AND base.captured_at >= now() - (($2::double precision + $4::double precision) * interval '1 second')
				)
			)
		FROM (
			SELECT DISTINCT repository_id
			FROM monitored_repositories
			WHERE status = 'active' AND ($1 = 'all' OR category = $1)
		) mr
		JOIN repositories r ON r.id = mr.repository_id
		LEFT JOIN repository_readmes rr ON rr.repository_id = r.id
		LEFT JOIN repository_scores sc ON sc.repository_id = r.id
		LEFT JOIN repository_external_trends external ON external.repository_id = r.id
			AND external.source = 'clickhouse_gharchive'
			AND external.trend_window = $5
			AND external.window_ended_at >= now() - ($6::double precision * interval '1 second')
		LEFT JOIN LATERAL (
			SELECT COALESCE(array_remove(array_agg(t.topic ORDER BY t.topic), NULL), '{}') AS topics
			FROM repository_topics t
			WHERE t.repository_id = r.id
		) topic_list ON true
		LEFT JOIN LATERAL (
			SELECT stars_count, forks_count, open_issues_count, captured_at
			FROM repository_metric_snapshots
			WHERE repository_id = r.id
			ORDER BY captured_at DESC
			LIMIT 1
		) latest ON true
		LEFT JOIN LATERAL (
			SELECT stars_count, forks_count, open_issues_count, captured_at
			FROM repository_metric_snapshots
			WHERE repository_id = r.id AND captured_at <= now() - ($2::double precision * interval '1 second')
			ORDER BY captured_at DESC
			LIMIT 1
		) base ON true
		LEFT JOIN LATERAL (
			SELECT stars_count, forks_count, open_issues_count, captured_at
			FROM repository_metric_snapshots
			WHERE repository_id = r.id
			ORDER BY captured_at ASC
			LIMIT 1
		) earliest ON true
		LEFT JOIN LATERAL (
			SELECT count(*)::int AS activity_events
			FROM repository_activity_events e
			WHERE e.repository_id = r.id AND e.occurred_at >= now() - ($2::double precision * interval '1 second')
		) activity ON true
		LEFT JOIN LATERAL (
			SELECT count(*)::int AS star_delta, min(se.starred_at) AS first_starred_at
			FROM repository_star_events se
			WHERE se.repository_id = r.id AND se.starred_at >= now() - ($2::double precision * interval '1 second')
		) star_events ON true
		WHERE true `+potentialFilter+`
		ORDER BY
			CASE
				WHEN external.repository_id IS NOT NULL THEN COALESCE(external.star_delta, 0)
				ELSE GREATEST(
					COALESCE(star_events.star_delta, 0),
					COALESCE(latest.stars_count, r.stars_count) - COALESCE(base.stars_count, earliest.stars_count, r.stars_count)
				)
			END DESC,
			COALESCE(sc.quality_score, 0) DESC,
			r.stars_count DESC
		LIMIT $3
	`, category, seconds, limit, toleranceSeconds, window, freshnessSeconds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.RadarTrendItem{}
	for rows.Next() {
		var repo domain.Repository
		var pushedAt time.Time
		var observedSince time.Time
		var observedUntil time.Time
		var windowStartedAt time.Time
		var topics []string
		var quality int
		var coverageComplete bool
		item := domain.RadarTrendItem{Window: window}
		if err := rows.Scan(
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
			&item.StarDelta,
			&item.ForkDelta,
			&item.IssueDelta,
			&item.ActivityEvents,
			&quality,
			&observedSince,
			&observedUntil,
			&windowStartedAt,
			&coverageComplete,
		); err != nil {
			return nil, err
		}
		repo.PushedAt = pushedAt.UTC().Format(time.RFC3339)
		repo.Topics = topics
		item.Repository = applyRepositoryNarrative(repo, "")
		item.VelocityScore = radarVelocityScore(item.StarDelta, window)
		item.AccelerationScore = radarAccelerationScore(item.StarDelta, repo.Stars)
		item.DataCoverage = radarCoverage(coverageComplete, observedSince, observedUntil, windowStartedAt)
		item.TrendScore = radarTrendScore(item, quality)
		item.Explanation = radarTrendExplanation(item, quality)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) externalRadarWindowSummary(ctx context.Context, category string, window string) (int, domain.RadarDataCoverage, bool, error) {
	seconds := int(radarWindowDuration(window).Seconds())
	freshnessSeconds := int(externalTrendFreshnessDuration(window).Seconds())

	var starDelta int
	var monitoredCount int
	var count int
	var observedSince time.Time
	var observedUntil time.Time
	var windowStartedAt time.Time
	if err := s.pool.QueryRow(ctx, `
		WITH scoped AS (
			SELECT DISTINCT repository_id
			FROM monitored_repositories
			WHERE status = 'active' AND ($1 = 'all' OR category = $1)
		)
		SELECT
			(SELECT count(*)::int FROM scoped),
			COALESCE(sum(external.star_delta), 0)::int,
			count(*)::int,
			COALESCE(min(external.window_started_at), now() - ($2::double precision * interval '1 second')),
			COALESCE(max(external.window_ended_at), now()),
			now() - ($2::double precision * interval '1 second')
		FROM scoped
		JOIN repository_external_trends external ON external.repository_id = scoped.repository_id
			AND external.source = $3
			AND external.trend_window = $4
			AND external.window_ended_at >= now() - ($5::double precision * interval '1 second')
	`, category, seconds, externalTrendSourceClickHouse, window, freshnessSeconds).Scan(
		&monitoredCount,
		&starDelta,
		&count,
		&observedSince,
		&observedUntil,
		&windowStartedAt,
	); err != nil {
		return 0, domain.RadarDataCoverage{}, false, err
	}
	if count == 0 {
		return 0, domain.RadarDataCoverage{}, false, nil
	}
	if !sufficientExternalTrendCoverage(count, monitoredCount) {
		return 0, domain.RadarDataCoverage{}, false, nil
	}
	return starDelta, radarCoverage(true, observedSince, observedUntil, windowStartedAt), true, nil
}

func (s *Store) listExternalRadarTrending(ctx context.Context, category string, window string, limit int, potentialOnly bool) ([]domain.RadarTrendItem, bool, error) {
	freshnessSeconds := int(externalTrendFreshnessDuration(window).Seconds())
	monitoredCount, freshCount, err := s.externalTrendCoverage(ctx, category, window, freshnessSeconds)
	if err != nil {
		return nil, false, err
	}
	if !sufficientExternalTrendCoverage(freshCount, monitoredCount) {
		return nil, false, nil
	}
	potentialFilter := ""
	if potentialOnly {
		potentialFilter = "AND repo.stars_count BETWEEN 10 AND 12000"
	}

	rows, err := s.pool.Query(ctx, `
		WITH scoped AS (
			SELECT DISTINCT repository_id
			FROM monitored_repositories
			WHERE status = 'active' AND ($1 = 'all' OR category = $1)
		),
		ranked AS (
			SELECT
				repo.id,
				repo.full_name,
				repo.owner,
				repo.name,
				repo.html_url,
				COALESCE(repo.avatar_url, '') AS avatar_url,
				repo.description,
				COALESCE(repo.primary_language, '') AS primary_language,
				repo.stars_count,
				repo.forks_count,
				COALESCE(repo.license_key, 'unknown') AS license_key,
				COALESCE(repo.pushed_at, now()) AS pushed_at,
				COALESCE(readme.summary, repo.description) AS summary,
				COALESCE(score.quality_score, 0) AS quality_score,
				external.star_delta,
				external.activity_events,
				external.window_started_at,
				external.window_ended_at
			FROM scoped
			JOIN repository_external_trends external ON external.repository_id = scoped.repository_id
				AND external.source = $4
				AND external.trend_window = $2
				AND external.window_ended_at >= now() - ($5::double precision * interval '1 second')
			JOIN repositories repo ON repo.id = scoped.repository_id
			LEFT JOIN repository_readmes readme ON readme.repository_id = repo.id
			LEFT JOIN repository_scores score ON score.repository_id = repo.id
			WHERE true `+potentialFilter+`
			ORDER BY external.star_delta DESC, COALESCE(score.quality_score, 0) DESC, repo.stars_count DESC
			LIMIT $3
		)
		SELECT
			ranked.full_name,
			ranked.owner,
			ranked.name,
			ranked.html_url,
			ranked.avatar_url,
			ranked.description,
			ranked.primary_language,
			ranked.stars_count,
			ranked.forks_count,
			ranked.license_key,
			ranked.pushed_at,
			ranked.summary,
			COALESCE(array_remove(array_agg(topic.topic ORDER BY topic.topic), NULL), '{}'),
			ranked.star_delta,
			ranked.activity_events,
			ranked.quality_score,
			ranked.window_started_at,
			ranked.window_ended_at
		FROM ranked
		LEFT JOIN repository_topics topic ON topic.repository_id = ranked.id
		GROUP BY ranked.id, ranked.full_name, ranked.owner, ranked.name, ranked.html_url,
			ranked.avatar_url, ranked.description, ranked.primary_language, ranked.stars_count,
			ranked.forks_count, ranked.license_key, ranked.pushed_at, ranked.summary,
			ranked.star_delta, ranked.activity_events, ranked.quality_score,
			ranked.window_started_at, ranked.window_ended_at
		ORDER BY ranked.star_delta DESC, ranked.quality_score DESC, ranked.stars_count DESC
	`, category, window, limit, externalTrendSourceClickHouse, freshnessSeconds)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	items := []domain.RadarTrendItem{}
	for rows.Next() {
		var repo domain.Repository
		var pushedAt time.Time
		var observedSince time.Time
		var observedUntil time.Time
		var topics []string
		var quality int
		item := domain.RadarTrendItem{Window: window}
		if err := rows.Scan(
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
			&item.StarDelta,
			&item.ActivityEvents,
			&quality,
			&observedSince,
			&observedUntil,
		); err != nil {
			return nil, false, err
		}
		repo.PushedAt = pushedAt.UTC().Format(time.RFC3339)
		repo.Topics = topics
		item.Repository = applyRepositoryNarrative(repo, "")
		item.DataCoverage = radarCoverage(true, observedSince, observedUntil, observedSince)
		item.VelocityScore = radarVelocityScore(item.StarDelta, window)
		item.AccelerationScore = radarAccelerationScore(item.StarDelta, repo.Stars)
		item.TrendScore = radarTrendScore(item, quality)
		item.Explanation = radarTrendExplanation(item, quality)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return items, len(items) > 0, nil
}

func (s *Store) externalTrendCoverage(ctx context.Context, category string, window string, freshnessSeconds int) (int, int, error) {
	var monitoredCount int
	var freshCount int
	if err := s.pool.QueryRow(ctx, `
		WITH scoped AS (
			SELECT DISTINCT repository_id
			FROM monitored_repositories
			WHERE status = 'active' AND ($1 = 'all' OR category = $1)
		)
		SELECT
			(SELECT count(*)::int FROM scoped),
			(
				SELECT count(*)::int
				FROM scoped
				JOIN repository_external_trends external ON external.repository_id = scoped.repository_id
					AND external.source = $2
					AND external.trend_window = $3
					AND external.window_ended_at >= now() - ($4::double precision * interval '1 second')
			)
	`, category, externalTrendSourceClickHouse, window, freshnessSeconds).Scan(&monitoredCount, &freshCount); err != nil {
		return 0, 0, err
	}
	return monitoredCount, freshCount, nil
}

func (s *Store) RadarMetrics(ctx context.Context, owner string, repoName string, window string) (domain.RadarMetricsResponse, bool, error) {
	window = normalizeRadarWindow(window)
	repo, ok, err := s.GetRepository(ctx, owner, repoName)
	if err != nil || !ok {
		return domain.RadarMetricsResponse{}, ok, err
	}
	repositoryID, err := s.repositoryID(ctx, owner, repoName)
	if err != nil {
		return domain.RadarMetricsResponse{}, false, err
	}
	if points, summary, coverage, ok, err := s.externalTrendBuckets(ctx, repositoryID, window, repo.Stars); err != nil {
		return domain.RadarMetricsResponse{}, false, err
	} else if ok {
		return domain.RadarMetricsResponse{
			Repository:   repo,
			Window:       window,
			Points:       points,
			Summary:      summary,
			DataCoverage: coverage,
		}, true, nil
	}
	seconds := int(radarWindowDuration(window).Seconds())
	rows, err := s.pool.Query(ctx, `
		SELECT captured_at, stars_count, forks_count, open_issues_count
		FROM repository_metric_snapshots
		WHERE repository_id = $1 AND (
			captured_at >= now() - ($2::double precision * interval '1 second')
			OR captured_at = (
				SELECT max(captured_at)
				FROM repository_metric_snapshots
				WHERE repository_id = $1 AND captured_at < now() - ($2::double precision * interval '1 second')
			)
		)
		ORDER BY captured_at ASC
	`, repositoryID, seconds)
	if err != nil {
		return domain.RadarMetricsResponse{}, false, err
	}
	defer rows.Close()

	points := []domain.RadarMetricPoint{}
	for rows.Next() {
		var capturedAt time.Time
		point := domain.RadarMetricPoint{}
		if err := rows.Scan(&capturedAt, &point.Stars, &point.Forks, &point.OpenIssues); err != nil {
			return domain.RadarMetricsResponse{}, false, err
		}
		point.CapturedAt = capturedAt.UTC().Format(time.RFC3339)
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return domain.RadarMetricsResponse{}, false, err
	}
	if len(points) == 0 {
		points = append(points, domain.RadarMetricPoint{
			CapturedAt: time.Now().UTC().Format(time.RFC3339),
			Stars:      repo.Stars,
			Forks:      repo.Forks,
		})
	}

	windowStartedAt := time.Now().UTC().Add(-radarWindowDuration(window))
	observedSince := windowStartedAt
	observedUntil := time.Now().UTC()
	coverageComplete := false
	if len(points) > 0 {
		if parsed, err := time.Parse(time.RFC3339, points[0].CapturedAt); err == nil {
			observedSince = parsed
			coverageComplete = !parsed.After(windowStartedAt) && !parsed.Before(windowStartedAt.Add(-radarBaselineTolerance(window)))
		}
		if parsed, err := time.Parse(time.RFC3339, points[len(points)-1].CapturedAt); err == nil {
			observedUntil = parsed
		}
	}

	summary := domain.RadarMetricSummary{}
	if len(points) > 1 {
		first := points[0]
		last := points[len(points)-1]
		summary.StarDelta = last.Stars - first.Stars
		summary.ForkDelta = last.Forks - first.Forks
	}
	if err := s.pool.QueryRow(ctx, `
		SELECT
			(
				SELECT count(*)::int
				FROM repository_activity_events
				WHERE repository_id = $1 AND occurred_at >= now() - ($2::double precision * interval '1 second')
			) +
			(
				SELECT count(*)::int
				FROM repository_star_events
				WHERE repository_id = $1 AND starred_at >= now() - ($2::double precision * interval '1 second')
			)
	`, repositoryID, seconds).Scan(&summary.ActivityEvents); err != nil {
		return domain.RadarMetricsResponse{}, false, err
	}
	starEventDelta := 0
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*)::int
		FROM repository_star_events
		WHERE repository_id = $1 AND starred_at >= now() - ($2::double precision * interval '1 second')
	`, repositoryID, seconds).Scan(&starEventDelta); err != nil {
		return domain.RadarMetricsResponse{}, false, err
	}
	if starEventDelta > summary.StarDelta {
		summary.StarDelta = starEventDelta
	}

	return domain.RadarMetricsResponse{
		Repository:   repo,
		Window:       window,
		Points:       points,
		Summary:      summary,
		DataCoverage: radarCoverage(coverageComplete, observedSince, observedUntil, windowStartedAt),
	}, true, nil
}

func (s *Store) ListRadarEvents(ctx context.Context, category string, limit int) ([]domain.RadarEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	category = domain.NormalizeCategory(category)
	rows, err := s.pool.Query(ctx, `
		SELECT r.full_name, e.event_type, COALESCE(e.actor_login, ''), COALESCE(e.actor_avatar_url, ''), e.occurred_at
		FROM repository_activity_events e
		JOIN repositories r ON r.id = e.repository_id
		JOIN (
			SELECT DISTINCT repository_id
			FROM monitored_repositories
			WHERE status = 'active' AND ($1 = 'all' OR category = $1)
		) mr ON mr.repository_id = r.id
		ORDER BY e.occurred_at DESC
		LIMIT $2
	`, category, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []domain.RadarEvent{}
	for rows.Next() {
		var occurredAt time.Time
		event := domain.RadarEvent{}
		if err := rows.Scan(&event.RepositoryFullName, &event.EventType, &event.ActorLogin, &event.ActorAvatarURL, &occurredAt); err != nil {
			return nil, err
		}
		event.OccurredAt = occurredAt.UTC().Format(time.RFC3339)
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) SaveRepositoryStarEvents(ctx context.Context, owner string, repoName string, events []domain.RepositoryStarEvent) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	repositoryID, err := s.repositoryID(ctx, owner, repoName)
	if err != nil {
		return 0, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	inserted := int64(0)
	for _, event := range events {
		if event.GitHubUserID == 0 || event.StarredAt.IsZero() {
			continue
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO repository_star_events (
				repository_id,
				github_user_id,
				login,
				starred_at,
				ingested_at
			)
			VALUES ($1, $2, $3, $4, now())
			ON CONFLICT (repository_id, github_user_id) DO NOTHING
		`, repositoryID, event.GitHubUserID, event.Login, event.StarredAt.UTC())
		if err != nil {
			return int(inserted), err
		}
		inserted += tag.RowsAffected()
	}
	if err := tx.Commit(ctx); err != nil {
		return int(inserted), err
	}
	return int(inserted), nil
}

func (s *Store) repositoryID(ctx context.Context, owner string, repoName string) (string, error) {
	var repositoryID string
	err := s.pool.QueryRow(ctx, `
		SELECT id::text
		FROM repositories
		WHERE lower(owner) = lower($1) AND lower(name) = lower($2)
	`, owner, repoName).Scan(&repositoryID)
	return repositoryID, err
}

func upsertMonitoredRepository(ctx context.Context, tx pgx.Tx, repositoryID string, category string, tier string) error {
	interval := radarTierInterval(tier)
	_, err := tx.Exec(ctx, `
		INSERT INTO monitored_repositories (
			repository_id,
			category,
			tier,
			status,
			refresh_interval_seconds,
			last_refreshed_at,
			next_refresh_at,
			updated_at
		)
		VALUES ($1, $2, $3, 'active', $4, now(), now() + ($5 * interval '1 second'), now())
		ON CONFLICT (repository_id, category) DO UPDATE SET
			tier = EXCLUDED.tier,
			status = 'active',
			refresh_interval_seconds = EXCLUDED.refresh_interval_seconds,
			last_refreshed_at = now(),
			next_refresh_at = now() + (EXCLUDED.refresh_interval_seconds::double precision * interval '1 second'),
			updated_at = now()
	`, repositoryID, category, tier, int(interval.Seconds()), interval.Seconds())
	return err
}

func recordMetricSnapshot(ctx context.Context, tx pgx.Tx, repositoryID string, repo domain.Repository) error {
	pushedAt, _ := time.Parse(time.RFC3339, repo.PushedAt)
	var pushed any
	if !pushedAt.IsZero() {
		pushed = pushedAt
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO repository_metric_snapshots (
			repository_id,
			stars_count,
			forks_count,
			watchers_count,
			open_issues_count,
			subscribers_count,
			pushed_at,
			source
		)
		VALUES ($1, $2, $3, $2, 0, 0, $4, 'github_rest')
	`, repositoryID, repo.Stars, repo.Forks, pushed)
	return err
}

func normalizeRadarWindow(window string) string {
	switch strings.ToLower(strings.TrimSpace(window)) {
	case "1h", "24h", "7d", "30d", "90d":
		return strings.ToLower(strings.TrimSpace(window))
	default:
		return defaultRadarWindow
	}
}

func radarWindowDuration(window string) time.Duration {
	switch normalizeRadarWindow(window) {
	case "1h":
		return time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	case "90d":
		return 90 * 24 * time.Hour
	case "24h":
		fallthrough
	default:
		return 24 * time.Hour
	}
}

func radarBaselineTolerance(window string) time.Duration {
	switch normalizeRadarWindow(window) {
	case "1h":
		return 2 * time.Hour
	case "24h":
		return 6 * time.Hour
	case "7d", "30d", "90d":
		return 24 * time.Hour
	default:
		return 6 * time.Hour
	}
}

func externalTrendFreshnessDuration(window string) time.Duration {
	switch normalizeRadarWindow(window) {
	case "1h":
		return 20 * time.Minute
	case "24h":
		return 2 * time.Hour
	case "7d", "30d", "90d":
		return 26 * time.Hour
	default:
		return 2 * time.Hour
	}
}

func sufficientExternalTrendCoverage(freshCount int, monitoredCount int) bool {
	if monitoredCount <= 0 {
		return false
	}
	return freshCount*10 >= monitoredCount*8
}

func radarCoverage(complete bool, observedSince time.Time, observedUntil time.Time, windowStartedAt time.Time) domain.RadarDataCoverage {
	return domain.RadarDataCoverage{
		Complete:        complete,
		ObservedSince:   observedSince.UTC().Format(time.RFC3339),
		ObservedUntil:   observedUntil.UTC().Format(time.RFC3339),
		WindowStartedAt: windowStartedAt.UTC().Format(time.RFC3339),
	}
}

func normalizeRadarTier(tier string) string {
	switch strings.ToLower(strings.TrimSpace(tier)) {
	case "hot", "watch", "candidate", "archive":
		return strings.ToLower(strings.TrimSpace(tier))
	default:
		return "candidate"
	}
}

func radarTierInterval(tier string) time.Duration {
	switch normalizeRadarTier(tier) {
	case "hot":
		return 10 * time.Minute
	case "watch":
		return 30 * time.Minute
	case "archive":
		return 24 * time.Hour
	case "candidate":
		fallthrough
	default:
		return 6 * time.Hour
	}
}

func radarVelocityScore(starDelta int, window string) float64 {
	hours := radarWindowDuration(window).Hours()
	if hours <= 0 || starDelta <= 0 {
		return 0
	}
	dailyPace := float64(starDelta) / hours * 24
	return math.Min(100, dailyPace*2.5)
}

func radarAccelerationScore(starDelta int, totalStars int) float64 {
	if starDelta <= 0 || totalStars <= 0 {
		return 0
	}
	return math.Min(100, float64(starDelta)/math.Sqrt(float64(totalStars))*18)
}

func radarTrendScore(item domain.RadarTrendItem, quality int) float64 {
	score := float64(item.StarDelta)*1.2 +
		float64(item.ForkDelta)*1.4 +
		float64(item.ActivityEvents)*0.8 +
		item.VelocityScore*0.2 +
		item.AccelerationScore*0.15 +
		float64(quality)*0.25
	if item.Repository.Stars > 12000 {
		score -= 8
	}
	return math.Max(0, math.Min(100, score))
}

func radarTrendExplanation(item domain.RadarTrendItem, quality int) string {
	if item.StarDelta > 0 {
		if !item.DataCoverage.Complete {
			return "当前窗口历史还在积累，仅能确认自 " + formatRadarCoverageTime(item.DataCoverage.ObservedSince) + " 起已观测新增 " + intToString(item.StarDelta) + " stars；这不是完整 " + item.Window + " 涨幅。"
		}
		return "过去 " + item.Window + " 内新增 " + intToString(item.StarDelta) + " stars，结合质量评分和近期活跃度，适合进入趋势观察。"
	}
	if quality >= 80 {
		return "当前窗口暂无明显 star 增量，但项目质量评分较高，已进入 Radar 监控等待趋势积累。"
	}
	return "项目已进入 Radar 监控，正在积累 star 曲线和活动数据。"
}

func formatRadarCoverageTime(value string) string {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return parsed.UTC().Format("2006-01-02 15:04 UTC")
}

func intToString(value int) string {
	return strconv.Itoa(value)
}
