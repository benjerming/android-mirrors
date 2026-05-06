import { useCallback, useEffect, useRef } from 'react';
import { touchFrame, type ControlFrame } from '@/lib/mirror/control-frame';

interface UseTouchGestureOpts {
  // send 由调用方决定 fan-out（broadcast 时把同一帧发给多目标）。
  send: (frame: ControlFrame) => void;
}

interface GestureState {
  active: boolean;
  lastSentX: number;
  lastSentY: number;
  pendingX: number;
  pendingY: number;
  hasPending: boolean;
  rafId: number | null;
}

// useTouchGesture 把 pointerdown/move/up 序列翻译成 down/move/up 控制帧；move 用
// requestAnimationFrame 合并，避免高频 pointermove 把 scrcpy 控制通道淹没。
export function useTouchGesture({ send }: UseTouchGestureOpts) {
  const sendRef = useRef(send);
  useEffect(() => {
    sendRef.current = send;
  }, [send]);

  // ref 保持 state 不触发 re-render；事件回调高频，没必要 setState。
  const stateRef = useRef<GestureState>({
    active: false,
    lastSentX: 0,
    lastSentY: 0,
    pendingX: 0,
    pendingY: 0,
    hasPending: false,
    rafId: null,
  });

  const flushMove = useCallback(() => {
    const s = stateRef.current;
    s.rafId = null;
    if (!s.active || !s.hasPending) return;
    if (s.pendingX === s.lastSentX && s.pendingY === s.lastSentY) {
      s.hasPending = false;
      return;
    }
    s.lastSentX = s.pendingX;
    s.lastSentY = s.pendingY;
    s.hasPending = false;
    sendRef.current(touchFrame('move', s.pendingX, s.pendingY));
  }, []);

  const onPointerDown = useCallback(
    (x: number, y: number) => {
      const s = stateRef.current;
      s.active = true;
      s.lastSentX = x;
      s.lastSentY = y;
      s.pendingX = x;
      s.pendingY = y;
      s.hasPending = false;
      if (s.rafId !== null) {
        cancelAnimationFrame(s.rafId);
        s.rafId = null;
      }
      sendRef.current(touchFrame('down', x, y));
    },
    [],
  );

  const onPointerMove = useCallback(
    (x: number, y: number) => {
      const s = stateRef.current;
      if (!s.active) return;
      s.pendingX = x;
      s.pendingY = y;
      s.hasPending = true;
      if (s.rafId === null) s.rafId = requestAnimationFrame(flushMove);
    },
    [flushMove],
  );

  const endGesture = useCallback((x: number, y: number) => {
    const s = stateRef.current;
    if (!s.active) return;
    s.active = false;
    if (s.rafId !== null) {
      cancelAnimationFrame(s.rafId);
      s.rafId = null;
    }
    s.hasPending = false;
    sendRef.current(touchFrame('up', x, y));
  }, []);

  const onPointerUp = useCallback(
    (x: number, y: number) => endGesture(x, y),
    [endGesture],
  );

  const onPointerCancel = useCallback(() => {
    const s = stateRef.current;
    endGesture(s.pendingX || s.lastSentX, s.pendingY || s.lastSentY);
  }, [endGesture]);

  // 卸载时若手势悬挂，强行 up，防止设备端拖影。
  useEffect(() => {
    return () => {
      const s = stateRef.current;
      if (s.active) {
        s.active = false;
        if (s.rafId !== null) cancelAnimationFrame(s.rafId);
        sendRef.current(touchFrame('up', s.pendingX || s.lastSentX, s.pendingY || s.lastSentY));
      }
    };
  }, []);

  return { onPointerDown, onPointerMove, onPointerUp, onPointerCancel };
}
