import { useQuery } from '@tanstack/react-query';
import { api } from '@/shared/api';
import { queryKeys } from '@/shared/api/queryKeys';

export function usePackage(publisher: string, slug: string) {
  return useQuery({ queryKey: queryKeys.package(publisher, slug), queryFn: () => api.getPackage(publisher, slug) });
}

export function usePackageById(id: string) {
  return useQuery({ queryKey: queryKeys.packageById(id), queryFn: () => api.getPackageById(id) });
}

export function useVersions(publisher: string, slug: string) {
  return useQuery({ queryKey: queryKeys.versions(publisher, slug), queryFn: () => api.listVersions(publisher, slug) });
}

export function useManifest(publisher: string, slug: string, version: string | undefined) {
  return useQuery({
    queryKey: queryKeys.manifest(publisher, slug, version ?? ''),
    queryFn: () => api.getManifest(publisher, slug, version!),
    enabled: Boolean(version),
  });
}

export function useArtifacts(publisher: string, slug: string, version: string | undefined) {
  return useQuery({
    queryKey: queryKeys.artifacts(publisher, slug, version ?? ''),
    queryFn: () => api.listArtifacts(publisher, slug, version!),
    enabled: Boolean(version),
  });
}
