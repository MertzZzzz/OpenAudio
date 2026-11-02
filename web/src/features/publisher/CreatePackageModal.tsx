import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { PACKAGE_KINDS } from '@/shared/types/domain';
import { Modal } from '@/shared/ui/Overlay';
import { Button } from '@/shared/ui/Button';
import { Field, Input, Textarea } from '@/shared/ui/Input';
import { Select } from '@/shared/ui/Select';
import { KIND_META } from '@/shared/lib/kinds';
import { useCreatePackage } from './api';

export const createPackageSchema = z.object({
  name: z.string().trim().min(2, 'At least 2 characters').max(80),
  slug: z
    .string()
    .trim()
    .min(2, 'At least 2 characters')
    .max(64)
    .regex(/^[a-z0-9]+(-[a-z0-9]+)*$/, 'Lowercase letters, digits and single dashes'),
  kind: z.enum(PACKAGE_KINDS),
  description: z.string().max(2000),
});

type FormValues = z.infer<typeof createPackageSchema>;

export const slugify = (s: string) =>
  s
    .toLowerCase()
    .normalize('NFKD')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 64);

export function CreatePackageModal({ publisher, open, onClose }: { publisher: string; open: boolean; onClose: () => void }) {
  const create = useCreatePackage(publisher);
  const { register, handleSubmit, watch, setValue, reset, formState } = useForm<FormValues>({
    resolver: zodResolver(createPackageSchema),
    defaultValues: { name: '', slug: '', kind: 'sample-pack', description: '' },
  });
  const name = watch('name');

  useEffect(() => {
    if (!formState.dirtyFields.slug) setValue('slug', slugify(name));
  }, [name, formState.dirtyFields.slug, setValue]);

  const submit = handleSubmit(async (v) => {
    await create.mutateAsync(v);
    reset();
    onClose();
  });

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Create package"
      description={`New package under @${publisher}. Releases are added separately.`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancel</Button>
          <Button type="submit" form="create-package" loading={create.isPending}>Create package</Button>
        </>
      }
    >
      <form id="create-package" onSubmit={submit} className="space-y-4" noValidate>
        <Field label="Name" error={formState.errors.name?.message}>
          {(p) => <Input {...p} {...register('name')} placeholder="Warm Tape Drums" />}
        </Field>
        <Field label="Slug" hint={`openaudio.dev/${publisher}/${watch('slug') || '…'}`} error={formState.errors.slug?.message}>
          {(p) => <Input {...p} {...register('slug')} className="font-mono" />}
        </Field>
        <Field label="Kind" error={formState.errors.kind?.message}>
          {(p) => <Select {...p} {...register('kind')} options={PACKAGE_KINDS.map((k) => ({ value: k, label: KIND_META[k].label }))} />}
        </Field>
        <Field label="Description" error={formState.errors.description?.message}>
          {(p) => <Textarea {...p} {...register('description')} placeholder="What is in this package?" />}
        </Field>
      </form>
    </Modal>
  );
}
