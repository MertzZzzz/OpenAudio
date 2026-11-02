import type { WizardValues } from './schema';
import { formatBytes } from '@/shared/lib/format';
import { termLabel } from '@/shared/lib/taxonomy';

export function StepReview({ values, manifest }: { values: WizardValues; manifest: Record<string, unknown> }) {
  const size = values.files.reduce((n, f) => n + f.size, 0);
  return (
    <div className="grid gap-6 lg:grid-cols-[320px_1fr]">
      <dl className="h-fit space-y-3 rounded-lg border border-border bg-surface p-5 text-body">
        {[
          ['Package', values.packageSlug],
          ['Version', values.version],
          ['License', values.license],
          ['Primary genre', values.primaryGenre ? termLabel('genre', values.primaryGenre) : '—'],
          ['Moods', values.moods.map((m) => termLabel('mood', m)).join(', ') || '—'],
          ['BPM', values.bpm ?? '—'],
          ['Key', values.keyTonic ? `${values.keyTonic} ${values.keyMode}` : '—'],
          ['Files', `${values.files.length} · ${formatBytes(size)}`],
        ].map(([k, v]) => (
          <div key={String(k)} className="flex justify-between gap-4">
            <dt className="text-fg-subtle">{k}</dt>
            <dd className="text-right font-medium">{v}</dd>
          </div>
        ))}
        <p className="border-t border-border pt-3 text-caption text-fg-subtle">Once published this release is immutable. Fixes ship as a new version; problems can be yanked.</p>
      </dl>
      <div className="min-w-0">
        <p className="mb-2 text-caption text-fg-muted">manifest.json</p>
        <pre className="max-h-[480px] overflow-auto rounded-lg border border-border bg-surface-sunken p-4 font-mono text-caption leading-relaxed text-fg-muted scrollbar-thin">
          {JSON.stringify(manifest, null, 2)}
        </pre>
      </div>
    </div>
  );
}
