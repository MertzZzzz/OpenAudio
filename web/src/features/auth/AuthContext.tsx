import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { setSessionExpiredHandler } from '@/shared/api/http';
import { httpAuthApi, type AuthApi } from './authApi';
import { createMockAuthApi } from './mockAuthApi';
import { apiMode } from '@/shared/lib/apiMode';
import type { AuthUser, OAuthProviderId, RegisterInput } from './types';

type Status = 'loading' | 'authenticated' | 'anonymous';

export interface AuthContextValue {
  status: Status;
  user: AuthUser | null;
  /** True after an explicit sign-out (guards send the user home instead of to /login). */
  signedOut: boolean;
  api: AuthApi;
  login: (email: string, password: string) => Promise<AuthUser>;
  register: (input: RegisterInput) => Promise<AuthUser>;
  logout: () => Promise<void>;
  startOAuth: (provider: OAuthProviderId, returnTo: string) => Promise<void>;
  completeOAuth: () => Promise<AuthUser>;
  reload: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

/** `VITE_API_MODE=http` talks to the Go API; anything else uses the in-memory mock. */
export function defaultAuthApi(): AuthApi {
  return apiMode === 'http' ? httpAuthApi : createMockAuthApi({ latencyMs: import.meta.env.MODE === 'test' ? 0 : 200 });
}

export function AuthProvider({ children, api }: { children: ReactNode; api?: AuthApi }) {
  const [authApi] = useState<AuthApi>(() => api ?? defaultAuthApi());
  const [user, setUser] = useState<AuthUser | null>(null);
  const [status, setStatus] = useState<Status>('loading');
  const [signedOut, setSignedOut] = useState(false);

  const qc = useQueryClient();
  const lastUserId = useRef<string | null | undefined>(undefined);
  const apply = useCallback((u: AuthUser | null) => {
    if (u) setSignedOut(false);
    // Per-user data (library, lockfiles, publishers) must not leak between accounts.
    const id = u?.id ?? null;
    if (lastUserId.current !== undefined && lastUserId.current !== id) qc.removeQueries();
    lastUserId.current = id;
    setUser(u);
    setStatus(u ? 'authenticated' : 'anonymous');
  }, [qc]);

  useEffect(() => {
    let cancelled = false;
    authApi
      .restore()
      .then((u) => !cancelled && apply(u))
      .catch(() => !cancelled && apply(null));
    setSessionExpiredHandler(() => apply(null));
    return () => {
      cancelled = true;
      setSessionExpiredHandler(null);
    };
  }, [authApi, apply]);

  const value = useMemo<AuthContextValue>(
    () => ({
      status,
      user,
      signedOut,
      api: authApi,
      async login(email, password) {
        const u = await authApi.login(email, password);
        apply(u);
        return u;
      },
      async register(input) {
        const u = await authApi.register(input);
        apply(u);
        return u;
      },
      async logout() {
        await authApi.logout().catch(() => undefined);
        setSignedOut(true);
        apply(null);
      },
      async startOAuth(provider, returnTo) {
        const u = await authApi.startOAuth(provider, returnTo);
        if (u) apply(u);
      },
      async completeOAuth() {
        const u = await authApi.completeOAuth();
        apply(u);
        return u;
      },
      async reload() {
        apply(await authApi.me());
      },
    }),
    [status, user, signedOut, authApi, apply],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within <AuthProvider>');
  return ctx;
}
