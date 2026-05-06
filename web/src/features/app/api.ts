import { useMutation } from '@tanstack/react-query';

import { apiClient } from '@/lib/apiClient';

export interface InstallResult {
  packageName: string;
  localeApplied: boolean;
}

// useInstallApp 给指定实例安装 APK，自动 set-app-locales 由后端处理。
export function useInstallApp() {
  return useMutation({
    mutationFn: ({ instanceId, artifactId }: { instanceId: number; artifactId: number }) =>
      apiClient<InstallResult>(`/api/v1/instances/${instanceId}/apps/install`, {
        method: 'POST',
        body: JSON.stringify({ artifactId }),
      }),
  });
}

export function useUninstallApp() {
  return useMutation({
    mutationFn: ({ instanceId, pkg }: { instanceId: number; pkg: string }) =>
      apiClient<{ success: boolean }>(`/api/v1/instances/${instanceId}/apps/uninstall`, {
        method: 'POST',
        body: JSON.stringify({ package: pkg }),
      }),
  });
}

export function useClearAppCache() {
  return useMutation({
    mutationFn: ({ instanceId, pkg }: { instanceId: number; pkg: string }) =>
      apiClient<{ success: boolean }>(`/api/v1/instances/${instanceId}/apps/clear-cache`, {
        method: 'POST',
        body: JSON.stringify({ package: pkg }),
      }),
  });
}
