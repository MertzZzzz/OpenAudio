# VST3 and CLAP plug-ins

OpenAudio **distributes** plug-ins; it does not run them (spec §1, §14 — hosting stays out of scope).
A release of an `instrument`, `effect`, `midi-tool` or `utility` package can ship VST3 and CLAP builds for
Windows, macOS and Linux. Listeners pick the build for their system on the package page and download it as
one zip, ready to unpack into their plug-in folder.

## Why bundles need special handling

Most plug-ins are not single files:

| Format | macOS | Windows | Linux |
|---|---|---|---|
| VST3 | `Tape.vst3/Contents/MacOS/Tape` (+ `Info.plist`) | `Tape.vst3/Contents/x86_64-win/Tape.vst3` (legacy: a single `Tape.vst3` file) | `Tape.vst3/Contents/x86_64-linux/Tape.so` |
| CLAP | `Tape.clap/Contents/MacOS/Tape` (+ `Info.plist`) | single file `Tape.clap` | single file `Tape.clap` |

A VST3 bundle may hold several platforms at once (`Contents/MacOS` + `Contents/x86_64-win` + …).
Architecture folders: `x86_64-win`, `arm64-win`, `arm64ec-win`, `x86-win`, `x86_64-linux`, `aarch64-linux`,
`i386-linux`; macOS uses one (universal) executable.

Every file of a bundle is uploaded as its own content-addressed artifact (`kind: "binary"`), so unchanged
files are never stored twice across versions.

## Manifest: `plugins[]`

```json
"plugins": [
  { "format": "vst3", "bundle": "Tape FX.vst3", "os": "macos",   "arch": ["x86_64", "arm64"] },
  { "format": "vst3", "bundle": "Tape FX.vst3", "os": "windows", "arch": ["x86_64"], "pluginIds": ["0123456789ABCDEF0123456789ABCDEF"] },
  { "format": "clap", "bundle": "linux/tape-fx.clap", "os": "linux", "arch": ["x86_64"], "pluginIds": ["com.tapelabs.tape-fx"] }
]
```

| Field | |
|---|---|
| `format` | `vst3` or `clap` |
| `bundle` | path of the `.vst3` / `.clap` folder (or single file) inside the release |
| `os` | `windows`, `macos`, `linux` — one entry per OS a bundle serves |
| `arch` | `x86_64`, `arm64`, `arm64ec` (Windows), `i386` |
| `pluginIds` | optional: CLAP ids (reverse-DNS) or VST3 class ids (32 hex digits) |
| `minOsVersion` | optional, e.g. `10.13` |

Checks on publish (`api/internal/manifest/plugins.go`, mirrored in `web/src/shared/lib/plugins.ts`):

- only for the four module kinds above; the bundle name ends in `.<format>`;
- the bundle exists in `artifacts` and all its files are `binary`;
- macOS builds are folders with `Contents/MacOS/…` (`Info.plist` missing → warning);
- CLAP on Windows/Linux is a single file; VST3 on Linux is a folder; single-file VST3 on Windows → warning
  (legacy layout);
- every declared VST3 architecture has its `Contents/<arch>-win|linux/` folder;
- no two builds claim the same format + OS + architecture; bundles are not nested;
- ids that do not look like CLAP / VST3 ids → warning.

## Uploading (web)

Step 3 of the release wizard accepts folders: **Add folder** (or drag & drop a `.vst3` / `.clap` folder).
`.DS_Store`, `Thumbs.db`, `desktop.ini`, `._*` and `__MACOSX/` are skipped; `_CodeSignature` is kept.
The wizard proposes the builds:

- folder layout gives the OS (and VST3 architectures);
- the executable header gives OS and architectures for single files and macOS binaries
  (PE machine → Windows, ELF `e_machine` → Linux, Mach-O / universal binary → macOS);
- proposals follow detection until the publisher edits them (OS, architectures, plug-in ids; more platforms
  for a multi-platform bundle with "Add platform").

At most 4 files hash/upload at once, so bundles with hundreds of files do not flood the API.

## Downloading

`GET /packages/{publisher}/{slug}/releases/{version}/bundles/{bundle}` streams the bundle as a zip whose root
is the bundle itself (`Tape FX.vst3/Contents/…`); executables (`Contents/MacOS/*`, `*-linux/*`, single-file
plug-ins) keep mode 0755. Only bundles declared in `plugins[]` can be fetched; each download is counted.
File name: `<publisher>-<package>-<version>-<format>-<os…>.zip`.

The package page lists builds with the visitor's OS first ("your system").

## Search

`pluginFormat` (`vst3`, `clap`) and `pluginOs` (`windows`, `macos`, `linux`) filter `/search` and
`/search/facets`. With both, **one build** must match both (`clap` + `macos` does not match a package that has
CLAP only for Linux and VST3 for macOS). The facets count the same way. `vst3` / `clap` in `q` match too.
Index columns: `package_search.plugin_formats`, `plugin_os`, `plugin_builds` (`"vst3:macos"`), migration 000007.

## Not yet

- Code-signing / notarisation checks, malware scanning of binaries.
- AU (`.component`) and AAX builds.
- Private packages: the zip link is a plain URL, so it works for public packages; members of private packages
  download individual files from the Files tab.
- Installing into plug-in folders automatically (needs a desktop client).
