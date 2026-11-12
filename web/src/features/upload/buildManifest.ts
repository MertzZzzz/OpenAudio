import type { Package } from '@/shared/types/domain';
import { MODULE_KINDS, type WizardValues } from './schema';
import { bundleRoot } from './detectPlugins';

/**
 * Form values come straight from inputs (react-hook-form `watch`), so numeric fields may still
 * be strings ("124") or empty (""). The manifest schema wants integers.
 */
function int(v: unknown): number | undefined {
  if (v === undefined || v === null || v === '') return undefined;
  const n = Number(v);
  return Number.isFinite(n) ? Math.round(n) : undefined;
}

function num(v: unknown): number | undefined {
  if (v === undefined || v === null || v === '') return undefined;
  const n = Number(v);
  return Number.isFinite(n) ? n : undefined;
}

/** Turns wizard state into an `openaudio.package/v1` manifest. */
export function buildManifest(v: WizardValues, pkg: Pick<Package, 'id' | 'name' | 'publisher'> | undefined): Record<string, unknown> {
  const isModule = MODULE_KINDS.includes(v.kind);
  const musical: Record<string, unknown> = {};
  const bpm = int(v.bpm);
  const bpmMin = int(v.bpmMin);
  const bpmMax = int(v.bpmMax);
  if (bpm !== undefined) {
    musical.bpm = {
      value: bpm,
      ...(bpmMin !== undefined && bpmMax !== undefined ? { range: [bpmMin, bpmMax] } : {}),
      sync: true,
    };
  }
  if (v.keyTonic) musical.key = { tonic: v.keyTonic, mode: v.keyMode };
  if (v.timeSignature) musical.timeSignature = v.timeSignature;

  // Files inside a .vst3/.clap bundle are plug-in binaries; their own extensions (.plist, .so) are not formats.
  const inBundle = (path: string) => bundleRoot(path) !== null;
  const formats = [
    ...new Set(
      [...v.plugins.map((p) => p.format), ...v.files.filter((f) => !inBundle(f.path)).map((f) => f.path.split('.').pop()?.toLowerCase() ?? '')].filter((f) =>
        /^[a-z0-9][a-z0-9.+-]*$/.test(f),
      ),
    ),
  ];

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
      energy: num(v.energy) ?? 0.5,
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
        kind: !inBundle(f.path) && f.contentType.startsWith('audio/') ? 'sample' : 'binary',
        path: f.path,
        digest: f.digest,
        size: f.size,
        contentType: f.contentType,
      })),
    ...(v.plugins.length
      ? {
          plugins: v.plugins.map((p) => {
            const ids = [...new Set((p.pluginIds ?? []).map((x) => x.trim()).filter(Boolean))];
            return { format: p.format, bundle: p.bundle, os: p.os, arch: p.arch, ...(ids.length ? { pluginIds: ids } : {}) };
          }),
        }
      : {}),
    tags: {
      free: v.freeTags
        .split(',')
        .map((t) => t.trim().toLowerCase())
        .filter(Boolean),
    },
  };
}
