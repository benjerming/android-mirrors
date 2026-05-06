import { fetchSessionProfile, readSessionToken } from '@/features/session/api';

const ACTIVE_INTERVAL_MS = 30_000;
const HIDDEN_INTERVAL_MS = 5 * 60_000;

// startHeartbeat 周期性 ping `/session/me` 让后端续活；窗口隐藏时降频。
//
// 后端真正的 /session/heartbeat 端点会在 M13（B-23）落地，那之前先复用 /session/me
// 走同一鉴权链路就够了——失效会被 apiClient 401 全局处理器接管。
export function startHeartbeat(): () => void {
  let timer: ReturnType<typeof setTimeout> | null = null;
  let cancelled = false;

  function schedule(delay: number) {
    if (cancelled) return;
    timer = setTimeout(tick, delay);
  }

  async function tick() {
    if (cancelled) return;
    if (!readSessionToken()) {
      schedule(ACTIVE_INTERVAL_MS);
      return;
    }
    try {
      await fetchSessionProfile();
    } catch {
      // apiClient 已经在 401 时清状态、跳登录；其他错误静默重试。
    }
    const interval = document.visibilityState === 'hidden' ? HIDDEN_INTERVAL_MS : ACTIVE_INTERVAL_MS;
    schedule(interval);
  }

  schedule(ACTIVE_INTERVAL_MS);

  return () => {
    cancelled = true;
    if (timer) clearTimeout(timer);
  };
}
