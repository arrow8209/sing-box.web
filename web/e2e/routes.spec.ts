import { expect, test, type Page } from "@playwright/test";

const profileName = "路由 E2E 配置";

interface PersistedDraft {
  buffers: Record<string, string>;
  editorState?: Record<string, unknown>;
  bindings: Array<{ resourceId: string; tag: string; enabled: boolean }>;
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

test("按原型编辑、停用、排序并持久化路由规则", async ({ page }) => {
  test.setTimeout(90_000);
  await login(page);
  await createProfile(page);
  await page.goto("/workspace");
  await page.getByRole("tab", { name: "路由", exact: true }).click();
  await expect(page.getByRole("heading", { name: "通用设置" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "路由规则" })).toBeVisible();
  await expect(page.getByText("Web 规则资源绑定")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "添加绑定" })).toHaveCount(0);

  await page.getByRole("switch", { name: "切换查找进程信息" }).click();
  await page.getByRole("switch", { name: "切换自动检测出站接口" }).click();
  await page.getByLabel("出站接口名称").selectOption("eth0");
  await page.getByLabel("解析节点域名的 DNS 服务器").selectOption("Remote-DNS");
  await page.getByLabel("默认出站标签").selectOption("GLOBAL");

  const initialCount = await page.locator(".route-rule-row").count();
  await page.getByRole("button", { name: "新增规则" }).click();
  let dialog = page.getByRole("dialog", { name: "新增规则" });
  await expect(dialog.getByText("全部规则集", { exact: true })).toBeVisible();
  await expect(dialog.locator(".ruleset-option")).toHaveCount(6);
  await expect(dialog.getByText("6 / 6", { exact: true })).toBeVisible();
  await expect(dialog.getByText("Category-Ads", { exact: true })).toBeVisible();
  await expect(dialog.getByText("GeoLocation-!CN", { exact: true })).toBeVisible();
  await expect(dialog.locator('input[name="route-action"]')).toHaveCount(6);
  await expect(dialog.getByLabel("规则类型").locator("option")).toHaveCount(20);
  expect((await dialog.boundingBox())!.width).toBeGreaterThan(800);
  await dialog.locator(".ruleset-option").filter({ hasText: "Category-Ads" }).locator(".ruleset-option-content").click();
  await dialog.getByRole("button", { name: "确认添加" }).click();

  await expect(page.locator(".route-rule-row")).toHaveCount(initialCount + 1);
  await expect(page.locator(".route-rule-row").nth(4)).toContainText("Category-Ads");

  await page.getByRole("button", { name: "新增规则" }).click();
  dialog = page.getByRole("dialog", { name: "新增规则" });
  await dialog.getByLabel("规则类型").selectOption("domain_suffix");
  await dialog.getByLabel("匹配内容").fill("blocked.example.test");
  await dialog.locator('input[name="route-action"][value="reject"]').check();
  await dialog.locator('input[value="drop"]').check();
  await dialog.getByRole("button", { name: "确认添加" }).click();

  await expect(page.locator(".route-rule-row")).toHaveCount(initialCount + 2);
  let customRule = page.locator(".route-rule-row").filter({ hasText: "blocked.example.test" });
  await expect(customRule).toContainText("拒绝连接");

  const dragHandle = customRule.locator("[data-route-rule-drag-handle]");
  await dragHandle.scrollIntoViewIfNeeded();
  const sourceBox = await dragHandle.boundingBox();
  const targetBox = await page.locator(".route-rule-row").first().boundingBox();
  expect(sourceBox).not.toBeNull();
  expect(targetBox).not.toBeNull();
  await page.mouse.move(sourceBox!.x + sourceBox!.width / 2, sourceBox!.y + sourceBox!.height / 2);
  await page.mouse.down();
  await page.mouse.move(targetBox!.x + 8, targetBox!.y + 2, { steps: 12 });
  await page.mouse.up();
  await expect(page.locator(".route-rule-row").first()).toContainText("blocked.example.test");

  await page.locator(".route-rule-row").first().locator("[data-route-rule-drag-handle]").press("ArrowDown");
  await expect(page.locator(".route-rule-row").nth(1)).toContainText("blocked.example.test");

