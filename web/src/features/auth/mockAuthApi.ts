import { HttpApiError } from '@/shared/api/http';
import type { AuthApi } from './authApi';
import type { AuthUser, OAuthProviderId } from './types';

/**
 * In-memory auth used when no backend is running (`npm run dev`, tests).
 * Mirrors the API's rules closely enough to exercise every screen:
 * email/password with validation, Google/Discord sign-in, verification and reset links.
 */
interface MockAccount {
  user: AuthUser;
  password: string | null;
}

export function createMockAuthApi({ latencyMs = 200 }: { latencyMs?: number } = {}): AuthApi & { lastEmailLink: () => string | null } {
  const accounts = new Map<string, MockAccount>();
  let current: string | null = null;
  let lastLink: string | null = null;
  const tokens = new Map<string, { email: string; purpose: 'verify' | 'reset' }>();

  const wait = <T>(v: T) => new Promise<T>((r) => setTimeout(() => r(structuredClone(v)), latencyMs));
  const fail = (status: number, code: string, message: string, issues: HttpApiError['issues'] = []) =>
    new Promise<never>((_, rej) => setTimeout(() => rej(new HttpApiError(status, code, message, issues)), latencyMs));

  const seed = (email: string, displayName: string, role: AuthUser['role'], password: string | null) => {
    accounts.set(email, {
      password,
      user: { id: crypto.randomUUID(), email, displayName, role, avatarUrl: null, emailVerified: true, hasPassword: password !== null, identities: [], createdAt: new Date().toISOString() },
    });
  };
  // Demo publisher so the dashboard is reachable without a backend.
  seed('demo@openaudio.dev', 'Sam Rivera', 'publisher', 'openaudio-demo');

  const issueLink = (email: string, purpose: 'verify' | 'reset') => {
    const token = crypto.randomUUID();
    tokens.set(token, { email, purpose });
    lastLink = `/${purpose === 'verify' ? 'verify-email' : 'reset-password'}?token=${token}`;
    console.info(`[mock auth] email to ${email}: ${lastLink}`);
  };

  const me = () => {
    const acc = current ? accounts.get(current) : undefined;
    return acc ? wait(acc.user) : fail(401, 'unauthenticated', 'sign in required');
  };

  return {
    mode: 'mock',
    lastEmailLink: () => lastLink,
    providers: () => wait({ email: true, oauth: [{ id: 'google', startUrl: '#' }, { id: 'discord', startUrl: '#' }] }),
    restore: () => (current ? me() : wait(null)),
    login(email, password) {
      const acc = accounts.get(email.trim().toLowerCase());
      if (!acc || acc.password !== password) return fail(401, 'invalid_credentials', 'email or password is incorrect');
      current = acc.user.email;
      return wait(acc.user);
    },
    register({ email, password, displayName }) {
      const e = email.trim().toLowerCase();
      if (accounts.has(e)) return fail(409, 'email_taken', 'an account with this email already exists');
      if (password.length < 10) return fail(422, 'validation_failed', 'request validation failed', [{ severity: 'error', path: 'password', message: 'password must be at least 10 characters' }]);
      seed(e, displayName.trim(), 'user', password);
      accounts.get(e)!.user.emailVerified = false;
      current = e;
      issueLink(e, 'verify');
      return wait(accounts.get(e)!.user);
    },
    logout() {
      current = null;
      return wait(undefined);
    },
    me,
    startOAuth(provider: OAuthProviderId) {
      const email = `${provider}.user@example.com`;
      if (!accounts.has(email)) {
        seed(email, provider === 'google' ? 'Google User' : 'Discord User', 'user', null);
      }
      const acc = accounts.get(email)!;
      if (!acc.user.identities?.some((i) => i.provider === provider)) {
        acc.user.identities = [...(acc.user.identities ?? []), { provider, email, linkedAt: new Date().toISOString() }];
      }
      current = email;
      return wait(acc.user);
    },
    completeOAuth: () => me(),
    verifyEmail(token) {
      const t = tokens.get(token);
      if (!t || t.purpose !== 'verify') return fail(401, 'token_invalid', 'this link is invalid or has expired');
      tokens.delete(token);
      accounts.get(t.email)!.user.emailVerified = true;
      return wait(undefined);
    },
    resendVerification() {
      if (!current) return fail(401, 'unauthenticated', 'sign in required');
      issueLink(current, 'verify');
      return wait(undefined);
    },
    forgotPassword(email) {
      const e = email.trim().toLowerCase();
      if (accounts.has(e)) issueLink(e, 'reset');
      return wait(undefined);
    },
    resetPassword(token, password) {
      const t = tokens.get(token);
      if (!t || t.purpose !== 'reset') return fail(401, 'token_invalid', 'this link is invalid or has expired');
      tokens.delete(token);
      const acc = accounts.get(t.email)!;
      acc.password = password;
      acc.user.hasPassword = true;
      acc.user.emailVerified = true;
      if (current === t.email) current = null;
      return wait(undefined);
    },
  };
}
