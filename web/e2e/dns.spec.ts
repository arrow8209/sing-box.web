import { expect, test, type Page } from "@playwright/test";

const profileName = "DNS E2E 配置";

interface PersistedDraft {
  buffers: Record<string, string>;
  editorState?: Record<string, unknown>;
}

async function login(page: Page): Promise<void> {
  await page.goto("/");
  const loginButton = page.getByRole("button", { name: "登录" });
  const dashboard = page.getByRole("heading", { name: "仪表盘" });
  await expect(loginButton.or(dashboard)).toBeVisible();
  if (await loginButton.isVisible()) {
    await page.getByLabel("管理密码").fill("correct horse battery staple");
    await loginButton.click();
  }
  await expect(dashboard).toBeVisible();
}

async function createProfile(page: Page): Promise<void> {
  await page.goto("/profiles");
  await page.getByRole("button", { name: "新建档案" }).click();
  const dialog = page.getByRole("dialog", { name: "新建档案" });
  await dialog.getByLabel("名称").fill(profileName);
  await dialog.getByRole("button", { name: "确认" }).click();
  await expect(page.getByRole("heading", { name: profileName })).toBeVisible();
}

async function persistedDraft(page: Page): Promise<PersistedDraft> {
  return page.evaluate(async (name) => {
    const profiles = await (await fetch("/api/v1/profiles")).json() as Array<{ id: string; name: string }>;
    const profile = profiles.find((item) => item.name === name);
    return await (await fetch(`/api/v1/profiles/${profile?.id}/draft`)).json() as PersistedDraft;
  }, profileName);
}

