import type { ApiClient } from './client';
import { createMockClient } from './mock/mockClient';

/**
 * Single injection point for the API implementation.
 * Phase 1 uses the in-memory mock; Phase 5 swaps in the HTTP client.
 */
export const api: ApiClient = createMockClient({ latencyMs: import.meta.env.MODE === 'test' ? 0 : 250 });

export type { ApiClient } from './client';
export { ApiError } from './client';
