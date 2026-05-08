ALTER TABLE repositories
  ADD COLUMN IF NOT EXISTS avatar_url text;
