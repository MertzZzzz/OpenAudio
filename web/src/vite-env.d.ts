/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** "http" → real Go API (proxied at /api); anything else → in-memory mock auth. */
  readonly VITE_API_MODE?: 'http' | 'mock';
  /** @deprecated use VITE_API_MODE */
  readonly VITE_AUTH_MODE?: 'http' | 'mock';
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
