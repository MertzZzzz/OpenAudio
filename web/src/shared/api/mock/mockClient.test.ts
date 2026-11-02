import { createMockClient } from './mockClient';

const client = () => createMockClient({ latencyMs: 0 });

describe('mock API client', () => {
  it('expands genre filters to child genres', async () => {
    const res = await client().searchPackages({ genre: ['house'] });
    expect(res.items.map((p) => p.slug)).toEqual(expect.arrayContaining(['deep-house-toolkit', 'vocal-chops-soul']));
    expect(res.items.every((p) => p.kind === 'sample-pack')).toBe(true);
  });

  it('resolves taxonomy aliases in free text (dnb → drum-and-bass)', async () => {
    const res = await client().searchPackages({ q: 'dnb' });
    expect(res.items[0]?.slug).toBe('liquid-dnb-essentials');
  });

  it('filters by BPM range and key', async () => {
    const res = await client().searchPackages({ bpmMin: 120, bpmMax: 130, key: ['F-minor'] });
    expect(res.items.map((p) => p.slug)).toEqual(['deep-house-toolkit']);
  });

  it('computes facets excluding the facet’s own selection', async () => {
    const facets = await client().searchFacets({ kind: ['effect'] });
    // Other kinds remain selectable while "effect" is chosen…
    expect(facets.kind.map((b) => b.value)).toContain('sample-pack');
    // …but other facets are narrowed to effects only.
    expect(facets.moduleClass).toEqual([{ value: 'audio-effect', label: 'Audio Effect', count: 3 }]);
  });

  it('paginates', async () => {
    const res = await client().searchPackages({ limit: 5, page: 2 });
    expect(res.items).toHaveLength(5);
    expect(res.page).toBe(2);
    expect(res.total).toBeGreaterThan(10);
  });

  it('enforces immutable releases: drafts publish once, versions cannot be reused', async () => {
    const c = client();
    await c.createDraftRelease({ publisher: 'acme', slug: 'convolver-spaces', version: '1.1.0', changelog: 'x', manifest: {}, files: [] });
    await expect(c.createDraftRelease({ publisher: 'acme', slug: 'convolver-spaces', version: '1.1.0', changelog: 'y', manifest: {}, files: [] })).rejects.toMatchObject({ code: 'version_exists' });
    const published = await c.publishRelease('acme', 'convolver-spaces', '1.1.0');
    expect(published.status).toBe('published');
    await expect(c.publishRelease('acme', 'convolver-spaces', '1.1.0')).rejects.toMatchObject({ code: 'release_immutable' });
    expect((await c.getPackage('acme', 'convolver-spaces')).latestVersion).toBe('1.1.0');
  });

  it('yanking the latest release moves latestVersion back', async () => {
    const c = client();
    await c.yankRelease('lattice', 'plate-verb', '2.3.0');
    expect((await c.getPackage('lattice', 'plate-verb')).latestVersion).toBe('2.2.0');
  });

  it('isolates state between client instances', async () => {
    const a = client();
    await a.removeFromLibrary('org.lattice.plate-verb');
    expect((await client().getLibrary()).items.some((i) => i.packageId === 'org.lattice.plate-verb')).toBe(true);
  });
});

describe('lockfiles (mock mirrors the API rules)', () => {
  it('exports, lists and re-imports with integrity checks', async () => {
    const c = createMockClient({ latencyMs: 0 });
    await c.pinVersion('org.tapeworks.warm-tape-drums', null); // clear the seeded conflict
    await c.updateLibraryItem('org.tapeworks.warm-tape-drums');
    const { saved, lockfile } = await c.saveLockfile();
    expect((await c.listLockfiles()).map((l) => l.id)).toEqual([saved.id]);
    expect(JSON.parse(await c.lockfileContent(saved.id))).toEqual(lockfile);
    expect(Object.values(lockfile.packages).some((e) => e.direct === false)).toBe(true);

    const res = await c.importLockfile(lockfile);
    expect(res.library.items.length).toBeGreaterThan(0);

    const tampered = structuredClone(lockfile);
    const first = Object.keys(tampered.packages)[0]!;
    tampered.packages[first]!.manifestDigest = 'sha256:' + '0'.repeat(64);
    await expect(c.importLockfile(tampered)).rejects.toMatchObject({ code: 'lockfile_invalid' });

    await c.deleteLockfile(saved.id);
    expect(await c.listLockfiles()).toEqual([]);
  });

  it('refuses to export while the library has conflicts', async () => {
    await expect(createMockClient({ latencyMs: 0 }).saveLockfile()).rejects.toMatchObject({ code: 'resolve_conflicts' });
  });
});
