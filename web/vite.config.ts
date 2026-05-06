import path from "node:path";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

const currentDir = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  plugins: [react()],
  server: {
    // 开发阶段固定监听本机端口，方便 VS Code、浏览器和后端联调脚本都复用同一地址。
    host: "127.0.0.1",
    port: 5173,
    strictPort: true,
    proxy: {
      // 前端保持使用相对路径访问接口，开发时由 Vite 转发给本地 Go 服务。
      "/api": {
        target: "http://127.0.0.1:8080",
        changeOrigin: true,
        ws: true,
      },
    },
  },
  resolve: {
    alias: {
      // @ 用来把 src 目录导入路径写短，减少后续页面层级变深时的相对路径噪音。
      "@": path.resolve(currentDir, "./src"),
    },
  },
  test: {
    // 前端组件测试需要一套浏览器环境，这里让 Vitest 用 jsdom 模拟页面。
    environment: "jsdom",
    setupFiles: "./src/test/setup.ts",
    // E2E 由 Playwright 跑，不让 Vitest 把 tests/e2e 当成单元测试拉起。
    exclude: ["**/node_modules/**", "**/dist/**", "tests/e2e/**"],
  },
});
