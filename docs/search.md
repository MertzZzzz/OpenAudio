# Search

## How packages are indexed

Search reads two tables maintained by the database itself (migration `000005_search`):

- **`package_search`** — one row per *searchable* package (public, with a published release), built from its
  **latest** release: a weighted `tsvector` (A: name · B: slug, publisher, free tags · C: taxonomy slugs and
  labels, module class · D: description) plus scalar facets extracted from the manifest JSONB — kind, BPM and
  BPM range, key (`F-minor`), license, commercial use, module class, energy, publish date.
- **`sample_index`** — every `sample` artifact of that release. Metadata follows spec §6.3: the per-sample entry
  in `manifest.samples` overrides package defaults (first instrument, key, moods, tags). Tempo is inherited only
  by loops. Without metadata a file under `loops/` counts as a loop, anything else as a one-shot.

`refresh_package_search(package_id)` rebuilds both rows. A trigger on `packages` calls it whenever
`latest_version`, name, description or visibility change — so publish, yank and metadata edits re-index inside
the same transaction, and there is no separate indexer to run.

Controlled-vocabulary terms (taxonomy `2026.11`, with hierarchy and aliases) live in `taxonomy_terms`;
`GET /api/v1/taxonomy` serves them. The web app ships the same vocabulary; the migration seed was generated from
`web/src/shared/api/mock/taxonomy.ts`.

## Query semantics

| Parameter | Behaviour |
|---|---|
| `q` | Prefix match on every word (`deep hous` → `deep:* & hous:*`). Words naming a vocabulary term or alias also match packages tagged with it: `dnb` → drum-and-bass, `house` → house + deep-house + … |
| `genre`, `instrument`, `mood`, `useCase`, `category` | Any of the values; terms include their sub-terms; aliases are accepted |
| `kind`, `moduleClass`, `key`, `license` | Any of the values |
| `bpmMin`, `bpmMax` | Overlap with the package BPM or BPM range (`122–126` matches `bpmMin=125`) |
| `energyMin`, `energyMax`, `commercial`, `publisher` | As named |
| `sort` | `relevance` (default with `q`; `ts_rank_cd` + bonus for vocabulary matches), `newest` (default), `downloads`, `name`. `rating` falls back to downloads until ratings exist |
| `page`, `limit` | `limit` ≤ 100 |

Lists accept `?kind=a,b` and `?kind=a&kind=b`.

**Facets** (`GET /search/facets`, same parameters) count packages per value with every filter applied *except
the facet's own*, so selected values never hide their alternatives. Buckets: kind, genre, instrument, mood,
useCase, category, moduleClass, key, license and **bpm** (`0-79`, `80-99`, `100-119`, `120-129`, `130-139`,
`140-159`, `160-179`, `180-400`; a package counts in every bucket its BPM range overlaps, matching the filter).

**Samples** (`GET /search/samples`): `q` (name, path, package name, instrument, tag), `package`
(`<publisher>.<package>`), `instrument` (with sub-terms), `contentType` (`loop` / `one-shot`), `key`,
`bpmMin` / `bpmMax`, `durationMax` (seconds).

## Downloads

`GET /packages/{publisher}/{slug}/releases/{version}/files/{path}` counts a download (manifest downloads are not
counted) and redirects to a short-lived presigned URL. Sample hits and the web app's download buttons use it;
`downloads` on packages feeds `sort=downloads` and the "Popular" carousel.

## Demo data

```bash
ADMIN_EMAILS=demo@openaudio.dev docker compose up -d
make seed        # = python3 scripts/seed_demo.py --api http://localhost:5173/api/v1
```

The seeder publishes 8 packages from 4 publishers through the public API (generated WAV files, resumable uploads,
validated manifests), so search, facets and the sample browser have something to show. Re-running it is safe.

## Not yet

- Vector / semantic search (out of MVP scope per the spec).
- Ratings; previews (the player is still a placeholder).
- Facet counts for the sample browser.
