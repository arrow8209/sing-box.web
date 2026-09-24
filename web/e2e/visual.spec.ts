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
  // Keep clicks in setup dialogs from leaving a card hovered in the snapshot.
  await page.mouse.move(0, 0);
}

async function ensureVisualProfile(page: Page): Promise<void> {
  await page.goto("/profiles");
  await expect(page.getByRole("heading", { name: "配置档案" })).toBeVisible();
  if (!await page.locator(".profile-list .profile").count()) {
    await page.getByRole("button", { name: "新建档案" }).click();
    const dialog = page.getByRole("dialog", { name: "新建档案" });
    await dialog.getByLabel("名称").fill("视觉基准档案");
    await dialog.getByRole("button", { name: "确认" }).click();
    await expect(page.getByRole("heading", { name: "视觉基准档案" })).toBeVisible();
  }
  // Use an isolated candidate that can be generated without downloading the
  // default rule fixtures; keep visual baselines focused on the actual UI.
  const changed = await page.evaluate(async () => {
    const session = await (await fetch("/api/v1/auth/session")).json();
    const profiles = await (await fetch("/api/v1/profiles")).json() as Array<{ id: string; name: string }>;
    const id = profiles.find((item) => item.name === "视觉基准档案")?.id;
    if (!id) return false;
    const response = await fetch(`/api/v1/profiles/${id}/draft`);
    const draft = await response.json() as { buffers: Record<string, string>; editorState: Record<string, unknown>; bindings: unknown[]; schemaVersion?: number; references?: unknown };
    const experimental = JSON.parse(draft.buffers.experimental!) as { clash_api: { external_controller: string } };
    // Keep the synthetic visual profile stable across default-port changes.
    const visualController = "127.0.0.1:20124";
    if (!draft.bindings.length && experimental.clash_api.external_controller === visualController) return false;
    const buffers = { ...draft.buffers };
    if (draft.bindings.length) {
      const dns = JSON.parse(draft.buffers.dns!) as Record<string, unknown>; dns.rules = [];
      const route = JSON.parse(draft.buffers.route!) as Record<string, unknown>; route.rules = [];
      buffers.dns = JSON.stringify(dns);
      buffers.route = JSON.stringify(route);
    }
    experimental.clash_api.external_controller = visualController;
    buffers.experimental = JSON.stringify(experimental);
    const saved = await fetch(`/api/v1/profiles/${id}/draft`, { method: "PUT", headers: { "Content-Type": "application/json", "X-CSRF-Token": session.csrfToken, "If-Match": response.headers.get("ETag")! },
      body: JSON.stringify({ buffers, bindings: [], editorState: draft.editorState,
        ...(draft.schemaVersion === 2 ? { schemaVersion: 2, references: draft.references } : {}) }) });
    if (!saved.ok) throw new Error(`visual profile setup failed: ${saved.status}`);
    return true;
  });
  if (changed) await page.reload();
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

async function ensureVisualInbound(page: Page): Promise<void> {
  await page.goto("/inbounds");
  await expect(page.getByRole("heading", { name: "入站配置", exact: true })).toBeVisible();
  if (await page.locator(".resource-row").count()) return;
  await page.getByRole("button", { name: "新建入站配置", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "新建入站配置" });
  await dialog.locator('input[name="tag"]').fill("visual-mixed-in");
  await dialog.locator('input[type="number"]').first().fill("19090");
  await dialog.getByRole("button", { name: "新建入站配置", exact: true }).click();
  await expect(page.locator(".resource-row")).toHaveCount(1);
  await page.reload();
  await expect(page.locator(".resource-row")).toHaveCount(1);
}

for (const scenario of scenarios) {
  test(`${scenario.name} dashboard`, async ({ page }) => {
    await page.setViewportSize({ width: scenario.width, height: scenario.height });
    await loginAndOpenProfile(page);
    await ensureVisualProfile(page);
    await page.goto("/");
    await setTheme(page, scenario.theme);
    await expect(page.getByRole("heading", { name: "早上好，管理员" })).toBeVisible();
    await expect(page.locator(".traffic-card")).toBeVisible();
    await expect(page.locator(".proxy-selector-card")).toBeVisible();
    await expect(page.locator("body")).not.toHaveCSS("overflow-x", "scroll");
    await expect(page).toHaveScreenshot(`${scenario.name}-dashboard.png`, { fullPage: true });
  });
}

for (const scenario of scenarios) {
  test(`${scenario.name} configuration and rules workspace`, async ({ page }) => {
    await page.setViewportSize({ width: scenario.width, height: scenario.height });
    await loginAndOpenProfile(page);
    await ensureVisualProfile(page);
    await setTheme(page, scenario.theme);
    await expect(page.getByRole("group", { name: "档案操作" })).toHaveCount(0);
    await expect(page.locator("body")).not.toHaveCSS("overflow-x", "scroll");
    await expect(page).toHaveScreenshot(`${scenario.name}-profiles.png`, { fullPage: true });

    await page.goto("/workspace");
    await setTheme(page, scenario.theme);
    const contextBar = page.locator(".config-context-bar");
    const applicationActions = contextBar.getByRole("group", { name: "配置操作" });
    const profileActions = contextBar.getByRole("group", { name: "档案操作" });
    await expect(contextBar).toBeVisible();
    await expect(contextBar.getByRole("link", { name: "管理档案" })).toHaveCount(0);
    await expect(contextBar.getByRole("button")).toHaveText(["重命名", "复制", "删除", "预览", "检测", "应用"]);
    await expect(profileActions.getByRole("button")).toHaveText(["重命名", "复制", "删除"]);
    await expect(applicationActions.getByRole("button")).toHaveText(["预览", "检测", "应用"]);
    await expect(applicationActions.getByRole("button", { name: "应用", exact: true })).toBeEnabled();
    await expect(page.locator(".application-actions")).toHaveCount(1);
    await expect(page.getByRole("tab")).toHaveCount(7);
    await expect(page.getByRole("tab", { name: "端点" })).toHaveCount(0);
    await expect(page.getByRole("tab", { name: "常规设置" })).toHaveAttribute("aria-selected", "true");
    await expect(page.getByRole("heading", { name: "工作模式与 API" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "日志设置" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "参数归属" })).toBeVisible();
    await expect(page.getByRole("radio", { name: "规则" })).toHaveAttribute("aria-checked", "true");
    await expect(page.getByLabel("API 监听地址")).toHaveValue("127.0.0.1:20124");
    await expect(page.getByLabel("日志保存路径")).toHaveValue("data/logs/sing-box.log");
    await expect(page.locator(".general-settings-panel .setting-row")).toHaveCount(4);
    await expect(page.getByLabel("Mixed 入站端口")).toHaveCount(0);
    await expect(page.locator("main")).toBeVisible();
    await expect(page.locator("body")).not.toHaveCSS("overflow-x", "scroll");
    await expect(page).toHaveScreenshot(`${scenario.name}-config.png`, { fullPage: true });

    await ensureVisualInbound(page);
    await setTheme(page, scenario.theme);
    await expect(page.locator(".resource-row")).toBeVisible();
    await expect(page.locator("body")).not.toHaveCSS("overflow-x", "scroll");
    await expect(page).toHaveScreenshot(`${scenario.name}-inbounds.png`, { fullPage: true });

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

    await page.goto("/core");
    await setTheme(page, scenario.theme);
    const versionPanel = page.getByLabel("版本管理");
    await expect(versionPanel.getByRole("article", { name: "当前版本" })).toContainText("1.0.0");
    await expect(versionPanel.getByRole("article", { name: "最新版本" })).toContainText("1.0.0");
    const coreDownload = versionPanel.locator("[data-download-core]");
    const downloadSpeed = versionPanel.getByText("4 MB/s");
    if (!await downloadSpeed.isVisible() && await coreDownload.isVisible() && await coreDownload.isEnabled()) await coreDownload.click();
    await expect(downloadSpeed).toBeVisible();
    await expect(page.locator("body")).not.toHaveCSS("overflow-x", "scroll");
    await expect(page).toHaveScreenshot(`${scenario.name}-core.png`, { fullPage: true });

    await page.goto("/workspace");
    await setTheme(page, scenario.theme);
    await page.getByRole("tab", { name: "高级", exact: true }).click();
    await expect(page.locator(".advanced-section-row")).toHaveCount(6);
    await expect(page.getByRole("heading", { name: "高级与共享模块" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "编辑策略" })).toBeVisible();
    await expect(page.locator(".advanced-panel")).not.toContainText("Schema");
    await expect(page.locator("body")).not.toHaveCSS("overflow-x", "scroll");
    await expect(page).toHaveScreenshot(`${scenario.name}-config-advanced.png`, { fullPage: true });
    await page.locator(".advanced-section-row").first().click();
    const moduleDialog = page.getByRole("dialog", { name: "编辑NTP" });
    await expect(moduleDialog).toContainText("完整维护 config.ntp");
    await expect(moduleDialog.getByRole("button", { name: "保存片段" })).toBeEnabled();
    await expect(page.locator("body")).not.toHaveCSS("overflow-x", "scroll");
    await moduleDialog.getByRole("button", { name: "关闭对话框" }).click();
  });
}
