import { LoaderCircle, Play, Square } from 'lucide-react';
import { useCallback, useEffect, useRef } from 'react';

import { Button } from '@/components/ui/button';
import type { GroupInstance } from '@/generated/types';
import { useStartInstance, useStopInstance } from '@/features/instance/mutations';
import { readSessionToken } from '@/features/session/api';
import { useTouchGesture } from '@/features/mirror/hooks/useTouchGesture';
import type { ControlFrame } from '@/lib/mirror/control-frame';
import { getControlTransport } from '@/lib/mirror/transport-registry';
import { WebSocketMirrorTransport } from '@/lib/mirror/websocket';
import { useMirrorStore } from '@/stores/mirror';

interface MirrorScreenProps {
  instance: GroupInstance;
  groupId: number;
  // resolveTapTargets 由父级注入：根据当前 mode 决定这次手势要 fanout 给哪些实例。
  // broadcast 模式下返回所有 running 实例；single 模式下只返回 [self]。
  resolveTapTargets: (selfId: number) => number[];
  highlighted?: boolean;
  onSelect?: (id: number) => void;
}

export function MirrorScreen({
  instance,
  groupId,
  resolveTapTargets,
  highlighted,
  onSelect,
}: MirrorScreenProps) {
  const startMutation = useStartInstance(groupId);
  const stopMutation = useStopInstance(groupId);
  const busy = startMutation.isPending || stopMutation.isPending;
  const isRunning = instance.status === 'running';
  const setStatus = useMirrorStore((s) => s.setStatus);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);

  useEffect(() => {
    if (!isRunning || !canvasRef.current) return;
    const transport = new WebSocketMirrorTransport();
    const off = transport.onStatusChange((s, err) => setStatus(instance.id, s, err));
    transport.attach(canvasRef.current);
    transport.connect({ instanceId: instance.id, fps: 30, token: readSessionToken() ?? '' });
    return () => {
      off();
      transport.disconnect();
    };
  }, [instance.id, isRunning, setStatus]);

  // send 由 gesture hook 调用：按当前 mode fan-out 到目标实例对应的 WS。
  const send = useCallback(
    (frame: ControlFrame) => {
      const targets = resolveTapTargets(instance.id);
      for (const id of targets) {
        const t = getControlTransport(id);
        if (t) t.sendControl(frame);
      }
    },
    [instance.id, resolveTapTargets],
  );

  const gesture = useTouchGesture({ send });

  // toDeviceCoords 把 canvas 像素坐标按 object-contain 的 letterbox 偏移换算
  // 成镜像分辨率坐标（scrcpy 内部再缩放回设备原生分辨率）。返回 null 表示
  // 手指落在了 letterbox 黑边外，应忽略。
  const toDeviceCoords = useCallback(
    (e: React.PointerEvent<HTMLCanvasElement>): { x: number; y: number } | null => {
      const canvas = canvasRef.current;
      if (!canvas || !canvas.width || !canvas.height) return null;
      const rect = canvas.getBoundingClientRect();
      if (rect.width === 0 || rect.height === 0) return null;
      const containerRatio = rect.width / rect.height;
      const frameRatio = canvas.width / canvas.height;
      let displayW = rect.width;
      let displayH = rect.height;
      let offsetX = 0;
      let offsetY = 0;
      if (containerRatio > frameRatio) {
        displayW = rect.height * frameRatio;
        offsetX = (rect.width - displayW) / 2;
      } else if (containerRatio < frameRatio) {
        displayH = rect.width / frameRatio;
        offsetY = (rect.height - displayH) / 2;
      }
      const localX = e.clientX - rect.left - offsetX;
      const localY = e.clientY - rect.top - offsetY;
      if (localX < 0 || localY < 0 || localX > displayW || localY > displayH) return null;
      return {
        x: (localX / displayW) * canvas.width,
        y: (localY / displayH) * canvas.height,
      };
    },
    [],
  );

  function handleDown(e: React.PointerEvent<HTMLCanvasElement>) {
    const c = toDeviceCoords(e);
    if (!c) return;
    e.currentTarget.setPointerCapture(e.pointerId);
    gesture.onPointerDown(c.x, c.y);
    onSelect?.(instance.id);
  }
  function handleMove(e: React.PointerEvent<HTMLCanvasElement>) {
    const c = toDeviceCoords(e);
    if (!c) return;
    gesture.onPointerMove(c.x, c.y);
  }
  function handleUp(e: React.PointerEvent<HTMLCanvasElement>) {
    const c = toDeviceCoords(e);
    if (c) gesture.onPointerUp(c.x, c.y);
    else gesture.onPointerCancel();
    try {
      e.currentTarget.releasePointerCapture(e.pointerId);
    } catch {
      // 已释放或浏览器不支持时忽略。
    }
  }
  function handleCancel() {
    gesture.onPointerCancel();
  }

  return (
    <div
      data-testid={`mirror-screen-${instance.id}`}
      className={`flex aspect-[9/16] flex-col rounded-xl border bg-stone-950 text-stone-50 ${
        highlighted ? 'border-orange-400 ring-2 ring-orange-400' : 'border-stone-200'
      }`}
    >
      <div className="relative flex flex-1 items-center justify-center overflow-hidden text-center text-xs opacity-80">
        {isRunning ? (
          <canvas
            ref={canvasRef}
            onPointerDown={handleDown}
            onPointerMove={handleMove}
            onPointerUp={handleUp}
            onPointerCancel={handleCancel}
            className="h-full w-full object-contain bg-black touch-none"
          />
        ) : (
          <div className="text-stone-400">
            <p>{instance.language ?? instance.name}</p>
            <p className="mt-1 text-[11px]">未启动</p>
          </div>
        )}
      </div>
      <div className="flex items-center justify-between gap-1 border-t border-white/10 px-2 py-1.5">
        <span className={`text-[10px] ${isRunning ? 'text-emerald-300' : 'text-stone-400'}`}>
          {isRunning ? '● running' : '○ stopped'}
        </span>
        <span className="truncate text-[10px] text-stone-400">{instance.language ?? instance.name}</span>
        <Button
          data-testid={`screen-toggle-${instance.id}`}
          variant="outline"
          size="sm"
          disabled={busy}
          onClick={(e) => {
            e.stopPropagation();
            if (isRunning) stopMutation.mutate(instance.id);
            else startMutation.mutate(instance.id);
          }}
          className="h-6 px-2"
        >
          {busy ? (
            <LoaderCircle className="h-3 w-3 animate-spin" />
          ) : isRunning ? (
            <Square className="h-3 w-3" />
          ) : (
            <Play className="h-3 w-3" />
          )}
        </Button>
      </div>
    </div>
  );
}
