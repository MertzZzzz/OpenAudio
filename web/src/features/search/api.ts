import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { api } from '@/shared/api';
import { queryKeys } from '@/shared/api/queryKeys';
import type { SearchParams } from '@/shared/types/domain';

export function useSearch(params: SearchParams) {
  return useQuery({ queryKey: queryKeys.search(params), queryFn: () => api.searchPackages(params), placeholderData: keepPreviousData });
}

export function useFacets(params: SearchParams) {
  return useQuery({ queryKey: queryKeys.facets(params), queryFn: () => api.searchFacets(params), placeholderData: keepPreviousData });
}
