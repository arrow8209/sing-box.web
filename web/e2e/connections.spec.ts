import { expect, test, type Page } from "@playwright/test";

async function login(page: Page): Promise<void> {
  await page.goto("/");
  const loginButton = page.getByRole("button", { name: "登录" });
  if (await loginButton.isVisible()) {
    await page.getByLabel("管理密码").fill("correct horse battery staple");
    await Promise.all([page.waitForURL((url) => url.pathname === "/"), loginButton.click()]);
  }
}

test("查看、筛选并关闭活动连接", async ({ page }) => {
  const fixture = {
    id: "e2e-connection-1",
    metadata: {
      type: "tun",
      network: "tcp",
      host: "api.example.test",
      sourceIP: "172.16.1.20",
      sourcePort: "51820",
      destinationIP: "203.0.113.10",
      destinationPort: "443",
      process: "browser",
      processPath: "/usr/bin/browser"
    },
    upload: 2048,
    download: 8192,
    start: "2026-08-31T06:00:00Z",
    chains: ["香港 01", "🚀 节点选择"],
    rule: "rule_set",
    rulePayload: "GeoLocation-!CN"
  };
  let active = [fixture];
  await page.route(/\/api\/v1\/runtime\/connections(?:\/[^/?]+)?(?:\?.*)?$/, async (route) => {
    const request = route.request();
    if (request.method() === "DELETE") {
      active = request.url().endsWith("/runtime/connections") ? [] : active.filter((connection) => !request.url().endsWith(`/${connection.id}`));
      await route.fulfill({ status: 204, body: "" });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ memory: 12 * 1024 * 1024, uploadTotal: 4096, downloadTotal: 16384, connections: active })
    });
  });

  await login(page);
  await page.locator('a.metric-card-link[href="/connections"]').click();
  await expect(page.getByRole("heading", { name: "活动连接", exact: true })).toBeVisible();
  await expect(page.getByText("api.example.test", { exact: true })).toBeVisible();
  await expect(page.getByText("GeoLocation-!CN", { exact: false })).toBeVisible();

  await page.getByLabel("搜索连接").fill("browser");
  await expect(page.locator(".connections-table tbody tr")).toHaveCount(1);
  await page.getByRole("button", { name: "查看 api.example.test 的连接详情" }).click();
  const details = page.getByRole("dialog", { name: "连接详情" });
  await expect(details).toContainText("203.0.113.10:443");
  await expect(details).toContainText("/usr/bin/browser");
  await details.getByRole("button", { name: "关闭", exact: true }).click();

  await page.getByLabel("搜索连接").fill("");
  await page.getByRole("button", { name: "关闭 api.example.test 的连接" }).click();
  await expect(page.locator(".connections-table tbody tr")).toHaveCount(1);
  await expect(page.locator(".connection-state.closed")).toHaveText("已关闭");
  await expect(page.getByRole("button", { name: "关闭 api.example.test 的连接" })).toHaveCount(0);
  await page.getByRole("tab", { name: /^活动/ }).click();
  await expect(page.getByText("当前没有活动连接")).toBeVisible();
  await page.getByRole("tab", { name: /已关闭/ }).click();
  await expect(page.getByText("api.example.test", { exact: true })).toBeVisible();
});

test("历史按配置时间过期，且只在活动连接页面打开期间记录", async ({ page }) => {
  const fixture = {
    id: "history-connection",
    metadata: { type: "mixed", network: "tcp", host: "history.example.test", sourceIP: "127.0.0.1", destinationIP: "203.0.113.12", destinationPort: "443" },
    upload: 1024, download: 4096, start: "2026-08-31T06:00:00Z", chains: ["direct"], rule: "final", rulePayload: ""
  };
  let active = [fixture];
  let requests = 0;
  await page.route("**/api/v1/runtime/connections", async (route) => {
    requests += 1;
    await route.fulfill({ json: { memory: 1024, uploadTotal: 1024, downloadTotal: 4096, connections: active } });
  });
  await login(page);
  await page.clock.install();
  await page.clock.fastForward(2500);
  expect(requests).toBe(0);
  await page.locator('a.metric-card-link[href="/connections"]').click();
  await expect(page.getByRole("tab", { name: /^全部/ })).toHaveAttribute("aria-selected", "true");
  await expect(page.locator(".connection-state")).toHaveText("活动");
  active = [];
  await page.getByRole("button", { name: "立即刷新连接" }).click();
  await expect(page.locator(".connection-state.closed")).toHaveText("已关闭");
  await page.getByRole("button", { name: "暂停实时刷新" }).click();
  await page.clock.fastForward(179_000);
  await expect(page.getByText("history.example.test", { exact: true })).toBeVisible();
  await page.clock.fastForward(2000);
  await expect(page.getByText("当前没有连接记录")).toBeVisible();

  active = [fixture];
  await page.getByRole("button", { name: "继续实时刷新" }).click();
  await expect(page.locator(".connection-state")).toHaveText("活动");
  active = [];
  await page.getByRole("button", { name: "立即刷新连接" }).click();
  await expect(page.locator(".connection-state.closed")).toHaveText("已关闭");
  await page.getByRole("button", { name: "连接列表设置", exact: true }).click();
  const settings = page.getByRole("dialog", { name: "连接列表设置" });
  const retention = settings.getByRole("spinbutton", { name: "历史保留时间（分钟）" });
  await expect(retention).toBeFocused();
  await expect(retention).toHaveValue("3");
  await retention.fill("5");
  await settings.getByRole("button", { name: "保存设置" }).click();
  await expect(page.locator(".history-summary")).toContainText("5 分钟");
  await page.getByRole("button", { name: "暂停实时刷新" }).click();
  await page.clock.fastForward(181_000);
  await expect(page.getByText("history.example.test", { exact: true })).toBeVisible();

  await page.getByRole("navigation", { name: "主导航" }).getByRole("link", { name: "仪表盘", exact: true }).click();
  await expect(page.getByRole("heading", { name: "仪表盘", exact: true })).toBeVisible();
  const requestsAfterLeaving = requests;
  active = [{ ...fixture, id: "not-observed", metadata: { ...fixture.metadata, host: "unseen.example.test" } }];
  await page.clock.fastForward(5000);
  active = [];
  await page.clock.fastForward(5000);
  expect(requests).toBe(requestsAfterLeaving);
  await page.goBack();
  await expect(page.getByText("当前没有连接记录")).toBeVisible();
  await expect(page.getByRole("tab", { name: /^已关闭/ })).toHaveText("已关闭0");
  await expect(page.locator(".history-summary")).toContainText("5 分钟");

  active = [fixture];
  await page.getByRole("button", { name: "立即刷新连接" }).click();
  await expect(page.locator(".connection-state")).toHaveText("活动");
  active = [];
  await page.getByRole("button", { name: "立即刷新连接" }).click();
  await expect(page.locator(".connection-state.closed")).toHaveText("已关闭");
  await page.reload();
  await expect(page.getByText("当前没有连接记录")).toBeVisible();
  expect(await page.evaluate(() => Object.values(localStorage).some((value) => value.includes("history.example.test")))).toBe(false);
});
