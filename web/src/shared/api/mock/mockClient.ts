import type { ApiClient } from '@/shared/api/client';
import { ApiError } from '@/shared/api/client';
import type { Library, Package, Release, SavedLockfile, ValidationIssue } from '@/shared/types/domain';
import { HttpApiError } from '@/shared/api/http';
import { buildFacets, searchPage } from '@/shared/api/catalogQuery';
import { compare } from '@/shared/lib/semver';
import { keyToSlug } from '@/shared/lib/format';
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
import { expandWithChildren, taxonomyTerms } from './taxonomy';
import { resolve } from './resolver';

const clone = <T>(v: T): T => structuredClone(v);

interface MockOptions {
  latencyMs?: number;
}

export function createMockClient({ latencyMs = 250 }: MockOptions = {}): ApiClient {
  // In-memory, per-client state so tests are isolated.
  const state = {
    packages: clone(seedPackages),
    releases: clone(seedReleases),
    library: clone(initialLibrary) as Library,
    members: clone(seedMembers),
    lockfiles: [] as (SavedLockfile & { content: string })[],
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
      return delay(searchPage(state.packages, params));
    },

    searchFacets(params) {
      return delay(buildFacets(state.packages, params));
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

    myPublishers() {
      return delay([{ slug: 'acme', name: 'Acme Audio', role: 'owner' as const }]);
    },

    createPublisher(slug, name) {
      if (publishers.some((p) => p.slug === slug)) {
        return Promise.reject(new ApiError(409, 'publisher_slug_taken', 'a publisher with this name already exists'));
      }
      publishers.push({ id: slug, slug, name, verified: false });
      return delay({ slug, name, role: 'owner' as const });
    },

    uploadBlob(file, { onProgress }) {
      // Simulated transfer: a few progress ticks, then "verifying" and done.
      return new Promise<void>((resolve) => {
        const steps = latencyMs === 0 ? 1 : 5;
        let i = 0;
        const tick = () => {
          i += 1;
          onProgress?.({ fraction: Math.min(1, i / steps), phase: i < steps ? 'uploading' : 'verifying' });
          if (i < steps) setTimeout(tick, Math.max(1, latencyMs / 2));
          else setTimeout(() => {
            onProgress?.({ fraction: 1, phase: 'done' });
            resolve();
          }, latencyMs / 2);
        };
        void file;
        tick();
      });
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
      // Same rules as the API: every entry must exist with the same manifest digest, else nothing is applied.
      const issues: ValidationIssue[] = [];
      const warnings: ValidationIssue[] = [];
      const pins: { id: string; version: string }[] = [];
      for (const [key, entry] of Object.entries(lockfile.packages)) {
        const id = entry.id || key;
        const rel = state.releases[id]?.find((r) => r.version === entry.version && r.status !== 'draft');
        if (!rel) {
          issues.push({ severity: 'error', path: `lockfile.packages.${key}`, message: `${id}@${entry.version} does not exist` });
          continue;
        }
        if (rel.manifestDigest !== entry.manifestDigest) {
          issues.push({ severity: 'error', path: `lockfile.packages.${key}.manifestDigest`, message: `integrity check failed for ${id}@${entry.version}` });
          continue;
        }
        if (rel.status === 'yanked') warnings.push({ severity: 'warning', path: `lockfile.packages.${key}`, message: `${id}@${entry.version} is yanked` });
        if (entry.direct !== false) pins.push({ id, version: entry.version });
      }
      if (issues.length) {
        return Promise.reject(new HttpApiError(422, 'lockfile_invalid', 'the lockfile does not match the registry', issues));
      }
      for (const { id, version } of pins) {
        const existing = state.library.items.find((i) => i.packageId === id);
        if (existing) {
          existing.pinnedVersion = version;
          existing.installedVersion = version;
        } else {
          state.library.items.push({ packageId: id, pinnedVersion: version, installedVersion: version, addedAt: new Date().toISOString() });
        }
      }
      return delay({ library: state.library, warnings });
    },

    saveLockfile() {
      const { lockfile, conflicts } = resolve({ library: state.library, packages: state.packages, releases: state.releases });
      if (conflicts.length) {
        return Promise.reject(new HttpApiError(409, 'resolve_conflicts', 'resolve the version conflicts before exporting a lockfile'));
      }
      const content = JSON.stringify(lockfile);
      const saved = { id: crypto.randomUUID(), name: lockfile.name, contentDigest: fakeDigest(content), createdAt: new Date().toISOString() };
      state.lockfiles.unshift({ ...saved, content });
      return delay({ saved, lockfile });
    },

    listLockfiles() {
      return delay(state.lockfiles.map(({ content: _content, ...rest }) => rest));
    },

    lockfileContent(id) {
      const l = state.lockfiles.find((x) => x.id === id);
      return l ? delay(l.content) : Promise.reject(new ApiError(404, 'lockfile_not_found', 'lockfile not found'));
    },

    deleteLockfile(id) {
      state.lockfiles = state.lockfiles.filter((x) => x.id !== id);
      return delay(undefined);
    },
  };
}
