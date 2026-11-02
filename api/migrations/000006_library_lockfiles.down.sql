DROP INDEX IF EXISTS project_lockfiles_owner_idx;
ALTER TABLE project_lockfiles DROP COLUMN IF EXISTS library_id;
ALTER TABLE project_lockfiles DROP COLUMN IF EXISTS content_canonical;
DROP INDEX IF EXISTS libraries_user_name_idx;
ALTER TABLE libraries DROP CONSTRAINT IF EXISTS libraries_name_length;
ALTER TABLE library_items DROP COLUMN IF EXISTS installed_version;
