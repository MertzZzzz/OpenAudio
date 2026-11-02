import type { Module } from '@/shared/types/domain';
import { Badge } from '@/shared/ui/Badge';
import { MODULE_CLASS_META } from '@/shared/lib/kinds';
import { humanize } from '@/shared/lib/format';

/** Visual summary of a module's ports and placement. */
export function ModuleDiagram({ module }: { module: Module }) {
  const meta = MODULE_CLASS_META[module.moduleClass];
  const ins = module.ports.filter((p) => p.direction === 'in');
  const outs = module.ports.filter((p) => p.direction === 'out');
  const Port = ({ signal, channels }: { signal: string; channels?: number }) => (
    <span className="inline-flex items-center gap-1.5 rounded border border-border bg-surface-sunken px-2 py-1 font-mono text-caption">
      <span className={signal === 'audio' ? 'h-2 w-2 rounded-full bg-accent' : signal === 'midi' ? 'h-2 w-2 rounded-full bg-warning' : 'h-2 w-2 rounded-full bg-fg-subtle'} aria-hidden />
      {signal}
      {channels ? ` ×${channels}` : ''}
    </span>
  );
  return (
    <div className="rounded-lg border border-border bg-surface p-5">
      <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-4">
        <div className="flex flex-col items-end gap-2">
          <span className="text-caption uppercase tracking-wide text-fg-subtle">Inputs</span>
          {ins.length ? ins.map((p) => <Port key={p.id} signal={p.signal} channels={p.channels} />) : <span className="text-caption text-fg-subtle">none</span>}
        </div>
        <div className="flex flex-col items-center gap-1 rounded-lg border border-primary/40 bg-primary/10 px-6 py-4 text-center">
          <meta.icon className="h-6 w-6 text-primary" aria-hidden />
          <span className="font-semibold">{meta.label}</span>
          <span className="font-mono text-caption text-fg-muted">{meta.io}</span>
        </div>
        <div className="flex flex-col items-start gap-2">
          <span className="text-caption uppercase tracking-wide text-fg-subtle">Outputs</span>
          {outs.length ? outs.map((p) => <Port key={p.id} signal={p.signal} channels={p.channels} />) : <span className="text-caption text-fg-subtle">none</span>}
        </div>
      </div>
      <dl className="mt-5 grid grid-cols-1 gap-3 border-t border-border pt-4 text-body sm:grid-cols-3">
        <div>
          <dt className="text-caption text-fg-subtle">Placement</dt>
          <dd className="mt-1 flex flex-wrap gap-1">{module.capabilities.placement.map((p) => <Badge key={p}>{humanize(p)}</Badge>)}</dd>
        </div>
        <div>
          <dt className="text-caption text-fg-subtle">Categories</dt>
          <dd className="mt-1 flex flex-wrap gap-1">{module.categories.map((c) => <Badge key={c} tone="primary">{humanize(c)}</Badge>)}</dd>
        </div>
        <div>
          <dt className="text-caption text-fg-subtle">Behaviour</dt>
          <dd className="mt-1 flex flex-wrap gap-1">
            {module.capabilities.bypassable && <Badge>Bypassable</Badge>}
            {module.capabilities.wetDry && <Badge>Wet/Dry</Badge>}
            {module.capabilities.stateful && <Badge>Stateful</Badge>}
          </dd>
        </div>
      </dl>
    </div>
  );
}
