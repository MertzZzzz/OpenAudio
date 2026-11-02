import type {
  Artifact,
  KeyTonic,
  Library,
  Module,
  ModuleClass,
  MusicalKey,
  Package,
  Publisher,
  PublisherMember,
  Release,
  Sample,
} from '@/shared/types/domain';
import { compare } from '@/shared/lib/semver';

/* ------------------------------------------------------------------ helpers */

/** Deterministic PRNG so mock data is stable between reloads and tests. */
function mulberry32(seed: number): () => number {
  let a = seed;
  return () => {
    a |= 0;
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function fakeDigest(seed: string): string {
  let h1 = 0x811c9dc5;
  let out = '';
  for (let round = 0; round < 8; round += 1) {
    for (let i = 0; i < seed.length; i += 1) {
      h1 ^= seed.charCodeAt(i) + round;
      h1 = Math.imul(h1, 0x01000193);
    }
    out += (h1 >>> 0).toString(16).padStart(8, '0');
  }
  return `sha256:${out}`;
}

const CAMELOT: Record<string, string> = {
  'C-major': '8B', 'G-major': '9B', 'D-major': '10B', 'A-major': '11B', 'E-major': '12B', 'B-major': '1B',
  'F#-major': '2B', 'C#-major': '3B', 'G#-major': '4B', 'D#-major': '5B', 'A#-major': '6B', 'F-major': '7B',
  'A-minor': '8A', 'E-minor': '9A', 'B-minor': '10A', 'F#-minor': '11A', 'C#-minor': '12A', 'G#-minor': '1A',
  'D#-minor': '2A', 'A#-minor': '3A', 'F-minor': '4A', 'C-minor': '5A', 'G-minor': '6A', 'D-minor': '7A',
};

function key(tonic: KeyTonic, mode: 'major' | 'minor'): MusicalKey {
  return { tonic, mode, camelot: CAMELOT[`${tonic}-${mode}`] };
}

function mod(moduleClass: ModuleClass, categories: string[], extra: Partial<Module> = {}): Module {
  const io: Record<ModuleClass, Pick<Module['capabilities'], 'accepts' | 'produces' | 'placement'>> = {
    instrument: { accepts: ['midi', 'automation'], produces: ['audio'], placement: ['instrument-track'] },
    'audio-effect': { accepts: ['audio', 'automation'], produces: ['audio'], placement: ['insert', 'send', 'bus', 'master'] },
    'note-effect': { accepts: ['midi', 'transport'], produces: ['midi'], placement: ['midi-track', 'instrument-track'] },
    'note-generator': { accepts: ['transport'], produces: ['midi'], placement: ['midi-track'] },
    'audio-generator': { accepts: ['transport'], produces: ['audio'], placement: ['audio-track'] },
    analyzer: { accepts: ['audio'], produces: ['analysis'], placement: ['insert', 'master'] },
    utility: { accepts: ['audio'], produces: ['audio'], placement: ['insert', 'bus', 'master'] },
    converter: { accepts: ['audio'], produces: ['midi'], placement: ['audio-track'] },
    macro: { accepts: ['audio', 'sidechain'], produces: ['audio'], placement: ['insert', 'bus'] },
    visualizer: { accepts: ['audio', 'midi'], produces: ['analysis'], placement: ['insert'] },
  };
  const caps = io[moduleClass];
  return {
    moduleClass,
    categories,
    capabilities: { ...caps, bypassable: moduleClass !== 'instrument', wetDry: moduleClass === 'audio-effect', stateful: true },
    ports: [
      ...caps.accepts.filter((s) => s !== 'automation').map((s, i) => ({ id: `in-${i}`, direction: 'in' as const, signal: s, channels: s === 'audio' ? 2 : undefined })),
      ...caps.produces.map((s, i) => ({ id: `out-${i}`, direction: 'out' as const, signal: s, channels: s === 'audio' ? 2 : undefined })),
    ],
    ...extra,
  };
}

/* --------------------------------------------------------------- publishers */

export const publishers: Publisher[] = [
  { id: 'org.acme', slug: 'acme', name: 'Acme Audio', verified: true },
  { id: 'org.tapeworks', slug: 'tapeworks', name: 'Tapeworks', verified: true },
  { id: 'org.nullsignal', slug: 'nullsignal', name: 'Null Signal', verified: false },
  { id: 'org.fieldnotes', slug: 'fieldnotes', name: 'Field Notes Collective', verified: true },
  { id: 'org.lattice', slug: 'lattice', name: 'Lattice DSP', verified: false },
];

const pub = (slug: string): Publisher => publishers.find((p) => p.slug === slug)!;

/* ----------------------------------------------------------------- packages */

type PackageSeed = Omit<Package, 'id' | 'createdAt' | 'updatedAt' | 'dependencies' | 'technical' | 'tags' | 'rating'> &
  Partial<Pick<Package, 'dependencies' | 'technical' | 'tags' | 'rating'>> & { daysAgo: number; ageDays: number };

const NOW = Date.parse('2026-11-01T12:00:00Z');
const day = 86_400_000;

function makePackage(seed: PackageSeed): Package {
  const { daysAgo, ageDays, ...rest } = seed;
  return {
    technical: { formats: ['wav'] },
    tags: [],
    rating: 4.5,
    dependencies: [],
    ...rest,
    id: `${seed.publisher.id}.${seed.slug}`,
    updatedAt: new Date(NOW - daysAgo * day).toISOString(),
    createdAt: new Date(NOW - (daysAgo + ageDays) * day).toISOString(),
  };
}

const CC_BY = { content: 'CC-BY-4.0', commercialUse: true, attributionRequired: true };
const CC0 = { content: 'CC0-1.0', commercialUse: true, attributionRequired: false };
const MIT = { content: 'MIT', commercialUse: true, attributionRequired: true };
const GPL = { content: 'GPL-3.0-or-later', commercialUse: true, attributionRequired: true };
const CC_BY_NC = { content: 'CC-BY-NC-4.0', commercialUse: false, attributionRequired: true };

export const packages: Package[] = [
  makePackage({
    publisher: pub('acme'), slug: 'deep-house-toolkit', name: 'Deep House Toolkit', kind: 'sample-pack',
    description: 'Deep house drums, bass loops and textures recorded through a vintage tape chain. 180+ loops and one-shots, all key- and tempo-labelled.',
    latestVersion: '1.2.0', license: CC_BY,
    taxonomy: { genres: { primary: 'deep-house', secondary: ['tech-house'] }, instruments: ['drums', 'bass', 'percussion'], moods: ['warm', 'hypnotic'], useCases: ['club', 'studio-production'], energy: 0.62 },
    musical: { bpm: { value: 124, range: [122, 126], sync: true }, key: key('F', 'minor'), timeSignature: '4/4' },
    technical: { sampleRate: 48000, bitDepth: 24, formats: ['wav', 'flac'] },
    tags: ['analog', 'tape', 'rolling'], downloads: 48210, rating: 4.8, sampleCount: 184, sizeBytes: 1_240_000_000,
    coverHue: 262, editorsPick: true, daysAgo: 3, ageDays: 210,
  }),
  makePackage({
    publisher: pub('tapeworks'), slug: 'warm-tape-drums', name: 'Warm Tape Drums', kind: 'sample-pack',
    description: 'Acoustic and drum-machine hits bounced to 1/4" tape. Saturated kicks, dusty snares and swung hat loops.',
    latestVersion: '2.0.1', license: CC0,
    taxonomy: { genres: { primary: 'lo-fi', secondary: ['hip-hop'] }, instruments: ['drums', 'kick', 'snare', 'hi-hat'], moods: ['warm', 'melancholic'], useCases: ['studio-production'], energy: 0.4 },
    musical: { bpm: { value: 88, range: [80, 92] }, key: key('A', 'minor'), timeSignature: '4/4' },
    technical: { sampleRate: 44100, bitDepth: 24, formats: ['wav'] },
    tags: ['dusty', 'tape', 'boom-bap'], downloads: 91240, rating: 4.9, sampleCount: 240, sizeBytes: 860_000_000,
    coverHue: 28, editorsPick: true, daysAgo: 12, ageDays: 400,
  }),
  makePackage({
    publisher: pub('nullsignal'), slug: 'warehouse-techno', name: 'Warehouse Techno Vol. 3', kind: 'sample-pack',
    description: 'Industrial percussion, distorted kicks and metallic textures captured in an empty warehouse.',
    latestVersion: '3.1.0', license: CC_BY,
    taxonomy: { genres: { primary: 'industrial-techno', secondary: ['techno'] }, instruments: ['drums', 'kick', 'percussion', 'fx'], moods: ['dark', 'aggressive'], useCases: ['club'], energy: 0.88 },
    musical: { bpm: { value: 132, range: [128, 136] }, key: key('D', 'minor'), timeSignature: '4/4' },
    technical: { sampleRate: 48000, bitDepth: 24, formats: ['wav'] },
    tags: ['industrial', 'metallic', 'distorted'], downloads: 27500, rating: 4.6, sampleCount: 132, sizeBytes: 980_000_000,
    coverHue: 0, daysAgo: 1, ageDays: 60,
  }),
  makePackage({
    publisher: pub('fieldnotes'), slug: 'nordic-field-recordings', name: 'Nordic Field Recordings', kind: 'sample-pack',
    description: 'Ice, wind, forests and harbours recorded across Norway and Finland. Long ambiences plus processed one-shots.',
    latestVersion: '1.0.0', license: CC0,
    taxonomy: { genres: { primary: 'ambient', secondary: ['cinematic'] }, instruments: ['fx'], moods: ['dreamy', 'dark'], useCases: ['sound-design', 'studio-production'], energy: 0.15 },
    technical: { sampleRate: 96000, bitDepth: 24, formats: ['wav', 'flac'] },
    tags: ['field-recording', 'nature', 'texture'], downloads: 12030, rating: 4.7, sampleCount: 96, sizeBytes: 3_100_000_000,
    coverHue: 196, daysAgo: 20, ageDays: 5,
  }),
  makePackage({
    publisher: pub('acme'), slug: 'liquid-dnb-essentials', name: 'Liquid DnB Essentials', kind: 'sample-pack',
    description: 'Rolling breaks, reese basses and lush pads for liquid drum & bass.',
    latestVersion: '1.4.2', license: CC_BY,
    taxonomy: { genres: { primary: 'drum-and-bass', secondary: [] }, instruments: ['drums', 'bass', 'pad'], moods: ['uplifting', 'dreamy'], useCases: ['studio-production', 'club'], energy: 0.75 },
    musical: { bpm: { value: 174 }, key: key('E', 'minor'), timeSignature: '4/4' },
    technical: { sampleRate: 44100, bitDepth: 24, formats: ['wav'] },
    tags: ['breaks', 'reese', 'liquid'], downloads: 33410, rating: 4.7, sampleCount: 150, sizeBytes: 1_020_000_000,
    coverHue: 170, daysAgo: 6, ageDays: 300,
  }),
  makePackage({
    publisher: pub('tapeworks'), slug: 'vocal-chops-soul', name: 'Soul Vocal Chops', kind: 'sample-pack',
    description: 'Original soul vocal phrases, chopped and pitch-labelled. Cleared for commercial use.',
    latestVersion: '1.1.0', license: CC_BY,
    taxonomy: { genres: { primary: 'hip-hop', secondary: ['house'] }, instruments: ['vocals'], moods: ['warm', 'uplifting'], useCases: ['studio-production'], energy: 0.5 },
    musical: { bpm: { value: 96 }, key: key('C', 'major') },
    technical: { sampleRate: 48000, bitDepth: 24, formats: ['wav'] },
    tags: ['soul', 'vocal', 'chops'], downloads: 22800, rating: 4.4, sampleCount: 88, sizeBytes: 540_000_000,
    coverHue: 340, daysAgo: 40, ageDays: 100,
  }),
  makePackage({
    publisher: pub('lattice'), slug: 'grain-cloud', name: 'Grain Cloud', kind: 'instrument',
    description: 'Granular synthesizer module with 8 grain streams, spectral freeze and per-voice modulation matrix.',
    latestVersion: '0.9.0', license: GPL,
    taxonomy: { genres: { primary: 'ambient', secondary: ['cinematic', 'electronic'] }, instruments: ['synth', 'pad'], moods: ['dreamy'], useCases: ['sound-design'] },
    technical: { formats: ['clap', 'vst3'] },
    module: mod('instrument', ['granular', 'sampler']),
    tags: ['granular', 'texture', 'open-source'], downloads: 15600, rating: 4.6, sampleCount: 24, sizeBytes: 64_000_000,
    coverHue: 210, editorsPick: true, daysAgo: 9, ageDays: 150,
    dependencies: [{ packageId: 'org.fieldnotes.nordic-field-recordings', range: '^1.0.0' }],
  }),
  makePackage({
    publisher: pub('lattice'), slug: 'plate-verb', name: 'Plate Verb', kind: 'effect',
    description: 'Algorithmic plate reverb modelled after classic studio plates. Pre-delay, damping and stereo width controls.',
    latestVersion: '2.3.0', license: MIT,
    taxonomy: { genres: { primary: 'electronic', secondary: [] }, instruments: [], moods: ['warm'], useCases: ['mixing'] },
    technical: { formats: ['clap', 'vst3', 'au'] },
    module: mod('audio-effect', ['reverb']),
    tags: ['reverb', 'plate', 'open-source'], downloads: 64300, rating: 4.8, sampleCount: 0, sizeBytes: 12_000_000,
    coverHue: 240, daysAgo: 4, ageDays: 500,
  }),
  makePackage({
    publisher: pub('acme'), slug: 'convolver-spaces', name: 'Convolver Spaces', kind: 'effect',
    description: 'Convolution reverb with 120 impulse responses: halls, churches, stairwells and springs.',
    latestVersion: '1.0.3', license: CC_BY,
    taxonomy: { genres: { primary: 'cinematic', secondary: [] }, instruments: [], moods: ['dreamy'], useCases: ['mixing', 'sound-design'] },
    technical: { sampleRate: 48000, bitDepth: 24, formats: ['clap', 'vst3'] },
    module: mod('audio-effect', ['reverb', 'convolution']),
    tags: ['impulse-response', 'space'], downloads: 18700, rating: 4.5, sampleCount: 120, sizeBytes: 420_000_000,
    coverHue: 280, daysAgo: 15, ageDays: 90,
    dependencies: [{ packageId: 'org.lattice.plate-verb', range: '^2.0.0' }],
  }),
  makePackage({
    publisher: pub('nullsignal'), slug: 'tape-saturator', name: 'Tape Saturator', kind: 'effect',
    description: 'Tape-style saturation with wow & flutter, hiss and head bump modelling.',
    latestVersion: '1.6.0', license: MIT,
    taxonomy: { genres: { primary: 'lo-fi', secondary: ['electronic'] }, instruments: [], moods: ['warm', 'gritty'], useCases: ['mixing', 'mastering'] },
    technical: { formats: ['clap', 'vst3'] },
    module: mod('audio-effect', ['saturation', 'distortion']),
    tags: ['tape', 'saturation', 'lo-fi'], downloads: 40100, rating: 4.7, sampleCount: 0, sizeBytes: 8_000_000,
    coverHue: 18, daysAgo: 2, ageDays: 260,
  }),
  makePackage({
    publisher: pub('lattice'), slug: 'polyarp', name: 'PolyArp', kind: 'midi-tool',
    description: 'Polyrhythmic arpeggiator with per-step probability, ratchets and scale quantisation.',
    latestVersion: '1.2.1', license: GPL,
    taxonomy: { genres: { primary: 'techno', secondary: ['melodic-techno'] }, instruments: [], moods: ['hypnotic'], useCases: ['live-performance', 'studio-production'] },
    technical: { formats: ['clap'] },
    module: mod('note-effect', ['arpeggiator']),
    tags: ['arp', 'generative', 'polyrhythm'], downloads: 9800, rating: 4.3, sampleCount: 0, sizeBytes: 3_000_000,
    coverHue: 150, daysAgo: 25, ageDays: 120,
  }),
  makePackage({
    publisher: pub('fieldnotes'), slug: 'step-seq-16', name: 'Step Seq 16', kind: 'midi-tool',
    description: '16-step MIDI sequencer with euclidean mode and per-lane swing.',
    latestVersion: '0.4.0', license: MIT,
    taxonomy: { genres: { primary: 'techno', secondary: [] }, instruments: ['drums'], moods: [], useCases: ['live-performance'] },
    technical: { formats: ['clap'] },
    module: mod('note-generator', ['sequencer']),
    tags: ['sequencer', 'euclidean'], downloads: 4100, rating: 4.1, sampleCount: 0, sizeBytes: 2_000_000,
    coverHue: 120, daysAgo: 30, ageDays: 15,
  }),
  makePackage({
    publisher: pub('nullsignal'), slug: 'loudness-meter', name: 'Loudness Meter', kind: 'utility',
    description: 'EBU R128 loudness meter with LUFS-I/S/M, true peak and a loudness history graph.',
    latestVersion: '1.0.0', license: MIT,
    taxonomy: { genres: { primary: 'electronic', secondary: [] }, instruments: [], moods: [], useCases: ['mastering', 'mixing'] },
    technical: { formats: ['clap', 'vst3', 'au'] },
    module: mod('analyzer', ['meter']),
    tags: ['lufs', 'meter', 'r128'], downloads: 52000, rating: 4.9, sampleCount: 0, sizeBytes: 4_000_000,
    coverHue: 90, daysAgo: 50, ageDays: 200,
  }),
  makePackage({
    publisher: pub('acme'), slug: 'analog-poly-presets', name: 'Analog Poly Presets', kind: 'preset-pack',
    description: '128 patches for Grain Cloud and other open synths: pads, keys, basses and plucks.',
    latestVersion: '1.0.1', license: CC_BY,
    taxonomy: { genres: { primary: 'synthwave', secondary: ['ambient'] }, instruments: ['synth', 'pad', 'bass'], moods: ['dreamy', 'bright'], useCases: ['studio-production'] },
    technical: { formats: ['json'] },
    tags: ['presets', 'retro', 'pads'], downloads: 7600, rating: 4.4, sampleCount: 0, sizeBytes: 2_000_000,
    coverHue: 300, daysAgo: 8, ageDays: 40,
    dependencies: [{ packageId: 'org.lattice.grain-cloud', range: '^0.9.0' }],
  }),
  makePackage({
    publisher: pub('tapeworks'), slug: 'lofi-beat-template', name: 'Lo-Fi Beat Template', kind: 'project-template',
    description: 'Starter project: drum bus with tape saturation, chopped keys and sidechained pads.',
    latestVersion: '1.0.0', license: CC0,
    taxonomy: { genres: { primary: 'lo-fi', secondary: ['hip-hop'] }, instruments: ['drums', 'keys'], moods: ['melancholic', 'warm'], useCases: ['studio-production'] },
    musical: { bpm: { value: 82 }, key: key('D', 'minor'), timeSignature: '4/4' },
    technical: { formats: ['openaudio-project'] },
    tags: ['template', 'starter'], downloads: 5400, rating: 4.2, sampleCount: 12, sizeBytes: 90_000_000,
    coverHue: 45, daysAgo: 18, ageDays: 30,
    dependencies: [
      { packageId: 'org.tapeworks.warm-tape-drums', range: '^2.0.0' },
      { packageId: 'org.nullsignal.tape-saturator', range: '^1.5.0' },
    ],
  }),
  makePackage({
    publisher: pub('lattice'), slug: 'stem-splitter', name: 'Stem Splitter', kind: 'model',
    description: 'Neural source-separation model (vocals / drums / bass / other). Weights + runtime requirements, no host integration yet.',
    latestVersion: '0.2.0', license: CC_BY_NC,
    taxonomy: { genres: { primary: 'electronic', secondary: [] }, instruments: ['vocals', 'drums', 'bass'], moods: [], useCases: ['sound-design'] },
    technical: { formats: ['onnx'] },
    module: mod('converter', ['neural']),
    tags: ['ml', 'separation', 'onnx'], downloads: 3100, rating: 4.0, sampleCount: 0, sizeBytes: 320_000_000,
    coverHue: 190, daysAgo: 0, ageDays: 10,
  }),
];

/* ----------------------------------------------------------------- releases */

const changelogLines = [
  'Initial public release.',
  'Fixed loop points on several bass loops.',
  'Added FLAC artifacts alongside WAV.',
  'Re-labelled keys using the 2026.11 taxonomy.',
  'New one-shots and improved previews.',
  'Manifest: added metadata provenance.',
];

function versionsUpTo(latest: string): string[] {
  const [maj, min, pat] = latest.split('.').map(Number) as [number, number, number];
  const out = new Set<string>([latest]);
  if (pat > 0) out.add(`${maj}.${min}.0`);
  if (min > 0) out.add(`${maj}.${min - 1}.0`);
  if (maj > 0 && (min > 0 || pat > 0)) out.add(`${maj}.0.0`);
  if (maj > 1) out.add(`${maj - 1}.0.0`);
  return [...out].sort(compare).reverse();
}

export const releases: Record<string, Release[]> = Object.fromEntries(
  packages.map((p) => {
    const versions = versionsUpTo(p.latestVersion);
    const base = Date.parse(p.updatedAt);
    return [
      p.id,
      versions.map((v, i) => ({
        version: v,
        status: i === versions.length - 1 && versions.length > 2 ? 'yanked' : 'published',
        publishedAt: new Date(base - i * 21 * day).toISOString(),
        changelog: changelogLines[(i + p.slug.length) % changelogLines.length]!,
        manifestDigest: fakeDigest(`${p.id}@${v}`),
      })) satisfies Release[],
    ];
  }),
);

// One draft on our own publisher so the dashboard shows the full lifecycle.
releases['org.acme.deep-house-toolkit']!.unshift({
  version: '1.3.0',
  status: 'draft',
  publishedAt: null,
  changelog: 'Draft: 40 new percussion loops.',
  manifestDigest: fakeDigest('org.acme.deep-house-toolkit@1.3.0'),
});

/* ------------------------------------------------------------------ samples */

const SAMPLE_SPECS: Record<string, { instrument: string; role: Sample['role']; words: string[] }[]> = {
  drums: [
    { instrument: 'kick', role: 'drums', words: ['Kick', 'Thump', 'Sub Kick'] },
    { instrument: 'snare', role: 'drums', words: ['Snare', 'Rim', 'Crack'] },
    { instrument: 'hi-hat', role: 'drums', words: ['Hat Loop', 'Open Hat', 'Shaker'] },
    { instrument: 'clap', role: 'drums', words: ['Clap', 'Snap'] },
    { instrument: 'percussion', role: 'drums', words: ['Perc Loop', 'Conga', 'Tom'] },
  ],
  bass: [{ instrument: 'bass', role: 'bass', words: ['Bass Loop', 'Reese', 'Sub', 'Pluck Bass'] }],
  pad: [{ instrument: 'pad', role: 'pad', words: ['Pad', 'Swell', 'Drone'] }],
  synth: [{ instrument: 'synth', role: 'lead', words: ['Lead', 'Stab', 'Arp'] }],
  vocals: [{ instrument: 'vocals', role: 'lead', words: ['Vox Chop', 'Phrase', 'Ad-lib'] }],
  keys: [{ instrument: 'keys', role: 'lead', words: ['Rhodes', 'Chords', 'Keys Loop'] }],
  fx: [{ instrument: 'fx', role: 'fx', words: ['Riser', 'Impact', 'Ambience', 'Texture'] }],
};

function buildSamples(): Sample[] {
  const out: Sample[] = [];
  const tonics: KeyTonic[] = ['C', 'D', 'E', 'F', 'G', 'A', 'B', 'F#', 'A#'];
  for (const p of packages) {
    if (p.sampleCount === 0) continue;
    const rand = mulberry32(p.slug.length * 7919 + p.coverHue);
    const specs = p.taxonomy.instruments.flatMap((i) => SAMPLE_SPECS[i] ?? []);
    const pool = specs.length ? specs : SAMPLE_SPECS.fx!;
    const count = Math.min(p.sampleCount, 24);
    for (let i = 0; i < count; i += 1) {
      const spec = pool[Math.floor(rand() * pool.length)]!;
      const word = spec.words[Math.floor(rand() * spec.words.length)]!;
      const isLoop = spec.role !== 'fx' && (word.includes('Loop') || rand() > 0.5);
      const baseBpm = p.musical?.bpm?.value;
      const bpm = isLoop && baseBpm ? baseBpm : undefined;
      const tonal = spec.role !== 'drums';
      const k = tonal ? (rand() > 0.4 && p.musical?.key ? p.musical.key : key(tonics[Math.floor(rand() * tonics.length)]!, rand() > 0.5 ? 'minor' : 'major')) : undefined;
      const durationSec = isLoop && bpm ? Number(((60 / bpm) * 4 * (rand() > 0.5 ? 4 : 2)).toFixed(3)) : Number((0.2 + rand() * (spec.role === 'fx' ? 12 : 1.6)).toFixed(3));
      const name = `${word} ${String(i + 1).padStart(2, '0')}`;
      const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, '-');
      const file = `${isLoop ? 'loops' : 'one-shots'}/${spec.instrument}/${slug}${bpm ? `-${bpm}` : ''}.wav`;
      out.push({
        id: `${p.id}/${slug}`,
        packageId: p.id,
        name,
        file,
        instrument: spec.instrument,
        role: spec.role,
        bpm,
        key: k,
        durationSec,
        contentType: isLoop ? 'loop' : 'one-shot',
        tags: p.tags.slice(0, 2),
        previewUrl: `mock://previews/${p.id}/${slug}.mp3`,
        digest: fakeDigest(`${p.id}/${file}`),
      });
    }
  }
  return out;
}

export const samples: Sample[] = buildSamples();

export function artifactsFor(pkg: Package, version: string): Artifact[] {
  const own = samples.filter((s) => s.packageId === pkg.id);
  const manifest: Artifact = {
    kind: 'manifest',
    path: 'manifest.json',
    digest: fakeDigest(`${pkg.id}@${version}`),
    size: 4_000 + own.length * 380,
    contentType: 'application/json',
  };
  if (pkg.module) {
    return [
      manifest,
      ...pkg.technical.formats.map<Artifact>((f) => ({
        kind: 'binary',
        path: `bin/${pkg.slug}.${f}`,
        digest: fakeDigest(`${pkg.id}@${version}/${f}`),
        size: Math.round(pkg.sizeBytes / Math.max(1, pkg.technical.formats.length)),
        contentType: 'application/octet-stream',
      })),
    ];
  }
  return [
    manifest,
    ...own.map<Artifact>((s) => ({
      kind: 'sample',
      path: s.file,
      digest: s.digest,
      size: Math.round(s.durationSec * 48000 * 2 * 3),
      contentType: 'audio/wav',
    })),
  ];
}

/* ------------------------------------------------------- library / members */

export const initialLibrary: Library = {
  id: 'lib-default',
  name: 'My Library',
  items: [
    { packageId: 'org.acme.deep-house-toolkit', pinnedVersion: null, installedVersion: '1.1.0', addedAt: new Date(NOW - 30 * day).toISOString() },
    { packageId: 'org.lattice.plate-verb', pinnedVersion: '2.3.0', installedVersion: '2.3.0', addedAt: new Date(NOW - 60 * day).toISOString() },
    { packageId: 'org.tapeworks.lofi-beat-template', pinnedVersion: null, installedVersion: '1.0.0', addedAt: new Date(NOW - 10 * day).toISOString() },
    { packageId: 'org.tapeworks.warm-tape-drums', pinnedVersion: '1.0.0', installedVersion: '1.0.0', addedAt: new Date(NOW - 90 * day).toISOString() },
  ],
};

/** The signed-in mock user is a maintainer of Acme Audio. */
export const currentPublisherSlug = 'acme';

export const publisherMembers: PublisherMember[] = [
  { userId: 'u1', name: 'Sam Rivera', email: 'sam@acme.audio', role: 'owner' },
  { userId: 'u2', name: 'Mira Okafor', email: 'mira@acme.audio', role: 'maintainer' },
  { userId: 'u3', name: 'Jun Park', email: 'jun@acme.audio', role: 'uploader' },
];

export { fakeDigest };
