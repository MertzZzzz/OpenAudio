# Architecture notes

## Monorepo

| Path | What |
|---|---|
| `web/` | React 18 + TypeScript + Vite + Tailwind. Phase 1 prototype on a typed mock API. |
| `api/` | Go backend: auth, migrations for all MVP tables, S3 adapter. Plus the OpenAPI contract. |
| `docs/` | Design notes. |
| `docker-compose.yml` | PostgreSQL 16, MinIO (+ bucket bootstrap), Go API, web (proxies `/api`). |

## Frontend

```text
web/src/
├── app/            router (lazy routes), providers (TanStack Query, toasts, player), layout
├── pages/          thin route components
├── features/       catalog, search, package-details, sample-browser, publisher, upload, library, auth
└── shared/
    ├── api/        ApiClient interface, query keys, in-memory mock implementation
    ├── ui/         design system components
    ├── player/     placeholder audio player (context + bar + play buttons)
    ├── lib/        semver, manifest schema (zod), module rules, formatting, taxonomy
    └── types/      domain types (to be generated from OpenAPI)
```

### The mock → real API seam

All server data flows through `ApiClient` (`shared/api/client.ts`) and TanStack Query hooks inside each
feature. `shared/api/index.ts` is the single place that picks the implementation, so Phase 5 replaces
`createMockClient()` with an HTTP client generated from `api/openapi.yaml` without touching features.

The mock is not a fixture dump — it implements the domain rules so the UI exercises them now:

- **Search**: text scoring, taxonomy aliases (`dnb` → `drum-and-bass`), hierarchical genre/instrument filters
  (`house` matches `deep-house`), BPM/key filters, facets where each facet ignores its own selection.
- **Releases**: draft → published → yanked; publishing twice or reusing a version fails (immutability);
  yanking the latest moves `latestVersion` back.
- **Manifest validation**: zod schema for `openaudio.package/v1` + module-class rules (§5.4) + metadata
  recommendations (§7.3). The same code will run server-side via JSON Schema.
- **Resolver**: pins, semver ranges (`^`, `~`, `>=`, exact), transitive dependencies, conflict reporting,
  yanked versions are never chosen; output is a lockfile with exact versions + manifest digests.
- **Uploads**: the wizard computes real SHA-256 digests in the browser (`crypto.subtle`); transfer progress is simulated.

### Design system

Tokens are CSS variables (`src/index.css`) exposed to Tailwind (`tailwind.config.ts`): background, surface,
primary, accent, success, warning, danger; typography `display / heading / body / caption / mono`; spacing
follows Tailwind's 4px grid (4/8/12/16/24/32/48). Dark theme is the default (`.light` overrides exist).

Components: Button, Input/Textarea/Field, Select, Checkbox, Badge, Card, Tabs (roving tabindex, arrow keys),
Modal and Drawer (focus trap, Esc, focus restore, scroll lock), Table, Toast (aria-live), Skeleton,
EmptyState, PackageCover, Waveform.

## Authentication

With `VITE_API_MODE=http` (set by docker compose) the web app talks to the Go API through
`shared/api/httpClient.ts` and `features/auth/authApi.ts`; without it everything runs on in-memory mocks with the
same interfaces. Everything — auth, catalog, publishing, search, libraries — comes from the API; see [auth.md](auth.md),
[publishing.md](publishing.md), [search.md](search.md) and [library.md](library.md).
See [auth.md](auth.md) and [publishing.md](publishing.md).
