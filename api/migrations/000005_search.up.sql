-- Phase 5: search.
--
-- package_search holds one row per searchable package (public, with a published release), derived
-- from its latest release: a weighted tsvector plus scalar facets extracted from the manifest JSONB.
-- sample_index holds the samples of that release with package defaults inherited (spec §6.3).
-- Both are maintained by refresh_package_search(), fired by a trigger on packages, so every
-- publish / yank / metadata change re-indexes inside the same transaction.

-- Controlled vocabulary (spec §7.2), version 2026.11. Generated from web/src/shared/api/mock/taxonomy.ts.
INSERT INTO taxonomy_terms (taxonomy_version, domain, slug, label, parent_slug, aliases) VALUES
  ('2026.11', 'genre', 'electronic', 'Electronic', NULL, ARRAY[]::text[]),
  ('2026.11', 'genre', 'house', 'House', 'electronic', ARRAY[]::text[]),
  ('2026.11', 'genre', 'deep-house', 'Deep House', 'house', ARRAY[]::text[]),
  ('2026.11', 'genre', 'tech-house', 'Tech House', 'house', ARRAY[]::text[]),
  ('2026.11', 'genre', 'progressive-house', 'Progressive House', 'house', ARRAY[]::text[]),
  ('2026.11', 'genre', 'techno', 'Techno', 'electronic', ARRAY[]::text[]),
  ('2026.11', 'genre', 'melodic-techno', 'Melodic Techno', 'techno', ARRAY[]::text[]),
  ('2026.11', 'genre', 'industrial-techno', 'Industrial Techno', 'techno', ARRAY[]::text[]),
  ('2026.11', 'genre', 'drum-and-bass', 'Drum & Bass', 'electronic', ARRAY['dnb', 'jungle']::text[]),
  ('2026.11', 'genre', 'ambient', 'Ambient', 'electronic', ARRAY[]::text[]),
  ('2026.11', 'genre', 'hip-hop', 'Hip Hop', NULL, ARRAY['rap']::text[]),
  ('2026.11', 'genre', 'lo-fi', 'Lo-Fi', 'hip-hop', ARRAY['lofi']::text[]),
  ('2026.11', 'genre', 'trap', 'Trap', 'hip-hop', ARRAY[]::text[]),
  ('2026.11', 'genre', 'cinematic', 'Cinematic', NULL, ARRAY['score', 'film']::text[]),
  ('2026.11', 'genre', 'synthwave', 'Synthwave', 'electronic', ARRAY['retrowave']::text[]),
  ('2026.11', 'instrument', 'drums', 'Drums', NULL, ARRAY[]::text[]),
  ('2026.11', 'instrument', 'kick', 'Kick', 'drums', ARRAY[]::text[]),
  ('2026.11', 'instrument', 'snare', 'Snare', 'drums', ARRAY[]::text[]),
  ('2026.11', 'instrument', 'clap', 'Clap', 'drums', ARRAY[]::text[]),
  ('2026.11', 'instrument', 'hi-hat', 'Hi-Hat', 'drums', ARRAY['hihat', 'hat']::text[]),
  ('2026.11', 'instrument', 'percussion', 'Percussion', 'drums', ARRAY[]::text[]),
  ('2026.11', 'instrument', 'bass', 'Bass', NULL, ARRAY[]::text[]),
  ('2026.11', 'instrument', 'synth', 'Synth', NULL, ARRAY[]::text[]),
  ('2026.11', 'instrument', 'pad', 'Pad', 'synth', ARRAY[]::text[]),
  ('2026.11', 'instrument', 'keys', 'Keys', NULL, ARRAY[]::text[]),
  ('2026.11', 'instrument', 'piano', 'Piano', 'keys', ARRAY[]::text[]),
  ('2026.11', 'instrument', 'guitar', 'Guitar', NULL, ARRAY[]::text[]),
  ('2026.11', 'instrument', 'vocals', 'Vocals', NULL, ARRAY['vox']::text[]),
  ('2026.11', 'instrument', 'strings', 'Strings', NULL, ARRAY[]::text[]),
  ('2026.11', 'instrument', 'fx', 'FX', NULL, ARRAY['sfx']::text[]),
  ('2026.11', 'mood', 'warm', 'Warm', NULL, ARRAY[]::text[]),
  ('2026.11', 'mood', 'dark', 'Dark', NULL, ARRAY[]::text[]),
  ('2026.11', 'mood', 'bright', 'Bright', NULL, ARRAY[]::text[]),
  ('2026.11', 'mood', 'hypnotic', 'Hypnotic', NULL, ARRAY[]::text[]),
  ('2026.11', 'mood', 'aggressive', 'Aggressive', NULL, ARRAY[]::text[]),
  ('2026.11', 'mood', 'melancholic', 'Melancholic', NULL, ARRAY[]::text[]),
  ('2026.11', 'mood', 'dreamy', 'Dreamy', NULL, ARRAY[]::text[]),
  ('2026.11', 'mood', 'uplifting', 'Uplifting', NULL, ARRAY[]::text[]),
  ('2026.11', 'mood', 'gritty', 'Gritty', NULL, ARRAY[]::text[]),
  ('2026.11', 'use-case', 'club', 'Club', NULL, ARRAY[]::text[]),
  ('2026.11', 'use-case', 'studio-production', 'Studio Production', NULL, ARRAY[]::text[]),
  ('2026.11', 'use-case', 'live-performance', 'Live Performance', NULL, ARRAY[]::text[]),
  ('2026.11', 'use-case', 'sound-design', 'Sound Design', NULL, ARRAY[]::text[]),
  ('2026.11', 'use-case', 'mixing', 'Mixing', NULL, ARRAY[]::text[]),
  ('2026.11', 'use-case', 'mastering', 'Mastering', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'reverb', 'Reverb', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'convolution', 'Convolution', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'delay', 'Delay', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'compressor', 'Compressor', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'eq', 'Eq', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'saturation', 'Saturation', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'distortion', 'Distortion', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'filter', 'Filter', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'chorus', 'Chorus', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'granular', 'Granular', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'wavetable', 'Wavetable', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'sampler', 'Sampler', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'arpeggiator', 'Arpeggiator', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'chord', 'Chord', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'sequencer', 'Sequencer', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'meter', 'Meter', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'spectrum', 'Spectrum', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'tuner', 'Tuner', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'gain', 'Gain', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'stereo', 'Stereo', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'pitch', 'Pitch', NULL, ARRAY[]::text[]),
  ('2026.11', 'module-category', 'neural', 'Neural', NULL, ARRAY[]::text[]);

