import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { queryKeys } from '@/api/keys';
import { apiClient } from '@/lib/apiClient';

export interface ArtifactDTO {
  id: number;
  type: string;
  originName: string;
  size: number;
  sha256: string;
  packageName?: string;
}

// useApkHistory 返回当前用户已上传的 APK 列表。
export function useApkHistory() {
  return useQuery({
    queryKey: queryKeys.artifacts.apkHistory,
    queryFn: () => apiClient<ArtifactDTO[]>('/api/v1/artifacts'),
    staleTime: 30_000,
  });
}

// useUploadApk 上传 APK；成功后刷新历史。
export function useUploadApk() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (file: File) => {
      const form = new FormData();
      form.append('file', file);
      // multipart 不能用 apiClient 默认 JSON 头，单独走 fetch。
      const token = localStorage.getItem('session_token');
      const res = await fetch('/api/v1/artifacts/upload', {
        method: 'POST',
        headers: token ? { Authorization: `Bearer ${token}` } : undefined,
        body: form,
      });
      if (!res.ok) {
        throw new Error(`上传失败 (${res.status})`);
      }
      return (await res.json()) as ArtifactDTO;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.artifacts.apkHistory }),
  });
}
