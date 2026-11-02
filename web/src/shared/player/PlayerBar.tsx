import { Pause, Play, Volume2, X } from 'lucide-react';
import { formatDuration } from '@/shared/lib/format';
import { Waveform } from '@/shared/ui/Waveform';
import { usePlayer } from './PlayerContext';

export function PlayerBar() {
  const p = usePlayer();
  if (!p.track) return null;
  const progress = p.track.durationSec ? p.position / p.track.durationSec : 0;
  return (
    <div role="region" aria-label="Audio preview player" className="fixed inset-x-0 bottom-0 z-40 border-t border-border bg-surface/95 backdrop-blur">
      <div className="mx-auto flex h-16 items-center gap-4 px-4 lg:pl-64">
        <button
          type="button"
          onClick={() => p.toggle()}
          aria-label={p.playing ? 'Pause' : 'Play'}
          className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-fg text-background hover:scale-105"
        >
          {p.playing ? <Pause className="h-4 w-4" fill="currentColor" /> : <Play className="h-4 w-4 translate-x-[1px]" fill="currentColor" />}
        </button>
        <div className="w-48 min-w-0 shrink-0">
          <p className="truncate text-body font-medium">{p.track.title}</p>
          <p className="truncate text-caption text-fg-muted">{p.track.subtitle}</p>
        </div>
        <span className="hidden w-10 text-right font-mono text-caption text-fg-subtle sm:block">{formatDuration(p.position)}</span>
        <div className="relative hidden flex-1 sm:block">
          <Waveform seed={p.track.id} bars={96} progress={progress} />
          <input
            type="range"
            min={0}
            max={p.track.durationSec}
            step={0.01}
            value={p.position}
            onChange={(e) => p.seek(Number(e.target.value))}
            aria-label="Seek"
            className="absolute inset-0 h-full w-full cursor-pointer opacity-0"
          />
        </div>
        <span className="hidden w-10 font-mono text-caption text-fg-subtle sm:block">{formatDuration(p.track.durationSec)}</span>
        <div className="hidden items-center gap-2 md:flex">
          <Volume2 className="h-4 w-4 text-fg-muted" aria-hidden />
          <input
            type="range"
            min={0}
            max={1}
            step={0.01}
            value={p.volume}
            onChange={(e) => p.setVolume(Number(e.target.value))}
            aria-label="Volume"
            className="w-24 accent-[rgb(var(--oa-primary))]"
          />
        </div>
        {p.error ? (
          <span role="alert" className="hidden text-caption text-danger xl:inline">{p.error}</span>
        ) : (
          !p.real && (
            <span className="hidden rounded border border-border px-1.5 py-0.5 text-caption text-fg-subtle xl:inline" title="Offline demo: no audio files">
              simulated preview
            </span>
          )
        )}
        <button type="button" onClick={p.stop} aria-label="Close player" className="rounded p-1 text-fg-muted hover:text-fg">
          <X className="h-4 w-4" />
        </button>
      </div>
    </div>
  );
}
