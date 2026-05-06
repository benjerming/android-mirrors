export const SESSION_USERNAME_HISTORY_KEY = 'session_username_history';
export const SESSION_USERNAME_LAST_USED_KEY = 'session_username_last_used';
export const SESSION_USERNAME_HISTORY_LIMIT = 8;

// getStoredUsernameHistory 用来读取浏览器里记住的用户名列表，给登录页下拉候选使用。
export function getStoredUsernameHistory() {
  const rawValue = localStorage.getItem(SESSION_USERNAME_HISTORY_KEY);

  if (!rawValue) {
    return [];
  }

  try {
    const parsedValue = JSON.parse(rawValue);

    if (!Array.isArray(parsedValue)) {
      return [];
    }

    return parsedValue
      .filter((item): item is string => typeof item === 'string')
      .map((item) => item.trim())
      .filter(Boolean)
      .filter((item, index, items) => items.indexOf(item) === index)
      .slice(0, SESSION_USERNAME_HISTORY_LIMIT);
  } catch {
    return [];
  }
}

// getStoredUsernameLastUsed 用来读取最近一次成功登录的用户名，方便下次打开时自动回填。
export function getStoredUsernameLastUsed() {
  return localStorage.getItem(SESSION_USERNAME_LAST_USED_KEY)?.trim() ?? '';
}

// storeUsernameAfterLogin 用来在登录成功后刷新本地历史，让最新用户名排在最前面且不重复。
export function storeUsernameAfterLogin(username: string) {
  const normalizedUsername = username.trim();

  if (!normalizedUsername) {
    return;
  }

  const nextHistory = [normalizedUsername, ...getStoredUsernameHistory().filter((item) => item !== normalizedUsername)].slice(
    0,
    SESSION_USERNAME_HISTORY_LIMIT,
  );

  localStorage.setItem(SESSION_USERNAME_LAST_USED_KEY, normalizedUsername);
  localStorage.setItem(SESSION_USERNAME_HISTORY_KEY, JSON.stringify(nextHistory));
}
