import { create } from 'zustand';

import type { MirrorStatus } from '@/lib/mirror/transport';

interface PerInstance {
  status: MirrorStatus;
  errorMessage?: string;
}

interface MirrorStoreState {
  byInstance: Record<number, PerInstance>;
  setStatus: (instanceId: number, status: MirrorStatus, error?: string) => void;
  reset: () => void;
}

// useMirrorStore 按 instanceId 维度记录镜像连接状态，给主屏 / 副屏各自渲染 transitioning 提示。
export const useMirrorStore = create<MirrorStoreState>((set) => ({
  byInstance: {},
  setStatus: (instanceId, status, error) =>
    set((s) => ({
      byInstance: {
        ...s.byInstance,
        [instanceId]: { status, errorMessage: error },
      },
    })),
  reset: () => set({ byInstance: {} }),
}));
