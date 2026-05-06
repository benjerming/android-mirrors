import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { LoginForm } from '@/features/session/components/LoginForm';
import { SESSION_USERNAME_HISTORY_KEY, SESSION_USERNAME_LAST_USED_KEY } from '@/features/session/storage';

const mutateSpy = vi.fn();

vi.mock('@/features/session/hooks', () => ({
  useLoginMutation: () => ({
    isError: false,
    isPending: false,
    mutate: mutateSpy,
  }),
}));

// renderLoginForm 用来在测试里统一挂载登录表单，减少每个用例重复准备动作。
function renderLoginForm() {
  return render(<LoginForm />);
}

describe('LoginForm', () => {
  beforeEach(() => {
    localStorage.clear();
    mutateSpy.mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  it('prefills the most recently used username and shows username history options', () => {
    localStorage.setItem(SESSION_USERNAME_LAST_USED_KEY, 'atlas');
    localStorage.setItem(SESSION_USERNAME_HISTORY_KEY, JSON.stringify(['atlas', 'beacon']));

    renderLoginForm();

    const usernameInput = screen.getByLabelText('用户名');
    const usernameHistory = document.getElementById('username-history-options');

    expect(usernameInput).toHaveValue('atlas');
    expect(usernameHistory?.querySelector('option[value="atlas"]')).not.toBeNull();
    expect(usernameHistory?.querySelector('option[value="beacon"]')).not.toBeNull();
  });

  it('stores the submitted username after a successful login', async () => {
    localStorage.setItem(SESSION_USERNAME_HISTORY_KEY, JSON.stringify(['beacon', 'atlas']));
    const user = userEvent.setup();

    mutateSpy.mockImplementation((_values, options) => {
      options?.onSuccess?.();
    });

    renderLoginForm();

    const usernameInput = screen.getByLabelText('用户名');
    await user.clear(usernameInput);
    await user.type(usernameInput, 'nova');
    await user.click(screen.getByRole('button', { name: '开始使用' }));

    expect(localStorage.getItem(SESSION_USERNAME_LAST_USED_KEY)).toBe('nova');
    expect(localStorage.getItem(SESSION_USERNAME_HISTORY_KEY)).toBe(JSON.stringify(['nova', 'beacon', 'atlas']));
  });
});
