// ControlChannel 抽象镜像页 → 后端控制层（spec §10）。
//
// 设计要点：
// - 16ms（一帧）窗口内多次 send 合并成 1 次 fetch（rAF 节流）；
// - 模式 = broadcast 时 fan-out 到当前分组全部实例；single 时只下发到独控目标。

export type ControlEvent =
  | { kind: 'tap'; x: number; y: number }
  | { kind: 'swipe'; x1: number; y1: number; x2: number; y2: number; durationMs: number }
  | { kind: 'text'; text: string }
  | { kind: 'key'; code: number };

export interface ControlChannel {
  send(event: ControlEvent): void;
  flush(): Promise<void>;
}
