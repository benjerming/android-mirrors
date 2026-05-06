import { expect, test } from '@playwright/test';

// E2E #2 + #3：分组列表展示 + 新建分组 Dialog（spec §15.2）。
//
// 真实运行需要 configs/options 已经加载；假设已登录态由 login.spec.ts 之前的步骤建立，
// 当前 spec 直接通过 token localStorage 注入。
test('groups page renders and create dialog opens', async ({ page }) => {
  await page.goto('/login');
  await page.getByLabel('用户名').fill('atlas');
  await page.getByRole('button', { name: '开始使用' }).click();
  await expect(page).toHaveURL(/\/groups$/);

  // 空态或列表都允许；只断言「新建分组」按钮可见即可（不实际创建避免后端清理负担）。
  await expect(page.getByTestId('open-create-group')).toBeVisible();
  await page.getByTestId('open-create-group').click();
  await expect(page.getByText('新建分组')).toBeVisible();
});
