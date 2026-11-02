import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/shared/api';
import type { CreatePackageInput } from '@/shared/api/client';
import { queryKeys } from '@/shared/api/queryKeys';
import type { PublisherMember } from '@/shared/types/domain';
import { useToast } from '@/shared/ui/Toast';

export function usePublisherPackages(publisher: string) {
  return useQuery({ queryKey: queryKeys.publisherPackages(publisher), queryFn: () => api.publisherPackages(publisher) });
}

export function usePublisherMembers(publisher: string) {
  return useQuery({ queryKey: queryKeys.publisherMembers(publisher), queryFn: () => api.publisherMembers(publisher) });
}

function useInvalidateCatalog() {
  const qc = useQueryClient();
  return () =>
    Promise.all(
      ['publisher-packages', 'package', 'versions', 'search', 'facets', 'discover', 'library'].map((k) => qc.invalidateQueries({ queryKey: [k] })),
    );
}

export function useCreatePackage(publisher: string) {
  const invalidate = useInvalidateCatalog();
  const { toast } = useToast();
  return useMutation({
    mutationFn: (input: CreatePackageInput) => api.createPackage(publisher, input),
    onSuccess: (p) => {
      void invalidate();
      toast({ tone: 'success', title: `Created ${p.name}`, description: 'Upload a first release to publish it.' });
    },
    onError: (e: Error) => toast({ tone: 'error', title: 'Could not create package', description: e.message }),
  });
}

export function useReleaseAction(publisher: string) {
  const invalidate = useInvalidateCatalog();
  const { toast } = useToast();
  return useMutation({
    mutationFn: ({ slug, version, action }: { slug: string; version: string; action: 'publish' | 'yank' }) =>
      action === 'publish' ? api.publishRelease(publisher, slug, version) : api.yankRelease(publisher, slug, version),
    onSuccess: (r, v) => {
      void invalidate();
      toast({ tone: v.action === 'publish' ? 'success' : 'warning', title: v.action === 'publish' ? `Published ${v.slug}@${r.version}` : `Yanked ${v.slug}@${r.version}` });
    },
    onError: (e: Error) => toast({ tone: 'error', title: 'Release action failed', description: e.message }),
  });
}

export function useInviteMember(publisher: string) {
  const qc = useQueryClient();
  const { toast } = useToast();
  return useMutation({
    mutationFn: ({ email, role }: { email: string; role: PublisherMember['role'] }) => api.inviteMember(publisher, email, role),
    onSuccess: (m) => {
      void qc.invalidateQueries({ queryKey: queryKeys.publisherMembers(publisher) });
      toast({ tone: 'success', title: `Invited ${m.email}` });
    },
    onError: (e: Error) => toast({ tone: 'error', title: 'Invite failed', description: e.message }),
  });
}

export function useRemoveMember(publisher: string) {
  const qc = useQueryClient();
  const { toast } = useToast();
  return useMutation({
    mutationFn: (userId: string) => api.removeMember(publisher, userId),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.publisherMembers(publisher) });
      toast({ tone: 'success', title: 'Member removed' });
    },
    onError: (e: Error) => toast({ tone: 'error', title: 'Could not remove member', description: e.message }),
  });
}
