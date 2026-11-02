import { Controller, useFormContext } from 'react-hook-form';
import { KEY_TONICS, MODULE_CLASSES } from '@/shared/types/domain';
import { Field, Input } from '@/shared/ui/Input';
import { Select } from '@/shared/ui/Select';
import { termsOf } from '@/shared/lib/taxonomy';
import { MODULE_CLASS_META } from '@/shared/lib/kinds';
import { humanize } from '@/shared/lib/format';
import { TermPicker, type TermOption } from './TermPicker';
import { MODULE_KINDS, PLACEMENTS, SIGNALS, type WizardValues } from './schema';

const depthOf = (slug: string, domain: 'genre' | 'instrument'): number => {
  const t = termsOf(domain).find((x) => x.slug === slug);
  return t?.parent ? 1 + depthOf(t.parent, domain) : 0;
};
const opts = (domain: 'genre' | 'instrument' | 'mood' | 'use-case' | 'module-category'): TermOption[] =>
  termsOf(domain).map((t) => ({ value: t.slug, label: t.label, depth: domain === 'genre' || domain === 'instrument' ? depthOf(t.slug, domain) : 0 }));

export function StepTaxonomy() {
  const { register, control, formState, watch } = useFormContext<WizardValues>();
  const e = formState.errors;
  const kind = watch('kind');
  const isModule = MODULE_KINDS.includes(kind);
  const primary = watch('primaryGenre');
  const energy = watch('energy');

  return (
    <div className="space-y-8">
      <section className="space-y-5" aria-labelledby="tax-h">
        <h3 id="tax-h" className="font-semibold">Taxonomy</h3>
        <Field label="Primary genre" error={e.primaryGenre?.message} hint="One canonical slug; children inherit from parents in search.">
          {(p) => <Select {...p} {...register('primaryGenre')} placeholder="Select…" options={opts('genre').map((o) => ({ value: o.value, label: `${'— '.repeat(o.depth ?? 0)}${o.label}` }))} />}
        </Field>
        <Controller
          control={control}
          name="secondaryGenres"
          render={({ field }) => (
            <TermPicker label="Secondary genres" max={3} options={opts('genre').filter((o) => o.value !== primary)} value={field.value} onChange={field.onChange} error={e.secondaryGenres?.message} hint="Up to 3." />
          )}
        />
        <Controller control={control} name="instruments" render={({ field }) => <TermPicker label="Instruments" options={opts('instrument')} value={field.value} onChange={field.onChange} hint="Primary instrument first; dashed chips are sub-categories." />} />
        <Controller control={control} name="moods" render={({ field }) => <TermPicker label="Moods" max={3} options={opts('mood')} value={field.value} onChange={field.onChange} error={e.moods?.message} hint="1–3 moods." />} />
        <Controller control={control} name="useCases" render={({ field }) => <TermPicker label="Use cases" options={opts('use-case')} value={field.value} onChange={field.onChange} />} />
        <div className="max-w-sm">
          <label htmlFor="energy" className="flex justify-between text-caption font-medium text-fg-muted">
            Energy <span className="font-mono">{energy.toFixed(2)}</span>
          </label>
          <input id="energy" type="range" min={0} max={1} step={0.01} {...register('energy', { valueAsNumber: true })} className="mt-2 w-full accent-[rgb(var(--oa-primary))]" />
        </div>
        <Field label="Free tags" hint="Comma-separated; only for what the vocabulary doesn’t cover (e.g. tape, dusty).">
          {(p) => <Input {...p} {...register('freeTags')} placeholder="analog, tape, rolling" />}
        </Field>
      </section>

      <section className="space-y-5" aria-labelledby="mus-h">
        <h3 id="mus-h" className="font-semibold">Musical metadata</h3>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
          <Field label="BPM" error={e.bpm?.message}>{(p) => <Input {...p} type="number" {...register('bpm')} placeholder="124" />}</Field>
          <Field label="BPM range min" error={e.bpmMin?.message}>{(p) => <Input {...p} type="number" {...register('bpmMin')} placeholder="122" />}</Field>
          <Field label="BPM range max" error={e.bpmMax?.message}>{(p) => <Input {...p} type="number" {...register('bpmMax')} placeholder="126" />}</Field>
          <Field label="Key">
            {(p) => (
              <div className="flex gap-2">
                <Select {...p} {...register('keyTonic')} placeholder="—" options={KEY_TONICS.map((t) => ({ value: t, label: t }))} />
                <Select aria-label="Mode" {...register('keyMode')} options={[{ value: 'minor', label: 'minor' }, { value: 'major', label: 'major' }]} />
              </div>
            )}
          </Field>
          <Field label="Time signature" error={e.timeSignature?.message}>{(p) => <Input {...p} {...register('timeSignature')} placeholder="4/4" />}</Field>
        </div>
      </section>

      {isModule && (
        <section className="space-y-5" aria-labelledby="mod-h">
          <h3 id="mod-h" className="font-semibold">Module definition</h3>
          <Field label="Module class" error={e.moduleClass?.message}>
            {(p) => (
              <Select {...p} {...register('moduleClass')} placeholder="Select…" options={MODULE_CLASSES.map((m) => ({ value: m, label: `${MODULE_CLASS_META[m].label} — ${MODULE_CLASS_META[m].io}` }))} />
            )}
          </Field>
          <div className="grid gap-5 md:grid-cols-2">
            <Controller control={control} name="accepts" render={({ field }) => <TermPicker label="Accepts" options={SIGNALS.map((s) => ({ value: s, label: s }))} value={field.value} onChange={(v) => field.onChange(v)} />} />
            <Controller control={control} name="produces" render={({ field }) => <TermPicker label="Produces" options={SIGNALS.map((s) => ({ value: s, label: s }))} value={field.value} onChange={(v) => field.onChange(v)} />} />
          </div>
          <Controller control={control} name="placement" render={({ field }) => <TermPicker label="Placement" options={PLACEMENTS.map((s) => ({ value: s, label: humanize(s) }))} value={field.value} onChange={(v) => field.onChange(v)} />} />
          <Controller control={control} name="categories" render={({ field }) => <TermPicker label="Categories" options={opts('module-category')} value={field.value} onChange={field.onChange} />} />
        </section>
      )}
    </div>
  );
}
