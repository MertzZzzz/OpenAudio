import type { SampleQuery, SearchParams } from '@/shared/types/domain';

export const queryKeys = {
  discover: ['discover'] as const,
  search: (p: SearchParams) => ['search', p] as const,
  facets: (p: SearchParams) => ['facets', p] as const,
  package: (publisher: string, slug: string) => ['package', publisher, slug] as const,
  packageById: (id: string) => ['package-by-id', id] as const,
  versions: (publisher: string, slug: string) => ['versions', publisher, slug] as const,
  manifest: (publisher: string, slug: string, version: string) => ['manifest', publisher, slug, version] as const,
  artifacts: (publisher: string, slug: string, version: string) => ['artifacts', publisher, slug, version] as const,
  samples: (q: SampleQuery) => ['samples', q] as const,
  taxonomy: ['taxonomy'] as const,
  publisherPackages: (publisher: string) => ['publisher-packages', publisher] as const,
  publisherMembers: (publisher: string) => ['publisher-members', publisher] as const,
  library: ['library'] as const,
  resolve: ['library', 'resolve'] as const,
};
