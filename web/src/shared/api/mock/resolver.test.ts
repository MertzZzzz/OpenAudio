import type { Library } from '@/shared/types/domain';
import { initialLibrary, packages, releases } from './data';
import { resolve } from './resolver';

const lib = (items: Library['items']): Library => ({ id: 'l', name: 'Test', items });
const item = (packageId: string, installedVersion: string, pinnedVersion: string | null = null) => ({ packageId, installedVersion, pinnedVersion, addedAt: '2026-01-01T00:00:00Z' });

describe('resolve', () => {
  it('includes transitive dependencies with exact versions and digests', () => {
    const { lockfile, conflicts } = resolve({ library: lib([item('org.tapeworks.lofi-beat-template', '1.0.0')]), packages, releases });
    expect(conflicts).toEqual([]);
    expect(Object.keys(lockfile.packages)).toEqual(['org.nullsignal.tape-saturator', 'org.tapeworks.lofi-beat-template', 'org.tapeworks.warm-tape-drums']);
    const drums = lockfile.packages['org.tapeworks.warm-tape-drums']!;
    expect(drums.version).toBe('2.0.1');
    expect(drums.manifestDigest).toMatch(/^sha256:[0-9a-f]{64}$/);
  });

  it('reports a conflict when a pin violates a dependency range', () => {
    const { conflicts } = resolve({ library: initialLibrary, packages, releases });
    expect(conflicts.map((c) => c.packageId)).toEqual(['org.tapeworks.warm-tape-drums']);
    expect(conflicts[0]!.requestedBy).toEqual(expect.arrayContaining([{ packageId: 'org.tapeworks.lofi-beat-template', range: '^2.0.0' }]));
  });

  it('keeps the installed version of unpinned items when still valid', () => {
    const { lockfile } = resolve({ library: lib([item('org.acme.deep-house-toolkit', '1.1.0')]), packages, releases });
    expect(lockfile.packages['org.acme.deep-house-toolkit']!.version).toBe('1.1.0');
  });

  it('never resolves to a yanked release', () => {
    const { lockfile } = resolve({ library: lib([item('org.tapeworks.warm-tape-drums', '0.0.0')]), packages, releases });
    const chosen = lockfile.packages['org.tapeworks.warm-tape-drums']!.version;
    expect(releases['org.tapeworks.warm-tape-drums']!.find((r) => r.version === chosen)!.status).toBe('published');
  });
});
