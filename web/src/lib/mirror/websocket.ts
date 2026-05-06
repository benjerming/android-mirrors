import type {
  MirrorConnectOptions,
  MirrorStatus,
  MirrorTransport,
} from '@/lib/mirror/transport';
import type { ControlFrame } from '@/lib/mirror/control-frame';
import {
  registerControlTransport,
  unregisterControlTransport,
} from '@/lib/mirror/transport-registry';

const MAX_RECONNECT_ATTEMPTS = 5;
const BASE_BACKOFF_MS = 1000; // 1s → 2s → 4s → 8s → 16s

// FRAME_HEADER_SIZE 见 transport.ts 的协议定义。
const FRAME_HEADER_SIZE = 13;

function hasWebCodecs(): boolean {
  return typeof globalThis !== 'undefined' && 'VideoDecoder' in globalThis;
}

// splitAnnexBNals 把 Annex-B 字节流切成裸 NAL 数组（去掉 start code）。
function splitAnnexBNals(data: Uint8Array): Uint8Array[] {
  const nals: Uint8Array[] = [];
  const findStartCode = (pos: number): number => {
    for (let i = pos; i < data.length - 3; i++) {
      if (data[i] === 0 && data[i + 1] === 0) {
        if (data[i + 2] === 1) return i;
        if (data[i + 2] === 0 && data[i + 3] === 1) return i;
      }
    }
    return -1;
  };
  const skipStartCode = (pos: number): number => {
    if (data[pos] === 0 && data[pos + 1] === 0 && data[pos + 2] === 0 && data[pos + 3] === 1) {
      return pos + 4;
    }
    if (data[pos] === 0 && data[pos + 1] === 0 && data[pos + 2] === 1) {
      return pos + 3;
    }
    return pos;
  };
  let sc = findStartCode(0);
  if (sc === -1) return nals;
  let start = skipStartCode(sc);
  let pos = start;
  while (pos < data.length) {
    const nextSc = findStartCode(pos);
    if (nextSc === -1) {
      nals.push(data.slice(start));
      break;
    }
    nals.push(data.slice(start, nextSc));
    start = skipStartCode(nextSc);
    pos = start;
  }
  return nals.filter((n) => n.length > 0);
}

// annexBToAvcc 把 Annex-B 转 AVCC（每个 NAL 前加 4 字节大端长度）。WebCodecs H.264 要 AVCC。
function annexBToAvcc(data: Uint8Array): Uint8Array {
  const nals = splitAnnexBNals(data);
  const totalLen = nals.reduce((acc, n) => acc + 4 + n.length, 0);
  const out = new Uint8Array(totalLen);
  const view = new DataView(out.buffer);
  let offset = 0;
  for (const nal of nals) {
    view.setUint32(offset, nal.length, false);
    offset += 4;
    out.set(nal, offset);
    offset += nal.length;
  }
  return out;
}

// buildAvccDescription 用 SPS+PPS 拼 AVCDecoderConfigurationRecord，
// 这个结构 VideoDecoder.configure({ description }) 会消费它来初始化硬件解码器。
function buildAvccDescription(sps: Uint8Array, pps: Uint8Array): ArrayBuffer {
  const buf = new ArrayBuffer(7 + 2 + sps.length + 1 + 2 + pps.length);
  const view = new DataView(buf);
  const u8 = new Uint8Array(buf);
  view.setUint8(0, 1);              // configurationVersion
  view.setUint8(1, sps[1]);         // AVCProfileIndication
  view.setUint8(2, sps[2]);         // profile_compatibility
  view.setUint8(3, sps[3]);         // AVCLevelIndication
  view.setUint8(4, 0xff);           // lengthSizeMinusOne = 3 (4 字节长度前缀)
  view.setUint8(5, 0xe1);           // numSPS | 0xe0
  view.setUint16(6, sps.length, false);
  u8.set(sps, 8);
  let off = 8 + sps.length;
  view.setUint8(off, 1);            // numPPS
  off++;
  view.setUint16(off, pps.length, false);
  off += 2;
  u8.set(pps, off);
  return buf;
}

function buildCodecString(sps: Uint8Array): string {
  const profile = sps[1].toString(16).padStart(2, '0');
  const compat  = sps[2].toString(16).padStart(2, '0');
  const level   = sps[3].toString(16).padStart(2, '0');
  return `avc1.${profile}${compat}${level}`;
}

