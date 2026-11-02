import { validateManifest } from './manifest';

const base = {
  schema: 'openaudio.package/v1',
  id: 'org.acme.deep-house-toolkit',
  name: 'Deep House Toolkit',
  kind: 'sample-pack',
  version: '1.2.0',
  description: 'Drums and bass',
  publisher: { id: 'org.acme', name: 'Acme Audio' },
  license: { content: 'CC-BY-4.0', commercialUse: true, attributionRequired: true },
  taxonomy: { genres: { primary: 'deep-house', secondary: ['tech-house'] }, instruments: ['drums'], moods: ['warm'], energy: 0.6 },
  musical: { bpm: { value: 124, range: [122, 126] as [number, number] }, key: { tonic: 'F', mode: 'minor' } },
  artifacts: [{ kind: 'sample', path: 'loops/a.wav', digest: `sha256:${'a'.repeat(64)}`, size: 10, contentType: 'audio/wav' }],
};

describe('validateManifest', () => {
  it('accepts the spec example shape', () => {
    expect(validateManifest(base).filter((i) => i.severity === 'error')).toEqual([]);
  });

  it('rejects bad semver, digests and too many moods', () => {
    const issues = validateManifest({
      ...base,
      version: '1.2',
      taxonomy: { ...base.taxonomy, moods: ['a', 'b', 'c', 'd'] },
      artifacts: [{ ...base.artifacts[0], digest: 'md5:abc' }],
    });
    const paths = issues.map((i) => i.path);
    expect(paths).toEqual(expect.arrayContaining(['version', 'taxonomy.moods', 'artifacts.0.digest']));
  });

  it('requires module for module kinds and forbids it for sample packs', () => {
    expect(validateManifest({ ...base, kind: 'effect' }).some((i) => i.path === 'module')).toBe(true);
    const withModule = { ...base, module: { moduleClass: 'audio-effect', capabilities: { accepts: ['audio'], produces: ['audio'], placement: ['insert'] }, categories: ['reverb'] } };
    expect(validateManifest(withModule).some((i) => i.message.includes('must not define module'))).toBe(true);
  });

  it('applies module class rules after schema validation', () => {
    const effect = { ...base, kind: 'effect', module: { moduleClass: 'audio-effect', capabilities: { accepts: ['midi'], produces: ['audio'], placement: ['insert'] }, categories: ['reverb'] } };
    expect(validateManifest(effect).map((i) => i.message)).toContain('audio-effect must accept audio');
  });

  it('rejects an inverted BPM range', () => {
    const issues = validateManifest({ ...base, musical: { bpm: { value: 124, range: [130, 120] } } });
    expect(issues.some((i) => i.path === 'musical.bpm.range')).toBe(true);
  });
});
