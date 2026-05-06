import { expect, test } from '@playwright/test';

// E2E #6：从分组卡片「进入镜像」按钮跳转 /groups/:id/mirror（spec §15.2）。
//
// 假设：测试运行时 atlas 用户至少有一个 running 分组；CI 联调阶段会通过 fixture 准备数据。
test('clicking enter-mirror navigates to mirror page', async ({ page }) => {
  await page.goto('/login');
  await page.getByLabel('用户名').fill('atlas');
  await page.getByRole('button', { name: '开始使用' }).click();
  await expect(page).toHaveURL(/\/groups$/);

  const enter = page.getByTestId('enter-mirror').first();
  if (await enter.count() === 0) {
    test.skip(true, '当前账户没有可镜像的分组，需 fixture seeding');
  }
  await enter.click();
  await expect(page).toHaveURL(/\/groups\/\d+\/mirror$/);
});
