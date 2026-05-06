import { beforeEach, describe, expect, it, vi } from 'vitest';

import { login } from '@/features/session/api';
import { getUnauthorizedHandler, setUnauthorizedHandler } from '@/lib/apiClient';
import { queryClient } from '@/lib/queryClient';

describe('session flow', () => {
  beforeEach(() => {
    localStorage.clear();
    queryClient.clear();
    vi.unstubAllGlobals();
    setUnauthorizedHandler(null);
  });

  it('stores token from login response', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(new Response(JSON.stringify({ token: 'abc' }), { status: 200 })),
    );

    await login({ username: 'atlas' });

    expect(localStorage.getItem('session_token')).toBe('abc');
  });

  it('runs unauthorized handler when request returns 401', async () => {
    const unauthorizedHandler = vi.fn();
    setUnauthorizedHandler(unauthorizedHandler);
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 401 })));

    await expect(login({ username: 'atlas' })).rejects.toThrowError();

    expect(getUnauthorizedHandler()).toBe(unauthorizedHandler);
    expect(unauthorizedHandler).toHaveBeenCalledTimes(1);
  });
});
