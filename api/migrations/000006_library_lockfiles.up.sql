-- Phase 6: libraries and lockfiles.

-- The version currently installed in the library (what the lockfile prefers while it still satisfies every range).
ALTER TABLE library_items ADD COLUMN installed_version text
  CHECK (installed_version IS NULL OR installed_version ~ '^\d+\.\d+\.\d+$');

ALTER TABLE libraries ADD CONSTRAINT libraries_name_length CHECK (char_length(name) BETWEEN 1 AND 80);
CREATE UNIQUE INDEX libraries_user_name_idx ON libraries (user_id, lower(name));

-- Lockfiles are hashed over their exact canonical bytes (JSONB re-formats JSON), kept next to the JSONB copy.
ALTER TABLE project_lockfiles ADD COLUMN content_canonical bytea;
ALTER TABLE project_lockfiles ADD COLUMN library_id uuid REFERENCES libraries(id) ON DELETE SET NULL;
CREATE INDEX project_lockfiles_owner_idx ON project_lockfiles (owner_user_id, created_at DESC);
