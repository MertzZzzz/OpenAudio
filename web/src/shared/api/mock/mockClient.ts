import type { ApiClient } from '@/shared/api/client';
import { ApiError } from '@/shared/api/client';
import type {
  FacetBucket,
  KeyMode,
  KeyTonic,
  Library,
  Package,
  Release,
  SearchFacets,
  SearchParams,
} from '@/shared/types/domain';
import { compare } from '@/shared/lib/semver';
import { formatKey, humanize, keyToSlug } from '@/shared/lib/format';
import { validateManifest } from '@/shared/lib/manifest';
import {
  artifactsFor,
  fakeDigest,
  initialLibrary,
  packages as seedPackages,
  publisherMembers as seedMembers,
  publishers,
  releases as seedReleases,
  samples,
} from './data';
import { expandWithChildren, resolveAlias, taxonomyTerms, termLabel } from './taxonomy';
import { resolve } from './resolver';

const clone = <T>(v: T): T => structuredClone(v);

interface MockOptions {
  latencyMs?: number;
}

type FacetKey = keyof SearchFacets;

export function createMockClient({ latencyMs = 250 }: MockOptions = {}): ApiClient {
  // In-memory, per-client state so tests are isolated.
  const state = {
    packages: clone(seedPackages),
    releases: clone(seedReleases),
    library: clone(initialLibrary) as Library,
    members: clone(seedMembers),
  };

  const delay = <T>(value: T): Promise<T> =>
    new Promise((res) => setTimeout(() => res(clone(value)), latencyMs));

  const find = (publisher: string, slug: string): Package => {
    const p = state.packages.find((x) => x.publisher.slug === publisher && x.slug === slug);
    if (!p) throw new ApiError(404, 'package_not_found', `Package ${publisher}/${slug} not found`);
    return p;
  };

  const findRelease = (pkg: Package, version: string): Release => {
    const r = state.releases[pkg.id]?.find((x) => x.version === version);
    if (!r) throw new ApiError(404, 'release_not_found', `Release ${version} not found`);
    return r;
  };

  /* ------------------------------------------------------------- search */

  function matchesText(p: Package, q: string): number {
    const needle = q.trim().toLowerCase();
    if (!needle) return 1;
    const alias = resolveAlias(needle);
    const tax = [p.taxonomy.genres.primary, ...p.taxonomy.genres.secondary, ...p.taxonomy.instruments, ...p.taxonomy.moods];
    let score = 0;
    if (p.name.toLowerCase().includes(needle)) score += 10;
    if (p.publisher.name.toLowerCase().includes(needle)) score += 4;
    if (p.tags.some((t) => t.includes(needle))) score += 3;
    if (tax.some((t) => t.includes(needle))) score += 3;
    if (alias && tax.includes(alias.slug)) score += 5;
    if (p.module?.categories.some((c) => c.includes(needle))) score += 3;
    if (p.description.toLowerCase().includes(needle)) score += 1;
    return score;
  }

  const facetValues: Record<FacetKey, (p: Package) => string[]> = {
    kind: (p) => [p.kind],
    genre: (p) => [p.taxonomy.genres.primary, ...p.taxonomy.genres.secondary],
    instrument: (p) => p.taxonomy.instruments,
    mood: (p) => p.taxonomy.moods,
    license: (p) => [p.license.content],
    moduleClass: (p) => (p.module ? [p.module.moduleClass] : []),
    key: (p) => (p.musical?.key ? [keyToSlug(p.musical.key)] : []),
  };

  function passes(p: Package, params: SearchParams, skip?: FacetKey): boolean {
    const check = (k: FacetKey, selected: string[] | undefined, expand?: (s: string) => string[]) => {
      if (k === skip || !selected?.length) return true;
      const values = facetValues[k](p);
      return selected.some((s) => (expand ? expand(s) : [s]).some((v) => values.includes(v)));
    };
    if (!check('kind', params.kind)) return false;
    if (!check('genre', params.genre, (s) => expandWithChildren('genre', s))) return false;
    if (!check('instrument', params.instrument, (s) => expandWithChildren('instrument', s))) return false;
    if (!check('mood', params.mood)) return false;
    if (!check('license', params.license)) return false;
    if (!check('moduleClass', params.moduleClass)) return false;
    if (!check('key', params.key)) return false;
    const bpm = p.musical?.bpm?.value;
    if (params.bpmMin !== undefined && (bpm === undefined || bpm < params.bpmMin)) return false;
    if (params.bpmMax !== undefined && (bpm === undefined || bpm > params.bpmMax)) return false;
    if (params.q && matchesText(p, params.q) === 0) return false;
    return true;
  }

  function facetLabel(k: FacetKey, v: string): string {
    switch (k) {
      case 'genre': return termLabel('genre', v);
      case 'instrument': return termLabel('instrument', v);
      case 'mood': return termLabel('mood', v);
      case 'key': {
        const [tonic, mode] = v.split('-') as [KeyTonic, KeyMode];
        return formatKey({ tonic, mode });
      }
      default: return humanize(v);
    }
  }

  function buildFacets(params: SearchParams): SearchFacets {
    const out = {} as SearchFacets;
    for (const k of Object.keys(facetValues) as FacetKey[]) {
      const counts = new Map<string, number>();
      for (const p of state.packages) {
        if (!passes(p, params, k)) continue;
        for (const v of new Set(facetValues[k](p))) counts.set(v, (counts.get(v) ?? 0) + 1);
      }
      out[k] = [...counts.entries()]
        .map<FacetBucket>(([value, count]) => ({ value, count, label: facetLabel(k, value) }))
        .sort((a, b) => b.count - a.count || a.label.localeCompare(b.label));
    }
    return out;
  }

  /* -------------------------------------------------------------- client */

  return {
    discover() {
      const all = state.packages;
      return delay({
        popular: [...all].sort((a, b) => b.downloads - a.downloads).slice(0, 8),
        newReleases: [...all].sort((a, b) => b.updatedAt.localeCompare(a.updatedAt)).slice(0, 8),
        editorsPicks: all.filter((p) => p.editorsPick),
      });
    },

    searchPackages(params) {
      const page = params.page ?? 1;
      const limit = params.limit ?? 12;
      const hits = state.packages.filter((p) => passes(p, params));
      const sort = params.sort ?? (params.q ? 'relevance' : 'downloads');
      hits.sort((a, b) => {
        switch (sort) {
          case 'newest': return b.updatedAt.localeCompare(a.updatedAt);
          case 'rating': return b.rating - a.rating;
          case 'downloads': return b.downloads - a.downloads;
          default: return matchesText(b, params.q ?? '') - matchesText(a, params.q ?? '') || b.downloads - a.downloads;
        }
      });
      return delay({ items: hits.slice((page - 1) * limit, page * limit), total: hits.length, page, limit });
    },

    searchFacets(params) {
      return delay(buildFacets(params));
    },

    getPackage(publisher, slug) {
      try {
        return delay(find(publisher, slug));
      } catch (e) {
        return Promise.reject(e);
      }
    },

    getPackageById(id) {
      const p = state.packages.find((x) => x.id === id);
      return p ? delay(p) : Promise.reject(new ApiError(404, 'package_not_found', id));
    },

    listVersions(publisher, slug) {
      const p = find(publisher, slug);
      return delay(state.releases[p.id] ?? []);
    },

    getManifest(publisher, slug, version) {
      const p = find(publisher, slug);
      findRelease(p, version);
      return delay({
        schema: 'openaudio.package/v1',
        id: p.id,
        name: p.name,
        kind: p.kind,
        version,
        description: p.description,
        publisher: { id: p.publisher.id, name: p.publisher.name },
        license: p.license,
        taxonomy: p.taxonomy,
        ...(p.musical ? { musical: p.musical } : {}),
        technical: p.technical,
        ...(p.module ? { module: p.module } : {}),
        ...(p.dependencies.length ? { dependencies: p.dependencies } : {}),
        artifacts: artifactsFor(p, version).filter((a) => a.kind !== 'manifest'),
        tags: { free: p.tags },
      });
    },

    listArtifacts(publisher, slug, version) {
      const p = find(publisher, slug);
      return delay(artifactsFor(p, version));
    },

    searchSamples(query) {
      const q = query.q?.toLowerCase().trim();
      const hits = samples.filter((s) => {
        if (query.packageId && s.packageId !== query.packageId) return false;
        if (q && !s.name.toLowerCase().includes(q) && !s.tags.some((t) => t.includes(q)) && !s.instrument.includes(q)) return false;
        if (query.instrument?.length && !query.instrument.some((i) => expandWithChildren('instrument', i).includes(s.instrument))) return false;
        if (query.contentType?.length && !query.contentType.includes(s.contentType)) return false;
        if (query.key?.length && !(s.key && query.key.includes(keyToSlug(s.key)))) return false;
        if (query.bpmMin !== undefined && (s.bpm === undefined || s.bpm < query.bpmMin)) return false;
        if (query.bpmMax !== undefined && (s.bpm === undefined || s.bpm > query.bpmMax)) return false;
        if (query.durationMax !== undefined && s.durationSec > query.durationMax) return false;
        return true;
      });
      return delay(hits);
    },

    taxonomy() {
      return delay(taxonomyTerms);
    },

    publisherPackages(publisher) {
      return delay(
        state.packages
          .filter((p) => p.publisher.slug === publisher)
          .map((p) => ({ ...p, releases: state.releases[p.id] ?? [] })),
      );
    },

    publisherMembers() {
      return delay(state.members);
    },

    inviteMember(_publisher, email, role) {
      if (state.members.some((m) => m.email === email)) {
        return Promise.reject(new ApiError(409, 'member_exists', `${email} is already a member`));
      }
      const member = { userId: `u${Date.now()}`, name: email.split('@')[0]!, email, role };
      state.members.push(member);
      return delay(member);
    },

    removeMember(_publisher, userId) {
      const m = state.members.find((x) => x.userId === userId);
      if (m?.role === 'owner') return Promise.reject(new ApiError(400, 'cannot_remove_owner', 'The owner cannot be removed'));
      state.members = state.members.filter((x) => x.userId !== userId);
      return delay(undefined);
    },

    createPackage(publisher, input) {
      const pub = publishers.find((p) => p.slug === publisher);
      if (!pub) return Promise.reject(new ApiError(404, 'publisher_not_found', publisher));
      if (state.packages.some((p) => p.publisher.slug === publisher && p.slug === input.slug)) {
        return Promise.reject(new ApiError(409, 'package_exists', `${publisher}/${input.slug} already exists`));
      }
      const now = new Date().toISOString();
      const pkg: Package = {
        id: `${pub.id}.${input.slug}`,
        publisher: pub,
        slug: input.slug,
        name: input.name,
        kind: input.kind,
        description: input.description,
        latestVersion: '0.0.0',
        license: { content: 'CC-BY-4.0', commercialUse: true, attributionRequired: true },
        taxonomy: { genres: { primary: 'electronic', secondary: [] }, instruments: [], moods: [], useCases: [] },
        technical: { formats: [] },
        tags: [],
        downloads: 0,
        rating: 0,
        updatedAt: now,
        createdAt: now,
        sampleCount: 0,
        sizeBytes: 0,
        coverHue: Math.floor(Math.random() * 360),
        dependencies: [],
      };
      state.packages.push(pkg);
      state.releases[pkg.id] = [];
      return delay(pkg);
    },

    validateManifest(manifest) {
      return delay(validateManifest(manifest));
    },

    createDraftRelease(input) {
      const p = find(input.publisher, input.slug);
      const existing = state.releases[p.id] ?? [];
      if (existing.some((r) => r.version === input.version)) {
        return Promise.reject(new ApiError(409, 'version_exists', `Version ${input.version} already exists — releases are immutable`));
      }
      const release: Release = {
        version: input.version,
        status: 'draft',
        publishedAt: null,
        changelog: input.changelog,
        manifestDigest: fakeDigest(JSON.stringify(input.manifest)),
      };
      state.releases[p.id] = [release, ...existing].sort((a, b) => compare(b.version, a.version));
      return delay(release);
    },

    publishRelease(publisher, slug, version) {
      const p = find(publisher, slug);
      const r = findRelease(p, version);
      if (r.status !== 'draft') return Promise.reject(new ApiError(409, 'release_immutable', `Release ${version} is already ${r.status}`));
      r.status = 'published';
      r.publishedAt = new Date().toISOString();
      if (compare(version, p.latestVersion) > 0) p.latestVersion = version;
      p.updatedAt = r.publishedAt;
      return delay(r);
    },

    yankRelease(publisher, slug, version) {
      const p = find(publisher, slug);
      const r = findRelease(p, version);
      if (r.status !== 'published') return Promise.reject(new ApiError(409, 'not_published', `Only published releases can be yanked`));
      r.status = 'yanked';
      if (p.latestVersion === version) {
        const next = (state.releases[p.id] ?? []).filter((x) => x.status === 'published').map((x) => x.version).sort(compare).pop();
        p.latestVersion = next ?? '0.0.0';
      }
      return delay(r);
    },

    getLibrary() {
      return delay(state.library);
    },

    addToLibrary(packageId, pinnedVersion = null) {
      const p = state.packages.find((x) => x.id === packageId);
      if (!p) return Promise.reject(new ApiError(404, 'package_not_found', packageId));
      if (!state.library.items.some((i) => i.packageId === packageId)) {
        state.library.items.push({
          packageId,
          pinnedVersion,
          installedVersion: pinnedVersion ?? p.latestVersion,
          addedAt: new Date().toISOString(),
        });
      }
      return delay(state.library);
    },

    removeFromLibrary(packageId) {
      state.library.items = state.library.items.filter((i) => i.packageId !== packageId);
      return delay(state.library);
    },

    pinVersion(packageId, version) {
      const item = state.library.items.find((i) => i.packageId === packageId);
      if (item) {
        item.pinnedVersion = version;
        if (version) item.installedVersion = version;
      }
      return delay(state.library);
    },

    updateLibraryItem(packageId) {
      const item = state.library.items.find((i) => i.packageId === packageId);
      const p = state.packages.find((x) => x.id === packageId);
      if (item && p && !item.pinnedVersion) item.installedVersion = p.latestVersion;
      return delay(state.library);
    },

    resolveLibrary() {
      return delay(resolve({ library: state.library, packages: state.packages, releases: state.releases }));
    },

    importLockfile(lockfile) {
      for (const entry of Object.values(lockfile.packages)) {
        const known = state.packages.some((p) => p.id === entry.id);
        if (!known) continue;
        const existing = state.library.items.find((i) => i.packageId === entry.id);
        if (existing) {
          existing.pinnedVersion = entry.version;
          existing.installedVersion = entry.version;
        } else {
          state.library.items.push({ packageId: entry.id, pinnedVersion: entry.version, installedVersion: entry.version, addedAt: new Date().toISOString() });
        }
      }
      return delay(state.library);
    },
  };
}
