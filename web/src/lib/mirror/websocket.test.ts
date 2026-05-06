import { afterEach, describe, expect, it, vi } from 'vitest';

import { WebSocketMirrorTransport } from '@/lib/mirror/websocket';

// 构造一帧 wire 协议字节：[flags(1) | pts(8) | len(4) | annexB...]
function buildFrame(isKey: boolean, payload: Uint8Array, pts = 0): ArrayBuffer {
  const buf = new ArrayBuffer(13 + payload.byteLength);
  const v = new DataView(buf);
  v.setUint8(0, isKey ? 1 : 0);
  v.setUint32(1, Math.floor(pts / 0x100000000), false);
  v.setUint32(5, pts >>> 0, false);
  v.setUint32(9, payload.byteLength, false);
  new Uint8Array(buf, 13).set(payload);
  return buf;
}

describe('WebSocketMirrorTransport', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    delete (globalThis as Record<string, unknown>).VideoDecoder;
    delete (globalThis as Record<string, unknown>).EncodedVideoChunk;
    delete (globalThis as Record<string, unknown>).WebSocket;
  });

  it('WebCodecs 不支持时 status=error 并附中文提示', () => {
    const t = new WebSocketMirrorTransport();
    const captured: { s?: string; err?: string } = {};
    t.onStatusChange((s, err) => {
      captured.s = s;
      captured.err = err;
    });
    t.connect({ instanceId: 1, fps: 30, token: 'tok' });
    expect(captured.s).toBe('error');
    expect(captured.err).toContain('WebCodecs');
  });

  it('connect 后状态先变成 connecting（在 WebCodecs 可用时）', () => {
    (globalThis as Record<string, unknown>).VideoDecoder = class {};
    class FakeWS {
      onopen: (() => void) | null = null;
      onclose: ((e: { reason: string }) => void) | null = null;
      onerror: (() => void) | null = null;
      onmessage: (() => void) | null = null;
      binaryType = '';
      close() {}
    }
    (globalThis as Record<string, unknown>).WebSocket = FakeWS as unknown as typeof WebSocket;

    const t = new WebSocketMirrorTransport();
    const seen: string[] = [];
    t.onStatusChange((s) => seen.push(s));
    t.connect({ instanceId: 1, fps: 30, token: 'tok' });
    expect(seen).toContain('connecting');
  });

  it('首个 IDR 帧（含 SPS+PPS）会触发 VideoDecoder.configure', () => {
    const configures: Array<{ codec: string }> = [];
    const decodes: unknown[] = [];
    class FakeDecoder {
      decodeQueueSize = 0;
      configure(cfg: { codec: string }) {
        configures.push(cfg);
      }
      decode(chunk: unknown) {
        decodes.push(chunk);
      }
      close() {}
    }
    class FakeChunk {
      constructor(public init: unknown) {}
    }
    (globalThis as Record<string, unknown>).VideoDecoder = FakeDecoder;
    (globalThis as Record<string, unknown>).EncodedVideoChunk = FakeChunk;

    class FakeWS {
      onopen: (() => void) | null = null;
      onclose: ((e: { reason: string }) => void) | null = null;
      onerror: (() => void) | null = null;
      onmessage: ((ev: { data: unknown }) => void) | null = null;
      binaryType = '';
      close() {}
    }
    let lastWs: FakeWS | null = null;
    (globalThis as Record<string, unknown>).WebSocket = function () {
      lastWs = new FakeWS();
      return lastWs;
    } as unknown as typeof WebSocket;

    const t = new WebSocketMirrorTransport();
    t.connect({ instanceId: 1, fps: 30, token: 'tok' });
    expect(lastWs).not.toBeNull();

    // 构造一段最小 Annex-B：SPS(NAL type 7) + PPS(NAL type 8) + IDR(NAL type 5)
    // SPS 必须有 ≥4 字节才能抽 profile/compat/level。
    const sc = [0, 0, 0, 1];
    const sps = [0x67, 0x64, 0x00, 0x28, 0xac, 0xd9]; // type=7
    const pps = [0x68, 0xee, 0x3c, 0x80];             // type=8
    const idr = [0x65, 0x88, 0x80, 0x10];             // type=5
    const annexB = new Uint8Array([...sc, ...sps, ...sc, ...pps, ...sc, ...idr]);
    lastWs!.onmessage?.({ data: buildFrame(true, annexB) });

    expect(configures.length).toBe(1);
    expect(configures[0].codec).toBe('avc1.640028');
    expect(decodes.length).toBe(1);
  });

});
