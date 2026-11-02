import type { TaxonomyTerm } from '@/shared/types/domain';

export const TAXONOMY_VERSION = '2026.11';

const t = (
  domain: TaxonomyTerm['domain'],
  slug: string,
  label: string,
  parent?: string,
  aliases?: string[],
): TaxonomyTerm => ({ domain, slug, label, parent, aliases });

export const taxonomyTerms: TaxonomyTerm[] = [
  t('genre', 'electronic', 'Electronic'),
  t('genre', 'house', 'House', 'electronic'),
  t('genre', 'deep-house', 'Deep House', 'house'),
  t('genre', 'tech-house', 'Tech House', 'house'),
  t('genre', 'progressive-house', 'Progressive House', 'house'),
  t('genre', 'techno', 'Techno', 'electronic'),
  t('genre', 'melodic-techno', 'Melodic Techno', 'techno'),
  t('genre', 'industrial-techno', 'Industrial Techno', 'techno'),
  t('genre', 'drum-and-bass', 'Drum & Bass', 'electronic', ['dnb', 'jungle']),
  t('genre', 'ambient', 'Ambient', 'electronic'),
  t('genre', 'hip-hop', 'Hip Hop', undefined, ['rap']),
  t('genre', 'lo-fi', 'Lo-Fi', 'hip-hop', ['lofi']),
  t('genre', 'trap', 'Trap', 'hip-hop'),
  t('genre', 'cinematic', 'Cinematic', undefined, ['score', 'film']),
  t('genre', 'synthwave', 'Synthwave', 'electronic', ['retrowave']),
  t('instrument', 'drums', 'Drums'),
  t('instrument', 'kick', 'Kick', 'drums'),
  t('instrument', 'snare', 'Snare', 'drums'),
  t('instrument', 'clap', 'Clap', 'drums'),
  t('instrument', 'hi-hat', 'Hi-Hat', 'drums', ['hihat', 'hat']),
  t('instrument', 'percussion', 'Percussion', 'drums'),
  t('instrument', 'bass', 'Bass'),
  t('instrument', 'synth', 'Synth'),
  t('instrument', 'pad', 'Pad', 'synth'),
  t('instrument', 'keys', 'Keys'),
  t('instrument', 'piano', 'Piano', 'keys'),
  t('instrument', 'guitar', 'Guitar'),
  t('instrument', 'vocals', 'Vocals', undefined, ['vox']),
  t('instrument', 'strings', 'Strings'),
  t('instrument', 'fx', 'FX', undefined, ['sfx']),
  ...['warm', 'dark', 'bright', 'hypnotic', 'aggressive', 'melancholic', 'dreamy', 'uplifting', 'gritty'].map(
    (m) => t('mood', m, m[0]!.toUpperCase() + m.slice(1)),
  ),
  t('use-case', 'club', 'Club'),
  t('use-case', 'studio-production', 'Studio Production'),
  t('use-case', 'live-performance', 'Live Performance'),
  t('use-case', 'sound-design', 'Sound Design'),
  t('use-case', 'mixing', 'Mixing'),
  t('use-case', 'mastering', 'Mastering'),
  ...[
    'reverb',
    'convolution',
    'delay',
    'compressor',
    'eq',
    'saturation',
    'distortion',
    'filter',
    'chorus',
    'granular',
    'wavetable',
    'sampler',
    'arpeggiator',
    'chord',
    'sequencer',
    'meter',
    'spectrum',
    'tuner',
    'gain',
    'stereo',
    'pitch',
    'neural',
  ].map((c) => t('module-category', c, c[0]!.toUpperCase() + c.slice(1))),
];

const labelIndex = new Map(taxonomyTerms.map((term) => [`${term.domain}:${term.slug}`, term.label]));

export function termLabel(domain: TaxonomyTerm['domain'], slug: string): string {
  return labelIndex.get(`${domain}:${slug}`) ?? slug;
}

export function termsOf(domain: TaxonomyTerm['domain']): TaxonomyTerm[] {
  return taxonomyTerms.filter((term) => term.domain === domain);
}

/** Returns the slug plus all its descendants, so filtering by "house" matches "deep-house". */
export function expandWithChildren(domain: TaxonomyTerm['domain'], slug: string): string[] {
  const out = [slug];
  for (const term of taxonomyTerms) {
    if (term.domain === domain && term.parent === slug) out.push(...expandWithChildren(domain, term.slug));
  }
  return out;
}

/** Resolves an alias (e.g. "dnb") to its canonical slug. */
export function resolveAlias(query: string): TaxonomyTerm | undefined {
  const q = query.toLowerCase();
  return taxonomyTerms.find((term) => term.slug === q || term.aliases?.includes(q));
}
