CREATE TABLE IF NOT EXISTS repository_external_trends (
  repository_id uuid NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  source text NOT NULL,
  trend_window text NOT NULL,
  window_started_at timestamptz NOT NULL,
  window_ended_at timestamptz NOT NULL,
  star_delta integer NOT NULL DEFAULT 0,
  activity_events integer NOT NULL DEFAULT 0,
  fetched_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (repository_id, source, trend_window)
);

CREATE TABLE IF NOT EXISTS repository_external_trend_buckets (
  repository_id uuid NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  source text NOT NULL,
  trend_window text NOT NULL,
  bucket_started_at timestamptz NOT NULL,
  bucket_ended_at timestamptz NOT NULL,
  star_delta integer NOT NULL DEFAULT 0,
  activity_events integer NOT NULL DEFAULT 0,
  fetched_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (repository_id, source, trend_window, bucket_started_at)
);

CREATE INDEX IF NOT EXISTS repository_external_trends_rank_idx
  ON repository_external_trends (source, trend_window, star_delta DESC, fetched_at DESC);

CREATE INDEX IF NOT EXISTS repository_external_trend_buckets_repo_idx
  ON repository_external_trend_buckets (repository_id, source, trend_window, bucket_started_at);
