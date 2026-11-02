import { MODULE_CLASSES, PACKAGE_KINDS, type ModuleClass, type PackageKind, type SearchParams, type SortOrder } from '@/shared/types/domain';

const SORTS: SortOrder[] = ['relevance', 'newest', 'downloads', 'rating'];
export const LIST_KEYS = ['kind', 'genre', 'instrument', 'mood', 'license', 'moduleClass', 'key'] as const;
export type ListKey = (typeof LIST_KEYS)[number];

const num = (v: string | null): number | undefined => {
  if (v === null || v === '') return undefined;
  const n = Number(v);
  return Number.isFinite(n) ? n : undefined;
};

/** URL is the source of truth for search state, so results are shareable. */
export function parseSearchParams(sp: URLSearchParams): SearchParams {
  const list = (k: string) => {
    const all = sp.getAll(k).flatMap((v) => v.split(',')).filter(Boolean);
    return all.length ? all : undefined;
  };
  const sort = sp.get('sort') as SortOrder | null;
  return {
    q: sp.get('q') ?? undefined,
    kind: list('kind')?.filter((k): k is PackageKind => (PACKAGE_KINDS as readonly string[]).includes(k)),
    genre: list('genre'),
    instrument: list('instrument'),
    mood: list('mood'),
    license: list('license'),
    moduleClass: list('moduleClass')?.filter((k): k is ModuleClass => (MODULE_CLASSES as readonly string[]).includes(k)),
    key: list('key'),
    bpmMin: num(sp.get('bpmMin')),
    bpmMax: num(sp.get('bpmMax')),
    sort: sort && SORTS.includes(sort) ? sort : undefined,
    page: num(sp.get('page')) ?? 1,
  };
}

export function toSearchString(p: SearchParams): string {
  const sp = new URLSearchParams();
  if (p.q) sp.set('q', p.q);
  for (const k of LIST_KEYS) {
    const v = p[k];
    if (v?.length) sp.set(k, v.join(','));
  }
  if (p.bpmMin !== undefined) sp.set('bpmMin', String(p.bpmMin));
  if (p.bpmMax !== undefined) sp.set('bpmMax', String(p.bpmMax));
  if (p.sort) sp.set('sort', p.sort);
  if (p.page && p.page > 1) sp.set('page', String(p.page));
  return sp.toString();
}

export function activeFilterCount(p: SearchParams): number {
  return LIST_KEYS.reduce((n, k) => n + (p[k]?.length ?? 0), 0) + (p.bpmMin !== undefined || p.bpmMax !== undefined ? 1 : 0);
}
