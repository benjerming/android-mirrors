import "@testing-library/jest-dom/vitest";

const NativeRequest = globalThis.Request;

// React Router 在测试里做跳转时会构造带 signal 的 Request。
// 但 Node 24 与 jsdom 混用时，signal 的实现来源不同，容易直接抛类型错误。
// 这里仅在测试环境里兜底，把不兼容的 signal 去掉，保证我们测的是路由行为本身。
class RequestWithSafeSignal extends NativeRequest {
  constructor(input: URL | RequestInfo, init?: RequestInit) {
    super(input, init ? { ...init, signal: undefined } : init);
  }
}

globalThis.Request = RequestWithSafeSignal as typeof Request;
