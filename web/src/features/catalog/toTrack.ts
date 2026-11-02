import type { Package, Sample } from '@/shared/types/domain';
import { API_BASE } from '@/shared/api/http';
import { apiMode } from '@/shared/lib/apiMode';
import type { PlayerTrack } from '@/shared/player/PlayerContext';

export function packageTrack(p: Package): PlayerTrack {
  return {
    id: `pkg:${p.id}`,
    title: p.name,
    subtitle: `${p.publisher.name} · demo`,
    durationSec: 30,
    // With the API: redirect to playable audio of the latest release (not counted as a download).
    previewUrl: apiMode === 'http' ? `${API_BASE}/packages/${p.publisher.slug}/${p.slug}/preview` : `mock://previews/${p.id}/demo.mp3`,
    hue: p.coverHue,
  };
}

export function sampleTrack(s: Sample, packageName?: string): PlayerTrack {
  return {
    id: `sample:${s.id}`,
    title: s.name,
    subtitle: packageName ?? s.file,
    durationSec: Math.max(1, s.durationSec),
    previewUrl: s.previewUrl || (s.downloadUrl ? `${s.downloadUrl}?preview=1` : ''),
  };
}
