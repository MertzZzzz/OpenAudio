import { useEffect, useMemo, useState } from 'react';
import { FormProvider, useForm, type Resolver } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { ArrowLeft, ArrowRight, Check, Rocket, Save } from 'lucide-react';
import { api } from '@/shared/api';
import type { ValidationIssue } from '@/shared/types/domain';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { Skeleton } from '@/shared/ui/Skeleton';
import { useToast } from '@/shared/ui/Toast';
import { cn } from '@/shared/lib/cn';
import { compare } from '@/shared/lib/semver';
import { can, type PublisherContext } from '@/features/publisher/currentPublisher';
import { usePublisherPackages } from '@/features/publisher/api';
import { buildManifest } from './buildManifest';
import { defaultValues, STEP_FIELDS, wizardSchema, type WizardValues } from './schema';
import { StepMetadata } from './StepMetadata';
import { StepTaxonomy } from './StepTaxonomy';
import { StepFiles } from './StepFiles';
import { StepValidation } from './StepValidation';
import { StepReview } from './StepReview';

const STEPS = [
  { title: 'Package', description: 'Metadata & license' },
  { title: 'Taxonomy', description: 'Genre, mood, BPM, key' },
  { title: 'Files', description: 'Upload & hash' },
  { title: 'Validation', description: 'Schema & rules' },
  { title: 'Review', description: 'Publish' },
] as const;

function bumpMinor(v: string): string {
  const [a = 0, b = 0] = v.split('.').map(Number);
  return `${a}.${b + 1}.0`;
}

