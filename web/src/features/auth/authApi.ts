import { API_BASE, HttpApiError, refreshAccessToken, request, setAccessToken, type SessionResponse } from '@/shared/api/http';
import type { AuthProviders, AuthUser, OAuthProviderId, RegisterInput } from './types';

/** Everything the UI needs for authentication; implemented over HTTP and in-memory (mock mode). */
export interface AuthApi {
  readonly mode: 'http' | 'mock';
  providers(): Promise<AuthProviders>;
  /** Restores a session (refresh cookie) on page load. Resolves to null when signed out. */
  restore(): Promise<AuthUser | null>;
  login(email: string, password: string): Promise<AuthUser>;
  register(input: RegisterInput): Promise<AuthUser>;
  logout(): Promise<void>;
  me(): Promise<AuthUser>;
  /** Starts Google/Discord sign-in. Over HTTP this navigates away and never resolves. */
  startOAuth(provider: OAuthProviderId, returnTo: string): Promise<AuthUser | void>;
  /** Called on /auth/callback after the provider redirected back. */
  completeOAuth(): Promise<AuthUser>;
  verifyEmail(token: string): Promise<void>;
  resendVerification(): Promise<void>;
  forgotPassword(email: string): Promise<void>;
  resetPassword(token: string, password: string): Promise<void>;
}

/* ------------------------------------------------------------------ HTTP */

export const httpAuthApi: AuthApi = {
  mode: 'http',
  providers: () => request<AuthProviders>('/auth/providers'),
  async restore() {
    if (!(await refreshAccessToken())) return null;
    return request<AuthUser>('/me');
  },
  async login(email, password) {
    const s = await request<SessionResponse>('/auth/login', { method: 'POST', body: { email, password }, noRetry: true });
    setAccessToken(s.accessToken);
    return request<AuthUser>('/me');
  },
  async register(input) {
    const s = await request<SessionResponse>('/auth/register', { method: 'POST', body: input, noRetry: true });
    setAccessToken(s.accessToken);
    return request<AuthUser>('/me');
  },
  async logout() {
    try {
      await request<void>('/auth/logout', { method: 'POST', noRetry: true });
    } finally {
      setAccessToken(null);
    }
  },
  me: () => request<AuthUser>('/me'),
  startOAuth(provider, returnTo) {
    window.location.assign(`${API_BASE}/auth/oauth/${provider}/start?returnTo=${encodeURIComponent(returnTo)}`);
    return new Promise<void>(() => undefined);
  },
  async completeOAuth() {
    if (!(await refreshAccessToken())) throw new HttpApiError(401, 'invalid_refresh_token', 'Sign-in did not complete');
    return request<AuthUser>('/me');
  },
  verifyEmail: (token) => request<void>('/auth/email/verify', { method: 'POST', body: { token } }),
  resendVerification: () => request<void>('/auth/email/resend', { method: 'POST' }),
  forgotPassword: (email) => request<void>('/auth/password/forgot', { method: 'POST', body: { email } }),
  resetPassword: (token, password) => request<void>('/auth/password/reset', { method: 'POST', body: { token, password } }),
};