// WebSocketMirrorTransport 用 WebCodecs 接收裸 H.264：
//   ws message = 13B 头 + Annex-B 帧（首帧 IDR 含 SPS+PPS）
//   首个 IDR → 抽 SPS/PPS → configure decoder
//   后续每帧 → Annex-B → AVCC → decoder.decode → VideoFrame → drawImage to canvas
//
// 解码、渲染都不经过 <video>，所以没有 MSE 的 buffered/live-edge/页面后台节流问题。
export class WebSocketMirrorTransport implements MirrorTransport {
  private status: MirrorStatus = 'idle';
  private statusListeners = new Set<(s: MirrorStatus, err?: string) => void>();
  private socket: WebSocket | null = null;
  private reconnectAttempts = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private currentOpts: MirrorConnectOptions | null = null;
  private canvas: HTMLCanvasElement | null = null;
  private ctx: CanvasRenderingContext2D | null = null;
  private decoder: VideoDecoder | null = null;
  private decoderReady = false;
  private onUnauthorized?: () => void;
  private currentInstanceId: number | null = null;

  constructor(opts?: { onUnauthorized?: () => void }) {
    this.onUnauthorized = opts?.onUnauthorized;
  }

  connect(opts: MirrorConnectOptions): void {
    this.currentOpts = opts;
    this.reconnectAttempts = 0;
    if (!hasWebCodecs()) {
      this.setStatus('error', '浏览器不支持镜像功能（缺少 WebCodecs）');
      return;
    }
    this.openSocket();
  }

  attach(canvas: HTMLCanvasElement): void {
    this.canvas = canvas;
    this.ctx = canvas.getContext('2d');
  }

