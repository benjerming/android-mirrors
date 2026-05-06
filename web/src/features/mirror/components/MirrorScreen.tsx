import { LoaderCircle, Play, Square } from 'lucide-react';
import { useEffect, useRef } from 'react';

import { Button } from '@/components/ui/button';
import type { GroupInstance } from '@/generated/types';
import { useStartInstance, useStopInstance } from '@/features/instance/mutations';
import { readSessionToken } from '@/features/session/api';
import { dispatchTap } from '@/lib/control/dispatch';
import { WebSocketMirrorTransport } from '@/lib/mirror/websocket';
import { useMirrorStore } from '@/stores/mirror';

interface MirrorScreenProps {
  instance: GroupInstance;
  groupId: number;
  // resolveTapTargets 由父级注入：根据当前 mode 决定这次点击要 fanout 给哪些实例。
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

  function handleCanvasPointerDown(e: React.PointerEvent<HTMLCanvasElement>) {
    const canvas = canvasRef.current;
    if (!canvas || !canvas.width || !canvas.height) return;
    const rect = canvas.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return;
    // object-contain 在 canvas intrinsic 比例和容器比例不一致时留黑边——按实际显示区域换算坐标。
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
    if (localX < 0 || localY < 0 || localX > displayW || localY > displayH) return;
    const deviceX = (localX / displayW) * canvas.width;
    const deviceY = (localY / displayH) * canvas.height;
    const targets = resolveTapTargets(instance.id);
    if (targets.length === 0) return;
    dispatchTap(targets, deviceX, deviceY);
    onSelect?.(instance.id);
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
            onPointerDown={handleCanvasPointerDown}
            className="h-full w-full object-contain bg-black"
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
