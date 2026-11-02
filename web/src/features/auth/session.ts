export interface SessionUser {
  id: string;
  displayName: string;
  email: string;
  role: 'user' | 'publisher' | 'admin';
  publisherSlug: string | null;
}

/** Phase 1: a fixed mock session. Phase 3 replaces this with JWT auth. */
export const mockSession: SessionUser = {
  id: 'u1',
  displayName: 'Sam Rivera',
  email: 'sam@acme.audio',
  role: 'publisher',
  publisherSlug: 'acme',
};

export function useSession(): SessionUser {
  return mockSession;
}
