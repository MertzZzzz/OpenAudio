import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/shared/api';
import { queryKeys } from '@/shared/api/queryKeys';
import type { Library, Lockfile } from '@/shared/types/domain';
import { useToast } from '@/shared/ui/Toast';

export function useLibrary() {
  return useQuery({ queryKey: queryKeys.library, queryFn: () => api.getLibrary() });
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
  return useLibraryMutation((lf: Lockfile) => api.importLockfile(lf), (lf) => `Imported ${Object.keys(lf.packages).length} packages from lockfile`);
}
