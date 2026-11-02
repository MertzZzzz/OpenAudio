import type { Package } from '@/shared/types/domain';
import { MODULE_KINDS, type WizardValues } from './schema';

/** Turns wizard state into an `openaudio.package/v1` manifest. */
export function buildManifest(v: WizardValues, pkg: Pick<Package, 'id' | 'name' | 'publisher'> | undefined): Record<string, unknown> {
  const isModule = MODULE_KINDS.includes(v.kind);
  const musical: Record<string, unknown> = {};
  if (v.bpm !== undefined) {
    musical.bpm = {
      value: v.bpm,
      ...(v.bpmMin !== undefined && v.bpmMax !== undefined ? { range: [v.bpmMin, v.bpmMax] } : {}),
      sync: true,
    };
  }
  if (v.keyTonic) musical.key = { tonic: v.keyTonic, mode: v.keyMode };
  if (v.timeSignature) musical.timeSignature = v.timeSignature;

  const formats = [...new Set(v.files.map((f) => f.path.split('.').pop()?.toLowerCase() ?? '').filter(Boolean))];

  return {
    schema: 'openaudio.package/v1',
    id: pkg?.id ?? `org.unknown.${v.packageSlug}`,
    name: pkg?.name ?? v.packageSlug,
    kind: v.kind,
    version: v.version,
    ...(v.description ? { description: v.description } : {}),
    publisher: { id: pkg?.publisher.id ?? 'org.unknown', name: pkg?.publisher.name ?? 'Unknown' },
    license: { content: v.license, commercialUse: v.commercialUse, attributionRequired: v.attributionRequired },
    taxonomy: {
      genres: { primary: v.primaryGenre, secondary: v.secondaryGenres },
      instruments: v.instruments,
      moods: v.moods,
      useCases: v.useCases,
      energy: v.energy,
    },
    ...(Object.keys(musical).length ? { musical } : {}),
    technical: { formats: formats.length ? formats : ['wav'] },
    ...(isModule && v.moduleClass
      ? {
          module: {
            moduleClass: v.moduleClass,
            capabilities: { accepts: v.accepts, produces: v.produces, placement: v.placement },
            categories: v.categories,
          },
        }
      : {}),
    artifacts: v.files
      .filter((f) => f.digest)
      .map((f) => ({
        kind: f.contentType.startsWith('audio/') ? 'sample' : 'binary',
        path: f.path,
        digest: f.digest,
        size: f.size,
        contentType: f.contentType,
      })),
    tags: {
      free: v.freeTags
        .split(',')
        .map((t) => t.trim().toLowerCase())
        .filter(Boolean),
    },
  };
}
