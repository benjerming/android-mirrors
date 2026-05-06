// transport-registry 是镜像 WS 控制帧 fan-out 的查找表。
// MirrorScreen 在 broadcast 模式下根据 instanceId 取到每条 WS 的 sendControl。

export interface ControlSendingTransport {
  sendControl(frame: unknown): void;
}

const registry = new Map<number, ControlSendingTransport>();

export function registerControlTransport(instanceId: number, t: ControlSendingTransport): void {
  registry.set(instanceId, t);
}

// 仅当当前注册的就是本对象时才注销，避免重连时旧 transport 的迟到 close 把新 transport 顶掉。
export function unregisterControlTransport(instanceId: number, t: ControlSendingTransport): void {
  if (registry.get(instanceId) === t) {
    registry.delete(instanceId);
  }
}

export function getControlTransport(instanceId: number): ControlSendingTransport | undefined {
  return registry.get(instanceId);
}

export function __clearControlTransportRegistryForTests(): void {
  registry.clear();
}
