import { useEffect, useState } from 'react';
import { ChevronDown } from 'lucide-react';
import type { FacetBucket, SearchFacets, SearchParams } from '@/shared/types/domain';
import { Checkbox } from '@/shared/ui/Checkbox';
import { Input } from '@/shared/ui/Input';
import { Button } from '@/shared/ui/Button';
import { Skeleton } from '@/shared/ui/Skeleton';
import { cn } from '@/shared/lib/cn';
import { useDebounce } from '@/shared/hooks/useDebounce';
import type { ListKey } from './params';

const FACET_TITLES: Record<ListKey, string> = {
  kind: 'Type',
  moduleClass: 'Module class',
  genre: 'Genre',
  instrument: 'Instrument',
  mood: 'Mood',
  key: 'Key',
  license: 'License',
};
const BPM_PRESETS: FacetBucket[] = [
  { value: '80-99', label: '80–99', count: 0 },
  { value: '120-129', label: '120–129', count: 0 },
  { value: '130-139', label: '130–139', count: 0 },
  { value: '160-179', label: '160–179', count: 0 },
];

const ORDER: ListKey[] = ['kind', 'moduleClass', 'genre', 'instrument', 'mood', 'key', 'license'];

function FacetGroup({ facet, buckets, selected, onToggle }: { facet: ListKey; buckets: FacetBucket[]; selected: string[]; onToggle: (v: string) => void }) {
  const [open, setOpen] = useState(true);
  const [expanded, setExpanded] = useState(false);
  // Selected values float to the top so they never hide behind "Show all".
  const ordered = [...buckets.filter((b) => selected.includes(b.value)), ...buckets.filter((b) => !selected.includes(b.value))];
  const visible = expanded ? ordered : ordered.slice(0, Math.max(6, selected.length));
  // Keep selected values visible even when they have zero hits under the other filters.
  const missing = selected.filter((s) => !buckets.some((b) => b.value === s));
  const panelId = `facet-${facet}`;
  return (
    <div className="border-b border-border py-3">
      <button
        type="button"
        className="flex w-full items-center justify-between text-body font-semibold"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={() => setOpen((o) => !o)}
      >
        {FACET_TITLES[facet]}
        <ChevronDown className={cn('h-4 w-4 text-fg-subtle transition-transform', !open && '-rotate-90')} aria-hidden />
      </button>
      {open && (
        <fieldset id={panelId} className="mt-2">
          <legend className="sr-only">{FACET_TITLES[facet]}</legend>
          {visible.map((b) => (
            <Checkbox key={b.value} label={b.label} trailing={b.count} checked={selected.includes(b.value)} onChange={() => onToggle(b.value)} />
          ))}
          {missing.map((v) => (
            <Checkbox key={v} label={v} trailing={0} checked onChange={() => onToggle(v)} />
          ))}
          {buckets.length === 0 && missing.length === 0 && <p className="px-1 text-caption text-fg-subtle">No values</p>}
          {buckets.length > 6 && (
            <button type="button" className="mt-1 px-1 text-caption text-primary hover:underline" onClick={() => setExpanded((e) => !e)}>
              {expanded ? 'Show less' : `Show all ${buckets.length}`}
            </button>
          )}
        </fieldset>
      )}
    </div>
  );
}

function BpmRange({ min, max, onChange, buckets }: { min?: number; max?: number; onChange: (min?: number, max?: number) => void; buckets?: FacetBucket[] }) {
  const [lo, setLo] = useState(min?.toString() ?? '');
  const [hi, setHi] = useState(max?.toString() ?? '');
  const dLo = useDebounce(lo, 400);
  const dHi = useDebounce(hi, 400);

  useEffect(() => {
    setLo(min?.toString() ?? '');
    setHi(max?.toString() ?? '');
  }, [min, max]);

  useEffect(() => {
    const nLo = dLo === '' ? undefined : Number(dLo);
    const nHi = dHi === '' ? undefined : Number(dHi);
    if (nLo !== min || nHi !== max) onChange(nLo, nHi);
  }, [dLo, dHi]);

  return (
    <div className="border-b border-border py-3">
      <p className="text-body font-semibold" id="bpm-label">BPM</p>
      <div className="mt-2 flex items-center gap-2" role="group" aria-labelledby="bpm-label">
        <Input type="number" inputMode="numeric" min={20} max={400} placeholder="Min" value={lo} onChange={(e) => setLo(e.target.value)} aria-label="Minimum BPM" />
        <span className="text-fg-subtle">–</span>
        <Input type="number" inputMode="numeric" min={20} max={400} placeholder="Max" value={hi} onChange={(e) => setHi(e.target.value)} aria-label="Maximum BPM" />
      </div>
      <div className="mt-2 flex flex-wrap gap-1" role="group" aria-label="BPM ranges">
        {(buckets?.length ? buckets : BPM_PRESETS).map((b) => {
          const [a, z] = b.value.split('-').map(Number) as [number, number];
          const lo = Math.max(a, 20);
          const active = min === lo && max === z;
          return (
            <button
              key={b.value}
              type="button"
              aria-pressed={active}
              onClick={() => (active ? onChange(undefined, undefined) : onChange(lo, z))}
              className={cn('rounded-full border px-2 py-0.5 text-caption', active ? 'border-primary text-fg' : 'border-border text-fg-muted hover:text-fg')}
            >
              {b.label}
              {b.count > 0 && <span className="ml-1 text-fg-subtle">{b.count}</span>}
            </button>
          );
        })}
      </div>
    </div>
  );
}

export interface FacetPanelProps {
  params: SearchParams;
  facets?: SearchFacets;
  loading: boolean;
  onToggle: (facet: ListKey, value: string) => void;
  onBpm: (min?: number, max?: number) => void;
  onClear: () => void;
  activeCount: number;
}

export function FacetPanel({ params, facets, loading, onToggle, onBpm, onClear, activeCount }: FacetPanelProps) {
  return (
    <div aria-label="Filters" role="region">
      <div className="flex items-center justify-between pb-2">
        <h2 className="text-body font-semibold uppercase tracking-wide text-fg-subtle">Filters</h2>
        {activeCount > 0 && (
          <Button variant="ghost" size="sm" onClick={onClear}>
            Clear ({activeCount})
          </Button>
        )}
      </div>
      {loading && !facets ? (
        <div className="space-y-3">
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton key={i} className="h-24" />
          ))}
        </div>
      ) : (
        facets && (
          <>
            {ORDER.slice(0, 4).map((k) => (
              <FacetGroup key={k} facet={k} buckets={facets[k]} selected={params[k] ?? []} onToggle={(v) => onToggle(k, v)} />
            ))}
            <BpmRange min={params.bpmMin} max={params.bpmMax} onChange={onBpm} buckets={facets.bpm} />
            {ORDER.slice(4).map((k) => (
              <FacetGroup key={k} facet={k} buckets={facets[k]} selected={params[k] ?? []} onToggle={(v) => onToggle(k, v)} />
            ))}
          </>
        )
      )}
    </div>
  );
}
