/**
 * "http" talks to the Go API (proxied at /api); anything else uses the in-memory mock.
 * VITE_API_MODE is the switch; VITE_AUTH_MODE is accepted for backwards compatibility.
 */
export const apiMode: 'http' | 'mock' =
  (import.meta.env.VITE_API_MODE ?? import.meta.env.VITE_AUTH_MODE) === 'http' ? 'http' : 'mock';
