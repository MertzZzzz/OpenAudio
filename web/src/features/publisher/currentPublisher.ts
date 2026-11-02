/**
 * Publisher memberships arrive with Phase 4 (packages & releases API).
 * Until then every publisher-role account manages the mock "acme" publisher.
 */
export const MOCK_PUBLISHER_SLUG = 'acme';

export function useCurrentPublisherSlug(): string {
  return MOCK_PUBLISHER_SLUG;
}