test("按参考图管理 DNS 通用设置、服务器与规则并持久化", async ({ page }) => {
  test.setTimeout(90_000);
  await login(page);
  await createProfile(page);
  await page.goto("/workspace");
  await page.getByRole("tab", { name: "DNS", exact: true }).click();
  await expect(page.getByRole("heading", { name: "DNS 设置" })).toBeVisible();

  await expect(page.getByRole("switch", { name: "切换禁用 DNS 缓存", exact: true })).toHaveAttribute("aria-checked", "false");
  await expect(page.getByRole("switch", { name: "切换禁用 DNS 缓存过期" })).toHaveAttribute("aria-checked", "false");
  await expect(page.getByRole("switch", { name: "切换独立 DNS 缓存" })).toHaveAttribute("aria-checked", "false");
  await expect(page.getByLabel("回退 DNS")).toHaveValue("Remote-DNS");
  await expect(page.getByLabel("DNS 解析策略")).toHaveValue("");
  await expect(page.getByLabel("DNS 客户端子网")).toHaveValue("");

  await page.getByRole("switch", { name: "切换禁用 DNS 缓存", exact: true }).click();
  await page.getByLabel("DNS 解析策略").selectOption("prefer_ipv4");
  await page.getByLabel("DNS 客户端子网").fill("1.2.3.0/24");
  await page.getByLabel("DNS 客户端子网").press("Tab");

  const sections = page.locator(".dns-section-tabs");
  await sections.getByRole("button", { name: /服务器/ }).click();
  await expect(page.locator(".server-row")).toHaveCount(5);
  await expect(page.locator(".server-row").nth(0)).toContainText("Fake-IP");
  await expect(page.locator(".server-row").nth(0)).toContainText("fake-ip://198.18.0.0/15,fc00::/18");
  await expect(page.locator(".server-row").nth(1)).toContainText("https://223.5.5.5:443/dns-query");
  await expect(page.locator(".server-row").nth(3)).toContainText("tls://8.8.8.8:853");
  await expect(page.locator(".server-row").nth(3)).toContainText("🚀 节点选择");

  await page.locator(".server-row").nth(3).getByRole("button", { name: "编辑 DNS 服务器 4" }).click();
  let dialog = page.getByRole("dialog", { name: "编辑 DNS 服务器" });
  await dialog.getByLabel("服务器标签").fill("Remote-DNS-E2E");
  await dialog.getByRole("button", { name: "保存" }).click();
  await expect(page.locator(".server-row").nth(3)).toContainText("Remote-DNS-E2E");

  await sections.getByRole("button", { name: /规则/ }).click();
  await expect(page.locator(".dns-rule-row")).toHaveCount(4);
  await expect(page.locator(".dns-insertion-point")).toBeVisible();
  await expect(page.locator(".dns-rule-row").nth(0)).toContainText("clash_mode, direct, route, Local-DNS");
  await expect(page.locator(".dns-rule-row").nth(1)).toContainText("clash_mode, global, route, Remote-DNS-E2E");
  await expect(page.locator(".dns-rule-row").nth(2)).toContainText("GeoSite-CN");

  await page.getByRole("button", { name: "新增规则" }).click();
  dialog = page.getByRole("dialog", { name: "新增 DNS 规则" });
  await dialog.getByLabel("DNS 规则类型").selectOption("domain_suffix");
  await dialog.getByLabel("DNS 匹配内容").fill("dns-e2e.example");
  await dialog.getByLabel("目标 DNS 服务器").selectOption("Remote-DNS-E2E");
  await dialog.getByRole("button", { name: "确认添加" }).click();
  await expect(page.locator(".dns-rule-row")).toHaveCount(5);
  let customRule = page.locator(".dns-rule-row").filter({ hasText: "dns-e2e.example" });
  await expect(customRule).toBeVisible();
  await customRule.getByRole("checkbox", { name: /停用第/ }).uncheck();
  await expect(customRule).toHaveClass(/is-disabled/);
  await expect(page.locator(".save-status")).toContainText("草稿已保存");

  let persisted = await persistedDraft(page);
  let savedDns = JSON.parse(persisted.buffers.dns) as {
    disable_cache: boolean;
    strategy?: string;
    client_subnet?: string;
    final: string;
    servers: Array<Record<string, unknown>>;
    rules: Array<Record<string, unknown>>;
  };
  expect(savedDns).toMatchObject({ disable_cache: true, strategy: "prefer_ipv4", client_subnet: "1.2.3.0/24", final: "Remote-DNS-E2E" });
  expect(savedDns.servers[3]?.tag).toBe("Remote-DNS-E2E");
  expect(savedDns.rules.some((rule) => rule.domain_suffix === "dns-e2e.example" || (Array.isArray(rule.domain_suffix) && rule.domain_suffix.includes("dns-e2e.example")))).toBe(false);
  expect(savedDns.rules[1]?.server).toBe("Remote-DNS-E2E");
  expect(savedDns.rules[3]?.server).toBe("Remote-DNS-E2E");
  const dnsState = persisted.editorState?.dns as { insertionPosition?: number; disabledRules?: Array<{ text: string }> } | undefined;
  expect(dnsState?.insertionPosition).toBe(2);
  expect(dnsState?.disabledRules?.[0]?.text).toContain("dns-e2e.example");

  await page.reload();
  await page.getByRole("tab", { name: "DNS", exact: true }).click();
  await page.locator(".dns-section-tabs").getByRole("button", { name: /规则/ }).click();
  customRule = page.locator(".dns-rule-row").filter({ hasText: "dns-e2e.example" });
  await expect(customRule).toHaveClass(/is-disabled/);
  await customRule.getByRole("checkbox", { name: /启用第/ }).click();
  await expect(customRule).not.toHaveClass(/is-disabled/);
  await expect(page.locator(".save-status")).toContainText("草稿已保存");

  persisted = await persistedDraft(page);
  savedDns = JSON.parse(persisted.buffers.dns) as typeof savedDns;
  expect(savedDns.rules[2]).toMatchObject({ domain_suffix: ["dns-e2e.example"], action: "route", server: "Remote-DNS-E2E" });
  expect((persisted.editorState?.dns as { disabledRules?: unknown[] })?.disabledRules).toEqual([]);

  for (const scenario of [
    { width: 1440, height: 900, theme: "dark" },
    { width: 900, height: 1024, theme: "light" },
    { width: 900, height: 1024, theme: "dark" },
    { width: 390, height: 844, theme: "light" },
    { width: 390, height: 844, theme: "dark" }
  ] as const) {
    await page.setViewportSize({ width: scenario.width, height: scenario.height });
    await page.evaluate((theme) => { document.documentElement.dataset.theme = theme; }, scenario.theme);
    await expect(page.locator(".dns-workbench")).toBeVisible();
    const layout = await page.evaluate(() => ({
      scrollWidth: document.documentElement.scrollWidth,
      clientWidth: document.documentElement.clientWidth,
      offenders: [...document.querySelectorAll<HTMLElement>("main *")].flatMap((element) => {
        const bounds = element.getBoundingClientRect();
        return bounds.right > document.documentElement.clientWidth + 1
          ? [{ tag: element.tagName, className: element.className, label: element.getAttribute("aria-label"), left: bounds.left, right: bounds.right, width: bounds.width }]
          : [];
      }).slice(0, 5)
    }));
    expect(layout.scrollWidth, JSON.stringify(layout.offenders)).toBeLessThanOrEqual(layout.clientWidth);
    await page.getByRole("button", { name: "新增规则" }).click();
    const responsiveDialog = page.getByRole("dialog", { name: "新增 DNS 规则" });
    await expect(responsiveDialog.getByLabel("DNS 规则类型")).toBeFocused();
    expect(await responsiveDialog.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
    await page.keyboard.press("Escape");
    await expect(responsiveDialog).toBeHidden();
  }

  await page.goto("/profiles");
  await page.getByRole("button", { name: "删除" }).click();
  await page.getByLabel("确认名称").fill(profileName);
  await page.getByRole("dialog", { name: "删除档案" }).getByRole("button", { name: "删除" }).click();
  await expect(page.getByRole("button", { name: new RegExp(profileName) })).toHaveCount(0);
});
