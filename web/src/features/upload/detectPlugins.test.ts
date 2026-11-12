import { describe, expect, it } from 'vitest';
import { bundleRoot, detectPlugins, findBundles, isExecutableCandidate, mergePlugins, sniffHeader } from './detectPlugins';
import { buildManifest } from './buildManifest';
import { defaultValues, type WizardValues } from './schema';

function bytes(len: number, set: (v: DataView) => void): ArrayBuffer {
  const buf = new ArrayBuffer(len);
  set(new DataView(buf));
  return buf;
}

describe('bundle detection', () => {
  it('finds bundle roots at any depth', () => {
    expect(bundleRoot('mac/Tape FX.vst3/Contents/MacOS/Tape FX')).toEqual({ bundle: 'mac/Tape FX.vst3', format: 'vst3', folder: true });
    expect(bundleRoot('linux/tape.CLAP')).toEqual({ bundle: 'linux/tape.CLAP', format: 'clap', folder: false });
    expect(bundleRoot('loops/kick.wav')).toBeNull();
    expect(findBundles(['a/X.vst3/Contents/Info.plist', 'a/X.vst3/Contents/MacOS/X', 'b.clap', 'c.wav']).map((b) => b.bundle)).toEqual(['a/X.vst3', 'b.clap']);
  });

  it('only sniffs plug-in executables', () => {
    expect(isExecutableCandidate('X.vst3/Contents/MacOS/X')).toBe(true);
    expect(isExecutableCandidate('X.vst3/Contents/x86_64-win/X.vst3')).toBe(true);
    expect(isExecutableCandidate('X.vst3/Contents/Resources/logo.png')).toBe(false);
    expect(isExecutableCandidate('x.clap')).toBe(true);
    expect(isExecutableCandidate('kick.wav')).toBe(false);
  });

  it('proposes builds from layout and headers', () => {
    const builds = detectPlugins([
      // multi-platform VST3 bundle
      { path: 'Tape.vst3/Contents/Info.plist' },
      { path: 'Tape.vst3/Contents/MacOS/Tape', binary: { os: 'macos', arch: ['x86_64', 'arm64'] } },
      { path: 'Tape.vst3/Contents/x86_64-win/Tape.vst3' },
      { path: 'Tape.vst3/Contents/arm64-win/Tape.vst3' },
      { path: 'Tape.vst3/Contents/x86_64-linux/Tape.so' },
      // single-file CLAP identified by its ELF header
      { path: 'linux/tape.clap', binary: { os: 'linux', arch: ['arm64'] } },
      { path: 'demo.wav' },
    ]);
    expect(builds).toEqual([
      { format: 'clap', bundle: 'linux/tape.clap', os: 'linux', arch: ['arm64'], auto: true },
      { format: 'vst3', bundle: 'Tape.vst3', os: 'macos', arch: ['x86_64', 'arm64'], auto: true },
      { format: 'vst3', bundle: 'Tape.vst3', os: 'windows', arch: ['x86_64', 'arm64'], auto: true },
      { format: 'vst3', bundle: 'Tape.vst3', os: 'linux', arch: ['x86_64'], auto: true },
    ]);
  });

  it('follows detection for untouched builds, keeps edited bundles, drops removed bundles', () => {
    const auto = { format: 'clap' as const, bundle: 'a.clap', os: 'linux' as const, arch: ['x86_64' as const], auto: true };
    const better = { ...auto, arch: ['arm64' as const] };
    const other = { format: 'clap' as const, bundle: 'b.clap', os: 'linux' as const, arch: ['x86_64' as const], auto: true };
    expect(mergePlugins([auto], [better, other], ['a.clap', 'b.clap'])).toEqual([better, other]);

    const edited = { ...auto, os: 'windows' as const, arch: ['i386' as const], auto: false };
    expect(mergePlugins([edited], [better, other], ['a.clap', 'b.clap'])).toEqual([edited, other]);
    expect(mergePlugins([edited, other], [], ['b.clap'])).toEqual([]);
  });
});

describe('sniffHeader', () => {
  it('reads PE, ELF, Mach-O and universal binaries', () => {
    const pe = bytes(0x100, (v) => {
      v.setUint16(0, 0x5a4d, true); // "MZ"
      v.setUint32(0x3c, 0x80, true);
      v.setUint32(0x80, 0x50450000, false); // "PE\0\0"
      v.setUint16(0x84, 0xaa64, true);
    });
    expect(sniffHeader(pe)).toEqual({ os: 'windows', arch: ['arm64'] });

    const elf = bytes(64, (v) => {
      v.setUint32(0, 0x7f454c46, false);
      v.setUint8(5, 1);
      v.setUint16(18, 62, true);
    });
    expect(sniffHeader(elf)).toEqual({ os: 'linux', arch: ['x86_64'] });

    const macho = bytes(32, (v) => {
      v.setUint32(0, 0xfeedfacf, true);
      v.setUint32(4, 0x0100000c, true);
    });
    expect(sniffHeader(macho)).toEqual({ os: 'macos', arch: ['arm64'] });

    const fat = bytes(64, (v) => {
      v.setUint32(0, 0xcafebabe, false);
      v.setUint32(4, 2, false);
      v.setUint32(8, 0x0100000c, false);
      v.setUint32(28, 0x01000007, false);
    });
    expect(sniffHeader(fat)).toEqual({ os: 'macos', arch: ['x86_64', 'arm64'] });

    expect(sniffHeader(bytes(16, (v) => v.setUint32(0, 0x52494646, false)))).toBeNull(); // "RIFF" (a WAV)
  });
});

describe('buildManifest with plug-ins', () => {
  it('marks bundle files as binaries and lists plug-in formats', () => {
    const file = (path: string, contentType = 'application/octet-stream') => ({
      id: path, path, size: 1, contentType, digest: `sha256:${'a'.repeat(64)}`, status: 'done' as const, progress: 1,
    });
    const v: WizardValues = {
      ...defaultValues,
      kind: 'effect',
      moduleClass: 'audio-effect',
      files: [file('Tape.vst3/Contents/Resources/click.wav', 'audio/wav'), file('Tape.vst3/Contents/MacOS/Tape'), file('demo.wav', 'audio/wav')],
      plugins: [{ format: 'vst3', bundle: 'Tape.vst3', os: 'macos', arch: ['arm64'], pluginIds: ['', ' '] }],
    };
    const m = buildManifest(v, undefined) as { artifacts: { path: string; kind: string }[]; technical: { formats: string[] }; plugins: unknown[] };
    expect(m.artifacts.map((a) => a.kind)).toEqual(['binary', 'binary', 'sample']);
    expect(m.technical.formats).toEqual(['vst3', 'wav']);
    expect(m.plugins).toEqual([{ format: 'vst3', bundle: 'Tape.vst3', os: 'macos', arch: ['arm64'] }]);
  });
});
