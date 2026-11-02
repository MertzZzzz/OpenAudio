/**
 * Controlled vocabulary helpers. The vocabulary is versioned (taxonomyVersion)
 * and currently bundled with the mock layer; Phase 5 will load it via GET /taxonomy.
 */
export { TAXONOMY_VERSION, termLabel, termsOf, expandWithChildren, resolveAlias } from '@/shared/api/mock/taxonomy';
