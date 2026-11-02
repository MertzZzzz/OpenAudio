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
