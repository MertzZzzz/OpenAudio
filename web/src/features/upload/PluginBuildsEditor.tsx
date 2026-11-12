import { useFormContext } from 'react-hook-form';
import { Plus, Puzzle, Trash2 } from 'lucide-react';
import { Badge } from '@/shared/ui/Badge';
import { Button } from '@/shared/ui/Button';
import { Checkbox } from '@/shared/ui/Checkbox';
import { Select } from '@/shared/ui/Select';
import { PLUGIN_ARCH_LABEL, PLUGIN_FORMAT_LABEL, PLUGIN_OS_LABEL } from '@/shared/lib/plugins';
import { PLUGIN_ARCHES, PLUGIN_OSES, type PluginBuild } from '@/shared/types/domain';
import { findBundles } from './detectPlugins';
import { PLUGIN_KINDS, type WizardValues } from './schema';

/**
 * One row per VST3/CLAP build. Rows are proposed from the uploaded bundles (layout + executable
 * headers); the publisher confirms OS/architectures and may add plug-in ids.
 */
export function PluginBuildsEditor() {
  const { watch, setValue, formState } = useFormContext<WizardValues>();
  const plugins = watch('plugins');
  const files = watch('files');
  const kind = watch('kind');
  const bundles = findBundles(files.map((f) => f.path));
  if (!bundles.length) return null;

  const update = (i: number, patch: Partial<PluginBuild>) =>
    setValue('plugins', plugins.map((p, j) => (j === i ? { ...p, ...patch, auto: false } : p)), { shouldDirty: true, shouldValidate: true });
  // Removing a build counts as an edit of its bundle, so detection does not bring it back.
  const remove = (i: number) =>
    setValue('plugins', plugins.filter((_, j) => j !== i).map((p) => (p.bundle === plugins[i]!.bundle ? { ...p, auto: false } : p)), { shouldDirty: true, shouldValidate: true });
  const add = (bundle: string) => {
    const b = bundles.find((x) => x.bundle === bundle)!;
    const used = new Set(plugins.filter((p) => p.bundle === bundle).map((p) => p.os));
    const os = PLUGIN_OSES.find((o) => !used.has(o)) ?? 'windows';
    setValue('plugins', [...plugins.map((p) => (p.bundle === bundle ? { ...p, auto: false } : p)), { format: b.format, bundle, os, arch: ['x86_64'], auto: false }], {
      shouldDirty: true,
      shouldValidate: true,
    });
  };
  const error = formState.errors.plugins?.message ?? formState.errors.plugins?.root?.message;

  return (
    <section aria-labelledby="plugin-builds-heading" className="space-y-3 rounded-lg border border-border p-4">
      <div className="flex items-start gap-3">
        <Puzzle className="mt-0.5 h-5 w-5 shrink-0 text-primary" aria-hidden />
        <div>
          <h3 id="plugin-builds-heading" className="font-semibold">Plug-in builds</h3>
          <p className="text-caption text-fg-muted">
            Detected from the uploaded bundles. Check the platform of each build — listeners pick the one for their system and download it as a zip.
          </p>
        </div>
      </div>
      {!PLUGIN_KINDS.includes(kind) && (
        <p className="text-caption text-warning" role="status">
          This package is a <b>{kind}</b>. VST3/CLAP builds are published with instrument, effect, MIDI tool or utility packages — change the kind in step 1.
        </p>
      )}
      {error && <p className="text-caption text-danger" role="alert">{error}</p>}

      <ul className="space-y-2" aria-label="Plug-in builds">
        {plugins.map((p, i) => (
          <li key={`${p.bundle}:${i}`} className="grid gap-3 rounded-md bg-surface-raised/50 p-3 md:grid-cols-[minmax(0,1fr)_9rem_minmax(0,1.2fr)_auto] md:items-center">
            <div className="min-w-0">
              <p className="flex items-center gap-2">
                <Badge tone="primary">{PLUGIN_FORMAT_LABEL[p.format]}</Badge>
                <span className="truncate font-mono text-caption" title={p.bundle}>{p.bundle}</span>
              </p>
              <input
                aria-label={`Plug-in ids for ${p.bundle} (${PLUGIN_OS_LABEL[p.os]})`}
                placeholder={p.format === 'clap' ? 'CLAP id, e.g. com.vendor.plugin (optional)' : 'VST3 class id, 32 hex (optional)'}
                className="mt-2 h-8 w-full rounded border border-border bg-surface-sunken px-2 font-mono text-caption"
                value={(p.pluginIds ?? []).join(', ')}
                onChange={(e) => update(i, { pluginIds: e.target.value.split(',').map((s) => s.trim()) })}
              />
            </div>
            <Select
              aria-label={`Operating system for ${p.bundle}`}
              value={p.os}
              options={PLUGIN_OSES.map((o) => ({ value: o, label: PLUGIN_OS_LABEL[o] }))}
              onChange={(e) => update(i, { os: e.target.value as PluginBuild['os'] })}
            />
            <fieldset className="flex flex-wrap gap-x-2" aria-label={`Architectures for ${p.bundle} (${PLUGIN_OS_LABEL[p.os]})`}>
              {PLUGIN_ARCHES.filter((a) => a !== 'arm64ec' || p.os === 'windows').map((a) => (
                <Checkbox
                  key={a}
                  label={PLUGIN_ARCH_LABEL[a]}
                  checked={p.arch.includes(a)}
                  onChange={(e) => update(i, { arch: e.target.checked ? [...p.arch, a] : p.arch.filter((x) => x !== a) })}
                />
              ))}
            </fieldset>
            <button type="button" aria-label={`Remove build ${p.bundle} (${PLUGIN_OS_LABEL[p.os]})`} onClick={() => remove(i)} className="justify-self-end rounded p-1 text-fg-subtle hover:text-danger">
              <Trash2 className="h-4 w-4" />
            </button>
          </li>
        ))}
      </ul>

      <div className="flex flex-wrap gap-2">
        {bundles.map((b) => (
          <Button key={b.bundle} type="button" variant="ghost" size="sm" leftIcon={<Plus className="h-4 w-4" />} onClick={() => add(b.bundle)}>
            Add platform to {b.bundle.split('/').pop()}
          </Button>
        ))}
      </div>
    </section>
  );
}
