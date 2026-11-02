import { z } from 'zod';
import type { FieldValues, Path, UseFormSetError } from 'react-hook-form';
import { HttpApiError } from '@/shared/api/http';

// Mirrors the server rules in api/internal/usecase/auth (10–128 chars, 1–64 char name).
const email = z.string().trim().min(1, 'Email is required').email('Enter a valid email address');
const password = z.string().min(10, 'At least 10 characters').max(128, 'At most 128 characters');

export const loginSchema = z.object({ email, password: z.string().min(1, 'Password is required') });

export const registerSchema = z
  .object({
    displayName: z.string().trim().min(1, 'Display name is required').max(64, 'At most 64 characters'),
    email,
    password,
    confirm: z.string(),
  })
  .refine((v) => v.password === v.confirm, { path: ['confirm'], message: 'Passwords do not match' });

export const forgotSchema = z.object({ email });

export const resetSchema = z
  .object({ password, confirm: z.string() })
  .refine((v) => v.password === v.confirm, { path: ['confirm'], message: 'Passwords do not match' });

/** Copies server-side field errors (422 issues) onto the form. Returns true if any were applied. */
export function applyServerIssues<T extends FieldValues>(err: unknown, setError: UseFormSetError<T>, fields: readonly Path<T>[]): boolean {
  if (!(err instanceof HttpApiError) || err.issues.length === 0) return false;
  let applied = false;
  for (const issue of err.issues) {
    if ((fields as readonly string[]).includes(issue.path)) {
      setError(issue.path as Path<T>, { type: 'server', message: issue.message });
      applied = true;
    }
  }
  return applied;
}

/** Only same-site relative paths are allowed as post-login destinations. */
export function safeReturnTo(raw: string | null): string {
  if (!raw || !raw.startsWith('/') || raw.startsWith('//') || raw.startsWith('/\\')) return '/';
  return raw;
}
