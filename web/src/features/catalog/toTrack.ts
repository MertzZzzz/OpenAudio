import type { Package, Sample } from '@/shared/types/domain';
import type { PlayerTrack } from '@/shared/player/PlayerContext';

export function packageTrack(p: Package): PlayerTrack {
  return {
    id: `pkg:${p.id}`,
    title: p.name,
    subtitle: `${p.publisher.name} · demo`,
    durationSec: 30,
    previewUrl: `mock://previews/${p.id}/demo.mp3`,
    hue: p.coverHue,
  };
}

export function sampleTrack(s: Sample, packageName?: string): PlayerTrack {
  return {
    id: `sample:${s.id}`,
    title: s.name,
    subtitle: packageName ?? s.file,
    durationSec: Math.max(1, s.durationSec),
    previewUrl: s.previewUrl,
  };
}
