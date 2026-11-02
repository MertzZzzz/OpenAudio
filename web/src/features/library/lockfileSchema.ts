import { z } from 'zod';

export const lockfileSchema = z.object({
  lockfileVersion: z.literal(1),
  name: z.string(),
  generatedAt: z.string(),
  packages: z.record(
    z.object({
      id: z.string(),
      version: z.string().regex(/^\d+\.\d+\.\d+$/),
      manifestDigest: z.string().regex(/^sha256:[0-9a-f]+$/),
      resolved: z.string(),
    }),
  ),
});
