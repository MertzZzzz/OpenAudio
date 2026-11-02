import type {
  Artifact,
  Library,
  Lockfile,
  Package,
  PackageKind,
  Page,
  PublisherMember,
  PublisherMembership,
  UploadProgress,
  Release,
  ResolveResult,
  Sample,
  SampleQuery,
  SearchFacets,
  SearchParams,
  TaxonomyTerm,
  ValidationIssue,
} from '@/shared/types/domain';

export interface DiscoverFeed {
  popular: Package[];
  newReleases: Package[];
  editorsPicks: Package[];
}

export interface CreatePackageInput {
  name: string;
  slug: string;
  kind: PackageKind;
  description: string;
}

export interface DraftReleaseInput {
  publisher: string;
  slug: string;
  version: string;
  changelog: string;
  manifest: Record<string, unknown>;
  files: { path: string; size: number; contentType: string }[];
}

/**
 * Typed contract for everything the UI needs from the backend.
 * `mockClient` implements it in-memory; an HTTP implementation generated
 * from `api/openapi.yaml` will replace it without touching feature code.
 */
export interface ApiClient {
  discover(): Promise<DiscoverFeed>;
  searchPackages(params: SearchParams): Promise<Page<Package>>;
  searchFacets(params: SearchParams): Promise<SearchFacets>;
  getPackage(publisher: string, slug: string): Promise<Package>;
  getPackageById(id: string): Promise<Package>;
  listVersions(publisher: string, slug: string): Promise<Release[]>;
  getManifest(publisher: string, slug: string, version: string): Promise<Record<string, unknown>>;
  listArtifacts(publisher: string, slug: string, version: string): Promise<Artifact[]>;
  searchSamples(query: SampleQuery): Promise<Sample[]>;
  taxonomy(): Promise<TaxonomyTerm[]>;

  myPublishers(): Promise<PublisherMembership[]>;
  createPublisher(slug: string, name: string): Promise<PublisherMembership>;
  publisherPackages(publisher: string): Promise<(Package & { releases: Release[] })[]>;
  publisherMembers(publisher: string): Promise<PublisherMember[]>;
  inviteMember(publisher: string, email: string, role: PublisherMember['role']): Promise<PublisherMember>;
  removeMember(publisher: string, userId: string): Promise<void>;
  createPackage(publisher: string, input: CreatePackageInput): Promise<Package>;
  /** Validates a manifest; with a target it is normalized against that package first (as the server would). */
  validateManifest(manifest: Record<string, unknown>, target?: { publisher: string; slug: string }): Promise<ValidationIssue[]>;
  /**
   * Uploads one content-addressed blob (resumable multipart). Resolves when the server has
   * verified the SHA-256. Blobs that already exist are skipped (`onProgress` jumps to done).
   */
  uploadBlob(file: Blob, opts: { digest: string; contentType: string; onProgress?: (p: UploadProgress) => void }): Promise<void>;
  createDraftRelease(input: DraftReleaseInput): Promise<Release>;
  publishRelease(publisher: string, slug: string, version: string): Promise<Release>;
  yankRelease(publisher: string, slug: string, version: string): Promise<Release>;

  getLibrary(): Promise<Library>;
  addToLibrary(packageId: string, pinnedVersion?: string | null): Promise<Library>;
  removeFromLibrary(packageId: string): Promise<Library>;
  pinVersion(packageId: string, version: string | null): Promise<Library>;
  updateLibraryItem(packageId: string): Promise<Library>;
  resolveLibrary(): Promise<ResolveResult>;
  importLockfile(lockfile: Lockfile): Promise<Library>;
}

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}
