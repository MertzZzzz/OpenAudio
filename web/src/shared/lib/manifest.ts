import { z } from 'zod';
import { MODULE_CLASSES, PACKAGE_KINDS, KEY_TONICS, type Module, type ValidationIssue } from '@/shared/types/domain';
import { validateModule } from './moduleRules';

const signal = z.enum(['audio', 'midi', 'transport', 'automation', 'sidechain', 'analysis']);
const MODULE_KINDS = ['instrument', 'effect', 'midi-tool', 'utility', 'model'] as const;

export const manifestSchema = z
  .object({
    schema: z.literal('openaudio.package/v1'),
    id: z.string().regex(/^[a-z0-9]+(\.[a-z0-9-]+)+$/, 'reverse-DNS id, e.g. org.acme.my-pack'),
    name: z.string().min(2).max(80),
    kind: z.enum(PACKAGE_KINDS),
    version: z.string().regex(/^\d+\.\d+\.\d+$/, 'semver MAJOR.MINOR.PATCH'),
    description: z.string().max(2000).optional(),
    publisher: z.object({ id: z.string(), name: z.string() }),
    license: z.object({ content: z.string().min(1), commercialUse: z.boolean(), attributionRequired: z.boolean() }),
    taxonomy: z.object({
      genres: z.object({ primary: z.string().min(1), secondary: z.array(z.string()).max(3) }),
      instruments: z.array(z.string()),
      moods: z.array(z.string()).max(3, 'use at most 3 moods'),
      useCases: z.array(z.string()).optional(),
      energy: z.number().min(0).max(1).optional(),
    }),
    musical: z
      .object({
        bpm: z.object({ value: z.number().int().min(20).max(400), range: z.tuple([z.number(), z.number()]).optional(), sync: z.boolean().optional() }).optional(),
        key: z.object({ tonic: z.enum(KEY_TONICS), mode: z.enum(['major', 'minor']), camelot: z.string().optional() }).optional(),
        timeSignature: z.string().regex(/^\d+\/\d+$/).optional(),
      })
      .optional(),
    technical: z.object({ sampleRate: z.number().optional(), bitDepth: z.number().optional(), formats: z.array(z.string()).min(1) }).optional(),
    module: z
      .object({
        moduleClass: z.enum(MODULE_CLASSES),
        capabilities: z.object({ accepts: z.array(signal), produces: z.array(signal), placement: z.array(z.string()) }),
        categories: z.array(z.string()),
        ports: z.array(z.object({ id: z.string(), direction: z.enum(['in', 'out']), signal })).optional(),
      })
      .optional(),
    artifacts: z.array(
      z.object({
        kind: z.enum(['manifest', 'sample', 'preview', 'binary', 'index']),
        path: z.string().min(1),
        digest: z.string().regex(/^sha256:[0-9a-f]{64}$/, 'digest must be sha256:<64 hex>'),
        size: z.number().int().nonnegative(),
        contentType: z.string().optional(),
      }),
    ),
    tags: z.object({ free: z.array(z.string()) }).optional(),
  })
  .superRefine((m, ctx) => {
    const needsModule = (MODULE_KINDS as readonly string[]).includes(m.kind);
    if (needsModule && !m.module) ctx.addIssue({ code: 'custom', path: ['module'], message: `module is required for kind "${m.kind}"` });
    if (m.kind === 'sample-pack' && m.module) ctx.addIssue({ code: 'custom', path: ['module'], message: 'sample-pack must not define module' });
    if (m.musical?.bpm?.range) {
      const [lo, hi] = m.musical.bpm.range;
      if (lo > hi) ctx.addIssue({ code: 'custom', path: ['musical', 'bpm', 'range'], message: 'range min must be ≤ max' });
    }
  });

export type Manifest = z.infer<typeof manifestSchema>;

/** Full validation: JSON-schema-equivalent structure + domain rules (§5.4) + recommendations (§7.3). */
export function validateManifest(input: unknown): ValidationIssue[] {
  const parsed = manifestSchema.safeParse(input);
  if (!parsed.success) {
    return parsed.error.issues.map((i) => ({ severity: 'error', path: i.path.join('.') || '(root)', message: i.message }));
  }
  const m = parsed.data;
  const issues: ValidationIssue[] = [];
  if (m.module) issues.push(...validateModule({ ports: [], ...m.module } as Module));
  if (m.taxonomy.moods.length === 0) issues.push({ severity: 'warning', path: 'taxonomy.moods', message: 'add 1–3 moods to improve discoverability' });
  if (m.kind === 'sample-pack' && !m.musical?.bpm) issues.push({ severity: 'info', path: 'musical.bpm', message: 'no package-level BPM; per-sample BPM will be used' });
  if (m.artifacts.length === 0) issues.push({ severity: 'error', path: 'artifacts', message: 'release must contain at least one artifact' });
  if (!m.description) issues.push({ severity: 'warning', path: 'description', message: 'description is empty' });
  return issues;
}
