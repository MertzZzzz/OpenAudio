import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router-dom';
import type { z } from 'zod';
import { Button } from '@/shared/ui/Button';
import { Field, Input } from '@/shared/ui/Input';
import { useAuth } from '../AuthContext';
import { authErrorMessage } from '../messages';
import { loginSchema, safeReturnTo } from '../forms';
import { OAuthButtons } from '../OAuthButtons';
import { Alert, AuthCard, Divider } from './AuthCard';

type Values = z.infer<typeof loginSchema>;

export function LoginView() {
  const { login, status, api } = useAuth();
  const [sp] = useSearchParams();
  const navigate = useNavigate();
  const returnTo = safeReturnTo(sp.get('returnTo'));
  const [error, setError] = useState<string | null>(null);
  const form = useForm<Values>({ resolver: zodResolver(loginSchema), defaultValues: { email: '', password: '' } });

  if (status === 'authenticated') return <Navigate to={returnTo} replace />;

  const submit = form.handleSubmit(async (v) => {
    setError(null);
    try {
      await login(v.email, v.password);
      navigate(returnTo, { replace: true });
    } catch (e) {
      setError(authErrorMessage(e));
    }
  });

  const e = form.formState.errors;
  return (
    <AuthCard
      title="Sign in to OpenAudio"
      subtitle="Publish packages, keep a library and pin exact versions."
      footer={
        <>
          New here?{' '}
          <Link to={`/register?returnTo=${encodeURIComponent(returnTo)}`} className="font-medium text-primary hover:underline">
            Create an account
          </Link>
        </>
      }
    >
      {error && <Alert tone="error">{error}</Alert>}
      <OAuthButtons returnTo={returnTo} onError={(err) => setError(authErrorMessage(err))} />
      <Divider label="or with email" />
      <form onSubmit={submit} className="space-y-4" noValidate>
        <Field label="Email" error={e.email?.message}>
          {(p) => <Input {...p} type="email" autoComplete="email" {...form.register('email')} />}
        </Field>
        <Field label="Password" error={e.password?.message}>
          {(p) => <Input {...p} type="password" autoComplete="current-password" {...form.register('password')} />}
        </Field>
        <div className="flex justify-end">
          <Link to="/forgot-password" className="text-caption text-primary hover:underline">
            Forgot password?
          </Link>
        </div>
        <Button type="submit" size="lg" className="w-full" loading={form.formState.isSubmitting}>
          Sign in
        </Button>
      </form>
      {api.mode === 'mock' && (
        <p className="rounded border border-dashed border-border p-3 text-caption text-fg-subtle">
          Mock mode (no backend): sign in as <span className="font-mono text-fg-muted">demo@openaudio.dev</span> /{' '}
          <span className="font-mono text-fg-muted">openaudio-demo</span> for publisher access, or use Google/Discord.
        </p>
      )}
    </AuthCard>
  );
}
