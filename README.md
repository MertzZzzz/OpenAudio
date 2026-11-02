# OpenAudio

Open platform for publishing, discovering and versioning audio packages — sample packs, presets,
instruments, effects, MIDI tools and (later) plugin modules.

> **Status:** MVP complete (phases 0–6): authentication (email, Google, Discord), publishing with
> SHA-256-verified uploads and immutable releases, server-side search with facets, audio previews, and
> libraries with dependency resolution and verifiable lockfiles.

## Quick start

```bash
# Whole stack: PostgreSQL + MinIO + Go API + web
cp .env.example .env   # optionally add Google/Discord credentials, see docs/auth.md
docker compose up      # http://localhost:5173 (API proxied at /api), MinIO console :9001

# Demo catalog (8 packages) — the seeding account must be an admin:
#   ADMIN_EMAILS=demo@openaudio.dev in .env, then
make seed

# Frontend only, no backend (everything mocked; demo@openaudio.dev / openaudio-demo)
cd web && npm ci && npm run dev

# Frontend against a locally running API (make api-run)
cd web && npm run dev:api
```

Other commands (`make help`):

```bash
make api-test               # Go unit + HTTP tests
make api-test-integration   # + PostgreSQL integration tests (TEST_DATABASE_URL)
make web-test               # Vitest + React Testing Library
make web-typecheck          # strict TypeScript
make openapi-lint           # validate api/openapi.yaml
make check                  # everything CI runs
```

## Authentication

Email + password (Argon2id, email verification, password reset), **Google** and **Discord** sign-in,
JWT access tokens + rotating HttpOnly refresh cookies, roles `user` / `publisher` / `admin`.
Details, provider setup and the OAuth flow: [`docs/auth.md`](docs/auth.md).

## Publishing

Create a publisher (verified email required), create packages, upload files (resumable multipart, verified
by SHA-256 on the server, deduplicated by content), validate the manifest against the
`openaudio.package/v1` JSON Schema, publish immutable releases, yank. Roles: owner / maintainer / uploader.
Flow, API and curl examples: [`docs/publishing.md`](docs/publishing.md).

## Libraries & lockfiles

Personal libraries with pinned versions, a resolver over per-version manifest dependencies (conflicts are explained
and fixable in one click), lockfiles with exact versions and manifest digests, and imports that verify every digest
against the registry. Details: [`docs/library.md`](docs/library.md).

## Search

PostgreSQL full-text search (prefix matching, taxonomy aliases such as `dnb`, genre hierarchy) with filters for
kind, genre, instrument, mood, BPM, key, license and module class; facets that ignore their own selection; sample
search with metadata inherited from the package; counted downloads. Details: [`docs/search.md`](docs/search.md).

## What's in the prototype

| Screen | Route | Highlights |
|---|---|---|
| Discover | `/` | Hero search, quick filters, Popular / New / Editors' picks carousels, genres |
| Search | `/search` | URL-synced facets (type, module class, genre, instrument, mood, key, license, BPM), sort, grid/list, pagination |
| Package details | `/packages/:publisher/:slug` | Version switcher, install modal, add to library, manifest download, tabs: Overview (module port diagram), Versions, Files (digests), Samples, Dependencies, Changelog, License |
| Sample browser | `/samples` | Filters (instrument, loop/one-shot, key, BPM, length), audition, multi-select batch download |
| Publisher dashboard | `/publisher` | Stats, drafts to publish, releases per package (publish / yank), create package, members (invite / remove) |
| Upload wizard | `/publisher/upload` | 5 steps: metadata → taxonomy & musical → files (browser SHA-256) → validation → review & publish |
| Library | `/library` | Installed vs latest, pin/unpin, updates, conflict resolution, lockfile export/import (sign-in required) |
| Auth | `/login`, `/register`, `/account`, … | Email/password, Google, Discord; verify email; reset password; linked accounts |

Audio previews use a placeholder player (simulated playback, generated waveforms) until previews exist in S3.

## Repository layout

```text
api/        Go backend (chi, pgx, golang-migrate, MinIO) + OpenAPI 3.1 contract
web/        React + TypeScript + Vite frontend
docs/       Architecture notes
docker-compose.yml
```

See [`docs/architecture.md`](docs/architecture.md) for how the mock API seam works and what domain rules it already enforces.

## Roadmap

- [x] **Phase 0** — monorepo, Docker Compose (PostgreSQL, MinIO, web), OpenAPI contract, CI
- [x] **Phase 1** — UI prototype on mock data
- [x] **Phase 2** — Go backend skeleton: config, logging, pgx, golang-migrate (all MVP tables), MinIO, health, error model
- [x] **Phase 3** — Auth: email/password (Argon2id), Google, Discord, JWT + rotating refresh tokens, email verification, password reset, roles
- [x] **Phase 4** — Packages & releases: publishers & members, manifests (JSON Schema), resumable blob uploads with SHA-256 verification, immutable publish, yank
- [x] **Phase 5** — Search over SQL + JSONB (tsvector index maintained by triggers), facets incl. BPM buckets, sample search; frontend on the real API
- [x] **Phase 6** — Libraries, pins, dependency resolution, lockfile export/import with integrity checks, audio previews

## Definition of Done (spec §15)

| Requirement | Where |
|---|---|
| Register and sign in | email + password, Google, Discord — [`docs/auth.md`](docs/auth.md) |
| Publisher creates a package and uploads a release | publisher dashboard + upload wizard — [`docs/publishing.md`](docs/publishing.md) |
| Manifest validated and stored in PostgreSQL JSONB | JSON Schema + domain rules; JSONB plus exact canonical bytes |
| Artifacts in MinIO/S3, checked by SHA-256 | server re-hashes every upload before storing `blobs/{sha256}` |
| Published releases are immutable | use case + database triggers on `releases` and `artifacts` |
| Search by genre, instrument, BPM, key, mood, kind, license | `/search` + facets — [`docs/search.md`](docs/search.md) |
| Listen to previews, add packages to a library | HTML5 audio previews; libraries — [`docs/library.md`](docs/library.md) |
| Lockfile with exact versions and digests | `/libraries/{id}/lockfiles`, verified import |
| Frontend on mock data first, then fully on the backend | `VITE_API_MODE=mock` (default for `npm run dev`) / `http` (docker compose) |
| Local run: `docker compose up` | PostgreSQL, MinIO, API, web |
