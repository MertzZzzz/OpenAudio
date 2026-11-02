import type { Library, Lockfile, LockfileEntry, Package, Release, ResolveConflict, ResolveResult } from '@/shared/types/domain';
import { maxSatisfying } from '@/shared/lib/semver';

interface ResolverInput {
  library: Library;
  packages: Package[];
  releases: Record<string, Release[]>;
  now?: Date;
}

/**
 * Resolves library items + transitive dependencies to exact versions.
 * Pinned items are fixed; unpinned items keep their installed version when it
 * still satisfies every range, otherwise the highest satisfying published version.
 */
export function resolve({ library, packages, releases, now = new Date() }: ResolverInput): ResolveResult {
  const byId = new Map(packages.map((p) => [p.id, p]));
  const installed = new Map(library.items.map((i) => [i.packageId, i.installedVersion]));
  const published = (id: string) => (releases[id] ?? []).filter((r) => r.status === 'published').map((r) => r.version);

  const requests = new Map<string, { packageId: string; range: string }[]>();
  const queue: { id: string; range: string; by: string }[] = library.items.map((i) => ({
    id: i.packageId,
    range: i.pinnedVersion ?? '*',
    by: 'library',
  }));
  const seen = new Set<string>();

  while (queue.length) {
    const { id, range, by } = queue.shift()!;
    const list = requests.get(id) ?? [];
    list.push({ packageId: by, range });
    requests.set(id, list);
    if (seen.has(id)) continue;
    seen.add(id);
    for (const dep of byId.get(id)?.dependencies ?? []) queue.push({ id: dep.packageId, range: dep.range, by: id });
  }

  const entries: Record<string, LockfileEntry> = {};
  const conflicts: ResolveConflict[] = [];

  for (const [id, reqs] of requests) {
    const candidates = published(id);
    let pool = candidates;
    for (const r of reqs) pool = pool.filter((v) => maxSatisfying([v], r.range) !== null);
    const preferred = installed.get(id);
    const chosen = preferred && pool.includes(preferred) ? preferred : maxSatisfying(pool, '*');
    if (!chosen) {
      conflicts.push({ packageId: id, requestedBy: reqs, candidates });
      continue;
    }
    const rel = releases[id]!.find((r) => r.version === chosen)!;
    const pkg = byId.get(id);
    entries[id] = {
      id,
      direct: library.items.some((i) => i.packageId === id),
      version: chosen,
      manifestDigest: rel.manifestDigest,
      resolved: pkg ? `openaudio://${pkg.publisher.slug}/${pkg.slug}@${chosen}` : `openaudio://${id}@${chosen}`,
    };
  }

  const sorted = Object.fromEntries(Object.entries(entries).sort(([a], [b]) => a.localeCompare(b)));
  const lockfile: Lockfile = { lockfileVersion: 1, name: library.name, generatedAt: now.toISOString(), packages: sorted };
  return { lockfile, conflicts };
}
