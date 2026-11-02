import type { ApiClient, DiscoverFeed } from './client';
import { ApiError } from './client';
import { request } from './http';
import { buildFacets, searchPage } from './catalogQuery';
import { taxonomyTerms } from './mock/taxonomy';
import { resolve } from './mock/resolver';
import type {
  Artifact,
  Library,
  LibraryItem,
  Package,
  PackageKind,
  Page,
  PublisherMember,
  PublisherMembership,
  Release,
  Sample,
  SearchParams,
  ValidationIssue,
} from '@/shared/types/domain';
import { compare } from '@/shared/lib/semver';

/* ------------------------------------------------------------ server DTOs */

interface PackageDTO {
  id: string;
  publisher: { id: string; slug: string; name: string; verified: boolean };
  slug: string;
  name: string;
  kind: PackageKind;
  description: string;
  latestVersion: string | null;
  createdAt: string;
  updatedAt: string;
  publishedAt: string | null;
  stats: { artifacts: number; samples: number; sizeBytes: number };
  license: Package['license'] | null;
  taxonomy: Partial<Package['taxonomy']> | null;
  musical: Package['musical'] | null;
  technical: Package['technical'] | null;
  module: (Omit<NonNullable<Package['module']>, 'ports'> & { ports?: NonNullable<Package['module']>['ports'] }) | null;
  dependencies: { id: string; range: string }[];
  tags: string[];
}

interface ReleaseDTO {
  version: string;
  status: Release['status'];
  publishedAt: string | null;
  changelog: string;
  manifestDigest: string;
}

interface ArtifactDTO {
  kind: Artifact['kind'];
  path: string;
  digest: string;
  size: number;
  contentType?: string;
  downloadUrl?: string;
}

interface UploadDTO {
  id: string;
  status: 'pending' | 'done' | 'failed';
  partSize: number;
  partCount: number;
  parts: { number: number; size: number }[];
}

/* --------------------------------------------------------------- mapping */

function hue(id: string): number {
  let h = 0;
  for (let i = 0; i < id.length; i += 1) h = (h * 31 + id.charCodeAt(i)) >>> 0;
  return h % 360;
}

export function toPackage(d: PackageDTO): Package {
  const t = d.taxonomy ?? {};
  return {
    id: d.id,
    publisher: { ...d.publisher, verified: d.publisher.verified ?? false },
    slug: d.slug,
    name: d.name,
    kind: d.kind,
    description: d.description,
    latestVersion: d.latestVersion ?? '0.0.0',
    license: d.license ?? { content: '', commercialUse: false, attributionRequired: false },
    taxonomy: {
      genres: { primary: t.genres?.primary ?? '', secondary: t.genres?.secondary ?? [] },
      instruments: t.instruments ?? [],
      moods: t.moods ?? [],
      useCases: t.useCases ?? [],
      energy: t.energy,
    },
    musical: d.musical ?? undefined,
    technical: { formats: [], ...(d.technical ?? {}) },
    module: d.module ? { ...d.module, ports: d.module.ports ?? [], categories: d.module.categories ?? [] } : undefined,
    tags: d.tags,
    downloads: 0,
    rating: 0,
    updatedAt: d.publishedAt ?? d.updatedAt,
    createdAt: d.createdAt,
    sampleCount: d.stats.samples,
    sizeBytes: d.stats.sizeBytes,
    coverHue: hue(d.id),
    dependencies: d.dependencies.map((x) => ({ packageId: x.id, range: x.range })),
  };
}

const toRelease = (r: ReleaseDTO): Release => ({
  version: r.version,
  status: r.status,
  publishedAt: r.publishedAt,
  changelog: r.changelog,
  manifestDigest: r.manifestDigest,
});

/** "acme.deep-house-toolkit" → ["acme", "deep-house-toolkit"] (slugs contain no dots). */
function splitId(id: string): [string, string] {
  const i = id.indexOf('.');
  if (i < 0) throw new ApiError(404, 'package_not_found', id);
  return [id.slice(0, i), id.slice(i + 1)];
}

const pkgPath = (publisher: string, slug: string) => `/packages/${encodeURIComponent(publisher)}/${encodeURIComponent(slug)}`;

