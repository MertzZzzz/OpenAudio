-- VST3 / CLAP builds (manifest plugins[]) in the search index: filter by format and OS.
ALTER TABLE package_search
  ADD COLUMN plugin_formats text[] NOT NULL DEFAULT '{}',
  ADD COLUMN plugin_os      text[] NOT NULL DEFAULT '{}',
  ADD COLUMN plugin_builds  text[] NOT NULL DEFAULT '{}'; -- "vst3:macos": a format available on an OS

CREATE INDEX package_search_plugin_builds_idx ON package_search USING gin (plugin_builds);

-- The value if it is a JSON array, else an empty array (missing key, null, scalar).
CREATE FUNCTION jsonb_array(v jsonb) RETURNS jsonb LANGUAGE sql IMMUTABLE AS $$
  SELECT CASE WHEN jsonb_typeof(v) = 'array' THEN v ELSE '[]'::jsonb END
$$;

CREATE OR REPLACE FUNCTION refresh_package_search(pkg uuid) RETURNS void LANGUAGE plpgsql AS $$
DECLARE
  p   record;
  rel record;
  m   jsonb;
  key text;
BEGIN
  DELETE FROM package_search WHERE package_id = pkg;
  DELETE FROM sample_index WHERE package_id = pkg;

  SELECT pk.id, pk.name, pk.slug, pk.description, pk.kind, pk.latest_version, pk.visibility, pub.slug AS pub_slug, pub.name AS pub_name
    INTO p FROM packages pk JOIN publishers pub ON pub.id = pk.publisher_id WHERE pk.id = pkg;
  IF NOT FOUND OR p.latest_version IS NULL OR p.visibility <> 'public' THEN
    RETURN;
  END IF;
  SELECT id, manifest, published_at INTO rel FROM releases WHERE package_id = pkg AND version = p.latest_version;
  IF NOT FOUND THEN
    RETURN;
  END IF;
  m := rel.manifest;
  key := CASE WHEN m #>> '{musical,key,tonic}' IS NOT NULL
              THEN (m #>> '{musical,key,tonic}') || '-' || (m #>> '{musical,key,mode}') END;

  INSERT INTO package_search (package_id, release_id, publisher_slug, kind, name, bpm, bpm_min, bpm_max, music_key,
                              license, commercial_use, module_class, energy, published_at,
                              plugin_formats, plugin_os, plugin_builds, document)
  VALUES (
    pkg, rel.id, p.pub_slug, p.kind, p.name,
    jsonb_int(m #> '{musical,bpm,value}'), jsonb_int(m #> '{musical,bpm,range,0}'), jsonb_int(m #> '{musical,bpm,range,1}'),
    key,
    m #>> '{license,content}', (m #>> '{license,commercialUse}')::boolean,
    m #>> '{module,moduleClass}',
    CASE WHEN jsonb_typeof(m #> '{taxonomy,energy}') = 'number' THEN (m #>> '{taxonomy,energy}')::numeric END,
    rel.published_at,
    ARRAY(SELECT DISTINCT x ->> 'format' FROM jsonb_array_elements(jsonb_array(m -> 'plugins')) x ORDER BY 1),
    ARRAY(SELECT DISTINCT x ->> 'os' FROM jsonb_array_elements(jsonb_array(m -> 'plugins')) x ORDER BY 1),
    ARRAY(SELECT DISTINCT (x ->> 'format') || ':' || (x ->> 'os') FROM jsonb_array_elements(jsonb_array(m -> 'plugins')) x ORDER BY 1),
    setweight(to_tsvector('simple', p.name), 'A') ||
    setweight(to_tsvector('simple', p.slug || ' ' || p.pub_slug || ' ' || p.pub_name), 'B') ||
    setweight(to_tsvector('simple', array_to_string(jsonb_text_array(m #> '{tags,free}'), ' ')), 'B') ||
    setweight(to_tsvector('simple', COALESCE((SELECT string_agg(t.slug::text || ' ' || COALESCE(v.label, ''), ' ')
                                              FROM package_taxonomy t
                                              LEFT JOIN taxonomy_terms v ON v.domain = t.domain AND v.slug = t.slug
                                              WHERE t.release_id = rel.id), '')), 'C') ||
    setweight(to_tsvector('simple', COALESCE(m #>> '{module,moduleClass}', '')), 'C') ||
    -- "vst3", "clap" find packages that ship such builds.
    setweight(to_tsvector('simple', COALESCE((SELECT string_agg(DISTINCT x ->> 'format', ' ') FROM jsonb_array_elements(jsonb_array(m -> 'plugins')) x), '')), 'C') ||
    setweight(to_tsvector('simple', COALESCE(p.description, '') || ' ' || COALESCE(m ->> 'description', '')), 'D')
  );

  -- Samples: artifacts of kind "sample", per-sample metadata overriding package defaults.
  INSERT INTO sample_index (release_id, package_id, path, name, digest, size_bytes, content_type, instrument, role, bpm,
                            music_key, duration_sec, moods, tags)
  SELECT rel.id, pkg, a ->> 'path',
         COALESCE(s ->> 'id', regexp_replace(regexp_replace(a ->> 'path', '^.*/', ''), '\.[A-Za-z0-9]+$', '')),
         a ->> 'digest', (a ->> 'size')::bigint,
         ct.content_type,
         COALESCE(s ->> 'instrument', m #>> '{taxonomy,instruments,0}'),
         s ->> 'role',
         -- Tempo is only inherited by loops; a one-shot kick has no meaningful BPM.
         COALESCE(jsonb_int(s -> 'bpm'), CASE WHEN ct.content_type = 'loop' THEN jsonb_int(m #> '{musical,bpm,value}') END),
         COALESCE(CASE WHEN s #>> '{key,tonic}' IS NOT NULL THEN (s #>> '{key,tonic}') || '-' || (s #>> '{key,mode}') END, key),
         CASE WHEN jsonb_typeof(s -> 'durationSec') = 'number' THEN (s ->> 'durationSec')::numeric END,
         CASE WHEN s ? 'moods' THEN jsonb_text_array(s -> 'moods') ELSE jsonb_text_array(m #> '{taxonomy,moods}') END,
         CASE WHEN s ? 'tags' THEN jsonb_text_array(s -> 'tags') ELSE jsonb_text_array(m #> '{tags,free}') END
  FROM jsonb_array_elements(COALESCE(m -> 'artifacts', '[]'::jsonb)) a
  LEFT JOIN LATERAL (
    SELECT x AS s FROM jsonb_array_elements(COALESCE(m -> 'samples', '[]'::jsonb)) x WHERE x ->> 'file' = a ->> 'path' LIMIT 1
  ) meta ON true
  CROSS JOIN LATERAL (
    SELECT COALESCE(s ->> 'contentType', CASE WHEN a ->> 'path' ~* '(^|/)loops?/' THEN 'loop' ELSE 'one-shot' END) AS content_type
  ) ct
  WHERE a ->> 'kind' = 'sample';
END;
$$;

SELECT refresh_package_search(id) FROM packages;
