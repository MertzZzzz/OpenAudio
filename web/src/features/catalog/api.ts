import { useQuery } from '@tanstack/react-query';
import { api } from '@/shared/api';
import { queryKeys } from '@/shared/api/queryKeys';

export function useDiscover() {
  return useQuery({ queryKey: queryKeys.discover, queryFn: () => api.discover() });
}

export function useTaxonomy() {
  return useQuery({ queryKey: queryKeys.taxonomy, queryFn: () => api.taxonomy(), staleTime: Infinity });
}
