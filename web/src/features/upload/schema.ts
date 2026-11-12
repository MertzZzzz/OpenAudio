import { z } from 'zod';
import { KEY_TONICS, MODULE_CLASSES, PACKAGE_KINDS, PLUGIN_ARCHES, PLUGIN_FORMATS, PLUGIN_OSES, type PackageKind } from '@/shared/types/domain';

export const MODULE_KINDS: PackageKind[] = ['instrument', 'effect', 'midi-tool', 'utility', 'model'];
export const PLUGIN_KINDS: PackageKind[] = ['instrument', 'effect', 'midi-tool', 'utility'];
export const SIGNALS = ['audio', 'midi', 'transport', 'automation', 'sidechain', 'analysis'] as const;
export const PLACEMENTS = ['instrument-track', 'audio-track', 'midi-track', 'insert', 'send', 'bus', 'master'] as const;
export const LICENSES = ['CC0-1.0', 'CC-BY-4.0', 'CC-BY-SA-4.0', 'CC-BY-NC-4.0', 'MIT', 'Apache-2.0', 'GPL-3.0-or-later'] as const;

const optionalNumber = (min: number, max: number) =>
  z.preprocess((v) => (v === '' || v === null || (typeof v === 'number' && Number.isNaN(v)) ? undefined : Number(v)), z.number().min(min).max(max).optional());

export const uploadFileSchema = z.object({
  id: z.string(),
  path: z.string().min(1),
  size: z.number(),
  contentType: z.string(),
  digest: z.string().nullable(),
  status: z.enum(['hashing', 'uploading', 'done', 'error']),
  progress: z.number(),
  /** Platform read from the executable header (plug-in binaries only). */
  binary: z.object({ os: z.enum(PLUGIN_OSES), arch: z.array(z.enum(PLUGIN_ARCHES)) }).nullable().optional(),
});
export type UploadFile = z.infer<typeof uploadFileSchema>;

export const pluginBuildSchema = z.object({
  format: z.enum(PLUGIN_FORMATS),
  bundle: z.string().min(1),
  os: z.enum(PLUGIN_OSES),
  arch: z.array(z.enum(PLUGIN_ARCHES)).min(1, 'Pick at least one architecture'),
  pluginIds: z.array(z.string()).optional(),
  /** Proposed by detection and not edited yet (never sent to the server). */
  auto: z.boolean().optional(),
});

export const wizardSchema = z
  .object({
    // Step 1 — package metadata
    packageSlug: z.string().min(1, 'Choose a package'),
    kind: z.enum(PACKAGE_KINDS),
    version: z.string().regex(/^\d+\.\d+\.\d+$/, 'Use semver: MAJOR.MINOR.PATCH'),
    changelog: z.string().trim().min(3, 'Describe what changed').max(2000),
    description: z.string().max(2000),
    license: z.enum(LICENSES),
    commercialUse: z.boolean(),
    attributionRequired: z.boolean(),
    // Step 2 — taxonomy & musical
    primaryGenre: z.string().min(1, 'Pick a primary genre'),
    secondaryGenres: z.array(z.string()).max(3, 'At most 3 secondary genres'),
    instruments: z.array(z.string()),
    moods: z.array(z.string()).max(3, 'Pick 1–3 moods'),
    useCases: z.array(z.string()),
    energy: z.number().min(0).max(1),
    bpm: optionalNumber(20, 400),
    bpmMin: optionalNumber(20, 400),
    bpmMax: optionalNumber(20, 400),
    keyTonic: z.union([z.enum(KEY_TONICS), z.literal('')]),
    keyMode: z.enum(['major', 'minor']),
    timeSignature: z.string().regex(/^(\d+\/\d+)?$/, 'e.g. 4/4'),
    freeTags: z.string(),
    moduleClass: z.union([z.enum(MODULE_CLASSES), z.literal('')]),
    accepts: z.array(z.enum(SIGNALS)),
    produces: z.array(z.enum(SIGNALS)),
    placement: z.array(z.enum(PLACEMENTS)),
    categories: z.array(z.string()),
    // Step 3 — files
    files: z.array(uploadFileSchema).min(1, 'Add at least one file'),
    plugins: z.array(pluginBuildSchema),
  })
  .superRefine((v, ctx) => {
    if (v.bpmMin !== undefined && v.bpmMax !== undefined && v.bpmMin > v.bpmMax) {
      ctx.addIssue({ code: 'custom', path: ['bpmMax'], message: 'Max must be ≥ min' });
    }
    if (MODULE_KINDS.includes(v.kind) && !v.moduleClass) {
      ctx.addIssue({ code: 'custom', path: ['moduleClass'], message: 'Module packages need a module class' });
    }
    if (v.plugins.length && !PLUGIN_KINDS.includes(v.kind)) {
      ctx.addIssue({ code: 'custom', path: ['plugins'], message: 'VST3/CLAP builds need an instrument, effect, MIDI tool or utility package (step 1)' });
    }
    if (v.files.some((f) => f.status !== 'done')) {
      ctx.addIssue({ code: 'custom', path: ['files'], message: 'Wait for all files to finish uploading' });
    }
  });

export type WizardValues = z.infer<typeof wizardSchema>;

export const STEP_FIELDS: (keyof WizardValues)[][] = [
  ['packageSlug', 'kind', 'version', 'changelog', 'description', 'license', 'commercialUse', 'attributionRequired'],
  ['primaryGenre', 'secondaryGenres', 'instruments', 'moods', 'useCases', 'energy', 'bpm', 'bpmMin', 'bpmMax', 'keyTonic', 'keyMode', 'timeSignature', 'moduleClass', 'accepts', 'produces', 'placement', 'categories'],
  ['files', 'plugins'],
  [],
  [],
];

export const defaultValues: WizardValues = {
  packageSlug: '',
  kind: 'sample-pack',
  version: '1.0.0',
  changelog: '',
  description: '',
  license: 'CC-BY-4.0',
  commercialUse: true,
  attributionRequired: true,
  primaryGenre: '',
  secondaryGenres: [],
  instruments: [],
  moods: [],
  useCases: [],
  energy: 0.5,
  bpm: undefined,
  bpmMin: undefined,
  bpmMax: undefined,
  keyTonic: '',
  keyMode: 'minor',
  timeSignature: '4/4',
  freeTags: '',
  moduleClass: '',
  accepts: [],
  produces: [],
  placement: [],
  categories: [],
  files: [],
  plugins: [],
};
