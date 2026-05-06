import { ApiError, NetworkError, ServerError, UnauthorizedError } from '@/lib/errors';

type UnauthorizedHandler = (() => void) | null;

let unauthorizedHandler: UnauthorizedHandler = null;
let requestId = 0;

// getUnauthorizedHandler 用来给测试读取当前 401 回调，确认应用级清理逻辑已经接上。
export function getUnauthorizedHandler() {
  return unauthorizedHandler;
}

// setUnauthorizedHandler 用来注册统一的 401 处理器，让请求层只负责上报，页面层决定怎么清理。
export function setUnauthorizedHandler(handler: UnauthorizedHandler) {
  unauthorizedHandler = handler;
}

// pad2 把数字补到两位，给日期/时间字段使用。
function pad2(n: number) {
  return n.toString().padStart(2, '0');
}

// newRequestId 生成形如 "2026-04-30-14-05-09-0007" 的 X-Request-Id：
// 时间戳让日志天然按时间排序，自增序号保证同一秒内多次请求也不冲突。
// requestId 是模块作用域的全局自增计数器，刷新页面会归零——调试时每次会话从 1 数起更直观。
function newRequestId() {
  const d = new Date();
  const ts =
    `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}` +
    `-${pad2(d.getHours())}-${pad2(d.getMinutes())}-${pad2(d.getSeconds())}`;
  requestId += 1;
  return `${requestId.toString().padStart(6, '0')}-${ts}`;
}

// apiClient 表示统一的请求入口，先把常用请求头和错误转换收口到一处。
export async function apiClient<T>(input: string, init?: RequestInit): Promise<T> {
  const token = localStorage.getItem('session_token');
  const reqId = newRequestId();
  const method = init?.method ?? 'GET';
  const start = performance.now();

  console.debug(`[req ${reqId}] → ${method} ${input}`);

  let response: Response;
  try {
    response = await fetch(input, {
      ...init,
      headers: {
        'Content-Type': 'application/json',
        'X-Request-Id': reqId,
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...init?.headers,
      },
    });
  } catch {
    throw new NetworkError();
  }

  const ms = Math.round(performance.now() - start);
  console.debug(`[req ${reqId}] ← ${response.status} ${method} ${input} (${ms}ms)`);

  if (response.status === 401) {
    unauthorizedHandler?.();
    throw new UnauthorizedError();
  }

  if (response.status >= 500) {
    throw new ServerError(await readErrorMessage(response), response.status);
  }

  if (!response.ok) {
    throw new ApiError(await readErrorMessage(response), response.status);
  }

  return (await response.json()) as T;
}

// readErrorMessage 优先解析后端 `{error: "..."}` 结构化字段，回退到原始 body。
async function readErrorMessage(response: Response): Promise<string> {
  const raw = await response.text();
  if (!raw) return '';
  try {
    const parsed = JSON.parse(raw);
    if (parsed && typeof parsed.error === 'string') return parsed.error;
  } catch {
    /* not JSON, fall through */
  }
  return raw;
}
