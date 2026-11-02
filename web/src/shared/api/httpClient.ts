import type { ApiClient, DiscoverFeed } from './client';
import { ApiError } from './client';
import { API_BASE, request } from './http';
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
  SearchFacets,
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
  downloads?: number;
}

interface SampleDTO {
  id: string;
  packageId: string;
  packageName: string;
  name: string;
  file: string;
  digest: string;
  contentType: Sample['contentType'];
  instrument?: string;
  role?: Sample['role'];
  bpm: number | null;
  key: Sample['key'] | null;
  durationSec: number | null;
  tags: string[];
  downloadUrl: string;
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
    downloads: d.downloads ?? 0,
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

function toSample(d: SampleDTO): Sample {
  return {
    id: d.id,
    packageId: d.packageId,
    packageName: d.packageName,
    name: d.name,
    file: d.file,
    instrument: d.instrument ?? 'fx',
    role: d.role ?? 'fx',
    bpm: d.bpm ?? undefined,
    key: d.key ?? undefined,
    durationSec: d.durationSec ?? 0,
    contentType: d.contentType,
    tags: d.tags,
    previewUrl: '',
    digest: d.digest,
    downloadUrl: d.downloadUrl,
  };
}

/** Maps UI search params to the /search query string (lists are comma-separated). */
export function searchQuery(p: SearchParams, paging: boolean): string {
  const qs = new URLSearchParams();
  if (p.q) qs.set('q', p.q);
  const lists: [string, string[] | undefined][] = [
    ['kind', p.kind], ['genre', p.genre], ['instrument', p.instrument], ['mood', p.mood],
    ['license', p.license], ['moduleClass', p.moduleClass], ['key', p.key],
  ];
  for (const [k, v] of lists) if (v?.length) qs.set(k, v.join(','));
  if (p.bpmMin !== undefined) qs.set('bpmMin', String(p.bpmMin));
  if (p.bpmMax !== undefined) qs.set('bpmMax', String(p.bpmMax));
  if (paging) {
    if (p.sort) qs.set('sort', p.sort);
    if (p.page) qs.set('page', String(p.page));
    if (p.limit) qs.set('limit', String(Math.min(p.limit, 100)));
  }
  return qs.toString();
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

export function createHttpClient(): ApiClient {
  const getPackage = async (publisher: string, slug: string) => toPackage(await request<PackageDTO>(pkgPath(publisher, slug)));
  const listVersions = async (publisher: string, slug: string) => (await request<ReleaseDTO[]>(`${pkgPath(publisher, slug)}/versions`)).map(toRelease);

  const manifestOf = async (publisher: string, slug: string, version: string): Promise<Record<string, unknown>> =>
    request<Record<string, unknown>>(`${pkgPath(publisher, slug)}/versions/${encodeURIComponent(version)}/manifest`);

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
      const [popular, newest] = await Promise.all([
        request<{ items: PackageDTO[] }>('/search?sort=downloads&limit=8'),
        request<{ items: PackageDTO[] }>('/search?sort=newest&limit=8'),
      ]);
      return { popular: popular.items.map(toPackage), newReleases: newest.items.map(toPackage), editorsPicks: [] };
    },

    async searchPackages(params): Promise<Page<Package>> {
      const res = await request<{ items: PackageDTO[]; total: number; page: number; limit: number }>(`/search?${searchQuery(params, true)}`);
      return { items: res.items.map(toPackage), total: res.total, page: res.page, limit: res.limit };
    },

    searchFacets: (params) => request<SearchFacets>(`/search/facets?${searchQuery(params, false)}`),

    getPackage,
    getPackageById(id) {
      const [pub, slug] = splitId(id);
      return getPackage(pub, slug);
    },
    listVersions,
    getManifest: manifestOf,

    async listArtifacts(publisher, slug, version) {
      const r = await request<{ artifacts: ArtifactDTO[] }>(`${pkgPath(publisher, slug)}/releases/${encodeURIComponent(version)}`);
      const filesBase = `${API_BASE}${pkgPath(publisher, slug)}/releases/${encodeURIComponent(version)}/files/`;
      return r.artifacts.map((a) => ({
        kind: a.kind,
        path: a.path,
        digest: a.digest,
        size: a.size,
        contentType: a.contentType ?? '',
        // Downloads go through the API (counted, then redirected to a presigned URL).
        downloadUrl: a.downloadUrl ? filesBase + a.path.split('/').map(encodeURIComponent).join('/') : undefined,
      }));
    },

    async searchSamples(query) {
      const qs = new URLSearchParams({ limit: '100' });
      if (query.q) qs.set('q', query.q);
      if (query.packageId) qs.set('package', query.packageId);
      if (query.instrument?.length) qs.set('instrument', query.instrument.join(','));
      if (query.contentType?.length === 1) qs.set('contentType', query.contentType[0]!);
      if (query.key?.length) qs.set('key', query.key.join(','));
      if (query.bpmMin !== undefined) qs.set('bpmMin', String(query.bpmMin));
      if (query.bpmMax !== undefined) qs.set('bpmMax', String(query.bpmMax));
      if (query.durationMax !== undefined) qs.set('durationMax', String(query.durationMax));
      const res = await request<{ items: SampleDTO[] }>(`/search/samples?${qs}`);
      return res.items.map(toSample);
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

