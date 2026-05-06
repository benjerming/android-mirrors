import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it } from 'vitest';

import { GroupRow } from '@/features/group/components/GroupRow';
import type { GroupSummary } from '@/generated/types';

function renderRow(group: GroupSummary) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <GroupRow group={group} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const baseGroup: GroupSummary = {
  id: 1,
  name: '组A',
  profileId: 'medium_phone',
  profileDisplayName: '中等手机',
  instanceCount: 5,
  runningCount: 0,
  errorCount: 0,
  aggregateState: 'all_stopped',
};

describe('GroupRow', () => {
  afterEach(() => cleanup());

  it('all_stopped 状态下显示「全部启动」按钮，且「进入镜像」按钮被禁用', () => {
    renderRow(baseGroup);
    expect(screen.getByTestId('primary-action').textContent).toContain('全部启动');
    expect(screen.getByTestId('enter-mirror').getAttribute('aria-disabled')).toBe('true');
  });

  it('partial 状态下主按钮显示「启动剩余」', () => {
    renderRow({ ...baseGroup, aggregateState: 'partial', runningCount: 2 });
    expect(screen.getByTestId('primary-action').textContent).toContain('启动剩余');
    expect(screen.getByTestId('aggregate-state').textContent).toContain('2/5');
  });

  it('all_running 状态下主按钮显示「全部停止」', () => {
    renderRow({ ...baseGroup, aggregateState: 'all_running', runningCount: 5 });
    expect(screen.getByTestId('primary-action').textContent).toContain('全部停止');
  });
});
