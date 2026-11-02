# Publishing packages

```text
create publisher ──▶ create package ──▶ upload blobs ──▶ draft release (manifest) ──▶ publish ──▶ (yank)
                                          resumable,          validated, normalized,     immutable,
                                          SHA-256 verified    canonical digest           all blobs checked
```

## Roles

Creating a publisher requires a verified email; the creator becomes its **owner** (and a plain `user`
account is promoted to the `publisher` platform role).

| | draft & upload | publish / yank | create & edit packages | manage members |
|---|:-:|:-:|:-:|:-:|
| owner | ✓ | ✓ | ✓ | ✓ |
| maintainer | ✓ | ✓ | ✓ | |
| uploader | ✓ | | | |

Platform admins act as owners of every publisher.

## Uploading blobs

Artifacts are content-addressed: each file is stored once at `blobs/{sha256}` no matter how many
releases or packages reference it.

1. `POST /api/v1/uploads {digest, size, contentType}` — returns `status: "done"` immediately if the blob
   already exists; otherwise an upload session with `partSize` and `partCount`.
2. `PUT /api/v1/uploads/{id}/parts/{n}` with the raw bytes. Every part except the last is exactly
   `partSize` (default 8 MiB, `UPLOAD_PART_SIZE`). Parts may arrive in any order and be retried;
   `GET /api/v1/uploads/{id}` lists the parts already received, so interrupted uploads resume.
3. `POST /api/v1/uploads/{id}/complete` — the server assembles the parts at `uploads/{id}`, **re-hashes the
   whole object**, and only if SHA-256 and size match the declared digest copies it to `blobs/{sha256}`.
   Otherwise it answers `422 digest_mismatch` and stores nothing.

Each part is sent to S3 with its MD5 and SHA-256, so storage also verifies bytes in transit.
Maximum blob size: `MAX_BLOB_SIZE` (default 2 GiB).

## Manifests

The `openaudio.package/v1` JSON Schema lives in
[`api/internal/manifest/openaudio.package.v1.json`](../api/internal/manifest/openaudio.package.v1.json) and is
served at `GET /api/v1/schemas/openaudio.package/v1`. On top of the schema the server checks:

- module class I/O rules (spec §5.4: instruments need event input and audio output, …);
- artifact paths are relative and normalised (no `..`), unique, and `manifest.json` is reserved;
- `samples[].file` refers to a listed artifact; dependency ranges are valid, no self-dependency;
- recommendations (warnings, never blocking): description, 1–3 moods, BPM value inside its range, …

The server owns identity fields: it fills `schema`, `id` (`<publisher>.<package>`), `publisher`, `kind`,
`name` and rejects conflicting values. The normalized manifest is serialised **canonically** (sorted
keys, no whitespace, numbers verbatim); its SHA-256 is the release's `manifestDigest`. The exact bytes are
stored in PostgreSQL (`releases.manifest_canonical`, plus JSONB for queries) and on publish in S3 at
`manifests/{packageId}/{version}/manifest.json`. `GET …/versions/{v}/manifest` serves those bytes with the
digest as ETag, so clients can verify what they download.

`POST /api/v1/manifests/validate` runs all checks without storing anything (the web wizard uses it).

## Releases

- `POST /packages/{publisher}/{slug}/releases {version?, changelog, manifest}` — creates a **draft**.
  Drafts can be replaced (`PUT`) or deleted and are only visible to members.
- `POST …/releases/{version}/publish` — re-validates, checks that every artifact exists in `blobs/` with
  its declared size (`422 release_incomplete` lists missing ones), stores the manifest in S3, records
  artifacts and taxonomy (for search) and recomputes `latestVersion`.
- Published releases are **immutable**: enforced in the use case and by database triggers on `releases`
  and `artifacts`. Fixes ship as a new version.
- `POST …/releases/{version}/yank` — hides a release from new installs (`latestVersion` falls back to the
  previous published version) while existing lockfiles can still download it.
- `GET …/releases/{version}` lists artifacts with short-lived presigned download URLs (`DOWNLOAD_URL_TTL`).

## Example with curl

```bash
API=http://localhost:5173/api/v1
TOKEN=...   # accessToken from /auth/login
FILE=kick.wav
DIGEST="sha256:$(sha256sum $FILE | cut -d' ' -f1)"; SIZE=$(stat -c%s $FILE)

curl -s -X POST $API/publishers -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"slug":"acme","name":"Acme Audio"}'
curl -s -X POST $API/packages -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"publisher":"acme","slug":"kit","name":"Warm Kit","kind":"sample-pack"}'

UP=$(curl -s -X POST $API/uploads -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"digest\":\"$DIGEST\",\"size\":$SIZE}")
ID=$(echo $UP | jq -r .id)
curl -s -X PUT $API/uploads/$ID/parts/1 -H "Authorization: Bearer $TOKEN" --data-binary @$FILE   # files < 8 MiB: one part
curl -s -X POST $API/uploads/$ID/complete -H "Authorization: Bearer $TOKEN"

curl -s -X POST $API/packages/acme/kit/releases -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "{
  \"changelog\": \"First release\",
  \"manifest\": {\"version\": \"1.0.0\",
    \"license\": {\"content\": \"CC0-1.0\", \"commercialUse\": true, \"attributionRequired\": false},
    \"taxonomy\": {\"genres\": {\"primary\": \"lo-fi\"}, \"instruments\": [\"drums\"], \"moods\": [\"warm\"]},
    \"artifacts\": [{\"kind\": \"sample\", \"path\": \"kick.wav\", \"digest\": \"$DIGEST\", \"size\": $SIZE}]}}"
curl -s -X POST $API/packages/acme/kit/releases/1.0.0/publish -H "Authorization: Bearer $TOKEN"
```

## Not yet

- Background preview generation (spec §8) and audio analysis.
- Server-side search with facets (Phase 5): the web app filters the first 100 packages client-side meanwhile.
- Server-side libraries and lockfiles (Phase 6): the web app keeps the library in `localStorage` meanwhile.
- Cleanup of abandoned uploads (`uploads/` objects of sessions never completed).
- Invitations for people without an account (members must sign up first).
