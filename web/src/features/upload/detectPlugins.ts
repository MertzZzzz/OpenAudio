import type { PluginArch, PluginBuild, PluginFormat, PluginOs } from '@/shared/types/domain';
import { VST3_ARCH_DIRS } from '@/shared/lib/plugins';

/** What the first bytes of an executable say about its platform. */
export interface BinaryInfo {
  os: PluginOs;
  arch: PluginArch[];
}

export interface DetectedBundle {
  /** Path of the .vst3/.clap folder or file, e.g. "mac/Tape.vst3". */
  bundle: string;
  format: PluginFormat;
  folder: boolean;
}

const BUNDLE_RE = /\.(vst3|clap)$/i;

/** The bundle a path belongs to: the first path segment ending in .vst3/.clap. */
export function bundleRoot(path: string): DetectedBundle | null {
  const segs = path.split('/');
  const i = segs.findIndex((s) => BUNDLE_RE.test(s));
  if (i < 0) return null;
  return {
    bundle: segs.slice(0, i + 1).join('/'),
    format: segs[i]!.slice(segs[i]!.lastIndexOf('.') + 1).toLowerCase() as PluginFormat,
    folder: i < segs.length - 1,
  };
}

export function findBundles(paths: string[]): DetectedBundle[] {
  const out = new Map<string, DetectedBundle>();
  for (const p of paths) {
    const b = bundleRoot(p);
    if (!b) continue;
    const prev = out.get(b.bundle);
    out.set(b.bundle, prev ? { ...prev, folder: prev.folder || b.folder } : b);
  }
  return [...out.values()].sort((a, b) => a.bundle.localeCompare(b.bundle));
}

/** Files whose headers are worth reading: plug-in executables inside (or as) a bundle. */
export function isExecutableCandidate(path: string): boolean {
  const b = bundleRoot(path);
  if (!b) return false;
  if (!b.folder) return true;
  const rest = path.slice(b.bundle.length + 1);
  return /^Contents\/MacOS\/[^/]+$/.test(rest) || /^Contents\/[^/]+-(win|linux)\/[^/]+$/.test(rest);
}

const uniq = <T,>(xs: T[]) => [...new Set(xs)];

/**
 * Proposes plugins[] entries for the uploaded files. Folder layout decides the OS for bundles
 * (Contents/MacOS → macOS, Contents/<arch>-win → Windows, Contents/<arch>-linux → Linux; one VST3
 * bundle may hold several); single files are identified by their executable header.
 */
/** A proposed build; `auto` stays true until the publisher edits it. */
export type DetectedBuild = PluginBuild & { auto?: boolean };

export function detectPlugins(files: { path: string; binary?: BinaryInfo | null }[]): DetectedBuild[] {
  const builds: DetectedBuild[] = [];
  for (const b of findBundles(files.map((f) => f.path))) {
    const inside = files.filter((f) => f.path === b.bundle || f.path.startsWith(`${b.bundle}/`));
    if (!b.folder) {
      const info = inside[0]?.binary;
      builds.push({ format: b.format, bundle: b.bundle, os: info?.os ?? (b.format === 'vst3' ? 'windows' : 'linux'), arch: info?.arch.length ? info.arch : ['x86_64'], auto: true });
      continue;
    }
    const rel = (f: { path: string }) => f.path.slice(b.bundle.length + 1);
    const mac = inside.filter((f) => rel(f).startsWith('Contents/MacOS/'));
    if (mac.length) {
      const arch = uniq(mac.flatMap((f) => (f.binary?.os === 'macos' ? f.binary.arch : [])));
      builds.push({ format: b.format, bundle: b.bundle, os: 'macos', arch: arch.length ? sortArch(arch) : ['x86_64', 'arm64'], auto: true });
    }
    for (const os of ['windows', 'linux'] as const) {
      const dirs = VST3_ARCH_DIRS[os]!;
      const arch = (Object.keys(dirs) as PluginArch[]).filter((a) => inside.some((f) => rel(f).startsWith(`Contents/${dirs[a]}/`)));
      if (arch.length) builds.push({ format: b.format, bundle: b.bundle, os, arch: sortArch(arch), auto: true });
    }
    if (!builds.some((x) => x.bundle === b.bundle)) {
      // Unknown layout: propose something editable; validation explains what is missing.
      builds.push({ format: b.format, bundle: b.bundle, os: 'macos', arch: ['x86_64', 'arm64'], auto: true });
    }
  }
  return builds;
}

