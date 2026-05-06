import { defineConfig, devices } from '@playwright/test';

// playwright.config.ts 表示 F-18 的 E2E 配置入口。
//
// 当前阶段只暴露最小可运行配置：
// - testDir 指向 tests/e2e/
// - 默认假设前端 dev server 已在 5173 端口运行；CI 中可结合 webServer 字段拉起后端
//   后再跑（M23 阶段联调时补上）。
// - 浏览器只跑 chromium，避免在 dev / CI 都安装 Firefox / WebKit。
export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: true,
  retries: 0,
  reporter: [['list']],
  use: {
    baseURL: process.env.E2E_BASE_URL ?? 'http://127.0.0.1:5173',
    trace: 'retain-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
});
