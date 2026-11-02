/**
 * UI domain model. Mirrors the `openaudio.package/v1` manifest from the spec.
 * Once the OpenAPI contract exists these types will be generated from it;
 * until then this file is the single source of truth for the mock layer.
 */

export const PACKAGE_KINDS = [
  'sample-pack',
  'preset-pack',
  'instrument',
  'effect',
  'midi-tool',
  'utility',
  'project-template',
  'model',
] as const;
export type PackageKind = (typeof PACKAGE_KINDS)[number];

export const MODULE_CLASSES = [
  'instrument',
  'audio-effect',
  'note-effect',
  'note-generator',
  'audio-generator',
  'analyzer',
  'utility',
  'converter',
  'macro',
  'visualizer',
] as const;
export type ModuleClass = (typeof MODULE_CLASSES)[number];

export type SignalKind = 'audio' | 'midi' | 'transport' | 'automation' | 'sidechain' | 'analysis';
export type Placement =
  | 'instrument-track'
  | 'audio-track'
  | 'midi-track'
  | 'bus'
  | 'master'
  | 'send'
  | 'insert';

export interface ModuleCapabilities {
  accepts: SignalKind[];
  produces: SignalKind[];
  placement: Placement[];
  bypassable?: boolean;
  wetDry?: boolean;
  stateful?: boolean;
}

export interface ModulePort {
  id: string;
  direction: 'in' | 'out';
  signal: SignalKind;
  channels?: number;
}

export interface Module {
  moduleClass: ModuleClass;
  capabilities: ModuleCapabilities;
  ports: ModulePort[];
  categories: string[];
}

export const KEY_TONICS = ['C', 'C#', 'D', 'D#', 'E', 'F', 'F#', 'G', 'G#', 'A', 'A#', 'B'] as const;
export type KeyTonic = (typeof KEY_TONICS)[number];
export type KeyMode = 'major' | 'minor';

export interface MusicalKey {
  tonic: KeyTonic;
  mode: KeyMode;
  camelot?: string;
}

export interface Musical {
  bpm?: { value: number; range?: [number, number]; sync?: boolean };
  key?: MusicalKey;
  timeSignature?: string;
}

export interface Technical {
  sampleRate?: number;
  bitDepth?: number;
  formats: string[];
}

export interface Taxonomy {
  genres: { primary: string; secondary: string[] };
  instruments: string[];
  moods: string[];
  useCases: string[];
  energy?: number;
}

export interface License {
  content: string;
  commercialUse: boolean;
  attributionRequired: boolean;
}

export interface Publisher {
  id: string;
  slug: string;
  name: string;
  verified: boolean;
}

export interface Package {
  id: string;
  publisher: Publisher;
  slug: string;
  name: string;
  kind: PackageKind;
  description: string;
  latestVersion: string;
  license: License;
  taxonomy: Taxonomy;
  musical?: Musical;
  technical: Technical;
  module?: Module;
  tags: string[];
  downloads: number;
  rating: number;
  updatedAt: string;
  createdAt: string;
  sampleCount: number;
  sizeBytes: number;
  coverHue: number;
  editorsPick?: boolean;
  dependencies: PackageDependency[];
}

export interface PackageDependency {
  packageId: string;
  range: string;
}

export type ReleaseStatus = 'draft' | 'published' | 'yanked';

export interface Release {
  version: string;
  status: ReleaseStatus;
  publishedAt: string | null;
  changelog: string;
  manifestDigest: string;
}

export interface Artifact {
  kind: 'manifest' | 'sample' | 'preview' | 'binary' | 'index';
  path: string;
  digest: string;
  size: number;
  contentType: string;
  downloadUrl?: string;
}

export type SampleContentType = 'loop' | 'one-shot';

export interface Sample {
  id: string;
  packageId: string;
  name: string;
  file: string;
  instrument: string;
  role: 'drums' | 'bass' | 'lead' | 'pad' | 'fx' | 'transition';
  bpm?: number;
  key?: MusicalKey;
  durationSec: number;
  contentType: SampleContentType;
  tags: string[];
  previewUrl: string;
  digest: string;
  /** Counted download (API path redirecting to a presigned URL); absent in mock mode. */
  downloadUrl?: string;
  packageName?: string;
}

export interface TaxonomyTerm {
  domain: 'genre' | 'instrument' | 'mood' | 'use-case' | 'module-category';
  slug: string;
  label: string;
  parent?: string;
  aliases?: string[];
}

export interface LibraryItem {
  packageId: string;
  pinnedVersion: string | null;
  installedVersion: string;
  addedAt: string;
}

export interface Library {
  id: string;
  name: string;
  items: LibraryItem[];
}

export interface LockfileEntry {
  id: string;
  version: string;
  manifestDigest: string;
  resolved: string;
}

export interface Lockfile {
  lockfileVersion: 1;
  name: string;
  generatedAt: string;
  packages: Record<string, LockfileEntry>;
}

export interface ResolveConflict {
  packageId: string;
  requestedBy: { packageId: string; range: string }[];
  candidates: string[];
}

export interface ResolveResult {
  lockfile: Lockfile;
  conflicts: ResolveConflict[];
}

export interface PublisherMember {
  userId: string;
  name: string;
  email: string;
  role: 'owner' | 'maintainer' | 'uploader';
}

export interface Page<T> {
  items: T[];
  total: number;
  page: number;
  limit: number;
}

export type SortOrder = 'relevance' | 'newest' | 'downloads' | 'rating';

export interface SearchParams {
  q?: string;
  kind?: PackageKind[];
  genre?: string[];
  instrument?: string[];
  mood?: string[];
  license?: string[];
  moduleClass?: ModuleClass[];
  key?: string[];
  bpmMin?: number;
  bpmMax?: number;
  sort?: SortOrder;
  page?: number;
  limit?: number;
}

export interface FacetBucket {
  value: string;
  label: string;
  count: number;
}

export interface SearchFacets {
  kind: FacetBucket[];
  genre: FacetBucket[];
  instrument: FacetBucket[];
  mood: FacetBucket[];
  license: FacetBucket[];
  moduleClass: FacetBucket[];
  key: FacetBucket[];
  /** BPM ranges, values like "120-129". */
  bpm: FacetBucket[];
}

export interface SampleQuery {
  q?: string;
  packageId?: string;
  instrument?: string[];
  contentType?: SampleContentType[];
  key?: string[];
  bpmMin?: number;
  bpmMax?: number;
  durationMax?: number;
}

export type ValidationSeverity = 'error' | 'warning' | 'info';

export interface ValidationIssue {
  severity: ValidationSeverity;
  path: string;
  message: string;
}

export interface PublisherMembership {
  slug: string;
  name: string;
  role: PublisherMember['role'];
}

export interface UploadProgress {
  /** 0..1 */
  fraction: number;
  phase: 'uploading' | 'verifying' | 'done';
}
