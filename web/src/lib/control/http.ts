import { apiClient } from '@/lib/apiClient';
import type { ControlChannel, ControlEvent } from '@/lib/control/channel';

const BATCH_WINDOW_MS = 16;

interface HttpControlChannelOptions {
  // targetIds 决定每次 flush 时事件被 fan-out 到哪些实例。
  // broadcast 模式 = 整组实例 id；single 模式 = 单台实例 id 数组。
  getTargets: () => number[];
}

// HttpControlChannel 把事件累积到一个 16ms 窗口内统一 fan-out。
export class HttpControlChannel implements ControlChannel {
  private queue: ControlEvent[] = [];
  private timer: ReturnType<typeof setTimeout> | null = null;
  private getTargets: () => number[];

  constructor(opts: HttpControlChannelOptions) {
    this.getTargets = opts.getTargets;
  }

  send(event: ControlEvent): void {
    this.queue.push(event);
    if (this.timer) return;
    this.timer = setTimeout(() => {
      this.timer = null;
      // 触发即解锁下一个窗口；await 留给调用方按需。
      void this.flush();
    }, BATCH_WINDOW_MS);
  }

  async flush(): Promise<void> {
    const batch = this.queue;
    if (batch.length === 0) return;
    this.queue = [];
    const targets = this.getTargets();
    if (targets.length === 0) return;
    // 简单实现：把队列里每个事件按顺序对每个目标 POST 一次；后端 spec §3.3.3 的 6 个端点。
    for (const event of batch) {
      const path = endpointFor(event);
      const body = bodyFor(event);
      await Promise.allSettled(
        targets.map((id) =>
          apiClient(`/api/v1/instances/${id}/control/${path}`, {
            method: 'POST',
            body: JSON.stringify(body),
          }),
        ),
      );
    }
  }
}

function endpointFor(e: ControlEvent): string {
  switch (e.kind) {
    case 'tap':
      return 'tap';
    case 'swipe':
      return 'swipe';
    case 'text':
      return 'text';
    case 'key':
      return 'key';
  }
}

function bodyFor(e: ControlEvent): Record<string, unknown> {
  switch (e.kind) {
    case 'tap':
      return { x: e.x, y: e.y };
    case 'swipe':
      return { x1: e.x1, y1: e.y1, x2: e.x2, y2: e.y2, durationMs: e.durationMs };
    case 'text':
      return { text: e.text };
    case 'key':
      return { code: e.code };
  }
}
