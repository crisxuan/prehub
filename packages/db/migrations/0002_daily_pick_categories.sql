ALTER TABLE daily_picks
  ADD COLUMN IF NOT EXISTS category text NOT NULL DEFAULT 'ai';

ALTER TABLE daily_picks
  DROP CONSTRAINT IF EXISTS daily_picks_date_key;

CREATE UNIQUE INDEX IF NOT EXISTS daily_picks_date_category_key
  ON daily_picks (date, category);

CREATE INDEX IF NOT EXISTS daily_picks_category_date_idx
  ON daily_picks (category, date DESC);
