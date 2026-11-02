import type { ApiClient, DiscoverFeed } from './client';
import { ApiError } from './client';
import { API_BASE, getAccessToken, request, requestRaw } from './http';
import { taxonomyTerms } from './mock/taxonomy';
import type {
  Artifact,
  Library,
  Lockfile,
  ResolveResult,
  SavedLockfile,
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

/** The "sub" claim of the current access token (no verification — only used as a cache key). */
function tokenSubject(): string | null {
  const t = getAccessToken();
  const payload = t?.split('.')[1];
  if (!payload) return null;
  try {
    return (JSON.parse(atob(payload.replace(/-/g, '+').replace(/_/g, '/'))) as { sub?: string }).sub ?? null;
  } catch {
    return null;
  }
}

const pkgPath = (publisher: string, slug: string) => `/packages/${encodeURIComponent(publisher)}/${encodeURIComponent(slug)}`;

interface LibraryItemDTO {
  packageId: string;
  package: PackageDTO;
  pinnedVersion: string | null;
  installedVersion: string | null;
  addedAt: string;
}

/* ---------------------------------------------------------------- client */

export function createHttpClient(): ApiClient {
  const getPackage = async (publisher: string, slug: string) => toPackage(await request<PackageDTO>(pkgPath(publisher, slug)));
  const listVersions = async (publisher: string, slug: string) => (await request<ReleaseDTO[]>(`${pkgPath(publisher, slug)}/versions`)).map(toRelease);

  const manifestOf = async (publisher: string, slug: string, version: string): Promise<Record<string, unknown>> =>
    request<Record<string, unknown>>(`${pkgPath(publisher, slug)}/versions/${encodeURIComponent(version)}/manifest`);

  // Libraries are per user; the UI works on the first one (the server creates "My Library" on demand).
  let defaultLibrary: Promise<{ id: string; name: string }> | null = null;
  let libraryOwner: string | null = null;
  const library = () => {
    // Cache per signed-in user (the JWT subject), so switching accounts in one tab is safe.
    const owner = tokenSubject();
    if (owner !== libraryOwner) {
      defaultLibrary = null;
      libraryOwner = owner;
    }
    defaultLibrary ??= request<{ id: string; name: string }[]>('/libraries')
      .then((libs) => libs[0]!)
      .catch((e: unknown) => {
        defaultLibrary = null;
        throw e;
      });
    return defaultLibrary;
  };
  const libPath = async (suffix = '') => `/libraries/${(await library()).id}${suffix}`;

  async function getLibrary(): Promise<Library> {
    const lib = await library();
    const items = await request<LibraryItemDTO[]>(`/libraries/${lib.id}/items`);
    return {
      id: lib.id,
      name: lib.name,
      items: items.map((i) => ({
        packageId: i.packageId,
        pinnedVersion: i.pinnedVersion,
        installedVersion: i.installedVersion ?? i.package.latestVersion ?? '0.0.0',
        addedAt: i.addedAt,
      })),
    };
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

    /* ------------------------------------------------------- library */

    getLibrary,

    async addToLibrary(packageId, pinnedVersion = null) {
      await request(await libPath('/items'), { method: 'POST', body: { packageId, ...(pinnedVersion ? { pinnedVersion } : {}) } });
      return getLibrary();
    },

    async removeFromLibrary(packageId) {
      await request(await libPath(`/items/${encodeURIComponent(packageId)}`), { method: 'DELETE' });
      return getLibrary();
    },

    async pinVersion(packageId, version) {
      await request(await libPath(`/items/${encodeURIComponent(packageId)}`), { method: 'PATCH', body: { pinnedVersion: version } });
      return getLibrary();
    },

    async updateLibraryItem(packageId) {
      await request(await libPath(`/items/${encodeURIComponent(packageId)}`), { method: 'PATCH', body: { upgrade: true } });
      return getLibrary();
    },

    async resolveLibrary() {
      return request<ResolveResult>(await libPath('/resolve'), { method: 'POST' });
    },

    async importLockfile(lockfile) {
      const res = await request<{ pinned: number; warnings: ValidationIssue[] }>(await libPath('/import'), { method: 'POST', body: { lockfile } });
      return { library: await getLibrary(), warnings: res.warnings };
    },

    async saveLockfile() {
      const res = await request<SavedLockfile & { lockfile: Lockfile }>(await libPath('/lockfiles'), { method: 'POST' });
      const { lockfile, ...saved } = res;
      return { saved, lockfile };
    },

    listLockfiles: () => request<SavedLockfile[]>('/lockfiles'),

    async lockfileContent(id) {
      return (await requestRaw(`/lockfiles/${encodeURIComponent(id)}`)).text();
    },

    deleteLockfile: (id) => request<void>(`/lockfiles/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  };
}

