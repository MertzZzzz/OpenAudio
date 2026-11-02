# OpenAudio API

Go backend: chi, pgx/v5, golang-migrate, MinIO SDK. Contract: [`openapi.yaml`](./openapi.yaml).

```text
cmd/api/main.go              config + DI only
internal/
├── domain/                  entities, repository interfaces, typed errors
├── usecase/auth/            sign-up/in, OAuth linking, sessions, email verification, password reset, roles
├── usecase/publishing/      publishers, members, packages, drafts, publish, yank
├── usecase/uploads/         resumable content-addressed uploads with server-side SHA-256 verification
├── manifest/                openaudio.package/v1 JSON Schema, domain rules, canonical digest
├── semver/                  versions and ranges
├── adapter/
│   ├── http/                router, middleware (auth, roles, CORS, rate limit), handlers, OAuth state cookie
│   ├── postgres/            repositories + embedded migrations runner
│   ├── oauth/               Google and Discord providers
│   ├── s3/                  content-addressed blob storage (blobs/{sha256}), presigned URLs
│   ├── mailer/              SMTP / log mailer
│   └── memory/              in-memory repositories for tests
├── config/                  environment variables
└── platform/                logger, security (Argon2id, JWT, opaque tokens)
migrations/                  SQL (users & auth, catalog with immutable releases, library, uploads)
```

## Run

```bash
docker compose up api              # from the repo root, or:
make api-run                       # needs PostgreSQL at DATABASE_URL
```

Migrations run on start-up (`AUTO_MIGRATE=true`). All settings are environment variables — see
[`../.env.example`](../.env.example) and [`internal/config/config.go`](internal/config/config.go).

## Test

```bash
go test ./...                                                     # unit + HTTP tests
TEST_DATABASE_URL=postgres://openaudio:openaudio@localhost:5432/openaudio_test?sslmode=disable go test ./...
```

The integration tests wipe and re-migrate the test database.

## Notes

- SQL is hand-written with pgx (no sqlc). Queries live next to their repository in `internal/adapter/postgres`.
- `users.password_hash` is nullable (the spec has it `NOT NULL`) so Google/Discord-only accounts are possible.
- Published releases are immutable at the database level (trigger), not only in application code.
- Release manifests are stored twice in PostgreSQL: exact canonical bytes (`manifest_canonical`, hashed into
  `manifest_digest`) and JSONB for querying. JSONB re-formats JSON, so it is never used to serve or hash manifests.
