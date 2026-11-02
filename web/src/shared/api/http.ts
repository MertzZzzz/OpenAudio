import { ApiError } from './client';

/**
 * Minimal fetch wrapper for the real API.
 * - Base path `/api/v1` (the Vite dev server proxies it to the Go API, so cookies are first-party).
 * - The access token lives in memory only; the refresh token is an HttpOnly cookie the JS never sees.
 * - On 401 it refreshes once (single-flight) and retries the request.
 */
export const API_BASE = '/api/v1';

let accessToken: string | null = null;
let refreshInFlight: Promise<boolean> | null = null;
let onSessionExpired: (() => void) | null = null;

export function setAccessToken(token: string | null): void {
  accessToken = token;
}

export function getAccessToken(): string | null {
  return accessToken;
}

export function setSessionExpiredHandler(fn: (() => void) | null): void {
  onSessionExpired = fn;
}

export interface ApiIssue {
  severity: 'error' | 'warning' | 'info';
  path: string;
  message: string;
}

export class HttpApiError extends ApiError {
  constructor(
    status: number,
    code: string,
    message: string,
    public readonly issues: ApiIssue[] = [],
  ) {
    super(status, code, message);
    this.name = 'HttpApiError';
  }
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  body?: unknown;
  /** Binary request body (sent as application/octet-stream), e.g. an upload part. */
  rawBody?: Blob;
  /** Skip the automatic refresh-and-retry (used by the auth endpoints themselves). */
  noRetry?: boolean;
}

async function parseError(res: Response): Promise<HttpApiError> {
  try {
    const data = (await res.json()) as { code?: string; message?: string; issues?: ApiIssue[] };
    return new HttpApiError(res.status, data.code ?? 'http_error', data.message ?? res.statusText, data.issues ?? []);
  } catch {
    return new HttpApiError(res.status, 'http_error', res.statusText || 'Request failed');
  }
}

async function send(path: string, opts: RequestOptions): Promise<Response> {
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (opts.body !== undefined) headers['Content-Type'] = 'application/json';
  if (opts.rawBody !== undefined) headers['Content-Type'] = 'application/octet-stream';
  if (accessToken) headers.Authorization = `Bearer ${accessToken}`;
  return fetch(`${API_BASE}${path}`, {
    method: opts.method ?? 'GET',
    headers,
    body: opts.rawBody ?? (opts.body === undefined ? undefined : JSON.stringify(opts.body)),
    credentials: 'include',
  });
}

export interface SessionResponse {
  accessToken: string;
  tokenType: 'Bearer';
  expiresIn: number;
  user: unknown;
}

/** Exchanges the refresh cookie for a new access token. Resolves to false when signed out. */
export function refreshAccessToken(): Promise<boolean> {
  refreshInFlight ??= (async () => {
    try {
      const res = await fetch(`${API_BASE}/auth/refresh`, { method: 'POST', credentials: 'include', headers: { Accept: 'application/json' } });
      if (!res.ok) {
        setAccessToken(null);
        return false;
      }
      const data = (await res.json()) as SessionResponse;
      setAccessToken(data.accessToken);
      return true;
    } catch {
      return false;
    } finally {
      refreshInFlight = null;
    }
  })();
  return refreshInFlight;
}

async function sendWithRefresh(path: string, opts: RequestOptions): Promise<Response> {
  let res = await send(path, opts);
  if (res.status === 401 && !opts.noRetry && accessToken) {
    if (await refreshAccessToken()) {
      res = await send(path, opts);
    } else {
      onSessionExpired?.();
    }
  }
  if (!res.ok) throw await parseError(res);
  return res;
}

export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const res = await sendWithRefresh(path, opts);
  if (res.status === 204 || res.status === 202) return undefined as T;
  return (await res.json()) as T;
}

/** Like request() but returns the raw response (headers + exact body bytes). */
export function requestRaw(path: string, opts: RequestOptions = {}): Promise<Response> {
  return sendWithRefresh(path, opts);
}