export function UploadWizardView({ ctx }: { ctx: PublisherContext }) {
  const publisher = ctx.publisher.slug;
  const canPublish = can(ctx.publisher.role, 'publish');
  const packagesQ = usePublisherPackages(publisher);
  const [sp] = useSearchParams();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const { toast } = useToast();
  const [step, setStep] = useState(0);
  const [issues, setIssues] = useState<ValidationIssue[] | undefined>();

  const form = useForm<WizardValues>({
    resolver: zodResolver(wizardSchema) as Resolver<WizardValues>,
    defaultValues,
    mode: 'onTouched',
  });
  const packages = packagesQ.data ?? [];
  const values = form.watch();
  const pkg = packages.find((p) => p.slug === values.packageSlug);
  const manifest = useMemo(() => buildManifest(values, pkg), [values, pkg]);

  /** Prefill from the chosen package: kind, taxonomy, next version. */
  const applyPackage = (slug: string) => {
    const p = packages.find((x) => x.slug === slug);
    if (!p) return;
    const latest = [...p.releases.map((r) => r.version), p.latestVersion].sort(compare).pop() ?? '0.0.0';
    const base = { ...form.getValues(), packageSlug: slug, kind: p.kind, version: latest === '0.0.0' ? '1.0.0' : bumpMinor(latest), description: p.description };
    // Packages without a published release carry no metadata yet: keep the wizard defaults.
    if (p.latestVersion === '0.0.0' || !p.license.content) {
      form.reset({ ...defaultValues, ...base });
      return;
    }
    form.reset({
      ...base,
      license: (['CC0-1.0', 'CC-BY-4.0', 'CC-BY-SA-4.0', 'CC-BY-NC-4.0', 'MIT', 'Apache-2.0', 'GPL-3.0-or-later'] as const).find((l) => l === p.license.content) ?? 'CC-BY-4.0',
      commercialUse: p.license.commercialUse,
      attributionRequired: p.license.attributionRequired,
      primaryGenre: p.taxonomy.genres.primary,
      secondaryGenres: p.taxonomy.genres.secondary,
      instruments: p.taxonomy.instruments,
      moods: p.taxonomy.moods,
      useCases: p.taxonomy.useCases,
      energy: p.taxonomy.energy ?? 0.5,
      bpm: p.musical?.bpm?.value,
      bpmMin: p.musical?.bpm?.range?.[0],
      bpmMax: p.musical?.bpm?.range?.[1],
      keyTonic: p.musical?.key?.tonic ?? '',
      keyMode: p.musical?.key?.mode ?? 'minor',
      freeTags: p.tags.join(', '),
      moduleClass: p.module?.moduleClass ?? '',
      accepts: p.module?.capabilities.accepts ?? [],
      produces: p.module?.capabilities.produces ?? [],
      placement: (p.module?.capabilities.placement ?? []).filter((x): x is WizardValues['placement'][number] => x !== undefined),
      categories: p.module?.categories ?? [],
    });
  };

  // Deep link from the dashboard: /publisher/upload?package=slug
  useEffect(() => {
    const slug = sp.get('package');
    if (slug && packages.length && !form.getValues('packageSlug')) applyPackage(slug);
  }, [packages.length]);

  const validate = useMutation({
    mutationFn: () => api.validateManifest(manifest, { publisher, slug: values.packageSlug }),
    onSuccess: setIssues,
  });

  const submit = useMutation({
    mutationFn: async (publish: boolean) => {
      const v = form.getValues();
      const release = await api.createDraftRelease({
        publisher,
        slug: v.packageSlug,
        version: v.version,
        changelog: v.changelog,
        manifest,
        files: v.files.map((f) => ({ path: f.path, size: f.size, contentType: f.contentType })),
      });
      return publish ? api.publishRelease(publisher, v.packageSlug, release.version) : release;
    },
    onSuccess: (r) => {
      void Promise.all(['publisher-packages', 'package', 'versions', 'search', 'discover'].map((k) => qc.invalidateQueries({ queryKey: [k] })));
      toast({ tone: 'success', title: r.status === 'published' ? `Published ${values.packageSlug}@${r.version}` : `Saved draft ${values.packageSlug}@${r.version}` });
      navigate('/publisher');
    },
    onError: (e: Error) => toast({ tone: 'error', title: 'Release failed', description: e.message }),
  });

  const next = async () => {
    const fields = STEP_FIELDS[step] ?? [];
    const ok = fields.length ? await form.trigger(fields, { shouldFocus: true }) : true;
    if (!ok) return;
    if (step === 2) {
      setIssues(undefined);
      validate.mutate();
    }
    setStep((s) => Math.min(s + 1, STEPS.length - 1));
  };

  const back = () => setStep((s) => Math.max(0, s - 1));
  const hasErrors = issues?.some((i) => i.severity === 'error') ?? true;

  if (packagesQ.isLoading) return <Skeleton className="h-[600px]" />;

  return (
    <div className="space-y-6">
      <div>
        <Link to="/publisher" className="text-caption text-fg-subtle hover:text-fg">← Dashboard</Link>
        <h1 className="mt-1 text-heading">Upload a new release</h1>
      </div>

      <ol className="grid grid-cols-5 gap-2" aria-label="Upload steps">
        {STEPS.map((s, i) => {
          const state = i < step ? 'done' : i === step ? 'current' : 'todo';
          return (
            <li key={s.title} aria-current={state === 'current' ? 'step' : undefined}>
              <button
                type="button"
                disabled={i > step}
                onClick={() => i < step && setStep(i)}
                className={cn('flex w-full flex-col gap-2 text-left', i < step && 'cursor-pointer')}
              >
                <span className={cn('h-1 rounded-full', state === 'todo' ? 'bg-surface-raised' : 'bg-primary')} />
                <span className="flex items-center gap-2">
                  <span className={cn('flex h-5 w-5 items-center justify-center rounded-full text-[10px] font-bold', state === 'done' ? 'bg-primary text-primary-fg' : state === 'current' ? 'border border-primary text-primary' : 'border border-border text-fg-subtle')}>
                    {state === 'done' ? <Check className="h-3 w-3" /> : i + 1}
                  </span>
                  <span className={cn('hidden text-body font-medium sm:inline', state === 'todo' && 'text-fg-subtle')}>{s.title}</span>
                </span>
                <span className="hidden text-caption text-fg-subtle md:block">{s.description}</span>
              </button>
            </li>
          );
        })}
      </ol>

      <FormProvider {...form}>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void next();
          }}
          noValidate
        >
          <Card className="p-6">
            <h2 className="mb-5 text-heading">
              Step {step + 1}: {STEPS[step]!.title}
            </h2>
            {step === 0 && <StepMetadata packages={packages} onPackageChange={applyPackage} />}
            {step === 1 && <StepTaxonomy />}
            {step === 2 && <StepFiles />}
            {step === 3 && <StepValidation issues={issues} loading={validate.isPending} />}
            {step === 4 && <StepReview values={values} manifest={manifest} />}
          </Card>

          <div className="mt-4 flex items-center justify-between">
            <Button variant="ghost" leftIcon={<ArrowLeft className="h-4 w-4" />} onClick={back} disabled={step === 0}>
              Back
            </Button>
            {step < 4 ? (
              <Button type="submit" disabled={step === 3 && hasErrors}>
                {step === 3 && hasErrors ? 'Fix errors to continue' : 'Continue'}
                <ArrowRight className="h-4 w-4" />
              </Button>
            ) : (
              <div className="flex gap-2">
                <Button variant="secondary" leftIcon={<Save className="h-4 w-4" />} loading={submit.isPending && submit.variables === false} onClick={() => submit.mutate(false)}>
                  Save draft
                </Button>
                {canPublish ? (
                  <Button leftIcon={<Rocket className="h-4 w-4" />} loading={submit.isPending && submit.variables === true} onClick={() => submit.mutate(true)}>
                    Publish release
                  </Button>
                ) : (
                  <span className="self-center text-caption text-fg-subtle">Uploaders save drafts; a maintainer publishes.</span>
                )}
              </div>
            )}
          </div>
        </form>
      </FormProvider>
    </div>
  );
}
