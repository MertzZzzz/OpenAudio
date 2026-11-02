# OpenAudio

Open platform for publishing, discovering and versioning audio packages — sample packs, presets,
instruments, effects, MIDI tools and (later) plugin modules.

> **Status:** Phase 0 (monorepo scaffolding) and Phase 1 (clickable UI prototype on a mock API) are done.
> The Go backend starts in Phase 2 — see the roadmap below.

## Quick start

```bash
# Frontend only (fastest)
cd web
npm ci
npm run dev          # http://localhost:5173

# Or the whole local stack (PostgreSQL + MinIO + web)
cp .env.example .env
docker compose up    # web on :5173, MinIO console on :9001
```

Other commands (`make help`):

```bash
make web-test        # Vitest + React Testing Library
make web-typecheck   # strict TypeScript
make web-build       # production build
make openapi-lint    # validate api/openapi.yaml
```

## What's in the prototype

| Screen | Route | Highlights |
|---|---|---|
| Discover | `/` | Hero search, quick filters, Popular / New / Editors' picks carousels, genres |
| Search | `/search` | URL-synced facets (type, module class, genre, instrument, mood, key, license, BPM), sort, grid/list, pagination |
| Package details | `/packages/:publisher/:slug` | Version switcher, install modal, add to library, manifest download, tabs: Overview (module port diagram), Versions, Files (digests), Samples, Dependencies, Changelog, License |
| Sample browser | `/samples` | Filters (instrument, loop/one-shot, key, BPM, length), audition, multi-select batch download |
| Publisher dashboard | `/publisher` | Stats, drafts to publish, releases per package (publish / yank), create package, members (invite / remove) |
| Upload wizard | `/publisher/upload` | 5 steps: metadata → taxonomy & musical → files (browser SHA-256) → validation → review & publish |
| Library | `/library` | Installed vs latest, pin/unpin, updates, conflict resolution, lockfile export/import |

Audio previews use a placeholder player (simulated playback, generated waveforms) until previews exist in S3.

## Repository layout

```text
api/        OpenAPI 3.1 contract (Go backend lands here in Phase 2)
web/        React + TypeScript + Vite frontend
docs/       Architecture notes
docker-compose.yml
```

See [`docs/architecture.md`](docs/architecture.md) for how the mock API seam works and what domain rules it already enforces.

## Roadmap

- [x] **Phase 0** — monorepo, Docker Compose (PostgreSQL, MinIO, web), OpenAPI contract, CI
- [x] **Phase 1** — UI prototype on mock data
- [ ] **Phase 2** — Go backend skeleton: config, logging, pgx, golang-migrate, MinIO, health, error model
- [ ] **Phase 3** — Auth: register/login/refresh/logout, Argon2id, JWT, roles
- [ ] **Phase 4** — Packages & releases: manifests (JSON Schema), blob uploads with SHA-256 verification, publish/yank
- [ ] **Phase 5** — Search over SQL + JSONB, facets; frontend switches to the real API
- [ ] **Phase 6** — Library, dependency resolution, lockfiles
