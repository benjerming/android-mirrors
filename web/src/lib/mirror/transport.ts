// MirrorTransport 抽象镜像传输层（WebCodecs 模式）。
//
// 协议：后端推 H.264 帧，每条 WS binary message = 一帧，13 字节定长头 + Annex-B payload：
//   offset 0   1B  flags         bit0=isKey
//   offset 1   8B  pts_micros    uint64 BE
//   offset 9   4B  payload_len   uint32 BE（冗余，便于断帧校验）
//   offset 13  N   annex_b_data  含 start code 的完整帧；首帧（IDR）必须包含 SPS+PPS
//
// 前端从首个 IDR 抽 SPS/PPS 配 VideoDecoder，后续 EncodedVideoChunk 直接喂解码器，
// 解码出的 VideoFrame 画到 <canvas>。绕开 MSE 的 buffered/live-edge/离屏节流，
// 群控 N 路下不会出现「点击发出但镜像帧停滞」的症状。
//
// 上层组件只依赖该接口，便于未来切换 transport（WebRTC 等）。

export type MirrorStatus =
  | 'idle'
  | 'connecting'
  | 'open'
  | 'reconnecting'
  | 'closed'
  | 'error';

export interface MirrorConnectOptions {
  instanceId: number;
  fps: number;
  token: string;
}

export interface MirrorTransport {
  connect(opts: MirrorConnectOptions): void;
  disconnect(): void;
  // attach 把目标 <canvas> 关联到 transport；解码出的 VideoFrame 会画到上面。
  // 调用顺序与 connect 解耦——canvas 在 React 渲染后才存在。
  attach(canvas: HTMLCanvasElement): void;
  onStatusChange(cb: (status: MirrorStatus, error?: string) => void): () => void;
}