ALTER TABLE packages ADD COLUMN download_count bigint NOT NULL DEFAULT 0;

CREATE TABLE package_search (
  package_id     uuid PRIMARY KEY REFERENCES packages(id) ON DELETE CASCADE,
  release_id     uuid NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
  publisher_slug citext NOT NULL,
  kind           text NOT NULL,
  name           text NOT NULL,
  bpm            int,
  bpm_min        int,
  bpm_max        int,
  music_key      text,          -- "F-minor"
  license        text,
  commercial_use boolean,
  module_class   text,
  energy         numeric,
  published_at   timestamptz,
  document       tsvector NOT NULL
);
CREATE INDEX package_search_document_idx ON package_search USING gin (document);
CREATE INDEX package_search_kind_idx ON package_search (kind);
CREATE INDEX package_search_bpm_idx ON package_search (bpm);
CREATE INDEX package_search_key_idx ON package_search (music_key);
CREATE INDEX package_search_published_idx ON package_search (published_at DESC);

CREATE TABLE sample_index (
  release_id   uuid NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
  package_id   uuid NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
  path         text NOT NULL,
  name         text NOT NULL,
  digest       text NOT NULL,
  size_bytes   bigint NOT NULL,
  content_type text NOT NULL,   -- loop | one-shot
  instrument   text,
  role         text,
  bpm          int,
  music_key    text,
  duration_sec numeric,
  moods        text[] NOT NULL DEFAULT '{}',
  tags         text[] NOT NULL DEFAULT '{}',
  PRIMARY KEY (release_id, path)
);
CREATE INDEX sample_index_package_idx ON sample_index (package_id);
CREATE INDEX sample_index_instrument_idx ON sample_index (instrument);
CREATE INDEX sample_index_bpm_idx ON sample_index (bpm);

