import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Trash2, UserPlus } from 'lucide-react';
import type { PublisherMember } from '@/shared/types/domain';
import { Card, CardBody, CardHeader, CardTitle } from '@/shared/ui/Card';
import { Button } from '@/shared/ui/Button';
import { Badge } from '@/shared/ui/Badge';
import { Modal } from '@/shared/ui/Overlay';
import { Field, Input } from '@/shared/ui/Input';
import { Select } from '@/shared/ui/Select';
import { Skeleton } from '@/shared/ui/Skeleton';
import { useInviteMember, usePublisherMembers, useRemoveMember } from './api';

const inviteSchema = z.object({
  email: z.string().trim().email('Enter a valid email'),
  role: z.enum(['maintainer', 'uploader']),
});
type InviteValues = z.infer<typeof inviteSchema>;

const roleTone: Record<PublisherMember['role'], 'primary' | 'accent' | 'neutral'> = { owner: 'primary', maintainer: 'accent', uploader: 'neutral' };

export function MembersCard({ publisher }: { publisher: string }) {
  const members = usePublisherMembers(publisher);
  const invite = useInviteMember(publisher);
  const remove = useRemoveMember(publisher);
  const [open, setOpen] = useState(false);
  const [confirm, setConfirm] = useState<PublisherMember | null>(null);
  const form = useForm<InviteValues>({ resolver: zodResolver(inviteSchema), defaultValues: { email: '', role: 'uploader' } });

  const submit = form.handleSubmit(async (v) => {
    await invite.mutateAsync(v);
    form.reset();
    setOpen(false);
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle>Members</CardTitle>
        <Button size="sm" variant="secondary" leftIcon={<UserPlus className="h-4 w-4" />} onClick={() => setOpen(true)}>
          Invite
        </Button>
      </CardHeader>
      <CardBody className="p-0">
        {members.isLoading ? (
          <div className="space-y-2 p-6">
            <Skeleton className="h-10" />
            <Skeleton className="h-10" />
          </div>
        ) : (
          <ul className="divide-y divide-border">
            {members.data?.map((m) => (
              <li key={m.userId} className="flex items-center gap-3 px-6 py-3">
                <div className="flex h-8 w-8 items-center justify-center rounded-full bg-surface-raised text-caption font-semibold uppercase">{m.name.slice(0, 2)}</div>
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium">{m.name}</p>
                  <p className="truncate text-caption text-fg-subtle">{m.email}</p>
                </div>
                <Badge tone={roleTone[m.role]}>{m.role}</Badge>
                {m.role !== 'owner' && (
                  <button type="button" aria-label={`Remove ${m.name}`} onClick={() => setConfirm(m)} className="rounded p-1.5 text-fg-subtle hover:bg-danger/10 hover:text-danger">
                    <Trash2 className="h-4 w-4" />
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}
      </CardBody>

      <Modal
        open={open}
        onClose={() => setOpen(false)}
        title="Invite member"
        footer={
          <>
            <Button variant="secondary" onClick={() => setOpen(false)}>Cancel</Button>
            <Button type="submit" form="invite-member" loading={invite.isPending}>Send invite</Button>
          </>
        }
      >
        <form id="invite-member" onSubmit={submit} className="space-y-4" noValidate>
          <Field label="Email" error={form.formState.errors.email?.message}>
            {(p) => <Input {...p} type="email" {...form.register('email')} placeholder="producer@example.com" />}
          </Field>
          <Field label="Role" hint="Maintainers can publish and yank; uploaders can create drafts.">
            {(p) => <Select {...p} {...form.register('role')} options={[{ value: 'maintainer', label: 'Maintainer' }, { value: 'uploader', label: 'Uploader' }]} />}
          </Field>
        </form>
      </Modal>

      <Modal
        open={confirm !== null}
        onClose={() => setConfirm(null)}
        title="Remove member?"
        description={confirm ? `${confirm.name} will lose access to @${publisher}.` : undefined}
        footer={
          <>
            <Button variant="secondary" onClick={() => setConfirm(null)}>Cancel</Button>
            <Button
              variant="danger"
              loading={remove.isPending}
              onClick={async () => {
                if (confirm) await remove.mutateAsync(confirm.userId);
                setConfirm(null);
              }}
            >
              Remove
            </Button>
          </>
        }
      >
        <p className="text-body text-fg-muted">This does not affect releases they already published.</p>
      </Modal>
    </Card>
  );
}
