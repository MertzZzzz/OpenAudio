CREATE TABLE publishers (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug          citext UNIQUE NOT NULL CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
  name          text NOT NULL,
  owner_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE publisher_members (
  publisher_id uuid NOT NULL REFERENCES publishers(id) ON DELETE CASCADE,
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role         text NOT NULL CHECK (role IN ('owner', 'maintainer', 'uploader')),
  PRIMARY KEY (publisher_id, user_id)
);

CREATE TABLE packages (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  publisher_id   uuid NOT NULL REFERENCES publishers(id) ON DELETE CASCADE,
  slug           citext NOT NULL CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
  name           text NOT NULL,
  kind           text NOT NULL CHECK (kind IN ('sample-pack', 'preset-pack', 'instrument', 'effect', 'midi-tool', 'utility', 'project-template', 'model')),
  description    text,
  latest_version text,
  visibility     text NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'unlisted', 'private')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (publisher_id, slug)
);

CREATE TABLE releases (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  package_id      uuid NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
  version         text NOT NULL CHECK (version ~ '^\d+\.\d+\.\d+$'),
  manifest        jsonb NOT NULL,
  manifest_digest text NOT NULL CHECK (manifest_digest ~ '^sha256:[0-9a-f]{64}$'),
  changelog       text NOT NULL DEFAULT '',
  status          text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'yanked')),
  published_at    timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (package_id, version)
);
CREATE INDEX releases_manifest_gin ON releases USING gin (manifest jsonb_path_ops);

-- Immutable releases: once published only the status may move to 'yanked'.
CREATE FUNCTION releases_enforce_immutability() RETURNS trigger AS $$
BEGIN
  IF OLD.status = 'draft' THEN
    RETURN NEW;
  END IF;
  IF NEW.manifest IS DISTINCT FROM OLD.manifest
     OR NEW.manifest_digest IS DISTINCT FROM OLD.manifest_digest
     OR NEW.version IS DISTINCT FROM OLD.version
     OR NEW.package_id IS DISTINCT FROM OLD.package_id
     OR NEW.published_at IS DISTINCT FROM OLD.published_at
     OR NEW.changelog IS DISTINCT FROM OLD.changelog THEN
    RAISE EXCEPTION 'release % is immutable once published', OLD.id USING ERRCODE = 'check_violation';
  END IF;
  IF NOT (NEW.status = OLD.status OR (OLD.status = 'published' AND NEW.status = 'yanked')) THEN
    RAISE EXCEPTION 'invalid release status transition % -> %', OLD.status, NEW.status USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER releases_immutable
  BEFORE UPDATE ON releases
  FOR EACH ROW EXECUTE FUNCTION releases_enforce_immutability();

CREATE FUNCTION releases_forbid_delete_published() RETURNS trigger AS $$
BEGIN
  IF OLD.status <> 'draft' THEN
    RAISE EXCEPTION 'published release % cannot be deleted', OLD.id USING ERRCODE = 'check_violation';
  END IF;
  RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER releases_no_delete
  BEFORE DELETE ON releases
  FOR EACH ROW WHEN (pg_trigger_depth() = 0)
  EXECUTE FUNCTION releases_forbid_delete_published();

CREATE TABLE artifacts (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  release_id   uuid NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
  kind         text NOT NULL CHECK (kind IN ('manifest', 'sample', 'preview', 'binary', 'index')),
  path         text NOT NULL,
  digest       text NOT NULL CHECK (digest ~ '^sha256:[0-9a-f]{64}$'),
  size_bytes   bigint NOT NULL CHECK (size_bytes >= 0),
  content_type text,
  storage_key  text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (release_id, path)
);
CREATE INDEX artifacts_digest_idx ON artifacts (digest);

CREATE TABLE taxonomy_terms (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  taxonomy_version text NOT NULL,
  domain           text NOT NULL CHECK (domain IN ('genre', 'instrument', 'mood', 'use-case', 'module-category')),
  slug             citext NOT NULL,
  label            text NOT NULL,
  parent_slug      text,
  aliases          text[] NOT NULL DEFAULT '{}',
  UNIQUE (taxonomy_version, domain, slug)
);
CREATE INDEX taxonomy_terms_aliases_gin ON taxonomy_terms USING gin (aliases);

CREATE TABLE package_taxonomy (
  package_id uuid NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
  release_id uuid NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
  domain     text NOT NULL,
  slug       citext NOT NULL,
  weight     numeric NOT NULL DEFAULT 1,
  PRIMARY KEY (release_id, domain, slug)
);
CREATE INDEX package_taxonomy_lookup_idx ON package_taxonomy (domain, slug);
