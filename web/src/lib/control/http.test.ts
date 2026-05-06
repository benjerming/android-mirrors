import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { HttpControlChannel } from '@/lib/control/http';

describe('HttpControlChannel', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('16ms 窗口内多次 send 合并 fan-out 到目标', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('{"success":true}', { status: 200 }),
    );
    const ch = new HttpControlChannel({ getTargets: () => [1, 2] });
    ch.send({ kind: 'tap', x: 10, y: 20 });
    ch.send({ kind: 'tap', x: 30, y: 40 });

    await vi.advanceTimersByTimeAsync(20);
    // 2 events × 2 targets = 4 fetches.
    expect(fetchSpy).toHaveBeenCalledTimes(4);
  });

  it('targets 为空不发出请求', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch');
    const ch = new HttpControlChannel({ getTargets: () => [] });
    ch.send({ kind: 'tap', x: 1, y: 2 });
    await vi.advanceTimersByTimeAsync(20);
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it('single 模式只发到一个目标', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('{"success":true}', { status: 200 }),
    );
    const ch = new HttpControlChannel({ getTargets: () => [42] });
    ch.send({ kind: 'key', code: 4 });
    await vi.advanceTimersByTimeAsync(20);
    expect(fetchSpy).toHaveBeenCalledTimes(1);
    const url = (fetchSpy.mock.calls[0][0] as string) ?? '';
    expect(url).toContain('/api/v1/instances/42/control/key');
  });
});
