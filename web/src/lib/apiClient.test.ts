import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import { apiClient, setUnauthorizedHandler } from '@/lib/apiClient';
import { NetworkError, ServerError, UnauthorizedError } from '@/lib/errors';

describe('apiClient error mapping', () => {
  beforeEach(() => {
    localStorage.clear();
    setUnauthorizedHandler(null);
  });
  afterEach(() => {
    vi.restoreAllMocks();
    setUnauthorizedHandler(null);
  });

  it('401 触发 unauthorizedHandler 并抛 UnauthorizedError', async () => {
    const handler = vi.fn();
    setUnauthorizedHandler(handler);
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('', { status: 401 }),
    );
    await expect(apiClient('/api/v1/foo')).rejects.toBeInstanceOf(UnauthorizedError);
    expect(handler).toHaveBeenCalledOnce();
  });

  it('4xx 抛 ApiError 并解析后端 error 字段作为 message', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: 'name taken' }), { status: 409 }),
    );
    await expect(apiClient('/api/v1/foo')).rejects.toMatchObject({
      name: 'ApiError',
      status: 409,
      message: 'name taken',
    });
  });

  it('5xx 抛 ServerError', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('', { status: 502 }));
    await expect(apiClient('/api/v1/foo')).rejects.toBeInstanceOf(ServerError);
  });

  it('fetch reject 抛 NetworkError', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValue(new TypeError('Failed to fetch'));
    await expect(apiClient('/api/v1/foo')).rejects.toBeInstanceOf(NetworkError);
  });
});
