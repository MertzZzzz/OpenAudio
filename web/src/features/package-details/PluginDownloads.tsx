import { Download, Puzzle } from 'lucide-react';
import { api } from '@/shared/api';
import { Badge } from '@/shared/ui/Badge';
import { buttonClasses } from '@/shared/ui/Button';
import { Table, TBody, TD, TH, THead, TR } from '@/shared/ui/Table';
import { PLUGIN_ARCH_LABEL, PLUGIN_FORMAT_LABEL, PLUGIN_OS_LABEL } from '@/shared/lib/plugins';
import type { Package, PluginBuild, PluginOs } from '@/shared/types/domain';
import { useManifest } from './api';

/** Best guess of the visitor's OS, to highlight the matching builds. */
export function currentOs(ua: string = typeof navigator === 'undefined' ? '' : navigator.userAgent): PluginOs | null {
  if (/Windows/i.test(ua)) return 'windows';
  if (/Mac OS X|Macintosh/i.test(ua) && !/iPhone|iPad/i.test(ua)) return 'macos';
  if (/Linux|X11/i.test(ua) && !/Android/i.test(ua)) return 'linux';
  return null;
}

/** VST3/CLAP builds of the viewed version, each downloadable as one zip. */
export function PluginDownloads({ pkg, version }: { pkg: Package; version: string }) {
  const manifestQ = useManifest(pkg.publisher.slug, pkg.slug, version === pkg.latestVersion ? undefined : version);
  const builds: PluginBuild[] = version === pkg.latestVersion ? pkg.plugins : ((manifestQ.data?.plugins as PluginBuild[] | undefined) ?? []);
  if (!builds.length) return null;
  const mine = currentOs();
  const sorted = [...builds].sort((a, b) => Number(b.os === mine) - Number(a.os === mine));

  return (
    <section aria-labelledby="plugin-downloads-heading" className="space-y-3">
      <h3 id="plugin-downloads-heading" className="flex items-center gap-2 font-semibold">
        <Puzzle className="h-4 w-4 text-primary" aria-hidden />
        Plug-in downloads
      </h3>
      <Table>
        <THead>
          <tr>
            <TH>Format</TH>
            <TH>Platform</TH>
            <TH>Architectures</TH>
            <TH>Bundle</TH>
            <TH className="w-36"><span className="sr-only">Download</span></TH>
          </tr>
        </THead>
        <TBody>
          {sorted.map((b) => {
            const url = api.bundleDownloadUrl(pkg.publisher.slug, pkg.slug, version, b.bundle);
            const label = `${PLUGIN_FORMAT_LABEL[b.format]} for ${PLUGIN_OS_LABEL[b.os]}`;
            return (
              <TR key={`${b.bundle}:${b.os}`}>
                <TD><Badge tone="primary">{PLUGIN_FORMAT_LABEL[b.format]}</Badge></TD>
                <TD>
                  {PLUGIN_OS_LABEL[b.os]}
                  {b.os === mine && <Badge tone="success" className="ml-2">your system</Badge>}
                </TD>
                <TD className="text-fg-muted">{b.arch.map((a) => PLUGIN_ARCH_LABEL[a]).join(', ')}</TD>
                <TD className="font-mono text-caption text-fg-subtle">{b.bundle.split('/').pop()}</TD>
                <TD className="text-right">
                  {url ? (
                    <a href={url} download aria-label={`Download ${label}`} className={buttonClasses({ variant: b.os === mine ? 'primary' : 'secondary', size: 'sm' })}>
                      <Download className="h-4 w-4" aria-hidden /> .zip
                    </a>
                  ) : (
                    <span className="text-caption text-fg-subtle" title="Bundle downloads need the real API (VITE_API_MODE=http)">demo data</span>
                  )}
                </TD>
              </TR>
            );
          })}
        </TBody>
      </Table>
      <p className="text-caption text-fg-subtle">
        Unzip into your plug-in folder (VST3: <code className="font-mono">Common Files/VST3</code>, <code className="font-mono">~/Library/Audio/Plug-Ins/VST3</code>, <code className="font-mono">~/.vst3</code>; CLAP: the matching <code className="font-mono">CLAP</code> folder). OpenAudio distributes plug-ins; your DAW runs them.
      </p>
    </section>
  );
}