/* ---------------------------------------------------------- local library */

// Libraries move server-side in Phase 6; until then they live in localStorage.
const LIBRARY_KEY = 'openaudio.library.v1';

function loadLibrary(): Library {
  try {
    const raw = localStorage.getItem(LIBRARY_KEY);
    if (raw) return JSON.parse(raw) as Library;
  } catch {
    /* ignore */
  }
  return { id: 'local', name: 'My Library', items: [] };
}

function saveLibrary(lib: Library): Library {
  try {
    localStorage.setItem(LIBRARY_KEY, JSON.stringify(lib));
  } catch {
    /* ignore */
  }
  return lib;
}

/* ---------------------------------------------------------------- client */

/** Catalog cache window for client-side search (server-side search arrives in Phase 5). */
const SEARCH_WINDOW = 100;

export function createHttpClient(): ApiClient {
  const allPackages = async (params: Pick<SearchParams, 'q' | 'kind'> = {}): Promise<Package[]> => {
    const qs = new URLSearchParams({ limit: String(SEARCH_WINDOW), sort: 'newest' });
    // Text search stays client-side (aliases, tags, taxonomy); kind can be filtered server-side.
    if (params.kind?.length === 1) qs.set('kind', params.kind[0]!);
    const page = await request<{ items: PackageDTO[] }>(`/packages?${qs}`);
    return page.items.map(toPackage);
  };

  const getPackage = async (publisher: string, slug: string) => toPackage(await request<PackageDTO>(pkgPath(publisher, slug)));
  const listVersions = async (publisher: string, slug: string) => (await request<ReleaseDTO[]>(`${pkgPath(publisher, slug)}/versions`)).map(toRelease);

  const manifestOf = async (publisher: string, slug: string, version: string): Promise<Record<string, unknown>> =>
    request<Record<string, unknown>>(`${pkgPath(publisher, slug)}/versions/${encodeURIComponent(version)}/manifest`);

  async function samplesOf(p: Package): Promise<Sample[]> {
    if (p.latestVersion === '0.0.0' || p.sampleCount === 0) return [];
    const m = (await manifestOf(p.publisher.slug, p.slug, p.latestVersion)) as {
      artifacts?: ArtifactDTO[];
      samples?: { id: string; file: string; contentType?: Sample['contentType']; instrument?: string; role?: Sample['role']; bpm?: number; key?: Sample['key']; durationSec?: number; tags?: string[] }[];
    };
    const meta = new Map((m.samples ?? []).map((s) => [s.file, s]));
    return (m.artifacts ?? [])
      .filter((a) => a.kind === 'sample')
      .map((a) => {
        const s = meta.get(a.path);
        const name = a.path.split('/').pop()!.replace(/\.[a-z0-9]+$/i, '');
        return {
          id: `${p.id}/${a.path}`,
          packageId: p.id,
          name: s?.id ?? name,
          file: a.path,
          instrument: s?.instrument ?? p.taxonomy.instruments[0] ?? 'fx',
          role: s?.role ?? 'fx',
          bpm: s?.bpm,
          key: s?.key,
          durationSec: s?.durationSec ?? 0,
          contentType: s?.contentType ?? 'one-shot',
          tags: s?.tags ?? p.tags.slice(0, 2),
          previewUrl: '',
          digest: a.digest,
        } satisfies Sample;
      });
  }

  const libraryPackages = async (lib: Library) => {
    const pkgs = await Promise.all(lib.items.map((i) => getPackageByIdSafe(i.packageId)));
    return pkgs.filter((p): p is Package => p !== null);
  };

  async function getPackageByIdSafe(id: string): Promise<Package | null> {
    try {
      const [pub, slug] = splitId(id);
      return await getPackage(pub, slug);
    } catch {
      return null;
    }
  }

  return {
    async discover(): Promise<DiscoverFeed> {
      const list = await allPackages();
      return { popular: list.slice(0, 8), newReleases: list.slice(0, 8), editorsPicks: [] };
    },

    async searchPackages(params): Promise<Page<Package>> {
      return searchPage(await allPackages(params), { ...params, sort: params.sort ?? (params.q ? 'relevance' : 'newest') });
    },

    async searchFacets(params) {
      return buildFacets(await allPackages(), params);
    },

    getPackage,
    getPackageById(id) {
      const [pub, slug] = splitId(id);
      return getPackage(pub, slug);
    },
    listVersions,
    getManifest: manifestOf,

    async listArtifacts(publisher, slug, version) {
      const r = await request<{ artifacts: ArtifactDTO[] }>(`${pkgPath(publisher, slug)}/releases/${encodeURIComponent(version)}`);
      return r.artifacts.map((a) => ({ kind: a.kind, path: a.path, digest: a.digest, size: a.size, contentType: a.contentType ?? '' }));
    },

    async searchSamples(query) {
      let pkgs: Package[];
      if (query.packageId) {
        pkgs = [await this.getPackageById(query.packageId)];
      } else {
        pkgs = (await allPackages({ kind: ['sample-pack'] })).filter((p) => p.sampleCount > 0).slice(0, 20);
      }
      const all = (await Promise.all(pkgs.map(samplesOf))).flat();
      const q = query.q?.toLowerCase().trim();
      return all.filter((s) => {
        if (q && !s.name.toLowerCase().includes(q) && !s.tags.some((t) => t.includes(q)) && !s.instrument.includes(q)) return false;
        if (query.instrument?.length && !query.instrument.includes(s.instrument)) return false;
        if (query.contentType?.length && !query.contentType.includes(s.contentType)) return false;
        if (query.bpmMin !== undefined && (s.bpm === undefined || s.bpm < query.bpmMin)) return false;
        if (query.bpmMax !== undefined && (s.bpm === undefined || s.bpm > query.bpmMax)) return false;
        return true;
      });
    },

    taxonomy: () => Promise.resolve(taxonomyTerms),

    /* --------------------------------------------------- publishing */

    async myPublishers() {
      const list = await request<{ slug: string; name: string; role: PublisherMembership['role'] }[]>('/me/publishers');
      return list.map((p) => ({ slug: p.slug, name: p.name, role: p.role }));
    },

    async createPublisher(slug, name) {
      const p = await request<{ slug: string; name: string }>('/publishers', { method: 'POST', body: { slug, name } });
      return { slug: p.slug, name: p.name, role: 'owner' };
    },

    async publisherPackages(publisher) {
      const rows = await request<(PackageDTO & { releases: ReleaseDTO[] })[]>(`/publishers/${encodeURIComponent(publisher)}/packages`);
      return rows.map((r) => ({ ...toPackage(r), releases: r.releases.map(toRelease) }));
    },

    publisherMembers: (publisher) => request<PublisherMember[]>(`/publishers/${encodeURIComponent(publisher)}/members`),

    inviteMember: (publisher, email, role) =>
      request<PublisherMember>(`/publishers/${encodeURIComponent(publisher)}/members`, { method: 'POST', body: { email, role } }),

    removeMember: (publisher, userId) =>
      request<void>(`/publishers/${encodeURIComponent(publisher)}/members/${encodeURIComponent(userId)}`, { method: 'DELETE' }),

    async createPackage(publisher, input) {
      return toPackage(await request<PackageDTO>('/packages', { method: 'POST', body: { publisher, ...input } }));
    },

    async validateManifest(manifest, target) {
      const res = await request<{ issues: ValidationIssue[] }>('/manifests/validate', {
        method: 'POST',
        body: { manifest, ...(target ? { publisher: target.publisher, package: target.slug } : {}) },
      });
      return res.issues;
    },

    async uploadBlob(file, { digest, contentType, onProgress }) {
      const u = await request<UploadDTO>('/uploads', { method: 'POST', body: { digest, size: file.size, contentType } });
      if (u.status === 'done') {
        onProgress?.({ fraction: 1, phase: 'done' });
        return;
      }
      // Resume: skip parts the server already has.
      const have = new Set(u.parts.map((p) => p.number));
      let sent = u.parts.reduce((n, p) => n + p.size, 0);
      onProgress?.({ fraction: sent / file.size, phase: 'uploading' });
      for (let n = 1; n <= u.partCount; n += 1) {
        if (have.has(n)) continue;
        const chunk = file.slice((n - 1) * u.partSize, Math.min(file.size, n * u.partSize));
        await request(`/uploads/${u.id}/parts/${n}`, { method: 'PUT', rawBody: chunk });
        sent += chunk.size;
        onProgress?.({ fraction: sent / file.size, phase: 'uploading' });
      }
      onProgress?.({ fraction: 1, phase: 'verifying' });
      await request<UploadDTO>(`/uploads/${u.id}/complete`, { method: 'POST' });
      onProgress?.({ fraction: 1, phase: 'done' });
    },

    async createDraftRelease(input) {
      const res = await request<{ release: ReleaseDTO }>(`${pkgPath(input.publisher, input.slug)}/releases`, {
        method: 'POST',
        body: { version: input.version, changelog: input.changelog, manifest: input.manifest },
      });
      return toRelease(res.release);
    },

    async publishRelease(publisher, slug, version) {
      return toRelease(await request<ReleaseDTO>(`${pkgPath(publisher, slug)}/releases/${encodeURIComponent(version)}/publish`, { method: 'POST' }));
    },

    async yankRelease(publisher, slug, version) {
      return toRelease(await request<ReleaseDTO>(`${pkgPath(publisher, slug)}/releases/${encodeURIComponent(version)}/yank`, { method: 'POST' }));
    },

    /* ------------------------------------- library (local until Phase 6) */

    getLibrary: () => Promise.resolve(loadLibrary()),

    async addToLibrary(packageId, pinnedVersion = null) {
      const lib = loadLibrary();
      if (!lib.items.some((i) => i.packageId === packageId)) {
        const p = await this.getPackageById(packageId);
        const item: LibraryItem = { packageId, pinnedVersion, installedVersion: pinnedVersion ?? p.latestVersion, addedAt: new Date().toISOString() };
        lib.items.push(item);
      }
      return saveLibrary(lib);
    },

    async removeFromLibrary(packageId) {
      const lib = loadLibrary();
      lib.items = lib.items.filter((i) => i.packageId !== packageId);
      return saveLibrary(lib);
    },

    async pinVersion(packageId, version) {
      const lib = loadLibrary();
      const item = lib.items.find((i) => i.packageId === packageId);
      if (item) {
        item.pinnedVersion = version;
        if (version) item.installedVersion = version;
      }
      return saveLibrary(lib);
    },

    async updateLibraryItem(packageId) {
      const lib = loadLibrary();
      const item = lib.items.find((i) => i.packageId === packageId);
      if (item && !item.pinnedVersion) item.installedVersion = (await this.getPackageById(packageId)).latestVersion;
      return saveLibrary(lib);
    },

    async resolveLibrary() {
      const lib = loadLibrary();
      // Fetch the library's packages plus their dependencies (one level per round).
      const pkgs = new Map((await libraryPackages(lib)).map((p) => [p.id, p]));
      for (let round = 0; round < 5; round += 1) {
        const missing = [...pkgs.values()].flatMap((p) => p.dependencies.map((d) => d.packageId)).filter((id) => !pkgs.has(id));
        if (!missing.length) break;
        for (const p of await Promise.all(missing.map(getPackageByIdSafe))) if (p) pkgs.set(p.id, p);
      }
      const releases: Record<string, Release[]> = {};
      await Promise.all(
        [...pkgs.values()].map(async (p) => {
          releases[p.id] = (await listVersions(p.publisher.slug, p.slug)).sort((a, b) => compare(b.version, a.version));
        }),
      );
      return resolve({ library: lib, packages: [...pkgs.values()], releases });
    },

    async importLockfile(lockfile) {
      const lib = loadLibrary();
      for (const entry of Object.values(lockfile.packages)) {
        const existing = lib.items.find((i) => i.packageId === entry.id);
        if (existing) {
          existing.pinnedVersion = entry.version;
          existing.installedVersion = entry.version;
        } else {
          lib.items.push({ packageId: entry.id, pinnedVersion: entry.version, installedVersion: entry.version, addedAt: new Date().toISOString() });
        }
      }
      return saveLibrary(lib);
    },
  };
}

