import { useMutation, useQueryClient } from '@tanstack/react-query';

import { queryKeys } from '@/api/keys';
import { startInstance, stopInstance } from '@/features/instance/api';

// useStartInstance / useStopInstance 启停单台实例，成功后刷新所属分组详情与全局列表。
export function useStartInstance(groupId?: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => startInstance(id),
    onSuccess: () => {
      if (groupId !== undefined) qc.invalidateQueries({ queryKey: queryKeys.groups.detail(groupId) });
      qc.invalidateQueries({ queryKey: queryKeys.groups.all });
    },
  });
}

export function useStopInstance(groupId?: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => stopInstance(id),
    onSuccess: () => {
      if (groupId !== undefined) qc.invalidateQueries({ queryKey: queryKeys.groups.detail(groupId) });
      qc.invalidateQueries({ queryKey: queryKeys.groups.all });
    },
  });
}
