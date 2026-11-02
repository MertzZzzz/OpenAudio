import { Download, Repeat, Zap } from 'lucide-react';
import type { Sample } from '@/shared/types/domain';
import { Table, TBody, TD, TH, THead, TR } from '@/shared/ui/Table';
import { Badge } from '@/shared/ui/Badge';
import { Waveform } from '@/shared/ui/Waveform';
import { PlayButton } from '@/shared/player/PlayButton';
import { usePlayer } from '@/shared/player/PlayerContext';
import { formatKey, formatLength } from '@/shared/lib/format';
import { termLabel } from '@/shared/lib/taxonomy';
import { sampleTrack } from '@/features/catalog/toTrack';
import { cn } from '@/shared/lib/cn';

export interface SampleTableProps {
  samples: Sample[];
  packageNames?: Record<string, string>;
  selected?: Set<string>;
  onToggleSelect?: (id: string) => void;
  onToggleAll?: () => void;
  onDownload?: (s: Sample) => void;
}

export function SampleTable({ samples, packageNames, selected, onToggleSelect, onToggleAll, onDownload }: SampleTableProps) {
  const player = usePlayer();
  const selectable = Boolean(selected && onToggleSelect);
  const allSelected = selectable && samples.length > 0 && samples.every((s) => selected!.has(s.id));

  return (
    <Table aria-label="Samples">
      <THead>
        <tr>
          {selectable && (
            <TH className="w-10">
              <input type="checkbox" aria-label="Select all samples" checked={allSelected} onChange={onToggleAll} className="h-4 w-4 accent-[rgb(var(--oa-primary))]" />
            </TH>
          )}
          <TH className="w-12"><span className="sr-only">Play</span></TH>
          <TH>Name</TH>
          <TH className="hidden w-48 md:table-cell">Waveform</TH>
          <TH>Instrument</TH>
          <TH className="text-right">BPM</TH>
          <TH>Key</TH>
          <TH className="text-right">Length</TH>
          <TH className="hidden lg:table-cell">Tags</TH>
          <TH className="w-12"><span className="sr-only">Download</span></TH>
        </tr>
      </THead>
      <TBody>
        {samples.map((s) => {
          const track = sampleTrack(s, packageNames?.[s.packageId]);
          const current = player.isCurrent(track.id);
          const progress = current && track.durationSec ? player.position / track.durationSec : 0;
          return (
            <TR key={s.id} className={cn(current && 'bg-primary/5', selected?.has(s.id) && 'bg-primary/10')}>
              {selectable && (
                <TD>
                  <input type="checkbox" aria-label={`Select ${s.name}`} checked={selected!.has(s.id)} onChange={() => onToggleSelect!(s.id)} className="h-4 w-4 accent-[rgb(var(--oa-primary))]" />
                </TD>
              )}
              <TD>
                <PlayButton track={track} size="sm" />
              </TD>
              <TD className="max-w-[16rem]">
                <p className="flex items-center gap-1.5 truncate font-medium">
                  {s.contentType === 'loop' ? <Repeat className="h-3.5 w-3.5 shrink-0 text-accent" aria-label="Loop" /> : <Zap className="h-3.5 w-3.5 shrink-0 text-warning" aria-label="One-shot" />}
                  {s.name}
                </p>
                <p className="truncate font-mono text-caption text-fg-subtle">{packageNames?.[s.packageId] ?? s.file}</p>
              </TD>
              <TD className="hidden md:table-cell">
                <Waveform seed={s.id} bars={40} progress={progress} className="h-6" />
              </TD>
              <TD>{termLabel('instrument', s.instrument)}</TD>
              <TD className="whitespace-nowrap text-right font-mono text-caption">{s.bpm ?? '—'}</TD>
              <TD className="whitespace-nowrap font-mono text-caption">{formatKey(s.key)}</TD>
              <TD className="whitespace-nowrap text-right font-mono text-caption">{formatLength(s.durationSec)}</TD>
              <TD className="hidden lg:table-cell">
                <div className="flex gap-1">
                  {s.tags.map((t) => (
                    <Badge key={t}>{t}</Badge>
                  ))}
                </div>
              </TD>
              <TD>
                <button type="button" onClick={() => onDownload?.(s)} aria-label={`Download ${s.name}`} className="rounded p-1.5 text-fg-muted hover:bg-surface-raised hover:text-fg">
                  <Download className="h-4 w-4" />
                </button>
              </TD>
            </TR>
          );
        })}
      </TBody>
    </Table>
  );
}
