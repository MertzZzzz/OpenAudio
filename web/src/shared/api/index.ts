import type { ApiClient } from './client';
import { createHttpClient } from './httpClient';
import { createMockClient } from './mock/mockClient';
import { apiMode } from '@/shared/lib/apiMode';

/**
 * Single injection point for the API implementation: the Go API in "http" mode,
 * the in-memory mock otherwise (no backend needed, used by tests).
 */
export const api: ApiClient =
  apiMode === 'http' ? createHttpClient() : createMockClient({ latencyMs: import.meta.env.MODE === 'test' ? 0 : 250 });

export type { ApiClient } from './client';
export { ApiError } from './client';
