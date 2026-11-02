# Libraries and lockfiles

## Libraries

Every user has personal libraries (a default **My Library** is created on first use; more can be created via
`POST /api/v1/libraries`). An item is a package plus:

- `pinnedVersion` — an exact version the user fixed (must be a published release when pinned);
- `installedVersion` — the version currently installed. **Upgrade** (`PATCH …/items/{id} {"upgrade": true}`)
  moves an unpinned item to the latest version.

Libraries and lockfiles are private: other users get 404s.

## Resolution

`POST /api/v1/libraries/{id}/resolve` computes one version per package:

1. a pinned item keeps its exact version — even if it was yanked later (old projects stay installable; a warning is
   returned);
2. an unpinned item keeps its installed version while it satisfies every range (no silent upgrades);
3. otherwise the highest **published** version satisfying every range is chosen; yanked versions are never chosen
   by ranges.

Dependencies come from the `dependencies` of the **chosen version's manifest** (`[{id, range}]` with `*`, exact,
`^`, `~`, `>=`), so the graph can change with the selection; the resolver iterates until it is stable (cycles are
fine). Conflicts list the package, the reason (`no_matching_version` / `package_not_found`), who requested which
range (`"library"` for the user's pin) and the available versions. The web app offers one-click fixes
(pin to the best common version, or unpin).

## Lockfile format

```json
{
  "lockfileVersion": 1,
  "name": "My Library",
  "generatedAt": "2026-11-02T12:00:00Z",
  "packages": {
    "lattice.grain-cloud": {
      "id": "lattice.grain-cloud",
      "version": "0.9.0",
      "manifestDigest": "sha256:01fc…",
      "resolved": "openaudio://lattice/grain-cloud@0.9.0",
      "direct": true,
      "dependencies": { "fieldnotes.nordic-field-recordings": "^1.0.0" }
    },
    "fieldnotes.nordic-field-recordings": { "…": "…", "direct": false }
  }
}
```

`manifestDigest` is the sha256 of the release's canonical manifest, which in turn lists the sha256 of every
artifact — so one digest pins the exact bytes of the whole release.

**Export** (`POST /libraries/{id}/lockfiles`) resolves, refuses with `409 resolve_conflicts` if anything conflicts,
and stores the lockfile as canonical JSON with its sha256 (`contentDigest`). `GET /lockfiles/{id}` returns those exact
bytes (the web app downloads them verbatim, so the file on disk hashes to `contentDigest`).

**Import** (`POST /libraries/{id}/import {lockfile}`) verifies *every* entry against the registry — package and
version exist, `manifestDigest` matches — and rejects the whole file (`422 lockfile_invalid`) on any mismatch, so a
tampered or foreign lockfile cannot change a library. Entries with `direct: true` (or without the field) become pins;
`direct: false` entries are verified and then resolved as dependencies.

## Previews

Play buttons use `GET /packages/{publisher}/{slug}/preview` (a `preview` artifact of the latest release, else its
first sample) and `…/files/{path}?preview=1`; both redirect to presigned URLs, play in an HTML5 audio element and
are not counted as downloads. Generating compressed previews in a background job (spec §8) is not implemented yet:
the original files are streamed.
