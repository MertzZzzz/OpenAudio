CREATE TABLE libraries (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name       text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX libraries_user_idx ON libraries (user_id);

CREATE TABLE library_items (
  library_id     uuid NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  package_id     uuid NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
  pinned_version text CHECK (pinned_version IS NULL OR pinned_version ~ '^\d+\.\d+\.\d+$'),
  added_at       timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (library_id, package_id)
);

CREATE TABLE project_lockfiles (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_user_id  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name           text NOT NULL,
  content        jsonb NOT NULL,
  content_digest text NOT NULL CHECK (content_digest ~ '^sha256:[0-9a-f]{64}$'),
  created_at     timestamptz NOT NULL DEFAULT now()
);
