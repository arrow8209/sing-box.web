import { expect, test, type Page } from "@playwright/test";

const scenarios = [
  { name: "desktop-light", width: 1440, height: 900, theme: "light" },
  { name: "desktop-dark", width: 1440, height: 900, theme: "dark" },
  { name: "tablet-light", width: 900, height: 1024, theme: "light" },
  { name: "tablet-dark", width: 900, height: 1024, theme: "dark" },
  { name: "mobile-light", width: 390, height: 844, theme: "light" },
  { name: "mobile-dark", width: 390, height: 844, theme: "dark" }
] as const;

async function loginAndOpenProfile(page: Page): Promise<void> {
  await page.goto("/");
  const loginButton = page.getByRole("button", { name: "登录" });
  const dashboardHeading = page.getByRole("heading", { name: "仪表盘" });
  await expect(loginButton.or(dashboardHeading)).toBeVisible();
  if (await loginButton.isVisible()) {
    await page.getByLabel("管理密码").fill("correct horse battery staple");
    await Promise.all([
      page.waitForURL((url) => url.pathname === "/"),
      loginButton.click()
    ]);
  }
  await expect(dashboardHeading).toBeVisible();
}

async function setTheme(page: Page, theme: "light" | "dark"): Promise<void> {
  await page.evaluate((value) => { document.documentElement.dataset.theme = value; }, theme);
}

async function ensureVisualProfile(page: Page): Promise<void> {
  await page.goto("/profiles");
  await expect(page.getByRole("heading", { name: "配置档案" })).toBeVisible();
  if (await page.locator(".profile-list .profile").count()) return;
  await page.getByRole("button", { name: "新建档案" }).click();
  const dialog = page.getByRole("dialog", { name: "新建档案" });
  await dialog.getByLabel("名称").fill("视觉基准档案");
  await dialog.getByRole("button", { name: "确认" }).click();
  await expect(page.getByRole("heading", { name: "视觉基准档案" })).toBeVisible();
}

async function ensureVisualSubscription(page: Page): Promise<void> {
  await page.goto("/subscriptions");
  await expect(page.getByRole("heading", { name: "订阅配置" })).toBeVisible();
  if (await page.locator(".subscription-card").count()) return;
  await page.getByRole("button", { name: "导入 JSON 订阅" }).click();
  const dialog = page.getByRole("dialog", { name: "导入 JSON 订阅" });
  await dialog.getByLabel("订阅 JSON").fill(JSON.stringify({
    name: "Visual Route",
    outbounds: [
      { type: "shadowsocks", tag: "香港 01", server: "hk01.example.test", server_port: 8388, password: "visual-only" },
      { type: "trojan", tag: "新加坡 01", server: "sg01.example.test", server_port: 443, password: "visual-only" },
      { type: "vless", tag: "日本 01", server: "jp01.example.test", server_port: 443, uuid: "00000000-0000-0000-0000-000000000000" },
      { type: "hysteria2", tag: "美国 01", server: "us01.example.test", server_port: 8443, password: "visual-only" }
    ]
  }));
  await dialog.getByRole("button", { name: "导入全部节点" }).click();
  await expect(page.locator(".subscription-card")).toHaveCount(1);
  await page.reload();
  await expect(page.locator(".subscription-card")).toHaveCount(1);
}

for (const scenario of scenarios) {
  test(`${scenario.name} configuration and rules workspace`, async ({ page }) => {
    await page.setViewportSize({ width: scenario.width, height: scenario.height });
    await loginAndOpenProfile(page);
    await ensureVisualProfile(page);
    await page.goto("/workspace");
    await setTheme(page, scenario.theme);
    await expect(page.locator(".config-context-bar")).toBeVisible();
    await expect(page.locator("main")).toBeVisible();
    await expect(page.locator("body")).not.toHaveCSS("overflow-x", "scroll");
    await expect(page).toHaveScreenshot(`${scenario.name}-config.png`, { fullPage: true });

    await page.goto("/rules");
    await setTheme(page, scenario.theme);
    await expect(page.getByRole("heading", { name: "规则集中心" })).toBeVisible();
    await expect(page.locator(".ruleset-table-card")).toBeVisible();
    await expect(page.locator(".rule-count").first()).toHaveText("905");
    const ruleSetColumn = await page.locator(".ruleset-data-table th").nth(0).boundingBox();
    const formatColumn = await page.locator(".ruleset-data-table th").nth(2).boundingBox();
    expect(formatColumn!.width).toBeGreaterThan(ruleSetColumn!.width);
    expect((await page.locator(".path").first().boundingBox())!.width).toBeGreaterThan(200);
    await page.locator(".path").evaluateAll((elements) => {
      for (const element of elements) {
        element.textContent = "data/rulesets/[managed-id]";
        element.removeAttribute("title");
      }
    });
    await expect(page.locator("body")).not.toHaveCSS("overflow-x", "scroll");
    await expect(page).toHaveScreenshot(`${scenario.name}-rules.png`, { fullPage: true });

    await ensureVisualSubscription(page);
    await setTheme(page, scenario.theme);
    await expect(page.locator(".subscription-card")).toBeVisible();
    await expect(page.locator("body")).not.toHaveCSS("overflow-x", "scroll");
    await expect(page).toHaveScreenshot(`${scenario.name}-subscriptions.png`, { fullPage: true });
  });
}
