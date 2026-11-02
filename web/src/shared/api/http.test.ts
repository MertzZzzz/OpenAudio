import { getAccessToken, HttpApiError, request, setAccessToken } from './http';

type Handler = (url: string, init: RequestInit) => Response;

function mockFetch(handler: Handler) {
  const calls: { url: string; auth: string | undefined }[] = [];
  vi.stubGlobal('fetch', (url: string, init: RequestInit) => {
    calls.push({ url, auth: (init.headers as Record<string, string> | undefined)?.Authorization });
    return Promise.resolve(handler(url, init));
  });
  return calls;
}

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

afterEach(() => {
  vi.unstubAllGlobals();
  setAccessToken(null);
});

describe('http client', () => {
  it('refreshes once on 401 and retries with the new token', async () => {
    setAccessToken('expired');
    const calls = mockFetch((url, init) => {
      if (url.endsWith('/auth/refresh')) return json(200, { accessToken: 'fresh', tokenType: 'Bearer', expiresIn: 900, user: {} });
      const auth = (init.headers as Record<string, string>).Authorization;
      return auth === 'Bearer fresh' ? json(200, { ok: true }) : json(401, { code: 'invalid_token', message: 'expired' });
    });
    await expect(request<{ ok: boolean }>('/me')).resolves.toEqual({ ok: true });
    expect(calls.map((c) => c.url)).toEqual(['/api/v1/me', '/api/v1/auth/refresh', '/api/v1/me']);
    expect(getAccessToken()).toBe('fresh');
  });

  it('surfaces the API error format, including field issues', async () => {
    mockFetch(() => json(422, { code: 'validation_failed', message: 'request validation failed', issues: [{ severity: 'error', path: 'email', message: 'bad' }] }));
    const err = await request('/auth/register', { method: 'POST', body: {} }).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(HttpApiError);
    expect(err).toMatchObject({ status: 422, code: 'validation_failed', issues: [{ path: 'email', message: 'bad' }] });
  });

  it('does not loop when refresh fails', async () => {
    setAccessToken('expired');
    const calls = mockFetch(() => json(401, { code: 'invalid_token', message: 'nope' }));
    await expect(request('/me')).rejects.toMatchObject({ status: 401 });
    expect(calls).toHaveLength(2);
    expect(getAccessToken()).toBeNull();
  });
});
