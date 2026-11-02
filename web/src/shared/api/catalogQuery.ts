import type { FacetBucket, KeyMode, KeyTonic, Package, Page, SearchFacets, SearchParams } from '@/shared/types/domain';
import { formatKey, humanize, keyToSlug } from '@/shared/lib/format';
import { expandWithChildren, resolveAlias, termLabel } from '@/shared/lib/taxonomy';

/**
 * Client-side search over a list of packages: text scoring, taxonomy-aware filters,
 * facets (each facet ignores its own selection), sorting and pagination.
 * Used by the mock API and — until server-side search lands in Phase 5 — by the HTTP client.
 */

type FacetKey = Exclude<keyof SearchFacets, 'bpm'>;

export function matchesText(p: Package, q: string): number {
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
  genre: (p) => [p.taxonomy.genres.primary, ...p.taxonomy.genres.secondary].filter(Boolean),
  instrument: (p) => p.taxonomy.instruments,
  mood: (p) => p.taxonomy.moods,
  license: (p) => (p.license.content ? [p.license.content] : []),
  moduleClass: (p) => (p.module ? [p.module.moduleClass] : []),
  key: (p) => (p.musical?.key ? [keyToSlug(p.musical.key)] : []),
};

export function passes(p: Package, params: SearchParams, skip?: FacetKey | 'bpm'): boolean {
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
  if (skip === 'bpm') return params.q ? matchesText(p, params.q) > 0 : true;
  if (params.bpmMin !== undefined && (bpm === undefined || bpm < params.bpmMin)) return false;
  if (params.bpmMax !== undefined && (bpm === undefined || bpm > params.bpmMax)) return false;
  if (params.q && matchesText(p, params.q) === 0) return false;
  return true;
}

function facetLabel(k: FacetKey, v: string): string {
  switch (k) {
    case 'genre':
      return termLabel('genre', v);
    case 'instrument':
      return termLabel('instrument', v);
    case 'mood':
      return termLabel('mood', v);
    case 'key': {
      const [tonic, mode] = v.split('-') as [KeyTonic, KeyMode];
      return formatKey({ tonic, mode });
    }
    default:
      return humanize(v);
  }
}

export function buildFacets(packages: Package[], params: SearchParams): SearchFacets {
  const out = {} as SearchFacets;
  for (const k of Object.keys(facetValues) as FacetKey[]) {
    const counts = new Map<string, number>();
    for (const p of packages) {
      if (!passes(p, params, k)) continue;
      for (const v of new Set(facetValues[k](p))) counts.set(v, (counts.get(v) ?? 0) + 1);
    }
    out[k] = [...counts.entries()]
      .map<FacetBucket>(([value, count]) => ({ value, count, label: facetLabel(k, value) }))
      .sort((a, b) => b.count - a.count || a.label.localeCompare(b.label));
  }
  const buckets = new Map<string, number>();
  for (const p of packages) {
    const bpm = p.musical?.bpm?.value;
    if (bpm === undefined || !passes(p, params, 'bpm')) continue;
    const b = BPM_BUCKETS.find(([lo, hi]) => bpm >= lo && bpm <= hi)!;
    const key = `${b[0]}-${b[1]}`;
    buckets.set(key, (buckets.get(key) ?? 0) + 1);
  }
  out.bpm = [...buckets.entries()]
    .map(([value, count]) => ({ value, count, label: bpmLabel(value) }))
    .sort((a, b) => b.count - a.count || a.value.localeCompare(b.value));
  return out;
}

/** Same ranges as the server (api/internal/adapter/postgres/search.go). */
export const BPM_BUCKETS: [number, number][] = [
  [0, 79], [80, 99], [100, 119], [120, 129], [130, 139], [140, 159], [160, 179], [180, 400],
];

export function bpmLabel(value: string): string {
  if (value === '0-79') return '< 80';
  if (value === '180-400') return '180+';
  return value.replace('-', '–');
}

export function searchPage(packages: Package[], params: SearchParams): Page<Package> {
  const page = params.page ?? 1;
  const limit = params.limit ?? 12;
  const hits = packages.filter((p) => passes(p, params));
  const sort = params.sort ?? (params.q ? 'relevance' : 'downloads');
  hits.sort((a, b) => {
    switch (sort) {
      case 'newest':
        return b.updatedAt.localeCompare(a.updatedAt);
      case 'rating':
        return b.rating - a.rating;
      case 'downloads':
        return b.downloads - a.downloads || b.updatedAt.localeCompare(a.updatedAt);
      default:
        return matchesText(b, params.q ?? '') - matchesText(a, params.q ?? '') || b.downloads - a.downloads;
    }
  });
  return { items: hits.slice((page - 1) * limit, page * limit), total: hits.length, page, limit };
}
