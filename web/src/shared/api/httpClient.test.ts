import { createHttpClient, toPackage } from './httpClient';
import { setAccessToken } from './http';
import { can } from '@/features/publisher/currentPublisher';

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

afterEach(() => {
  vi.unstubAllGlobals();
  setAccessToken(null);
});

const dto = {
  id: 'acme.kit',
  publisher: { id: 'acme', slug: 'acme', name: 'Acme Audio', verified: false },
  slug: 'kit',
  name: 'Warm Kit',
  kind: 'sample-pack' as const,
  description: '',
  latestVersion: null,
  createdAt: '2026-11-01T00:00:00Z',
  updatedAt: '2026-11-02T00:00:00Z',
  publishedAt: null,
  stats: { artifacts: 0, samples: 0, sizeBytes: 0 },
  license: null,
  taxonomy: null,
  musical: null,
  technical: null,
  module: null,
  dependencies: [{ id: 'lattice.plate-verb', range: '^2.0.0' }],
  tags: [],
};

describe('toPackage', () => {
  it('fills safe defaults for packages without a published release', () => {
    const p = toPackage(dto);
    expect(p.latestVersion).toBe('0.0.0');
    expect(p.taxonomy).toEqual({ genres: { primary: '', secondary: [] }, instruments: [], moods: [], useCases: [], energy: undefined });
    expect(p.license.content).toBe('');
    expect(p.technical.formats).toEqual([]);
    expect(p.dependencies).toEqual([{ packageId: 'lattice.plate-verb', range: '^2.0.0' }]);
    expect(p.coverHue).toBe(toPackage(dto).coverHue);
  });
});

describe('uploadBlob', () => {
  it('resumes: sends only missing parts, then completes', async () => {
    const calls: string[] = [];
    vi.stubGlobal('fetch', (url: string, init: RequestInit) => {
      calls.push(`${init.method} ${url}`);
      if (url === '/api/v1/uploads') return Promise.resolve(json(201, { id: 'u1', status: 'pending', partSize: 4, partCount: 3, parts: [{ number: 2, size: 4 }] }));
      if (url.includes('/parts/')) {
        expect((init.headers as Record<string, string>)['Content-Type']).toBe('application/octet-stream');
        return Promise.resolve(json(200, { number: 1, size: 4 }));
      }
      return Promise.resolve(json(200, { id: 'u1', status: 'done', partSize: 4, partCount: 0, parts: [] }));
    });
    const progress: string[] = [];
    await createHttpClient().uploadBlob(new Blob(['0123456789']), {
      digest: 'sha256:x',
      contentType: 'audio/wav',
      onProgress: (p) => progress.push(`${p.phase}:${p.fraction.toFixed(1)}`),
    });
    expect(calls).toEqual(['POST /api/v1/uploads', 'PUT /api/v1/uploads/u1/parts/1', 'PUT /api/v1/uploads/u1/parts/3', 'POST /api/v1/uploads/u1/complete']);
    expect(progress).toEqual(['uploading:0.4', 'uploading:0.8', 'uploading:1.0', 'verifying:1.0', 'done:1.0']);
  });

  it('skips content the server already has', async () => {
    const calls: string[] = [];
    vi.stubGlobal('fetch', (url: string) => {
      calls.push(url);
      return Promise.resolve(json(200, { id: 'u2', status: 'done', partSize: 8, partCount: 0, parts: [] }));
    });
    await createHttpClient().uploadBlob(new Blob(['abc']), { digest: 'sha256:y', contentType: '' });
    expect(calls).toEqual(['/api/v1/uploads']);
  });
});

describe('publisher permissions', () => {
  it.each([
    ['owner', 'members', true],
    ['maintainer', 'publish', true],
    ['maintainer', 'members', false],
    ['uploader', 'draft', true],
    ['uploader', 'publish', false],
    ['uploader', 'packages', false],
  ] as const)('%s can %s → %s', (role, action, ok) => expect(can(role, action)).toBe(ok));
});
