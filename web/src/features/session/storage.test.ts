import { describe, expect, it } from 'vitest';

import { getStoredUsernameHistory, getStoredUsernameLastUsed, storeUsernameAfterLogin } from '@/features/session/storage';

describe('session username storage', () => {
  it('returns empty defaults when there is no stored username history', () => {
    expect(getStoredUsernameLastUsed()).toBe('');
    expect(getStoredUsernameHistory()).toEqual([]);
  });

  it('moves the latest successful username to the front without duplicates', () => {
    localStorage.setItem('session_username_history', JSON.stringify(['beacon', 'atlas', 'beacon']));

    storeUsernameAfterLogin('atlas');

    expect(getStoredUsernameLastUsed()).toBe('atlas');
    expect(getStoredUsernameHistory()).toEqual(['atlas', 'beacon']);
  });
});
