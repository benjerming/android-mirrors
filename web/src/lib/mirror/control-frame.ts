// control-frame 描述镜像 WS 客户端 → 服务端的控制帧。
// 协议见 docs/superpowers/specs/2026-05-06-mirror-swipe-design.md §2.3。

export type TouchAction = 'down' | 'move' | 'up';

export interface TouchControlFrame {
  type: 'touch';
  action: TouchAction;
  x: number;
  y: number;
  pressure: number;
}

export type ControlFrame = TouchControlFrame;

export function touchFrame(action: TouchAction, x: number, y: number): TouchControlFrame {
  return {
    type: 'touch',
    action,
    x: Math.round(x),
    y: Math.round(y),
    pressure: action === 'up' ? 0 : 1,
  };
}
