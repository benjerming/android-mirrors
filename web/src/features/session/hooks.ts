import { useMutation } from '@tanstack/react-query';
import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';

import { login, clearSessionToken } from '@/features/session/api';
import { startHeartbeat } from '@/features/session/heartbeat';
import type { LoginFormValues } from '@/features/session/schemas';
import { setUnauthorizedHandler } from '@/lib/apiClient';
import { queryClient } from '@/lib/queryClient';

// redirectToLogin 用来把失效会话统一收口到一个跳转动作，避免多处拼接跳转逻辑。
function redirectToLogin() {
  window.location.href = '/login';
}

// resetSessionState 用来把本地 token 和查询缓存一起清干净，防止旧数据残留到下一次登录。
export function resetSessionState() {
  clearSessionToken();
  queryClient.clear();
}

// handleUnauthorized 用来响应后端返回 401 的场景，表示当前令牌已经不能继续使用。
export function handleUnauthorized() {
  resetSessionState();
  redirectToLogin();
}

// logout 用来处理用户主动退出，和 401 一样清理本地状态后回到登录页。
export function logout() {
  resetSessionState();
  redirectToLogin();
}

// useLoginMutation 用来把登录请求、成功跳转和按钮 loading 状态集中在一个 hook 里。
export function useLoginMutation() {
  const navigate = useNavigate();

  return useMutation({
    mutationFn: (values: LoginFormValues) => login(values),
    onSuccess: () => {
      queryClient.clear();
      navigate('/groups', { replace: true });
    },
  });
}

// useSessionLifecycle 用来在应用启动时接上 401 清理 / 多标签页同步 / 心跳续活，保证会话状态一致。
export function useSessionLifecycle() {
  useEffect(() => {
    setUnauthorizedHandler(handleUnauthorized);

    const handleStorage = (event: StorageEvent) => {
      // 只关心 token 被清空的场景，这样其他标签页退出登录时，当前页也会同步回到登录页。
      if (event.key === 'session_token' && !event.newValue) {
        resetSessionState();
        redirectToLogin();
      }
    };

    window.addEventListener('storage', handleStorage);
    const stopHeartbeat = startHeartbeat();

    return () => {
      setUnauthorizedHandler(null);
      window.removeEventListener('storage', handleStorage);
      stopHeartbeat();
    };
  }, []);
}
