# Object storage (S3)

Every uploaded file is stored once, content-addressed, at `blobs/<sha256>` in one bucket; release manifests go
to `manifests/<package>/<version>/manifest.json`. Browsers download through short-lived presigned URLs, so the
bucket stays private. Any S3-compatible service works.

## Local: PGSTY Silo (default)

`docker compose up` starts [PGSTY Silo](https://silo.pgsty.com/) — a maintained fork of the open-source MinIO
server (`pgsty/silo` image) — and a one-shot `silo-init` container that creates the bucket with the bundled
`mcli` client. Both belong to the compose profile `local-s3`, which `.env.example` enables:

```dotenv
COMPOSE_PROFILES=local-s3
SILO_ROOT_USER=openaudio
SILO_ROOT_PASSWORD=openaudio-secret
SILO_API_PORT=9000        # S3 API on http://localhost:9000
SILO_CONSOLE_PORT=9001    # web console on http://localhost:9001
# SILO_IMAGE=pgsty/silo:RELEASE.2026-11-12T00-00-00Z
```

Silo keeps MinIO's protocol, `MINIO_*` variables and data format; the server binary is `silo`, the client
`mcli`. Data lives in the `silodata` volume. Without a `.env` file (no profile) Silo does not start; copy
`.env.example` first.

## External S3

Set `COMPOSE_PROFILES=` (empty) so Silo is not started, and point the API at your storage:

| Variable | Meaning | Default |
|---|---|---|
| `S3_ENDPOINT` | `http(s)://host[:port]`, no path | `http://silo:9000` |
| `S3_ACCESS_KEY`, `S3_SECRET_KEY` | credentials | the `SILO_ROOT_*` values |
| `S3_BUCKET` | bucket name | `openaudio-dev` |
| `S3_REGION` | signing region | `us-east-1` |
| `S3_PUBLIC_ENDPOINT` | address browsers use for presigned URLs (CDN, public host) | `S3_ENDPOINT`; `http://localhost:9000` for local Silo |
| `S3_FORCE_PATH_STYLE` | `true`: `host/bucket`; `false`: `bucket.host` (AWS) | `true` |
| `S3_CREATE_BUCKET` | create the bucket on start if missing; `false` only checks it exists | `true` |

| Provider | `S3_ENDPOINT` | `S3_REGION` | `S3_FORCE_PATH_STYLE` |
|---|---|---|---|
| AWS S3 | `https://s3.<region>.amazonaws.com` | the bucket's region | `false` |
| Cloudflare R2 | `https://<account-id>.r2.cloudflarestorage.com` | `auto` | `true` |
| Backblaze B2 | `https://s3.<region>.backblazeb2.com` | e.g. `us-west-004` | `true` |
| Your own MinIO / Silo | `https://s3.example.com` | as configured (`us-east-1`) | `true` |

The credentials need `GetObject`, `PutObject` (incl. multipart), `DeleteObject`, `ListBucket` / `HeadBucket`
on the bucket (plus `CreateBucket` if `S3_CREATE_BUCKET=true`). Keep the bucket private. Browsers download
directly from storage; if they upload nothing there, no CORS rules are needed (uploads go through the API).

On start the API logs `object storage ready` (endpoint, bucket, public endpoint) or `object storage
unavailable` with the reason; `/api/v1/health/ready` reports the same.

Outside compose (e.g. `make api-run`) the same `S3_*` variables apply; the default endpoint there is
`http://localhost:9000`.