  disconnect(): void {
    this.currentOpts = null;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.socket) {
      this.socket.close();
      this.socket = null;
    }
    if (this.currentInstanceId !== null) {
      unregisterControlTransport(this.currentInstanceId, this);
      this.currentInstanceId = null;
    }
    this.resetDecoder();
    this.setStatus('closed');
  }

  // sendControl 在 WS OPEN 时把控制帧序列化成 JSON 文本帧发出去。
  // 未 OPEN 时静默丢弃，避免把过期的 move 排队后下发到设备。
  sendControl(frame: ControlFrame): void {
    const ws = this.socket;
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    try {
      ws.send(JSON.stringify(frame));
    } catch {
      // 写失败让 onclose / onerror 走重连，这里不再升级状态。
    }
  }

  onStatusChange(cb: (s: MirrorStatus, err?: string) => void): () => void {
    this.statusListeners.add(cb);
    cb(this.status);
    return () => this.statusListeners.delete(cb);
  }

  private resetDecoder() {
    if (this.decoder) {
      try {
        this.decoder.close();
      } catch {
        // decoder 已经在 error 状态时 close 会抛，忽略。
      }
      this.decoder = null;
    }
    this.decoderReady = false;
  }

  private ensureDecoder(annexB: Uint8Array): boolean {
    if (this.decoderReady && this.decoder) return true;
    const nals = splitAnnexBNals(annexB);
    const sps = nals.find((n) => n.length > 0 && (n[0] & 0x1f) === 7);
    const pps = nals.find((n) => n.length > 0 && (n[0] & 0x1f) === 8);
    if (!sps || !pps) return false;
    const codec = buildCodecString(sps);
    const description = buildAvccDescription(sps, pps);
    this.resetDecoder();
    const decoder = new VideoDecoder({
      output: (frame) => this.drawFrame(frame),
      error: (e) => {
        this.decoderReady = false;
        this.setStatus('error', `解码错误: ${e.message}`);
      },
    });
    try {
      // hardwareAcceleration: 'prefer-software' —— Linux 上 Chrome 的 VA-API H.264
      // 硬解器在 4 路并发或某些 SPS 参数下会输出错位宏块（花屏），强制走软解避开。
      // 代价是 CPU：单路 1080p30 ≈ 5%，群控 4 路 ≈ 20% 一个核，可接受。
      decoder.configure({
        codec,
        description,
        hardwareAcceleration: 'prefer-software',
        optimizeForLatency: true,
      });
    } catch (e) {
      this.setStatus('error', `VideoDecoder configure 失败: ${(e as Error).message}`);
      return false;
    }
    this.decoder = decoder;
    this.decoderReady = true;
    return true;
  }

  private drawFrame(frame: VideoFrame) {
    const canvas = this.canvas;
    const ctx = this.ctx;
    if (!canvas || !ctx) {
      frame.close();
      return;
    }
    if (canvas.width !== frame.displayWidth || canvas.height !== frame.displayHeight) {
      canvas.width = frame.displayWidth;
      canvas.height = frame.displayHeight;
    }
    ctx.drawImage(frame, 0, 0);
    frame.close();
  }

  private handleFrame(buf: ArrayBuffer) {
    if (buf.byteLength < FRAME_HEADER_SIZE) return;
    const view = new DataView(buf);
    const flags = view.getUint8(0);
    const isKey = (flags & 0x01) !== 0;
    // pts_micros 不直接喂解码器（解码顺序由 IDR/AVCC 自己保证），
    // 但作为 EncodedVideoChunk.timestamp 必填字段。
    const ptsHi = view.getUint32(1, false);
    const ptsLo = view.getUint32(5, false);
    const ptsMicros = ptsHi * 0x100000000 + ptsLo;
    const payloadLen = view.getUint32(9, false);
    if (FRAME_HEADER_SIZE + payloadLen !== buf.byteLength) {
      // 长度对不上当作脏帧丢弃；下一帧若是 IDR 自然恢复。
      return;
    }
    const annexB = new Uint8Array(buf, FRAME_HEADER_SIZE, payloadLen);

    if (isKey && !this.ensureDecoder(annexB)) return;
    if (!this.decoderReady || !this.decoder) return;

    // 不再按 decodeQueueSize 丢非关键帧——丢一帧 P 就会断参考链导致 ~20s 花屏，
    // 下个自然 IDR 才能修复。WebCodecs 内部对积压的处理是排队解码、延迟自然增长，
    // 数据完整性永远 > 实时性。延迟过高时由 backend 节流而不是 frontend 丢字节。

    const avcc = annexBToAvcc(annexB);
    try {
      this.decoder.decode(
        new EncodedVideoChunk({
          type: isKey ? 'key' : 'delta',
          timestamp: ptsMicros,
          data: avcc,
        }),
      );
    } catch {
      this.decoderReady = false;
    }
  }

  private openSocket() {
    if (!this.currentOpts) return;
    const { instanceId, fps, token } = this.currentOpts;
    this.currentInstanceId = instanceId;
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
    // fmt=raw 让后端跳过 fMP4 muxer，直接发 13B header + Annex-B。后端 TODO。
    const url = `${proto}://${window.location.host}/api/v1/instances/${instanceId}/mirror/ws?token=${encodeURIComponent(token)}&fps=${fps}&fmt=raw`;
    this.setStatus('connecting');
    try {
      const ws = new WebSocket(url);
      ws.binaryType = 'arraybuffer';
      ws.onopen = () => {
        this.reconnectAttempts = 0;
        registerControlTransport(instanceId, this);
        this.setStatus('open');
      };
      ws.onclose = (ev) => {
        unregisterControlTransport(instanceId, this);
        if (typeof ev.reason === 'string' && ev.reason.includes('auth')) {
          this.setStatus('error', 'auth');
          this.onUnauthorized?.();
          return;
        }
        this.handleDisconnect();
      };
      ws.onerror = () => {
        this.handleDisconnect();
      };
      ws.onmessage = (ev) => {
        if (!(ev.data instanceof ArrayBuffer)) return;
        this.handleFrame(ev.data);
      };
      this.socket = ws;
    } catch {
      this.handleDisconnect();
    }
  }

  private handleDisconnect() {
    if (!this.currentOpts) return;
    // 重连后 SPS/PPS 可能换（设备方向变了），decoder 必须等下一个 IDR 重 configure。
    this.resetDecoder();
    this.reconnectAttempts += 1;
    if (this.reconnectAttempts > MAX_RECONNECT_ATTEMPTS) {
      this.setStatus('error', '连接失败，已超过重试上限');
      return;
    }
    const delay = BASE_BACKOFF_MS * 2 ** (this.reconnectAttempts - 1);
    this.setStatus('reconnecting');
    this.reconnectTimer = setTimeout(() => {
      this.openSocket();
    }, delay);
  }

  private setStatus(s: MirrorStatus, err?: string) {
    this.status = s;
    for (const cb of this.statusListeners) cb(s, err);
  }
}
