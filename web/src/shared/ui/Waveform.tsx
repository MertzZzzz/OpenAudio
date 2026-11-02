import { useMemo } from 'react';
import { cn } from '@/shared/lib/cn';

export interface WaveformProps {
  seed: string;
  bars?: number;
  progress?: number;
  className?: string;
}

/** Deterministic placeholder waveform (real peaks come from the preview job later). */
export function Waveform({ seed, bars = 48, progress = 0, className }: WaveformProps) {
  const heights = useMemo(() => {
    let h = 2166136261;
    for (let i = 0; i < seed.length; i += 1) h = Math.imul(h ^ seed.charCodeAt(i), 16777619);
    return Array.from({ length: bars }, (_, i) => {
      h = Math.imul(h ^ (i + 1), 16777619);
      const r = ((h >>> 0) % 1000) / 1000;
      const env = Math.sin((i / bars) * Math.PI) * 0.5 + 0.5;
      return 0.15 + r * 0.85 * env;
    });
  }, [seed, bars]);

  return (
    <div aria-hidden className={cn('flex h-8 items-center gap-[2px]', className)}>
      {heights.map((v, i) => (
        <span
          key={i}
          className={cn('w-full min-w-[2px] rounded-full', i / bars < progress ? 'bg-primary' : 'bg-fg-subtle/40')}
          style={{ height: `${Math.round(v * 100)}%` }}
        />
      ))}
    </div>
  );
}
