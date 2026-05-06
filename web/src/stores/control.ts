import { create } from 'zustand';

export type ControlMode = 'broadcast' | 'single';

interface ControlStoreState {
  mode: ControlMode;
  // singleTargetId 在 single 模式下为独控目标实例 id；broadcast 模式忽略。
  singleTargetId: number | null;
  setMode: (mode: ControlMode) => void;
  setSingleTarget: (id: number | null) => void;
  reset: () => void;
}

// useControlStore 维护镜像页的控制模式状态。
//
// spec §10：默认 broadcast；切到 single 时 topbar 变橙色，事件只下发到 singleTargetId。
export const useControlStore = create<ControlStoreState>((set) => ({
  mode: 'broadcast',
  singleTargetId: null,
  setMode: (mode) => set({ mode }),
  setSingleTarget: (id) => set({ singleTargetId: id }),
  reset: () => set({ mode: 'broadcast', singleTargetId: null }),
}));
