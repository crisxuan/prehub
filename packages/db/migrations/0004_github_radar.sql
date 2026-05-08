CREATE TABLE IF NOT EXISTS monitored_repositories (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  repository_id uuid NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  category text NOT NULL DEFAULT 'ai',
  tier text NOT NULL DEFAULT 'candidate',
  status text NOT NULL DEFAULT 'active',
  refresh_interval_seconds integer NOT NULL DEFAULT 21600,
  last_scheduled_at timestamptz,
  last_refreshed_at timestamptz,
  next_refresh_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (repository_id, category)
);

CREATE TABLE IF NOT EXISTS repository_metric_snapshots (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  repository_id uuid NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  captured_at timestamptz NOT NULL DEFAULT now(),
  stars_count integer NOT NULL DEFAULT 0,
  forks_count integer NOT NULL DEFAULT 0,
  watchers_count integer NOT NULL DEFAULT 0,
  open_issues_count integer NOT NULL DEFAULT 0,
  subscribers_count integer NOT NULL DEFAULT 0,
  pushed_at timestamptz,
  source text NOT NULL DEFAULT 'github_rest'
);

CREATE TABLE IF NOT EXISTS repository_activity_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  repository_id uuid REFERENCES repositories(id) ON DELETE CASCADE,
  github_event_id text NOT NULL UNIQUE,
  event_type text NOT NULL,
  actor_login text,
  actor_avatar_url text,
  payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  occurred_at timestamptz NOT NULL,
  ingested_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS repository_star_events (
  repository_id uuid NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  github_user_id bigint NOT NULL,
  login text NOT NULL,
  starred_at timestamptz NOT NULL,
  ingested_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (repository_id, github_user_id)
);

CREATE TABLE IF NOT EXISTS repository_trend_scores (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  repository_id uuid NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  category text NOT NULL DEFAULT 'ai',
  trend_window text NOT NULL,
  star_delta integer NOT NULL DEFAULT 0,
  fork_delta integer NOT NULL DEFAULT 0,
  issue_delta integer NOT NULL DEFAULT 0,
  activity_events integer NOT NULL DEFAULT 0,
  velocity_score numeric(8, 3) NOT NULL DEFAULT 0,
  acceleration_score numeric(8, 3) NOT NULL DEFAULT 0,
  novelty_score numeric(8, 3) NOT NULL DEFAULT 0,
  quality_score numeric(8, 3) NOT NULL DEFAULT 0,
  suspicious_score numeric(8, 3) NOT NULL DEFAULT 0,
  trend_score numeric(8, 3) NOT NULL DEFAULT 0,
  explanation text NOT NULL DEFAULT '',
  calculated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS monitored_repositories_category_status_idx
  ON monitored_repositories (category, status, tier);

CREATE INDEX IF NOT EXISTS monitored_repositories_next_refresh_idx
  ON monitored_repositories (next_refresh_at)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS repository_metric_snapshots_repo_time_idx
  ON repository_metric_snapshots (repository_id, captured_at DESC);

CREATE INDEX IF NOT EXISTS repository_activity_events_repo_time_idx
  ON repository_activity_events (repository_id, occurred_at DESC);

CREATE INDEX IF NOT EXISTS repository_activity_events_type_time_idx
  ON repository_activity_events (event_type, occurred_at DESC);

CREATE INDEX IF NOT EXISTS repository_star_events_repo_time_idx
  ON repository_star_events (repository_id, starred_at DESC);

CREATE INDEX IF NOT EXISTS repository_trend_scores_rank_idx
  ON repository_trend_scores (category, trend_window, trend_score DESC, calculated_at DESC);
