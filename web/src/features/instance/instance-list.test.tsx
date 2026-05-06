import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { InstancesPage } from '@/pages/InstancesPage';

// F-17（M21）测试焦点：
// - 顶部弃用横幅存在；
// - 列表只读：没有「新建实例」「批量进入镜像」「删除」三个旧入口；
// - 单实例启停按钮保留。

const instancesState = vi.hoisted(() => ({
  data: [
    { id: 1, name: 'emu-01', tag: 'alpha', mode: 'reusable', status: 'running', templateId: 7 },
    { id: 2, name: 'emu-02', tag: 'beta', mode: 'reusable', status: 'stopped', templateId: 7 },
  ],
  isLoading: false,
  isError: false,
}));

vi.mock('@/features/instance/hooks', () => ({
  useInstancesQuery: () => instancesState,
}));

vi.mock('@/features/instance/mutations', () => ({
  useStartInstance: () => ({ isPending: false, mutate: vi.fn() }),
  useStopInstance: () => ({ isPending: false, mutate: vi.fn() }),
}));

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <InstancesPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('InstancesPage (deprecated)', () => {
  beforeEach(() => {
    instancesState.data = [
      { id: 1, name: 'emu-01', tag: 'alpha', mode: 'reusable', status: 'running', templateId: 7 },
      { id: 2, name: 'emu-02', tag: 'beta', mode: 'reusable', status: 'stopped', templateId: 7 },
    ];
  });

  afterEach(() => cleanup());

  it('shows deprecation banner', () => {
    renderPage();
    expect(screen.getByTestId('instances-deprecated-banner').textContent).toContain('已弃用');
  });

  it('does not render create / bulk-mirror / delete entries', () => {
    renderPage();
    expect(screen.queryByText('+ 新建实例')).toBeNull();
    expect(screen.queryByRole('button', { name: /进入镜像/ })).toBeNull();
    expect(screen.queryByText('删除')).toBeNull();
  });

  it('keeps per-instance start/stop toggle for ops', () => {
    renderPage();
    expect(screen.getByTestId('legacy-toggle-1').textContent).toContain('停止');
    expect(screen.getByTestId('legacy-toggle-2').textContent).toContain('启动');
  });
});