const ARCH_ORDER: PluginArch[] = ['x86_64', 'arm64', 'arm64ec', 'i386'];
const sortArch = (a: PluginArch[]) => [...a].sort((x, y) => ARCH_ORDER.indexOf(x) - ARCH_ORDER.indexOf(y));

/**
 * Reconciles proposals with the publisher's edits: once any build of a bundle was edited, that
 * bundle's builds are left alone; untouched (auto) builds follow detection, which improves as
 * executable headers are read. Builds of bundles no longer uploaded are dropped.
 */
export function mergePlugins(current: DetectedBuild[], detected: DetectedBuild[], paths: string[]): DetectedBuild[] {
  const roots = new Set(findBundles(paths).map((b) => b.bundle));
  const edited = new Set(current.filter((p) => !p.auto && roots.has(p.bundle)).map((p) => p.bundle));
  const out: DetectedBuild[] = [];
  const seen = new Set<string>();
  for (const p of current) {
    if (!roots.has(p.bundle)) continue;
    if (edited.has(p.bundle)) out.push(p);
    else if (!seen.has(p.bundle)) {
      seen.add(p.bundle);
      out.push(...detected.filter((d) => d.bundle === p.bundle));
    }
  }
  const known = new Set(out.map((p) => p.bundle));
  return [...out, ...detected.filter((d) => !known.has(d.bundle) && !edited.has(d.bundle))];
}

/* ------------------------------------------------------- header sniffing */

const PE_MACHINE: Record<number, PluginArch> = { 0x8664: 'x86_64', 0xaa64: 'arm64', 0xa641: 'arm64ec', 0x014c: 'i386' };
const ELF_MACHINE: Record<number, PluginArch> = { 62: 'x86_64', 183: 'arm64', 3: 'i386' };
const MACHO_CPU: Record<number, PluginArch> = { 0x01000007: 'x86_64', 0x0100000c: 'arm64', 7: 'i386' };

/** Reads the executable header: PE (Windows), ELF (Linux), Mach-O or universal binary (macOS). */
export function sniffHeader(buf: ArrayBuffer): BinaryInfo | null {
  const v = new DataView(buf);
  if (v.byteLength < 8) return null;
  const b0 = v.getUint8(0);
  const b1 = v.getUint8(1);
  if (b0 === 0x4d && b1 === 0x5a && v.byteLength >= 0x40) {
    const pe = v.getUint32(0x3c, true);
    if (pe + 6 > v.byteLength || v.getUint32(pe, false) !== 0x50450000) return null;
    const arch = PE_MACHINE[v.getUint16(pe + 4, true)];
    return { os: 'windows', arch: arch ? [arch] : [] };
  }
  if (v.getUint32(0, false) === 0x7f454c46 && v.byteLength >= 20) {
    const little = v.getUint8(5) === 1;
    const arch = ELF_MACHINE[v.getUint16(18, little)];
    return { os: 'linux', arch: arch ? [arch] : [] };
  }
  const magicLE = v.getUint32(0, true);
  if (magicLE === 0xfeedfacf || magicLE === 0xfeedface) {
    const arch = MACHO_CPU[v.getUint32(4, true)];
    return { os: 'macos', arch: arch ? [arch] : [] };
  }
  if (v.getUint32(0, false) === 0xcafebabe) {
    const n = v.getUint32(4, false);
    if (n === 0 || n > 16) return null; // Java class files share the magic; real fat headers list a few archs
    const arch: PluginArch[] = [];
    for (let i = 0; i < n && 8 + i * 20 + 4 <= v.byteLength; i += 1) {
      const a = MACHO_CPU[v.getUint32(8 + i * 20, false)];
      if (a && !arch.includes(a)) arch.push(a);
    }
    return { os: 'macos', arch: sortArch(arch) };
  }
  return null;
}

export async function sniffFile(file: Blob): Promise<BinaryInfo | null> {
  const head = await file.slice(0, 4096).arrayBuffer();
  return sniffHeader(head);
}
