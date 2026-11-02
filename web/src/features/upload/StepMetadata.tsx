import { Controller, useFormContext } from 'react-hook-form';
import type { Package } from '@/shared/types/domain';
import { Field, Input, Textarea } from '@/shared/ui/Input';
import { Select } from '@/shared/ui/Select';
import { Checkbox } from '@/shared/ui/Checkbox';
import { KIND_META } from '@/shared/lib/kinds';
import { LICENSES, type WizardValues } from './schema';

export function StepMetadata({ packages, onPackageChange }: { packages: (Package & { releases: { version: string }[] })[]; onPackageChange: (slug: string) => void }) {
  const { register, formState, watch, control } = useFormContext<WizardValues>();
  const e = formState.errors;
  const slug = watch('packageSlug');
  const pkg = packages.find((p) => p.slug === slug);
  const existing = pkg?.releases.map((r) => r.version) ?? [];

  return (
    <div className="grid gap-5 md:grid-cols-2">
      <Field label="Package" error={e.packageSlug?.message} hint="Create new packages from the dashboard.">
        {(p) => (
          <Select
            {...p}
            {...register('packageSlug', { onChange: (ev: React.ChangeEvent<HTMLSelectElement>) => onPackageChange(ev.target.value) })}
            placeholder="Select a package…"
            options={packages.map((x) => ({ value: x.slug, label: `${x.name} (${KIND_META[x.kind].label})` }))}
          />
        )}
      </Field>
      <Field label="Version" error={e.version?.message} hint={existing.length ? `Existing: ${existing.slice(0, 4).join(', ')}${existing.length > 4 ? '…' : ''}` : 'Semantic version'}>
        {(p) => <Input {...p} {...register('version')} className="font-mono" placeholder="1.0.0" />}
      </Field>
      <Field label="Kind" hint="Inherited from the package." className="md:col-span-1">
        {(p) => <Input {...p} value={KIND_META[watch('kind')].label} readOnly disabled />}
      </Field>
      <Field label="License (SPDX)" error={e.license?.message}>
        {(p) => <Select {...p} {...register('license')} options={LICENSES.map((l) => ({ value: l, label: l }))} />}
      </Field>
      <div className="flex gap-6 md:col-span-2">
        <Controller control={control} name="commercialUse" render={({ field }) => <Checkbox label="Commercial use allowed" checked={field.value} onChange={(ev) => field.onChange(ev.target.checked)} />} />
        <Controller control={control} name="attributionRequired" render={({ field }) => <Checkbox label="Attribution required" checked={field.value} onChange={(ev) => field.onChange(ev.target.checked)} />} />
      </div>
      <Field label="Description" error={e.description?.message} className="md:col-span-2">
        {(p) => <Textarea {...p} {...register('description')} placeholder="Shown on the package page." />}
      </Field>
      <Field label="Changelog" error={e.changelog?.message} className="md:col-span-2">
        {(p) => <Textarea {...p} {...register('changelog')} placeholder="What changed in this version?" className="min-h-[72px]" />}
      </Field>
    </div>
  );
}
