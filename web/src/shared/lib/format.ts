import type { MusicalKey } from '@/shared/types/domain';

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  let value = bytes / 1024;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(value >= 10 ? 0 : 1)} ${units[unit]}`;
}

export function formatCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(n >= 10_000 ? 0 : 1)}k`;
  return String(n);
}

export function formatDuration(sec: number): string {
  const m = Math.floor(sec / 60);
  const s = Math.floor(sec % 60);
  return `${m}:${String(s).padStart(2, '0')}`;
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' });
}

export function formatKey(key: MusicalKey | undefined): string {
  if (!key) return '—';
  return `${key.tonic} ${key.mode === 'minor' ? 'min' : 'maj'}`;
}

/** Stable string form used in filters / query params: "F-minor". */
export function keyToSlug(key: MusicalKey): string {
  return `${key.tonic}-${key.mode}`;
}

export function shortDigest(digest: string): string {
  const [algo, hex = ''] = digest.split(':');
  return `${algo}:${hex.slice(0, 12)}…`;
}

export function humanize(slug: string): string {
  return slug
    .split('-')
    .map((w) => (w.length ? w[0]!.toUpperCase() + w.slice(1) : w))
    .join(' ');
}

/** Sample length: "0.4s", "7.7s", "1:12". */
export function formatLength(sec: number): string {
  if (sec < 60) return `${sec.toFixed(1)}s`;
  return formatDuration(sec);
}
