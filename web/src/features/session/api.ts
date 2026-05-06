import { apiClient } from '@/lib/apiClient';

import type { LoginFormValues } from '@/features/session/schemas';

// SESSION_TOKEN_KEY 表示浏览器里保存登录令牌的固定键名，避免各处手写字符串。
export const SESSION_TOKEN_KEY = 'session_token';

// SessionLoginResponse 表示登录接口返回的数据结构，当前只需要后端发回 token。
export type SessionLoginResponse = {
  token: string;
};

// SessionProfile 表示会话自检接口返回的数据结构，阶段 2 只用它判断令牌是否仍然有效。
export type SessionProfile = {
  username?: string;
};

// readSessionToken 用来读取当前浏览器里保存的登录令牌，给路由守卫和请求层复用。
export function readSessionToken() {
  return localStorage.getItem(SESSION_TOKEN_KEY);
}

// storeSessionToken 用来把登录成功后的令牌写入浏览器，方便页面刷新后继续保持登录状态。
export function storeSessionToken(token: string) {
  localStorage.setItem(SESSION_TOKEN_KEY, token);
}

// clearSessionToken 用来清掉本地登录状态，通常在主动退出或令牌失效时调用。
export function clearSessionToken() {
  localStorage.removeItem(SESSION_TOKEN_KEY);
}

// login 用来向后端申请新会话，并在成功后立刻把 token 持久化到浏览器。
export async function login(input: LoginFormValues) {
  const result = await apiClient<SessionLoginResponse>('/api/v1/session/login', {
    method: 'POST',
    body: JSON.stringify(input),
  });

  storeSessionToken(result.token);

  return result;
}

// fetchSessionProfile 用来校验当前 token 是否还能被后端接受，供受保护路由在进入页面前检查。
export function fetchSessionProfile() {
  return apiClient<SessionProfile>('/api/v1/session/me');
}
