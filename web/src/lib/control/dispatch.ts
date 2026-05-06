import { apiClient } from '@/lib/apiClient';

export function dispatchTap(targetIds: number[], x: number, y: number): void {
  for (const id of targetIds) {
    void apiClient(`/api/v1/instances/${id}/control/tap`, {
      method: 'POST',
      body: JSON.stringify({ x: Math.round(x), y: Math.round(y) }),
    }).catch(() => {});
  }
}
