/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** "http" → real Go API (proxied at /api); anything else → in-memory mock auth. */
  readonly VITE_API_MODE?: 'http' | 'mock';
  /** @deprecated use VITE_API_MODE */
  readonly VITE_AUTH_MODE?: 'http' | 'mock';
  /** Page users land on after sign-in when no returnTo was given (default "/"). Mirrors AUTH_REDIRECT_PATH. */
  readonly VITE_AUTH_REDIRECT_PATH?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
