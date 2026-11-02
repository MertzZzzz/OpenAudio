import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Download, FileAudio, Search, X } from 'lucide-react';
import { KEY_TONICS, type SampleContentType, type SampleQuery } from '@/shared/types/domain';
import { Input } from '@/shared/ui/Input';
import { Select } from '@/shared/ui/Select';
import { Button } from '@/shared/ui/Button';
import { Badge } from '@/shared/ui/Badge';
import { Skeleton } from '@/shared/ui/Skeleton';
import { EmptyState } from '@/shared/ui/EmptyState';
import { useToast } from '@/shared/ui/Toast';
import { useDebounce } from '@/shared/hooks/useDebounce';
import { cn } from '@/shared/lib/cn';
import { formatBytes } from '@/shared/lib/format';
import { downloadJson } from '@/shared/lib/download';
import { useSearch } from '@/features/search/api';
import { useSamples } from './api';
import { SampleTable, downloadSample } from './SampleTable';

const ALL_PACKAGES = { limit: 200 };
const INSTRUMENTS = ['drums', 'kick', 'snare', 'hi-hat', 'clap', 'percussion', 'bass', 'synth', 'pad', 'keys', 'vocals', 'fx'];
const KEY_OPTIONS = KEY_TONICS.flatMap((t) => [
  { value: `${t}-minor`, label: `${t} minor` },
  { value: `${t}-major`, label: `${t} major` },
]);
const DURATIONS = [
  { value: '1', label: '≤ 1 s' },
  { value: '4', label: '≤ 4 s' },
  { value: '10', label: '≤ 10 s' },
  { value: '30', label: '≤ 30 s' },
];

