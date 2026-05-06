import { useMutation, useQuery } from '@tanstack/react-query';
import { toast } from 'sonner';

import { queryKeys } from '@/api/keys';
import {
  createInstance,
  deleteInstance,
  listInstances,
  listTemplates,
  type CreateInstanceInput,
} from '@/features/instance/api';
import { ApiError } from '@/lib/errors';
import { queryClient } from '@/lib/queryClient';

// useInstancesQuery 用来读取实例列表，并把刷新节奏统一交给 TanStack Query 管理。
export function useInstancesQuery() {
  return useQuery({
    queryKey: queryKeys.instances.all,
    queryFn: listInstances,
  });
}

// useTemplatesQuery 用来读取模板列表，给新建实例弹窗提供下拉数据源。
export function useTemplatesQuery() {
  return useQuery({
    queryKey: queryKeys.templates,
    queryFn: listTemplates,
  });
}

// useCreateInstanceMutation 用来集中处理建机请求、成功提示和列表刷新。
export function useCreateInstanceMutation() {
  return useMutation({
    mutationFn: (input: CreateInstanceInput) => createInstance(input),
    onSuccess: () => {
      toast.success('实例已创建，稍后就会出现在列表里。');
      void queryClient.invalidateQueries({ queryKey: queryKeys.instances.all });
    },
    onError: (error) => {
      toast.error(error instanceof ApiError ? error.message || '创建实例失败，请稍后重试。' : '创建实例失败，请稍后重试。');
    },
  });
}

// useDeleteInstanceMutation 用来处理删除实例请求、成功提示和列表刷新。
export function useDeleteInstanceMutation() {
  return useMutation({
    mutationFn: (id: number) => deleteInstance(id),
    onSuccess: () => {
      toast.success('实例已删除。');
      void queryClient.invalidateQueries({ queryKey: queryKeys.instances.all });
    },
    onError: (error) => {
      toast.error(error instanceof ApiError ? error.message || '删除实例失败，请稍后重试。' : '删除实例失败，请稍后重试。');
    },
  });
}
