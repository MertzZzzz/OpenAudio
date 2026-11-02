DROP TABLE IF EXISTS upload_parts;
DROP TABLE IF EXISTS uploads;
DROP INDEX IF EXISTS packages_updated_idx;
DROP INDEX IF EXISTS releases_package_status_idx;
DROP TRIGGER IF EXISTS artifacts_immutable ON artifacts;
DROP FUNCTION IF EXISTS artifacts_enforce_immutability();
ALTER TABLE releases DROP COLUMN IF EXISTS manifest_canonical;
ALTER TABLE releases DROP COLUMN IF EXISTS published_by;
ALTER TABLE releases DROP COLUMN IF EXISTS created_by;
