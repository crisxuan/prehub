ALTER TABLE repositories
  ADD COLUMN IF NOT EXISTS search_vector tsvector;

CREATE OR REPLACE FUNCTION repositories_build_search_vector(
  repo_full_name text,
  repo_description text,
  repo_language text,
  readme_summary text,
  topics_text text
) RETURNS tsvector AS $$
  SELECT
    setweight(to_tsvector('simple', COALESCE(repo_full_name, '')), 'A') ||
    setweight(to_tsvector('simple', COALESCE(repo_description, '')), 'B') ||
    setweight(to_tsvector('simple', COALESCE(repo_language, '')), 'C') ||
    setweight(to_tsvector('simple', COALESCE(readme_summary, '')), 'B') ||
    setweight(to_tsvector('simple', COALESCE(topics_text, '')), 'C');
$$ LANGUAGE sql IMMUTABLE;

CREATE OR REPLACE FUNCTION repositories_refresh_search_vector(repository_uuid uuid) RETURNS void AS $$
BEGIN
  UPDATE repositories r
  SET search_vector = repositories_build_search_vector(
    r.full_name,
    r.description,
    r.primary_language,
    (
      SELECT rr.summary
      FROM repository_readmes rr
      WHERE rr.repository_id = r.id
    ),
    (
      SELECT array_to_string(array_agg(t.topic ORDER BY t.topic), ' ')
      FROM repository_topics t
      WHERE t.repository_id = r.id
    )
  )
  WHERE r.id = repository_uuid;
END
$$ LANGUAGE plpgsql;

UPDATE repositories r
SET search_vector = repositories_build_search_vector(
  r.full_name,
  r.description,
  r.primary_language,
  (
    SELECT rr.summary
    FROM repository_readmes rr
    WHERE rr.repository_id = r.id
  ),
  (
    SELECT array_to_string(array_agg(t.topic ORDER BY t.topic), ' ')
    FROM repository_topics t
    WHERE t.repository_id = r.id
  )
);

CREATE INDEX IF NOT EXISTS repositories_search_vector_idx
  ON repositories USING gin(search_vector);

CREATE OR REPLACE FUNCTION repositories_search_vector_update() RETURNS trigger AS $$
BEGIN
  NEW.search_vector := repositories_build_search_vector(
    NEW.full_name,
    NEW.description,
    NEW.primary_language,
    (
      SELECT rr.summary
      FROM repository_readmes rr
      WHERE rr.repository_id = NEW.id
    ),
    (
      SELECT array_to_string(array_agg(t.topic ORDER BY t.topic), ' ')
      FROM repository_topics t
      WHERE t.repository_id = NEW.id
    )
  );
  RETURN NEW;
END
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS repositories_search_vector_trigger ON repositories;
CREATE TRIGGER repositories_search_vector_trigger
  BEFORE INSERT OR UPDATE ON repositories
  FOR EACH ROW EXECUTE FUNCTION repositories_search_vector_update();

CREATE OR REPLACE FUNCTION repository_related_search_vector_refresh() RETURNS trigger AS $$
DECLARE
  repository_uuid uuid;
BEGIN
  IF TG_OP = 'DELETE' THEN
    repository_uuid := OLD.repository_id;
  ELSE
    repository_uuid := NEW.repository_id;
  END IF;

  PERFORM repositories_refresh_search_vector(repository_uuid);

  IF TG_OP = 'DELETE' THEN
    RETURN OLD;
  END IF;
  RETURN NEW;
END
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS repository_topics_search_vector_trigger ON repository_topics;
CREATE TRIGGER repository_topics_search_vector_trigger
  AFTER INSERT OR UPDATE OR DELETE ON repository_topics
  FOR EACH ROW EXECUTE FUNCTION repository_related_search_vector_refresh();

DROP TRIGGER IF EXISTS repository_readmes_search_vector_trigger ON repository_readmes;
CREATE TRIGGER repository_readmes_search_vector_trigger
  AFTER INSERT OR UPDATE OR DELETE ON repository_readmes
  FOR EACH ROW EXECUTE FUNCTION repository_related_search_vector_refresh();