  customRule = page.locator(".route-rule-row").filter({ hasText: "blocked.example.test" });
  await customRule.getByRole("checkbox", { name: /停用第/ }).uncheck();
  await expect(customRule).toHaveClass(/is-disabled/);
  await expect(page.locator(".route-rule-row")).toHaveCount(initialCount + 2);
  await expect(page.locator(".save-status")).toContainText("草稿已保存");

  let persisted = await persistedDraft(page);
  let savedRoute = JSON.parse(persisted.buffers.route) as { rules: Array<Record<string, unknown>>; find_process?: boolean; auto_detect_interface?: boolean; default_interface?: string; default_domain_resolver?: { server?: string }; final?: string };
  expect(savedRoute).toMatchObject({
    find_process: true,
    auto_detect_interface: false,
    default_interface: "eth0",
    default_domain_resolver: { server: "Remote-DNS" },
    final: "GLOBAL"
  });
  expect(savedRoute.rules.some((rule) => rule.domain_suffix === "blocked.example.test" || (Array.isArray(rule.domain_suffix) && rule.domain_suffix.includes("blocked.example.test")))).toBe(false);
  expect(persisted.bindings.find((binding) => binding.tag === "Category-Ads")).toMatchObject({ enabled: true });
  const routeState = persisted.editorState?.route as { disabledRules?: Array<{ text: string }> } | undefined;
  expect(routeState?.disabledRules?.[0]?.text).toContain("blocked.example.test");

  await page.reload();
  await page.getByRole("tab", { name: "路由", exact: true }).click();
  await expect(page.locator(".route-rule-row")).toHaveCount(initialCount + 2);
  customRule = page.locator(".route-rule-row").filter({ hasText: "blocked.example.test" });
  await expect(customRule).toHaveClass(/is-disabled/);
  await expect(customRule.getByRole("checkbox")).not.toBeChecked();
  await expect(page.getByRole("switch", { name: "切换查找进程信息" })).toHaveAttribute("aria-checked", "true");
  await expect(page.getByRole("switch", { name: "切换自动检测出站接口" })).toHaveAttribute("aria-checked", "false");
  await expect(page.getByLabel("出站接口名称")).toHaveValue("eth0");

  await customRule.getByRole("checkbox", { name: /启用第/ }).click();
  await expect(customRule).not.toHaveClass(/is-disabled/);
  await expect(page.locator(".save-status")).toContainText("草稿已保存");
  persisted = await persistedDraft(page);
  savedRoute = JSON.parse(persisted.buffers.route) as typeof savedRoute;
  expect(savedRoute.rules[1]).toMatchObject({ domain_suffix: ["blocked.example.test"], action: "reject", method: "drop" });
  expect((persisted.editorState?.route as { disabledRules?: unknown[] })?.disabledRules).toEqual([]);

  for (const scenario of [
    { width: 1440, height: 900, theme: "dark" },
    { width: 900, height: 1024, theme: "light" },
    { width: 900, height: 1024, theme: "dark" },
    { width: 390, height: 844, theme: "light" },
    { width: 390, height: 844, theme: "dark" }
  ] as const) {
    await page.setViewportSize({ width: scenario.width, height: scenario.height });
    await page.evaluate((theme) => { document.documentElement.dataset.theme = theme; }, scenario.theme);
    await expect(page.locator(".route-list-card")).toBeVisible();
    const layout = await page.evaluate(() => ({
      scrollWidth: document.documentElement.scrollWidth,
      clientWidth: document.documentElement.clientWidth,
      offenders: [...document.querySelectorAll<HTMLElement>("main *")].flatMap((element) => {
        const bounds = element.getBoundingClientRect();
        return bounds.right > document.documentElement.clientWidth + 1
          ? [{ tag: element.tagName, className: element.className, label: element.getAttribute("aria-label"), parentClass: element.parentElement?.className, left: bounds.left, right: bounds.right, width: bounds.width }]
          : [];
      }).slice(0, 5)
    }));
    expect(layout.scrollWidth, JSON.stringify(layout.offenders)).toBeLessThanOrEqual(layout.clientWidth);
    await page.getByRole("button", { name: "新增规则" }).click();
    const responsiveDialog = page.getByRole("dialog", { name: "新增规则" });
    await expect(responsiveDialog.locator(".ruleset-option")).toHaveCount(6);
    await expect(responsiveDialog.getByLabel("搜索规则集")).toBeFocused();
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
