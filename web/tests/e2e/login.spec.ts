import { expect, test } from '@playwright/test';

// E2E #1：登录 / 登出（spec §15.2）。
//
// 假设：前后端已经启动，后端无状态登录会自动给任意用户名分配 token。
test('login and reach groups page', async ({ page }) => {
  await page.goto('/login');
  await page.getByLabel('用户名').fill('atlas');
  await page.getByRole('button', { name: '开始使用' }).click();
  await expect(page).toHaveURL(/\/groups$/);
});
