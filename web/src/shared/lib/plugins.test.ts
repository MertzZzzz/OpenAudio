import { describe, expect, it } from 'vitest';
import { buildsMatch, validatePlugins } from './plugins';
import type { PluginBuild } from '@/shared/types/domain';

const bin = (path: string) => ({ path, kind: 'binary' });

describe('validatePlugins (client mirror of the server rules)', () => {
  const mac: PluginBuild = { format: 'vst3', bundle: 'Tape.vst3', os: 'macos', arch: ['arm64'] };

  it('accepts a proper macOS bundle', () => {
    expect(validatePlugins('effect', [bin('Tape.vst3/Contents/MacOS/Tape'), bin('Tape.vst3/Contents/Info.plist')], [mac])).toEqual([]);
  });

  it('explains what is wrong', () => {
    const msgs = (kind: string, arts: { path: string; kind: string }[], builds: PluginBuild[]) => validatePlugins(kind, arts, builds).map((i) => `${i.severity} ${i.path}: ${i.message}`);
    expect(msgs('sample-pack', [], [mac])[0]).toMatch(/only allowed/);
    expect(msgs('effect', [], [mac])[0]).toMatch(/not in artifacts/);
    expect(msgs('effect', [bin('Tape.vst3')], [mac])[0]).toMatch(/bundle folder/);
    expect(msgs('effect', [bin('Tape.vst3/Contents/Info.plist')], [mac])[0]).toMatch(/MacOS\/ is missing/);
    expect(msgs('effect', [bin('t.clap/x.so')], [{ format: 'clap', bundle: 't.clap', os: 'linux', arch: ['x86_64'] }])[0]).toMatch(/single \.clap file/);
    expect(msgs('effect', [bin('T.vst3/Contents/x86_64-win/T.vst3')], [{ format: 'vst3', bundle: 'T.vst3', os: 'windows', arch: ['x86_64', 'arm64'] }])[0]).toMatch(/arm64-win\/ is missing/);
  });
});

describe('buildsMatch', () => {
  const builds: PluginBuild[] = [
    { format: 'vst3', bundle: 'a.vst3', os: 'macos', arch: ['arm64'] },
    { format: 'clap', bundle: 'a.clap', os: 'linux', arch: ['x86_64'] },
  ];
  it('requires one build to satisfy both format and OS', () => {
    expect(buildsMatch(builds, ['vst3'], ['macos'])).toBe(true);
    expect(buildsMatch(builds, ['clap'], ['macos'])).toBe(false);
    expect(buildsMatch(builds, undefined, ['linux'])).toBe(true);
    expect(buildsMatch([], undefined, undefined)).toBe(true);
    expect(buildsMatch([], ['vst3'])).toBe(false);
  });
});
