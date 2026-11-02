import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Link, useSearchParams } from 'react-router-dom';
import type { z } from 'zod';
import { Button, ButtonLink } from '@/shared/ui/Button';
import { Field, Input } from '@/shared/ui/Input';
import { useAuth } from '../AuthContext';
import { authErrorMessage } from '../messages';
import { applyServerIssues, forgotSchema, resetSchema } from '../forms';
import { Alert, AuthCard } from './AuthCard';

export function ForgotPasswordView() {
  const { api } = useAuth();
  const [sent, setSent] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const form = useForm<z.infer<typeof forgotSchema>>({ resolver: zodResolver(forgotSchema), defaultValues: { email: '' } });

  const submit = form.handleSubmit(async ({ email }) => {
    setError(null);
    try {
      await api.forgotPassword(email);
      setSent(email);
    } catch (e) {
      setError(authErrorMessage(e));
    }
  });

  return (
    <AuthCard title="Reset your password" subtitle="Also works for accounts created with Google or Discord — it adds a password." footer={<Link to="/login" className="text-primary hover:underline">Back to sign in</Link>}>
      {error && <Alert tone="error">{error}</Alert>}
      {sent ? (
        <Alert tone="success">If an account exists for {sent}, a reset link is on its way. It expires in one hour.</Alert>
      ) : (
        <form onSubmit={submit} className="space-y-4" noValidate>
          <Field label="Email" error={form.formState.errors.email?.message}>
            {(p) => <Input {...p} type="email" autoComplete="email" {...form.register('email')} />}
          </Field>
          <Button type="submit" size="lg" className="w-full" loading={form.formState.isSubmitting}>
            Send reset link
          </Button>
        </form>
      )}
    </AuthCard>
  );
}

export function ResetPasswordView() {
  const { api } = useAuth();
  const [sp] = useSearchParams();
  const token = sp.get('token') ?? '';
  const [done, setDone] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const form = useForm<z.infer<typeof resetSchema>>({ resolver: zodResolver(resetSchema), defaultValues: { password: '', confirm: '' } });

  const submit = form.handleSubmit(async ({ password }) => {
    setError(null);
    try {
      await api.resetPassword(token, password);
      setDone(true);
    } catch (e) {
      if (!applyServerIssues(e, form.setError, ['password'])) setError(authErrorMessage(e));
    }
  });

  if (!token) {
    return (
      <AuthCard title="Reset your password">
        <Alert tone="error">This link is incomplete. <Link to="/forgot-password" className="underline">Request a new one</Link>.</Alert>
      </AuthCard>
    );
  }

  return (
    <AuthCard title="Choose a new password" subtitle="All your other sessions will be signed out.">
      {error && <Alert tone="error">{error}</Alert>}
      {done ? (
        <>
          <Alert tone="success">Password updated.</Alert>
          <ButtonLink to="/login" size="lg" className="w-full">Sign in</ButtonLink>
        </>
      ) : (
        <form onSubmit={submit} className="space-y-4" noValidate>
          <Field label="New password" hint="At least 10 characters." error={form.formState.errors.password?.message}>
            {(p) => <Input {...p} type="password" autoComplete="new-password" {...form.register('password')} />}
          </Field>
          <Field label="Confirm password" error={form.formState.errors.confirm?.message}>
            {(p) => <Input {...p} type="password" autoComplete="new-password" {...form.register('confirm')} />}
          </Field>
          <Button type="submit" size="lg" className="w-full" loading={form.formState.isSubmitting}>
            Set password
          </Button>
        </form>
      )}
    </AuthCard>
  );
}
