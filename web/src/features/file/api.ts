import { useMutation } from '@tanstack/react-query';

import { apiClient } from '@/lib/apiClient';

// usePushFile 上传单个文件到指定实例的白名单目录；后端会做严格前缀校验。
export function usePushFile() {
  return useMutation({
    mutationFn: async ({
      instanceId,
      file,
      remotePath,
    }: {
      instanceId: number;
      file: File;
      remotePath: string;
    }) => {
      const form = new FormData();
      form.append('file', file);
      form.append('remotePath', remotePath);
      const token = localStorage.getItem('session_token');
      const res = await fetch(`/api/v1/instances/${instanceId}/files/upload`, {
        method: 'POST',
        headers: token ? { Authorization: `Bearer ${token}` } : undefined,
        body: form,
      });
      if (!res.ok) throw new Error(`上传失败 (${res.status})`);
      return (await res.json()) as { success: boolean; remotePath: string };
    },
  });
}

// useDeleteFile 从设备删除文件。
export function useDeleteFile() {
  return useMutation({
    mutationFn: ({ instanceId, remotePath }: { instanceId: number; remotePath: string }) =>
      apiClient<{ success: boolean }>(`/api/v1/instances/${instanceId}/files`, {
        method: 'DELETE',
        body: JSON.stringify({ remotePath }),
      }),
  });
}
