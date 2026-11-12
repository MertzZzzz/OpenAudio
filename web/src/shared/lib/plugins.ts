import type { PluginArch, PluginBuild, PluginFormat, PluginOs, ValidationIssue } from '@/shared/types/domain';

export const PLUGIN_FORMAT_LABEL: Record<PluginFormat, string> = { vst3: 'VST3', clap: 'CLAP' };
export const PLUGIN_OS_LABEL: Record<PluginOs, string> = { windows: 'Windows', macos: 'macOS', linux: 'Linux' };
export const PLUGIN_ARCH_LABEL: Record<PluginArch, string> = { x86_64: 'x64', arm64: 'ARM64', arm64ec: 'ARM64EC', i386: 'x86 (32-bit)' };

/** Does the build list offer a format/OS combination the search asked for? (mirrors the server's rule) */
export function buildsMatch(builds: PluginBuild[], formats?: string[], oses?: string[]): boolean {
  if (!formats?.length && !oses?.length) return true;
  return builds.some((b) => (!formats?.length || formats.includes(b.format)) && (!oses?.length || oses.includes(b.os)));
}

export function describeBuild(b: PluginBuild): string {
  return `${PLUGIN_FORMAT_LABEL[b.format]} · ${PLUGIN_OS_LABEL[b.os]} · ${b.arch.map((a) => PLUGIN_ARCH_LABEL[a]).join(' + ')}`;
}

/* --------------------------------------------------------- validation */

export const PLUGIN_KINDS = ['instrument', 'effect', 'midi-tool', 'utility'] as const;

/** VST3 bundle sub-folders per OS and architecture (VST3 SDK "Plug-in Format Structure"). */
export const VST3_ARCH_DIRS: Partial<Record<PluginOs, Partial<Record<PluginArch, string>>>> = {
  windows: { x86_64: 'x86_64-win', arm64: 'arm64-win', arm64ec: 'arm64ec-win', i386: 'x86-win' },
  linux: { x86_64: 'x86_64-linux', arm64: 'aarch64-linux', i386: 'i386-linux' },
};

export function bundleFiles(paths: string[], bundle: string): { files: string[]; folder: boolean } {
  const files = paths.filter((p) => p === bundle || p.startsWith(`${bundle}/`)).sort();
  return { files, folder: files.some((f) => f !== bundle) };
}

/** Client mirror of api/internal/manifest/plugins.go (used by the mock API and for instant feedback). */
export function validatePlugins(kind: string, artifacts: { path: string; kind: string }[], plugins: PluginBuild[]): ValidationIssue[] {
  if (!plugins.length) return [];
  const err = (path: string, message: string): ValidationIssue => ({ severity: 'error', path, message });
  const warn = (path: string, message: string): ValidationIssue => ({ severity: 'warning', path, message });
  if (!(PLUGIN_KINDS as readonly string[]).includes(kind)) {
    return [err('plugins', `plug-in builds are only allowed for ${PLUGIN_KINDS.join(', ')} packages`)];
  }
  const issues: ValidationIssue[] = [];
  const paths = artifacts.map((a) => a.path);
  const kinds = new Map(artifacts.map((a) => [a.path, a.kind]));
  const taken = new Map<string, number>();
  plugins.forEach((p, i) => {
    const at = (f: string) => `plugins.${i}.${f}`;
    if (!p.bundle.toLowerCase().endsWith(`.${p.format}`)) issues.push(err(at('bundle'), `a ${p.format} bundle must end in .${p.format}`));
    const { files, folder } = bundleFiles(paths, p.bundle);
    if (!files.length) {
      issues.push(err(at('bundle'), `"${p.bundle}" is not in artifacts: upload the whole folder (or file)`));
      return;
    }
    if (files.some((f) => kinds.get(f) !== 'binary')) issues.push(err(at('bundle'), 'files of a plug-in bundle must have artifact kind "binary"'));
    const under = (sub: string) => files.some((f) => f.startsWith(`${p.bundle}/${sub}`));
    if (p.os === 'macos') {
      if (!folder) issues.push(err(at('bundle'), `on macOS a ${PLUGIN_FORMAT_LABEL[p.format]} plug-in is a bundle folder; upload the folder, not a single file`));
      else {
        if (!under('Contents/MacOS/')) issues.push(err(at('bundle'), `${p.bundle}/Contents/MacOS/ is missing: the bundle has no executable`));
        if (!under('Contents/Info.plist')) issues.push(warn(at('bundle'), `${p.bundle}/Contents/Info.plist is missing; hosts may not load the bundle`));
      }
    } else if (p.format === 'clap') {
      if (folder) issues.push(err(at('bundle'), `on ${PLUGIN_OS_LABEL[p.os]} a CLAP plug-in is a single .clap file, not a folder`));
    } else if (!folder) {
      issues.push(p.os === 'linux' ? err(at('bundle'), 'on Linux a VST3 plug-in is a bundle folder') : warn(at('bundle'), 'single-file VST3 is the legacy layout; current hosts expect a bundle folder'));
    } else {
      for (const a of p.arch) {
        const dir = VST3_ARCH_DIRS[p.os]?.[a];
        if (!dir) issues.push(err(at('arch'), `${a} is not a VST3 architecture on ${PLUGIN_OS_LABEL[p.os]}`));
        else if (!under(`Contents/${dir}/`)) issues.push(err(at('arch'), `${a} declared but ${p.bundle}/Contents/${dir}/ is missing`));
      }
    }
    for (const a of p.arch) {
      const key = `${p.format}:${p.os}:${a}`;
      const j = taken.get(key);
      if (j !== undefined) issues.push(err(at('arch'), `${PLUGIN_FORMAT_LABEL[p.format]} for ${PLUGIN_OS_LABEL[p.os]}/${a} is already provided by plugins.${j}`));
      taken.set(key, i);
    }
  });
  return issues;
}
