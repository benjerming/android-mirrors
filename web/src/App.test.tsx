import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import App from "@/App";

// App 路由冒烟测试用来确认阶段 2 的鉴权入口已经生效，未登录时应先看到登录页。
describe("App router shell", () => {
  beforeEach(() => {
    window.history.replaceState({}, "", "/");
    localStorage.clear();
  });

  it("redirects root route to the login page when there is no session token", async () => {
    render(<App />);

    expect(await screen.findByRole("heading", { name: "登录" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "开始使用" })).toBeInTheDocument();
  });
});
