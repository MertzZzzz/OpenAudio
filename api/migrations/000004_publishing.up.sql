-- Audit columns for releases (set while the row is still a draft, so the immutability trigger allows them).
ALTER TABLE releases ADD COLUMN created_by uuid REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE releases ADD COLUMN published_by uuid REFERENCES users(id) ON DELETE SET NULL;
-- Exact canonical bytes of the manifest. JSONB (releases.manifest) re-formats JSON, so the digest
-- is computed over — and downloads are served from — this column; JSONB stays for querying.
ALTER TABLE releases ADD COLUMN manifest_canonical bytea;

CREATE OR REPLACE FUNCTION releases_enforce_immutability() RETURNS trigger AS $$
BEGIN
  IF OLD.status = 'draft' THEN
    RETURN NEW;
  END IF;
  IF NEW.manifest IS DISTINCT FROM OLD.manifest
     OR NEW.manifest_canonical IS DISTINCT FROM OLD.manifest_canonical
     OR NEW.manifest_digest IS DISTINCT FROM OLD.manifest_digest
     OR NEW.version IS DISTINCT FROM OLD.version
     OR NEW.package_id IS DISTINCT FROM OLD.package_id
     OR NEW.published_at IS DISTINCT FROM OLD.published_at
     OR NEW.changelog IS DISTINCT FROM OLD.changelog
     -- audit columns may only be cleared (ON DELETE SET NULL when a user is deleted)
     OR (NEW.created_by IS DISTINCT FROM OLD.created_by AND NEW.created_by IS NOT NULL)
     OR (NEW.published_by IS DISTINCT FROM OLD.published_by AND NEW.published_by IS NOT NULL) THEN
    RAISE EXCEPTION 'release % is immutable once published', OLD.id USING ERRCODE = 'check_violation';
  END IF;
  IF NOT (NEW.status = OLD.status OR (OLD.status = 'published' AND NEW.status = 'yanked')) THEN
    RAISE EXCEPTION 'invalid release status transition % -> %', OLD.status, NEW.status USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Artifacts belong to the immutable part of a release.
CREATE FUNCTION artifacts_enforce_immutability() RETURNS trigger AS $$
DECLARE st text;
BEGIN
  SELECT status INTO st FROM releases WHERE id = COALESCE(NEW.release_id, OLD.release_id);
  IF TG_OP = 'INSERT' AND st = 'draft' THEN
    RETURN NEW;
  END IF;
  IF TG_OP = 'DELETE' AND (st IS NULL OR pg_trigger_depth() > 1) THEN
    RETURN OLD;
  END IF;
  RAISE EXCEPTION 'artifacts of release % cannot be modified', COALESCE(NEW.release_id, OLD.release_id) USING ERRCODE = 'check_violation';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER artifacts_immutable
  BEFORE INSERT OR UPDATE OR DELETE ON artifacts
  FOR EACH ROW EXECUTE FUNCTION artifacts_enforce_immutability();

CREATE INDEX releases_package_status_idx ON releases (package_id, status);
CREATE INDEX packages_updated_idx ON packages (updated_at DESC);

-- Multipart uploads of content-addressed blobs (spec §8, uploads/{uploadId}).
CREATE TABLE uploads (
  id           uuid PRIMARY KEY,
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  digest       text NOT NULL CHECK (digest ~ '^sha256:[0-9a-f]{64}$'),
  size_bytes   bigint NOT NULL CHECK (size_bytes > 0),
  content_type text NOT NULL DEFAULT '',
  status       text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'done', 'failed')),
  multipart_id text NOT NULL DEFAULT '',
  part_size    bigint NOT NULL CHECK (part_size > 0),
  error        text NOT NULL DEFAULT '',
  created_at   timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz
);
CREATE INDEX uploads_user_idx ON uploads (user_id, created_at DESC);

CREATE TABLE upload_parts (
  upload_id   uuid NOT NULL REFERENCES uploads(id) ON DELETE CASCADE,
  part_number int NOT NULL CHECK (part_number BETWEEN 1 AND 10000),
  etag        text NOT NULL,
  size_bytes  bigint NOT NULL CHECK (size_bytes >= 0),
  PRIMARY KEY (upload_id, part_number)
);
