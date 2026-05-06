import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import { createElement, type ReactNode } from 'react';
import { describe, it, expect, vi, afterEach } from 'vitest';

import { useConfigOptions } from '@/features/config/api';

describe('useConfigOptions', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('解析后端 /api/v1/configs/options 返回的 profiles 与 languages', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(
        JSON.stringify({
          profiles: [
            {
              id: 'P1',
              displayName: '中等手机',
              device: 'medium_phone',
              resolution: '1080x1920',
              density: 480,
            },
          ],
          languages: [
            { code: 'zh-CN', label: '简体中文' },
            { code: 'en-US', label: 'English (US)' },
          ],
        }),
        { status: 200 },
      ),
    );

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: qc }, children);
    const { result } = renderHook(() => useConfigOptions(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.profiles).toHaveLength(1);
    expect(result.current.data?.profiles[0].displayName).toBe('中等手机');
    expect(result.current.data?.languages).toHaveLength(2);
    expect(result.current.data?.languages[0].code).toBe('zh-CN');
  });
});
