import { useQuery } from '@tanstack/react-query';

import { queryKeys } from '@/api/keys';
import type { ConfigOptions } from '@/generated/types';
import { apiClient } from '@/lib/apiClient';

// useConfigOptions 拉取 profiles + languages，全应用共享缓存且永不过期（启动时拉一次即可）。
export function useConfigOptions() {
  return useQuery({
    queryKey: queryKeys.configs.options,
    queryFn: () => apiClient<ConfigOptions>('/api/v1/configs/options'),
    staleTime: Infinity,
  });
}
