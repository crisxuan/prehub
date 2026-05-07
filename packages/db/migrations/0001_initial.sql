CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS repositories (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  github_id bigint UNIQUE,
  node_id text,
  full_name text NOT NULL UNIQUE,
  owner text NOT NULL,
  name text NOT NULL,
  html_url text NOT NULL,
  api_url text,
  description text NOT NULL DEFAULT '',
  homepage_url text,
  default_branch text NOT NULL DEFAULT 'main',
  primary_language text,
  stars_count integer NOT NULL DEFAULT 0,
  forks_count integer NOT NULL DEFAULT 0,
  watchers_count integer NOT NULL DEFAULT 0,
  open_issues_count integer NOT NULL DEFAULT 0,
  license_key text,
  is_fork boolean NOT NULL DEFAULT false,
  is_archived boolean NOT NULL DEFAULT false,
  is_disabled boolean NOT NULL DEFAULT false,
  pushed_at timestamptz,
  created_at timestamptz,
  updated_at timestamptz,
  last_crawled_at timestamptz
);

CREATE TABLE IF NOT EXISTS repository_topics (
  repository_id uuid NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  topic text NOT NULL,
  PRIMARY KEY (repository_id, topic)
);

CREATE TABLE IF NOT EXISTS repository_languages (
  repository_id uuid NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  language text NOT NULL,
  bytes bigint NOT NULL DEFAULT 0,
  percentage numeric(5, 2) NOT NULL DEFAULT 0,
  PRIMARY KEY (repository_id, language)
);

CREATE TABLE IF NOT EXISTS repository_readmes (
  repository_id uuid PRIMARY KEY REFERENCES repositories(id) ON DELETE CASCADE,
  sha text,
  raw_text text,
  summary text,
  fetched_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS repository_embeddings (
  repository_id uuid PRIMARY KEY REFERENCES repositories(id) ON DELETE CASCADE,
  embedding vector(1536),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS repository_scores (
  repository_id uuid PRIMARY KEY REFERENCES repositories(id) ON DELETE CASCADE,
  quality_score integer NOT NULL DEFAULT 0,
  popularity_score integer NOT NULL DEFAULT 0,
  freshness_score integer NOT NULL DEFAULT 0,
  momentum_score integer NOT NULL DEFAULT 0,
  documentation_score integer NOT NULL DEFAULT 0,
  maintenance_score integer NOT NULL DEFAULT 0,
  community_score integer NOT NULL DEFAULT 0,
  license_score integer NOT NULL DEFAULT 0,
  novelty_score integer NOT NULL DEFAULT 0,
  explanation_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  scored_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS repository_candidates (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  repository_id uuid NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  source text NOT NULL,
  status text NOT NULL DEFAULT 'discovered',
  score_snapshot_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  ai_summary text,
  ai_tags_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  editorial_note text,
  rejection_reason text,
  created_at timestamptz NOT NULL DEFAULT now(),
  reviewed_at timestamptz,
  reviewed_by uuid
);

CREATE TABLE IF NOT EXISTS daily_picks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  date date NOT NULL,
  category text NOT NULL DEFAULT 'ai',
  primary_repository_id uuid REFERENCES repositories(id),
  theme text NOT NULL,
  editorial_title text NOT NULL,
  editorial_note text,
  status text NOT NULL DEFAULT 'draft',
  published_at timestamptz,
  UNIQUE (date, category)
);

CREATE TABLE IF NOT EXISTS daily_pick_items (
  daily_pick_id uuid NOT NULL REFERENCES daily_picks(id) ON DELETE CASCADE,
  repository_id uuid NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  position integer NOT NULL,
  reason text NOT NULL DEFAULT '',
  PRIMARY KEY (daily_pick_id, repository_id)
);

CREATE TABLE IF NOT EXISTS admin_users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL UNIQUE,
  role text NOT NULL DEFAULT 'admin',
  created_at timestamptz NOT NULL DEFAULT now(),
  last_login_at timestamptz
);

CREATE TABLE IF NOT EXISTS blacklist_entries (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  type text NOT NULL,
  value text NOT NULL,
  reason text,
  created_by uuid REFERENCES admin_users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (type, value)
);

CREATE TABLE IF NOT EXISTS audit_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id uuid REFERENCES admin_users(id),
  action text NOT NULL,
  entity_type text NOT NULL,
  entity_id text NOT NULL,
  before_json jsonb,
  after_json jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS jobs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  type text NOT NULL,
  payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  status text NOT NULL DEFAULT 'queued',
  attempts integer NOT NULL DEFAULT 0,
  run_at timestamptz NOT NULL DEFAULT now(),
  locked_at timestamptz,
  last_error text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS search_queries (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid,
  raw_query text NOT NULL,
  parsed_intent_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  result_count integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_feedback (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid,
  repository_id uuid REFERENCES repositories(id) ON DELETE SET NULL,
  action text NOT NULL,
  context text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS repositories_full_name_idx ON repositories (full_name);
CREATE INDEX IF NOT EXISTS repositories_language_idx ON repositories (primary_language);
CREATE INDEX IF NOT EXISTS repositories_pushed_at_idx ON repositories (pushed_at DESC);
CREATE INDEX IF NOT EXISTS repository_candidates_status_idx ON repository_candidates (status);
CREATE INDEX IF NOT EXISTS daily_picks_category_date_idx ON daily_picks (category, date DESC);
CREATE INDEX IF NOT EXISTS jobs_status_run_at_idx ON jobs (status, run_at);
CREATE INDEX IF NOT EXISTS repository_embeddings_hnsw_idx ON repository_embeddings USING hnsw (embedding vector_cosine_ops);
