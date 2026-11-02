import { useMemo, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { LayoutGrid, List, SearchX, SlidersHorizontal, X } from 'lucide-react';
import type { SearchParams, SortOrder } from '@/shared/types/domain';
import { Button } from '@/shared/ui/Button';
import { Select } from '@/shared/ui/Select';
import { Drawer } from '@/shared/ui/Overlay';
import { EmptyState } from '@/shared/ui/EmptyState';
import { Skeleton } from '@/shared/ui/Skeleton';
import { cn } from '@/shared/lib/cn';
import { PackageCard, PackageRow } from '@/features/catalog/PackageCard';
import { useFacets, useSearch } from './api';
import { FacetPanel } from './FacetPanel';
import { activeFilterCount, LIST_KEYS, parseSearchParams, toSearchString, type ListKey } from './params';

const sortOptions: { value: SortOrder; label: string }[] = [
  { value: 'relevance', label: 'Relevance' },
  { value: 'newest', label: 'Newest' },
  { value: 'downloads', label: 'Most downloaded' },
  { value: 'rating', label: 'Top rated' },
];

const LIMIT = 12;

export function SearchView() {
  const [sp, setSp] = useSearchParams();
  const params = useMemo(() => ({ ...parseSearchParams(sp), limit: LIMIT }), [sp]);
  const facetParams = useMemo(() => ({ ...params, page: undefined, sort: undefined, limit: undefined }), [params]);
  const search = useSearch(params);
  const facets = useFacets(facetParams);
  const [view, setView] = useState<'grid' | 'list'>('grid');
  const [filtersOpen, setFiltersOpen] = useState(false);

  const update = (patch: Partial<SearchParams>) => {
    const next = { ...params, ...patch };
    if (!('page' in patch)) next.page = 1;
    setSp(toSearchString(next), { replace: false });
  };

  const toggle = (k: ListKey, v: string) => {
    const cur = (params[k] ?? []) as string[];
    update({ [k]: cur.includes(v) ? cur.filter((x) => x !== v) : [...cur, v] } as Partial<SearchParams>);
  };

  const clear = () => setSp(toSearchString({ q: params.q, sort: params.sort }));
  const activeCount = activeFilterCount(params);
  const total = search.data?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / LIMIT));

  const labelFor = (k: ListKey, v: string) => facets.data?.[k].find((b) => b.value === v)?.label ?? v;

  const panel = (
    <FacetPanel
      params={params}
      facets={facets.data}
      loading={facets.isLoading}
      onToggle={toggle}
      onBpm={(bpmMin, bpmMax) => update({ bpmMin, bpmMax })}
      onClear={clear}
      activeCount={activeCount}
    />
  );

  return (
    <div className="flex gap-8">
      <aside className="hidden w-60 shrink-0 lg:block">{panel}</aside>
      <Drawer open={filtersOpen} onClose={() => setFiltersOpen(false)} title="Filters">
        {panel}
      </Drawer>

      <section className="min-w-0 flex-1 space-y-4" aria-labelledby="search-heading">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <h1 id="search-heading" className="text-heading">
              {params.q ? <>Results for “{params.q}”</> : 'All packages'}
            </h1>
            <p className="text-caption text-fg-muted" aria-live="polite">
              {search.isLoading ? 'Searching…' : `${total} package${total === 1 ? '' : 's'}`}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div role="radiogroup" aria-label="Result type" className="flex rounded border border-border p-0.5">
              <span role="radio" aria-checked="true" className="rounded bg-surface-raised px-3 py-1 text-caption font-medium">Packages</span>
              <Link role="radio" aria-checked="false" to={`/samples${params.q ? `?q=${encodeURIComponent(params.q)}` : ''}`} className="rounded px-3 py-1 text-caption text-fg-muted hover:text-fg">
                Samples
              </Link>
            </div>
            <Button variant="secondary" size="sm" className="lg:hidden" leftIcon={<SlidersHorizontal className="h-4 w-4" />} onClick={() => setFiltersOpen(true)}>
              Filters{activeCount ? ` (${activeCount})` : ''}
            </Button>
            <div className="w-44">
              <Select
                aria-label="Sort by"
                options={sortOptions}
                value={params.sort ?? (params.q ? 'relevance' : 'downloads')}
                onChange={(e) => update({ sort: e.target.value as SortOrder })}
              />
            </div>
            <div className="flex rounded border border-border p-0.5" role="group" aria-label="Layout">
              <button type="button" aria-label="Grid view" aria-pressed={view === 'grid'} onClick={() => setView('grid')} className={cn('rounded p-1.5', view === 'grid' ? 'bg-surface-raised text-fg' : 'text-fg-subtle')}>
                <LayoutGrid className="h-4 w-4" />
              </button>
              <button type="button" aria-label="List view" aria-pressed={view === 'list'} onClick={() => setView('list')} className={cn('rounded p-1.5', view === 'list' ? 'bg-surface-raised text-fg' : 'text-fg-subtle')}>
                <List className="h-4 w-4" />
              </button>
            </div>
          </div>
        </div>

        {activeCount > 0 && (
          <ul className="flex flex-wrap gap-2" aria-label="Active filters">
            {LIST_KEYS.flatMap((k) =>
              (params[k] ?? []).map((v) => (
                <li key={`${k}:${v}`}>
                  <button type="button" onClick={() => toggle(k, v)} className="inline-flex items-center gap-1 rounded-full border border-primary/40 bg-primary/10 px-2.5 py-1 text-caption text-fg hover:bg-primary/20" aria-label={`Remove filter ${labelFor(k, v)}`}>
                    {labelFor(k, v)}
                    <X className="h-3 w-3" aria-hidden />
                  </button>
                </li>
              )),
            )}
            {(params.bpmMin !== undefined || params.bpmMax !== undefined) && (
              <li>
                <button type="button" onClick={() => update({ bpmMin: undefined, bpmMax: undefined })} className="inline-flex items-center gap-1 rounded-full border border-primary/40 bg-primary/10 px-2.5 py-1 text-caption hover:bg-primary/20" aria-label="Remove BPM filter">
                  {params.bpmMin ?? '…'}–{params.bpmMax ?? '…'} BPM
                  <X className="h-3 w-3" aria-hidden />
                </button>
              </li>
            )}
          </ul>
        )}

        {search.isLoading ? (
          <div className="grid grid-cols-2 gap-4 md:grid-cols-3 xl:grid-cols-4">
            {Array.from({ length: 8 }, (_, i) => (
              <Skeleton key={i} className="h-72" />
            ))}
          </div>
        ) : search.isError ? (
          <EmptyState title="Search failed" description="Something went wrong talking to the API." action={<Button onClick={() => search.refetch()}>Retry</Button>} />
        ) : total === 0 ? (
          <EmptyState icon={<SearchX className="h-6 w-6" />} title="No packages match" description="Try removing a filter or searching for a broader term." action={activeCount > 0 ? <Button variant="secondary" onClick={clear}>Clear filters</Button> : undefined} />
        ) : (
          <div className={cn(search.isFetching && 'opacity-70 transition-opacity')}>
            {view === 'grid' ? (
              <div className="grid grid-cols-2 gap-4 md:grid-cols-3 xl:grid-cols-4">
                {search.data?.items.map((p) => <PackageCard key={p.id} pkg={p} />)}
              </div>
            ) : (
              <div className="space-y-2">{search.data?.items.map((p) => <PackageRow key={p.id} pkg={p} />)}</div>
            )}
          </div>
        )}

        {pages > 1 && (
          <nav aria-label="Pagination" className="flex items-center justify-center gap-2 pt-4">
            <Button variant="secondary" size="sm" disabled={(params.page ?? 1) <= 1} onClick={() => update({ page: (params.page ?? 1) - 1 })}>
              Previous
            </Button>
            {Array.from({ length: pages }, (_, i) => i + 1).map((n) => (
              <Button key={n} size="sm" variant={n === params.page ? 'primary' : 'ghost'} aria-current={n === params.page ? 'page' : undefined} onClick={() => update({ page: n })}>
                {n}
              </Button>
            ))}
            <Button variant="secondary" size="sm" disabled={(params.page ?? 1) >= pages} onClick={() => update({ page: (params.page ?? 1) + 1 })}>
              Next
            </Button>
          </nav>
        )}
      </section>
    </div>
  );
}
