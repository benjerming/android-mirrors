import { expect, test } from '@playwright/test';

// E2E #7：旧 /instances 页面横幅显示（spec §15.2）。
test('legacy /instances page shows deprecation banner and no sidebar entry', async ({ page }) => {
  await page.goto('/login');
  await page.getByLabel('用户名').fill('atlas');
  await page.getByRole('button', { name: '开始使用' }).click();
  await expect(page).toHaveURL(/\/groups$/);

  // 直接访问 /instances 仍可达，但顶部应显示弃用横幅。
  await page.goto('/instances');
  await expect(page.getByTestId('instances-deprecated-banner')).toBeVisible();

  // 左栏不再渲染「实例列表」入口。
  await expect(page.getByText('实例列表')).toHaveCount(0);
});
