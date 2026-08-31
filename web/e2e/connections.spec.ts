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
  await expect(page.getByText("当前没有活动连接")).toBeVisible();
  await page.getByRole("tab", { name: /已关闭/ }).click();
  await expect(page.getByText("api.example.test", { exact: true })).toBeVisible();
});
