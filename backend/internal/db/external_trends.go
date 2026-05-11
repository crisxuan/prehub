package db

import (
	"context"
	"strings"
	"time"

	"github.com/prehub/prehub/backend/internal/domain"
)

const externalTrendSourceClickHouse = "clickhouse_gharchive"

func (s *Store) ListMonitoredRepositoryRefs(ctx context.Context, category string, limit int, shard int, shards int) ([]domain.MonitoredRepositoryRef, error) {
	category = domain.NormalizeCategory(category)
	limitSQL := ""
	args := []any{category, shards, shard}
	if limit > 0 {
		limitSQL = " LIMIT $4"
		args = append(args, limit)
	}
	rows, err := s.pool.Query(ctx, `
		WITH refs AS (
			SELECT DISTINCT ON (r.id)
				r.id::text AS repository_id,
				r.full_name,
				mr.category,
				mr.updated_at
			FROM monitored_repositories mr
			JOIN repositories r ON r.id = mr.repository_id
			WHERE mr.status = 'active' AND ($1 = 'all' OR mr.category = $1)
			ORDER BY r.id, mr.updated_at DESC
		),
		ranked AS (
			SELECT
				repository_id,
				full_name,
				category,
				row_number() OVER (ORDER BY lower(full_name), repository_id) - 1 AS ordinal
			FROM refs
		)
		SELECT repository_id, full_name, category
		FROM ranked
		WHERE $2 <= 1 OR mod(ordinal, $2::bigint) = $3::bigint
		ORDER BY ordinal
	`+limitSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := []domain.MonitoredRepositoryRef{}
	for rows.Next() {
		var ref domain.MonitoredRepositoryRef
		if err := rows.Scan(&ref.RepositoryID, &ref.FullName, &ref.Category); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

func (s *Store) SaveExternalRepositoryTrends(ctx context.Context, refs []domain.MonitoredRepositoryRef, trends map[string]domain.ExternalRepositoryTrend, source string, window string, windowStartedAt time.Time, windowEndedAt time.Time) (domain.RadarBackfillWindowResult, error) {
	if source == "" {
		source = externalTrendSourceClickHouse
	}
	window = normalizeRadarWindow(window)
	windowStartedAt = windowStartedAt.UTC()
	windowEndedAt = windowEndedAt.UTC()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.RadarBackfillWindowResult{}, err
	}
	defer tx.Rollback(ctx)

	result := domain.RadarBackfillWindowResult{
		Window:          window,
		RepositoryCount: len(refs),
		WindowStartedAt: windowStartedAt.Format(time.RFC3339),
		WindowEndedAt:   windowEndedAt.Format(time.RFC3339),
	}

	for _, ref := range refs {
		trend := trends[strings.ToLower(ref.FullName)]
		if trend.RepositoryFullName != "" {
			result.MatchedCount++
		}
		result.StarDelta += trend.StarDelta
		result.ActivityEvents += trend.ActivityEvents

		if _, err := tx.Exec(ctx, `
			INSERT INTO repository_external_trends (
				repository_id,
				source,
				trend_window,
				window_started_at,
				window_ended_at,
				star_delta,
				activity_events,
				fetched_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, now())
			ON CONFLICT (repository_id, source, trend_window) DO UPDATE SET
				window_started_at = EXCLUDED.window_started_at,
				window_ended_at = EXCLUDED.window_ended_at,
				star_delta = EXCLUDED.star_delta,
				activity_events = EXCLUDED.activity_events,
				fetched_at = now()
		`, ref.RepositoryID, source, window, windowStartedAt, windowEndedAt, trend.StarDelta, trend.ActivityEvents); err != nil {
			return result, err
		}

		if _, err := tx.Exec(ctx, `
			DELETE FROM repository_external_trend_buckets
			WHERE repository_id = $1 AND source = $2 AND trend_window = $3
		`, ref.RepositoryID, source, window); err != nil {
			return result, err
		}
		for _, bucket := range trend.Buckets {
			if _, err := tx.Exec(ctx, `
				INSERT INTO repository_external_trend_buckets (
					repository_id,
					source,
					trend_window,
					bucket_started_at,
					bucket_ended_at,
					star_delta,
					activity_events,
					fetched_at
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7, now())
				ON CONFLICT (repository_id, source, trend_window, bucket_started_at) DO UPDATE SET
					bucket_ended_at = EXCLUDED.bucket_ended_at,
					star_delta = EXCLUDED.star_delta,
					activity_events = EXCLUDED.activity_events,
					fetched_at = now()
			`, ref.RepositoryID, source, window, bucket.BucketStartedAt.UTC(), bucket.BucketEndedAt.UTC(), bucket.StarDelta, bucket.ActivityEvents); err != nil {
				return result, err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

func (s *Store) externalTrendBuckets(ctx context.Context, repositoryID string, window string, currentStars int) ([]domain.RadarMetricPoint, domain.RadarMetricSummary, domain.RadarDataCoverage, bool, error) {
	window = normalizeRadarWindow(window)
	freshnessSeconds := int(externalTrendFreshnessDuration(window).Seconds())
	var windowStartedAt time.Time
	var windowEndedAt time.Time
	var starDelta int
	var activityEvents int
	err := s.pool.QueryRow(ctx, `
		SELECT window_started_at, window_ended_at, star_delta, activity_events
		FROM repository_external_trends
		WHERE repository_id = $1
			AND source = $2
			AND trend_window = $3
			AND fetched_at >= now() - ($4::double precision * interval '1 second')
	`, repositoryID, externalTrendSourceClickHouse, window, freshnessSeconds).Scan(&windowStartedAt, &windowEndedAt, &starDelta, &activityEvents)
	if err != nil {
		return nil, domain.RadarMetricSummary{}, domain.RadarDataCoverage{}, false, nil
	}

	rows, err := s.pool.Query(ctx, `
		SELECT bucket_started_at, bucket_ended_at, star_delta, activity_events
		FROM repository_external_trend_buckets
		WHERE repository_id = $1 AND source = $2 AND trend_window = $3
		ORDER BY bucket_started_at ASC
	`, repositoryID, externalTrendSourceClickHouse, window)
	if err != nil {
		return nil, domain.RadarMetricSummary{}, domain.RadarDataCoverage{}, false, err
	}
	defer rows.Close()

	points := []domain.RadarMetricPoint{{
		CapturedAt: windowStartedAt.UTC().Format(time.RFC3339),
		Stars:      currentStars - starDelta,
	}}
	runningStars := currentStars - starDelta
	lastPointAt := windowStartedAt
	for rows.Next() {
		var bucketStartedAt time.Time
		var bucketEndedAt time.Time
		var bucketStarDelta int
		var bucketActivityEvents int
		if err := rows.Scan(&bucketStartedAt, &bucketEndedAt, &bucketStarDelta, &bucketActivityEvents); err != nil {
			return nil, domain.RadarMetricSummary{}, domain.RadarDataCoverage{}, false, err
		}
		runningStars += bucketStarDelta
		points = append(points, domain.RadarMetricPoint{
			CapturedAt: bucketEndedAt.UTC().Format(time.RFC3339),
			Stars:      runningStars,
		})
		lastPointAt = bucketEndedAt
	}
	if err := rows.Err(); err != nil {
		return nil, domain.RadarMetricSummary{}, domain.RadarDataCoverage{}, false, err
	}
	if lastPointAt.Before(windowEndedAt) {
		points = append(points, domain.RadarMetricPoint{
			CapturedAt: windowEndedAt.UTC().Format(time.RFC3339),
			Stars:      currentStars,
		})
	}

	coverage := radarCoverage(true, windowStartedAt, windowEndedAt, windowStartedAt)
	return points, domain.RadarMetricSummary{StarDelta: starDelta, ActivityEvents: activityEvents}, coverage, true, nil
}
