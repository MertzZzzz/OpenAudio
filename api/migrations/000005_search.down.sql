DROP TRIGGER IF EXISTS packages_search_refresh ON packages;
DROP FUNCTION IF EXISTS packages_search_trigger();
DROP FUNCTION IF EXISTS refresh_package_search(uuid);
DROP FUNCTION IF EXISTS jsonb_text_array(jsonb);
DROP FUNCTION IF EXISTS jsonb_int(jsonb);
DROP TABLE IF EXISTS sample_index;
DROP TABLE IF EXISTS package_search;
ALTER TABLE packages DROP COLUMN IF EXISTS download_count;
DELETE FROM taxonomy_terms WHERE taxonomy_version = '2026.11';
