import { QueryClient } from '@tanstack/react-query';

// queryClient 统一管理前端查询缓存，避免每个页面自己重复创建实例。
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
      staleTime: 30_000,
    },
  },
});
