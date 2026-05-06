import { describe, expect, it, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useTouchGesture } from '@/features/mirror/hooks/useTouchGesture';
import type { ControlFrame } from '@/lib/mirror/control-frame';

function makeFakeRaf() {
  const cbs: FrameRequestCallback[] = [];
  const raf = (cb: FrameRequestCallback) => {
    cbs.push(cb);
    return cbs.length;
  };
  const caf = () => {};
  const flush = () => {
    const batch = cbs.splice(0);
    for (const cb of batch) cb(performance.now());
  };
  return { raf, caf, flush };
}

describe('useTouchGesture', () => {
  let frames: ControlFrame[];
  let send: (f: ControlFrame) => void;
  let fakeRaf: ReturnType<typeof makeFakeRaf>;
  let origRaf: typeof globalThis.requestAnimationFrame;
  let origCaf: typeof globalThis.cancelAnimationFrame;

  beforeEach(() => {
    frames = [];
    send = (f) => frames.push(f);
    fakeRaf = makeFakeRaf();
    origRaf = globalThis.requestAnimationFrame;
    origCaf = globalThis.cancelAnimationFrame;
    globalThis.requestAnimationFrame = fakeRaf.raf as typeof globalThis.requestAnimationFrame;
    globalThis.cancelAnimationFrame = fakeRaf.caf as typeof globalThis.cancelAnimationFrame;
  });

  afterEach(() => {
    globalThis.requestAnimationFrame = origRaf;
    globalThis.cancelAnimationFrame = origCaf;
  });

  it('down → move(rAF coalesced) → up emits one down, one move per frame, one up', () => {
    const { result } = renderHook(() => useTouchGesture({ send }));

    act(() => result.current.onPointerDown(10, 20));
    act(() => result.current.onPointerMove(11, 21));
    act(() => result.current.onPointerMove(12, 22));
    act(() => result.current.onPointerMove(13, 23));
    act(() => fakeRaf.flush());
    act(() => result.current.onPointerUp(13, 23));

    expect(frames.map((f) => f.action)).toEqual(['down', 'move', 'up']);
    expect(frames[1]).toMatchObject({ x: 13, y: 23 });
  });

  it('cancel between down and up emits up at last position', () => {
    const { result } = renderHook(() => useTouchGesture({ send }));
    act(() => result.current.onPointerDown(10, 20));
    act(() => result.current.onPointerMove(50, 60));
    act(() => fakeRaf.flush());
    act(() => result.current.onPointerCancel());

    const last = frames[frames.length - 1];
    expect(last.action).toBe('up');
    expect(last).toMatchObject({ x: 50, y: 60, pressure: 0 });
  });

  it('move without prior down is ignored', () => {
    const { result } = renderHook(() => useTouchGesture({ send }));
    act(() => result.current.onPointerMove(1, 2));
    act(() => fakeRaf.flush());
    expect(frames).toEqual([]);
  });

  it('move that does not change position is not re-sent', () => {
    const { result } = renderHook(() => useTouchGesture({ send }));
    act(() => result.current.onPointerDown(10, 20));
    act(() => result.current.onPointerMove(10, 20));
    act(() => fakeRaf.flush());
    act(() => fakeRaf.flush());
    act(() => result.current.onPointerUp(10, 20));
    expect(frames.map((f) => f.action)).toEqual(['down', 'up']);
  });
});
