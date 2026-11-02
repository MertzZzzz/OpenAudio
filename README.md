# OpenAudio

Open platform for publishing, discovering and versioning audio packages — sample packs, presets,
instruments, effects, MIDI tools and (later) plugin modules.

> **Status:** Phases 0–3 are done: monorepo, clickable UI prototype, Go backend skeleton and authentication
> (email + password, Google, Discord). The catalog still runs on mock data until Phase 4/5.

## Quick start

```bash
# Whole stack: PostgreSQL + MinIO + Go API + web
cp .env.example .env   # optionally add Google/Discord credentials, see docs/auth.md
docker compose up      # http://localhost:5173 (API proxied at /api), MinIO console :9001

# Frontend only, no backend (auth is mocked: demo@openaudio.dev / openaudio-demo)
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
- [ ] **Phase 4** — Packages & releases: manifests (JSON Schema), blob uploads with SHA-256 verification, publish/yank
- [ ] **Phase 5** — Search over SQL + JSONB, facets; frontend switches to the real API
- [ ] **Phase 6** — Library, dependency resolution, lockfiles
