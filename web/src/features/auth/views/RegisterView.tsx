import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router-dom';
import type { z } from 'zod';
import { Button } from '@/shared/ui/Button';
import { Field, Input } from '@/shared/ui/Input';
import { useAuth } from '../AuthContext';
import { authErrorMessage } from '../messages';
import { applyServerIssues, registerSchema, safeReturnTo } from '../forms';
import { OAuthButtons } from '../OAuthButtons';
import { Alert, AuthCard, Divider } from './AuthCard';

type Values = z.infer<typeof registerSchema>;

export function RegisterView() {
  const { register: signUp, status } = useAuth();
  const [sp] = useSearchParams();
  const navigate = useNavigate();
  const returnTo = safeReturnTo(sp.get('returnTo'));
  const [error, setError] = useState<string | null>(null);
  const form = useForm<Values>({ resolver: zodResolver(registerSchema), defaultValues: { displayName: '', email: '', password: '', confirm: '' } });

  if (status === 'authenticated' && !form.formState.isSubmitSuccessful) return <Navigate to={returnTo} replace />;

  const submit = form.handleSubmit(async (v) => {
    setError(null);
    try {
      await signUp({ displayName: v.displayName, email: v.email, password: v.password });
      navigate(`/account?welcome=1`, { replace: true });
    } catch (e) {
      if (!applyServerIssues(e, form.setError, ['displayName', 'email', 'password'])) setError(authErrorMessage(e));
    }
  });

  const e = form.formState.errors;
  return (
    <AuthCard
      title="Create your account"
      subtitle="Free and open. We'll send a link to confirm your email."
      footer={
        <>
          Already have an account?{' '}
          <Link to={`/login?returnTo=${encodeURIComponent(returnTo)}`} className="font-medium text-primary hover:underline">
            Sign in
          </Link>
        </>
      }
    >
      {error && <Alert tone="error">{error}</Alert>}
      <OAuthButtons returnTo={returnTo} onError={(err) => setError(authErrorMessage(err))} />
      <Divider label="or with email" />
      <form onSubmit={submit} className="space-y-4" noValidate>
        <Field label="Display name" error={e.displayName?.message}>
          {(p) => <Input {...p} autoComplete="nickname" {...form.register('displayName')} />}
        </Field>
        <Field label="Email" error={e.email?.message}>
          {(p) => <Input {...p} type="email" autoComplete="email" {...form.register('email')} />}
        </Field>
        <Field label="Password" hint="At least 10 characters." error={e.password?.message}>
          {(p) => <Input {...p} type="password" autoComplete="new-password" {...form.register('password')} />}
        </Field>
        <Field label="Confirm password" error={e.confirm?.message}>
          {(p) => <Input {...p} type="password" autoComplete="new-password" {...form.register('confirm')} />}
        </Field>
        <Button type="submit" size="lg" className="w-full" loading={form.formState.isSubmitting}>
          Create account
        </Button>
      </form>
    </AuthCard>
  );
}
