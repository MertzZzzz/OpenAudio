import { useEffect, useState, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Building2 } from 'lucide-react';
import { api } from '@/shared/api';
import type { PublisherMembership } from '@/shared/types/domain';
import { Button, ButtonLink } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { Field, Input } from '@/shared/ui/Input';
import { Select } from '@/shared/ui/Select';
import { Skeleton } from '@/shared/ui/Skeleton';
import { EmptyState } from '@/shared/ui/EmptyState';
import { useToast } from '@/shared/ui/Toast';
import { ApiError } from '@/shared/api/client';
import { HttpApiError } from '@/shared/api/http';
import { useAuth } from '@/features/auth/AuthContext';
import { slugify } from './CreatePackageModal';

const STORAGE_KEY = 'openaudio.currentPublisher';

export function useMyPublishers() {
  const { user } = useAuth();
  return useQuery({ queryKey: ['my-publishers', user?.id], queryFn: () => api.myPublishers(), enabled: Boolean(user) });
}

/** Who may do what inside a publisher (mirrors the server's permission matrix). */
export function can(role: PublisherMembership['role'], action: 'draft' | 'publish' | 'packages' | 'members'): boolean {
  if (role === 'owner') return true;
  if (role === 'maintainer') return action !== 'members';
  return action === 'draft';
}

const schema = z.object({
  name: z.string().trim().min(2, 'At least 2 characters').max(80),
  slug: z
    .string()
    .trim()
    .min(2, 'At least 2 characters')
    .max(40)
    .regex(/^[a-z0-9]+(-[a-z0-9]+)*$/, 'Lowercase letters, digits and single dashes'),
});

function CreatePublisherCard({ first }: { first: boolean }) {
  const qc = useQueryClient();
  const { api: authApi, reload } = useAuth();
  const { toast } = useToast();
  const [error, setError] = useState<ReactNode>(null);
  const form = useForm<z.infer<typeof schema>>({ resolver: zodResolver(schema), defaultValues: { name: '', slug: '' } });
  const name = form.watch('name');
  useEffect(() => {
    if (!form.formState.dirtyFields.slug) form.setValue('slug', slugify(name).slice(0, 40));
  }, [name, form]);

  const create = useMutation({
    mutationFn: (v: z.infer<typeof schema>) => api.createPublisher(v.slug, v.name),
    onSuccess: async (p) => {
      try {
        localStorage.setItem(STORAGE_KEY, p.slug);
      } catch {
        /* ignore */
      }
      // The server promoted the account to "publisher": refresh the session so the token carries it.
      await authApi.restore().catch(() => undefined);
      await reload().catch(() => undefined);
      await qc.invalidateQueries({ queryKey: ['my-publishers'] });
      toast({ tone: 'success', title: `Publisher @${p.slug} created` });
    },
    onError: (e: Error) => {
      if (e instanceof HttpApiError && e.issues.length) {
        for (const i of e.issues) if (i.path === 'slug' || i.path === 'name') form.setError(i.path, { message: i.message });
        return;
      }
      if (e instanceof ApiError && e.code === 'email_unverified') {
        setError(
          <>
            Confirm your email address first. <ButtonLink to="/account" variant="ghost" size="sm">Resend confirmation</ButtonLink>
          </>,
        );
        return;
      }
      setError(e.message);
    },
  });

  const e = form.formState.errors;
  return (
    <Card className="mx-auto max-w-lg p-8">
      <div className="mb-6 flex items-center gap-3">
        <div className="rounded-lg bg-primary/15 p-2.5 text-primary">
          <Building2 className="h-5 w-5" aria-hidden />
        </div>
        <div>
          <h1 className="text-heading">{first ? 'Become a publisher' : 'New publisher'}</h1>
          <p className="text-body text-fg-muted">A publisher owns packages. You can invite collaborators later.</p>
        </div>
      </div>
      {error && <p role="alert" className="mb-4 rounded border border-danger/40 bg-danger/10 px-3 py-2 text-body text-danger">{error}</p>}
      <form onSubmit={form.handleSubmit((v) => create.mutate(v))} className="space-y-4" noValidate>
        <Field label="Display name" error={e.name?.message}>
          {(p) => <Input {...p} {...form.register('name')} placeholder="Acme Audio" />}
        </Field>
        <Field label="Handle" hint={`Package ids look like ${form.watch('slug') || 'acme'}.my-package`} error={e.slug?.message}>
          {(p) => <Input {...p} {...form.register('slug')} className="font-mono" placeholder="acme" />}
        </Field>
        <Button type="submit" size="lg" className="w-full" loading={create.isPending}>
          Create publisher
        </Button>
      </form>
    </Card>
  );
}

export interface PublisherContext {
  publisher: PublisherMembership;
  all: PublisherMembership[];
  select: (slug: string) => void;
}

/**
 * Resolves the publisher the user is working on (remembered per browser). Shows onboarding when
 * the user has none yet.
 */
export function PublisherGate({ children }: { children: (ctx: PublisherContext) => ReactNode }) {
  const q = useMyPublishers();
  const [selected, setSelected] = useState<string | null>(() => {
    try {
      return localStorage.getItem(STORAGE_KEY);
    } catch {
      return null;
    }
  });

  if (q.isLoading) return <Skeleton className="h-[480px]" />;
  if (q.isError) {
    return <EmptyState title="Could not load your publishers" description={q.error.message} action={<Button onClick={() => void q.refetch()}>Retry</Button>} />;
  }
  const all = q.data ?? [];
  if (all.length === 0) return <CreatePublisherCard first />;
  const publisher = all.find((p) => p.slug === selected) ?? all[0]!;
  const select = (slug: string) => {
    setSelected(slug);
    try {
      localStorage.setItem(STORAGE_KEY, slug);
    } catch {
      /* ignore */
    }
  };
  return <>{children({ publisher, all, select })}</>;
}

export function PublisherSwitcher({ ctx }: { ctx: PublisherContext }) {
  if (ctx.all.length < 2) return null;
  return (
    <div className="w-48">
      <Select
        aria-label="Publisher"
        value={ctx.publisher.slug}
        onChange={(e) => ctx.select(e.target.value)}
        options={ctx.all.map((p) => ({ value: p.slug, label: `@${p.slug} (${p.role})` }))}
      />
    </div>
  );
}

export { CreatePublisherCard };
