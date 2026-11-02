import { buildFacets, bpmLabel } from './catalogQuery';
import { packages } from './mock/data';

describe('client-side facets', () => {
  it('counts BPM buckets ignoring the BPM filter itself', () => {
    const all = buildFacets(packages, {});
    const narrowed = buildFacets(packages, { bpmMin: 170, bpmMax: 180 });
    expect(narrowed.bpm).toEqual(all.bpm);
    expect(all.bpm.find((b) => b.value === '120-129')?.count).toBeGreaterThan(0);
    expect(narrowed.genre.map((g) => g.value)).toContain('drum-and-bass');
    expect(narrowed.genre.map((g) => g.value)).not.toContain('deep-house');
  });

  it('labels buckets like the server', () => {
    expect(bpmLabel('120-129')).toBe('120–129');
    expect(bpmLabel('0-79')).toBe('< 80');
    expect(bpmLabel('180-400')).toBe('180+');
  });
});
