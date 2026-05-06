import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { queryKeys } from '@/api/keys';
import type {
  GroupActionResult,
  GroupCreateResult,
  GroupDetail,
  GroupSummary,
} from '@/generated/types';
import { apiClient } from '@/lib/apiClient';

// CreateGroupInput 对应 POST /api/v1/groups 的请求体。
export interface CreateGroupInput {
  name: string;
  profileId: string;
  languages: string[];
}

// useGroup 拉取单个分组详情（含实例列表）。
export function useGroup(id: number | undefined) {
  return useQuery({
    queryKey: id !== undefined ? queryKeys.groups.detail(id) : ['groups', 'detail', 'noop'],
    queryFn: () => apiClient<GroupDetail>(`/api/v1/groups/${id}`),
    enabled: id !== undefined,
    staleTime: 5_000,
  });
}

// useGroups 拉取当前用户全部分组（含聚合状态）。任意分组处于 transitioning（starting/stopping）
// 时按 2s 轮询，自动收敛到稳态后停轮。
export function useGroups() {
  return useQuery({
    queryKey: queryKeys.groups.all,
    queryFn: () => apiClient<GroupSummary[]>('/api/v1/groups'),
    staleTime: 10_000,
    refetchInterval: (query) => {
      const data = query.state.data;
      return data?.some((g) => g.aggregateState === 'transitioning') ? 2000 : false;
    },
  });
}

// useCreateGroup 提交建组请求并在成功后刷新列表。
export function useCreateGroup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateGroupInput) =>
      apiClient<GroupCreateResult>('/api/v1/groups', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.groups.all });
    },
  });
}

// useRenameGroup 用来改名，成功后刷新列表。
export function useRenameGroup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, name }: { id: number; name: string }) =>
      apiClient<{ success: boolean }>(`/api/v1/groups/${id}`, {
        method: 'PATCH',
        body: JSON.stringify({ name }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.groups.all });
    },
  });
}

// useDeleteGroup 用来删除分组，成功后刷新列表。
export function useDeleteGroup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) =>
      apiClient<{ success: boolean }>(`/api/v1/groups/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.groups.all });
    },
  });
}

// useStartGroup 整组启动派发：handler 立即 202 返回，后端后台跑 boot；前端用 onSettled 兜底
// 网络错误也刷新缓存（避免 fetch 抛错导致状态停在点击前的快照）。后续真正的状态变化由
// useGroups 的 transitioning 轮询接力。
export function useStartGroup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) =>
      apiClient<GroupActionResult>(`/api/v1/groups/${id}/start`, { method: 'POST' }),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: queryKeys.groups.all });
    },
  });
}

// useStopGroup 整组停止派发，对称 useStartGroup。
export function useStopGroup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) =>
      apiClient<GroupActionResult>(`/api/v1/groups/${id}/stop`, { method: 'POST' }),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: queryKeys.groups.all });
    },
  });
}