CREATE FUNCTION jsonb_int(v jsonb) RETURNS int LANGUAGE sql IMMUTABLE AS $$
  SELECT CASE WHEN jsonb_typeof(v) = 'number' THEN round((v #>> '{}')::numeric)::int END
$$;

CREATE FUNCTION jsonb_text_array(v jsonb) RETURNS text[] LANGUAGE sql IMMUTABLE AS $$
  SELECT COALESCE(ARRAY(SELECT jsonb_array_elements_text(CASE WHEN jsonb_typeof(v) = 'array' THEN v ELSE '[]'::jsonb END)), '{}')
$$;

CREATE FUNCTION refresh_package_search(pkg uuid) RETURNS void LANGUAGE plpgsql AS $$
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
                              license, commercial_use, module_class, energy, published_at, document)
  VALUES (
    pkg, rel.id, p.pub_slug, p.kind, p.name,
    jsonb_int(m #> '{musical,bpm,value}'), jsonb_int(m #> '{musical,bpm,range,0}'), jsonb_int(m #> '{musical,bpm,range,1}'),
    key,
    m #>> '{license,content}', (m #>> '{license,commercialUse}')::boolean,
    m #>> '{module,moduleClass}',
    CASE WHEN jsonb_typeof(m #> '{taxonomy,energy}') = 'number' THEN (m #>> '{taxonomy,energy}')::numeric END,
    rel.published_at,
    setweight(to_tsvector('simple', p.name), 'A') ||
    setweight(to_tsvector('simple', p.slug || ' ' || p.pub_slug || ' ' || p.pub_name), 'B') ||
    setweight(to_tsvector('simple', array_to_string(jsonb_text_array(m #> '{tags,free}'), ' ')), 'B') ||
    setweight(to_tsvector('simple', COALESCE((SELECT string_agg(t.slug::text || ' ' || COALESCE(v.label, ''), ' ')
                                              FROM package_taxonomy t
                                              LEFT JOIN taxonomy_terms v ON v.domain = t.domain AND v.slug = t.slug
                                              WHERE t.release_id = rel.id), '')), 'C') ||
    setweight(to_tsvector('simple', COALESCE(m #>> '{module,moduleClass}', '')), 'C') ||
    setweight(to_tsvector('simple', COALESCE(p.description, '') || ' ' || COALESCE(m ->> 'description', '')), 'D')
  );

  -- Samples: artifacts of kind "sample", per-sample metadata overriding package defaults.
  INSERT INTO sample_index (release_id, package_id, path, name, digest, size_bytes, content_type, instrument, role, bpm,
                            music_key, duration_sec, moods, tags)
  SELECT rel.id, pkg, a ->> 'path',
         COALESCE(s ->> 'id', regexp_replace(regexp_replace(a ->> 'path', '^.*/', ''), '\.[A-Za-z0-9]+$', '')),
         a ->> 'digest', (a ->> 'size')::bigint,
         COALESCE(s ->> 'contentType', CASE WHEN a ->> 'path' ~* '(^|/)loops?/' THEN 'loop' ELSE 'one-shot' END),
         COALESCE(s ->> 'instrument', m #>> '{taxonomy,instruments,0}'),
         s ->> 'role',
         COALESCE(jsonb_int(s -> 'bpm'), jsonb_int(m #> '{musical,bpm,value}')),
         COALESCE(CASE WHEN s #>> '{key,tonic}' IS NOT NULL THEN (s #>> '{key,tonic}') || '-' || (s #>> '{key,mode}') END, key),
         CASE WHEN jsonb_typeof(s -> 'durationSec') = 'number' THEN (s ->> 'durationSec')::numeric END,
         CASE WHEN s ? 'moods' THEN jsonb_text_array(s -> 'moods') ELSE jsonb_text_array(m #> '{taxonomy,moods}') END,
         CASE WHEN s ? 'tags' THEN jsonb_text_array(s -> 'tags') ELSE jsonb_text_array(m #> '{tags,free}') END
  FROM jsonb_array_elements(COALESCE(m -> 'artifacts', '[]'::jsonb)) a
  LEFT JOIN LATERAL (
    SELECT x AS s FROM jsonb_array_elements(COALESCE(m -> 'samples', '[]'::jsonb)) x WHERE x ->> 'file' = a ->> 'path' LIMIT 1
  ) meta ON true
  WHERE a ->> 'kind' = 'sample';
END;
$$;

CREATE FUNCTION packages_search_trigger() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  PERFORM refresh_package_search(NEW.id);
  RETURN NULL;
END;
$$;

CREATE TRIGGER packages_search_refresh
  AFTER INSERT OR UPDATE OF latest_version, name, description, visibility ON packages
  FOR EACH ROW EXECUTE FUNCTION packages_search_trigger();

-- Backfill.
SELECT refresh_package_search(id) FROM packages;
