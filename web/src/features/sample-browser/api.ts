import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { api } from '@/shared/api';
import { queryKeys } from '@/shared/api/queryKeys';
import type { SampleQuery } from '@/shared/types/domain';

export function useSamples(q: SampleQuery) {
  return useQuery({ queryKey: queryKeys.samples(q), queryFn: () => api.searchSamples(q), placeholderData: keepPreviousData });
}
