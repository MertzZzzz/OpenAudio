import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/shared/api';
import { useAuth } from '@/features/auth/AuthContext';
import { queryKeys } from '@/shared/api/queryKeys';
import type { Library, Lockfile } from '@/shared/types/domain';
import { useToast } from '@/shared/ui/Toast';
import { HttpApiError } from '@/shared/api/http';
import { shortDigest } from '@/shared/lib/format';

export function useLibrary() {
  const { user } = useAuth();
  return useQuery({ queryKey: queryKeys.library, queryFn: () => api.getLibrary(), enabled: Boolean(user) });
}

export function useResolve(enabled = true) {
  return useQuery({ queryKey: queryKeys.resolve, queryFn: () => api.resolveLibrary(), enabled });
}

function useLibraryMutation<V>(fn: (v: V) => Promise<Library>, success?: (v: V) => string) {
  const qc = useQueryClient();
  const { toast } = useToast();
  return useMutation({
    mutationFn: fn,
    onSuccess: (lib, v) => {
      qc.setQueryData(queryKeys.library, lib);
      void qc.invalidateQueries({ queryKey: queryKeys.resolve });
      if (success) toast({ tone: 'success', title: success(v) });
    },
    onError: (e: Error) => toast({ tone: 'error', title: 'Library update failed', description: e.message }),
  });
}

export function useAddToLibrary() {
  return useLibraryMutation(
    (v: { packageId: string; name: string; version?: string | null }) => api.addToLibrary(v.packageId, v.version ?? null),
    (v) => `Added ${v.name} to your library`,
  );
}

export function useRemoveFromLibrary() {
  return useLibraryMutation((v: { packageId: string; name: string }) => api.removeFromLibrary(v.packageId), (v) => `Removed ${v.name}`);
}

export function usePinVersion() {
  return useLibraryMutation(
    (v: { packageId: string; version: string | null }) => api.pinVersion(v.packageId, v.version),
    (v) => (v.version ? `Pinned to ${v.version}` : 'Unpinned — will track latest'),
  );
}

export function useUpdateItem() {
  return useLibraryMutation((v: { packageId: string; name: string }) => api.updateLibraryItem(v.packageId), (v) => `Updated ${v.name}`);
}

export function useImportLockfile() {
  const qc = useQueryClient();
  const { toast } = useToast();
  return useMutation({
    mutationFn: (lf: Lockfile) => api.importLockfile(lf),
    onSuccess: (res, lf) => {
      qc.setQueryData(queryKeys.library, res.library);
      void qc.invalidateQueries({ queryKey: queryKeys.resolve });
      toast({ tone: 'success', title: `Imported ${Object.keys(lf.packages).length} packages from lockfile`, description: 'Every entry matched the registry digests.' });
      for (const w of res.warnings) toast({ tone: 'warning', title: w.message });
    },
    onError: (e: Error) => {
      const first = e instanceof HttpApiError ? e.issues[0]?.message : undefined;
      toast({ tone: 'error', title: 'Lockfile rejected', description: first ?? e.message });
    },
  });
}

export function useLockfiles(enabled = true) {
  return useQuery({ queryKey: ['lockfiles'], queryFn: () => api.listLockfiles(), enabled });
}

export function useSaveLockfile() {
  const qc = useQueryClient();
  const { toast } = useToast();
  return useMutation({
    mutationFn: () => api.saveLockfile(),
    onSuccess: ({ saved }) => {
      void qc.invalidateQueries({ queryKey: ['lockfiles'] });
      toast({ tone: 'success', title: 'Lockfile saved', description: shortDigest(saved.contentDigest) });
    },
    onError: (e: Error) => toast({ tone: 'error', title: 'Could not export', description: e.message }),
  });
}

export function useDeleteLockfile() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.deleteLockfile(id),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['lockfiles'] }),
  });
}