export function SampleBrowserView() {
  const [sp, setSp] = useSearchParams();
  const [text, setText] = useState(sp.get('q') ?? '');
  const q = useDebounce(text, 250);
  const [instrument, setInstrument] = useState<string[]>([]);
  const [contentType, setContentType] = useState<SampleContentType | ''>('');
  const [key, setKey] = useState('');
  const [bpmMin, setBpmMin] = useState('');
  const [bpmMax, setBpmMax] = useState('');
  const [duration, setDuration] = useState('');
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const { toast } = useToast();
  const packageId = sp.get('package') ?? undefined;

  const query = useMemo<SampleQuery>(
    () => ({
      q: q || undefined,
      packageId,
      instrument: instrument.length ? instrument : undefined,
      contentType: contentType ? [contentType] : undefined,
      key: key ? [key] : undefined,
      bpmMin: bpmMin ? Number(bpmMin) : undefined,
      bpmMax: bpmMax ? Number(bpmMax) : undefined,
      durationMax: duration ? Number(duration) : undefined,
    }),
    [q, packageId, instrument, contentType, key, bpmMin, bpmMax, duration],
  );

  const samplesQ = useSamples(query);
  // Package names for display.
  const allPackages = useSearch(ALL_PACKAGES);
  const packageNames = useMemo(() => Object.fromEntries((allPackages.data?.items ?? []).map((p) => [p.id, p.name])), [allPackages.data]);

  const samples = samplesQ.data ?? [];
  const toggleSelect = (id: string) =>
    setSelected((s) => {
      const n = new Set(s);
      if (n.has(id)) n.delete(id);
      else n.add(id);
      return n;
    });
  const toggleAll = () => setSelected((s) => (samples.every((x) => s.has(x.id)) ? new Set() : new Set(samples.map((x) => x.id))));

  const selectedSamples = samples.filter((s) => selected.has(s.id));
  const estSize = selectedSamples.reduce((n, s) => n + s.durationSec * 48000 * 6, 0);

  const batchDownload = () => {
    downloadJson('openaudio-download-plan.json', {
      note: 'Each URL counts a download and redirects to a short-lived presigned link.',
      items: selectedSamples.map((s) => ({
        id: s.id,
        file: s.file,
        digest: s.digest,
        url: s.downloadUrl ? new URL(s.downloadUrl, window.location.origin).toString() : `mock://blobs/${s.digest.slice(7)}`,
      })),
    });
    toast({ tone: 'success', title: `Prepared ${selectedSamples.length} samples`, description: 'Download plan saved (mock).' });
  };

  const reset = () => {
    setText('');
    setInstrument([]);
    setContentType('');
    setKey('');
    setBpmMin('');
    setBpmMax('');
    setDuration('');
  };

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-heading">Sample browser</h1>
          <p className="text-caption text-fg-muted">Audition loops and one-shots across every published sample pack.</p>
        </div>
        {packageId && (
          <Badge tone="primary" className="py-1">
            Package: {packageNames[packageId] ?? packageId}
            <button type="button" aria-label="Clear package filter" onClick={() => setSp({})} className="ml-1">
              <X className="h-3 w-3" />
            </button>
          </Badge>
        )}
      </div>

      <section aria-label="Sample filters" className="space-y-3 rounded-lg border border-border bg-surface p-4">
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-7">
          <div className="xl:col-span-2">
            <Input value={text} onChange={(e) => setText(e.target.value)} placeholder="Search name, tag, instrument…" aria-label="Search samples" icon={<Search className="h-4 w-4" />} />
          </div>
          <Select aria-label="Type" value={contentType} onChange={(e) => setContentType(e.target.value as SampleContentType | '')} placeholder="Loops & one-shots" options={[{ value: 'loop', label: 'Loops' }, { value: 'one-shot', label: 'One-shots' }]} />
          <Select aria-label="Key" value={key} onChange={(e) => setKey(e.target.value)} placeholder="Any key" options={KEY_OPTIONS} />
          <div className="flex items-center gap-2 xl:col-span-2">
            <Input type="number" placeholder="BPM min" aria-label="Minimum BPM" value={bpmMin} onChange={(e) => setBpmMin(e.target.value)} />
            <Input type="number" placeholder="BPM max" aria-label="Maximum BPM" value={bpmMax} onChange={(e) => setBpmMax(e.target.value)} />
          </div>
          <Select aria-label="Duration" value={duration} onChange={(e) => setDuration(e.target.value)} placeholder="Any length" options={DURATIONS} />
        </div>
        <div className="flex flex-wrap items-center gap-1.5" role="group" aria-label="Instrument">
          {INSTRUMENTS.map((i) => {
            const on = instrument.includes(i);
            return (
              <button
                key={i}
                type="button"
                aria-pressed={on}
                onClick={() => setInstrument((xs) => (on ? xs.filter((x) => x !== i) : [...xs, i]))}
                className={cn('rounded-full border px-3 py-1 text-caption capitalize transition-colors', on ? 'border-primary bg-primary/15 text-fg' : 'border-border text-fg-muted hover:text-fg')}
              >
                {i.replace('-', ' ')}
              </button>
            );
          })}
          <Button variant="ghost" size="sm" className="ml-auto" onClick={reset}>
            Reset
          </Button>
        </div>
      </section>

      {selected.size > 0 && (
        <div className="sticky top-16 z-20 flex items-center justify-between gap-3 rounded-lg border border-primary/40 bg-surface-raised px-4 py-2 shadow-elevated" role="status">
          <span className="text-body">
            <strong>{selected.size}</strong> selected · ~{formatBytes(estSize)}
          </span>
          <div className="flex gap-2">
            <Button variant="ghost" size="sm" onClick={() => setSelected(new Set())}>Clear</Button>
            <Button size="sm" leftIcon={<Download className="h-4 w-4" />} onClick={batchDownload}>
              Download selected
            </Button>
          </div>
        </div>
      )}

      <p className="text-caption text-fg-muted" aria-live="polite">
        {samplesQ.isLoading ? 'Loading samples…' : `${samples.length} samples`}
      </p>

      {samplesQ.isLoading ? (
        <Skeleton className="h-96" />
      ) : samples.length === 0 ? (
        <EmptyState icon={<FileAudio className="h-6 w-6" />} title="No samples match" description="Loosen the BPM range or remove an instrument filter." action={<Button variant="secondary" onClick={reset}>Reset filters</Button>} />
      ) : (
        <div className={cn(samplesQ.isFetching && 'opacity-70')}>
          <SampleTable
            samples={samples}
            packageNames={packageNames}
            selected={selected}
            onToggleSelect={toggleSelect}
            onToggleAll={toggleAll}
            onDownload={(s) => downloadSample(s, toast)}
          />
        </div>
      )}
    </div>
  );
}
