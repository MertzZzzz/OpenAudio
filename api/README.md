# OpenAudio API

Go backend — **not implemented yet** (starts in Phase 2).

What is here now:

- [`openapi.yaml`](./openapi.yaml) — the API-first contract (OpenAPI 3.1) for every MVP endpoint from the spec.
  The web app's mock layer (`web/src/shared/api`) implements these shapes, so switching to the real backend
  means swapping one `ApiClient` implementation.

Planned layout (Phase 2):

```text
api/
├── cmd/api/main.go          # config + DI only
├── internal/
│   ├── domain/              # entities, interfaces, business rules
│   ├── usecase/             # auth, package, release, search, upload, library
│   ├── adapter/{http,postgres,s3}
│   ├── config/
│   └── platform/
├── migrations/              # golang-migrate
└── Dockerfile               # used by `docker compose --profile api up`
```
