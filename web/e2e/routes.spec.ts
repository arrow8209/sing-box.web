import { expect, test, type Page } from "@playwright/test";

const profileName = "路由 E2E 配置";

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

test("路由规则从新增规则弹窗选择全部规则集并持久化", async ({ page }) => {
  await login(page);
  await createProfile(page);
  await page.goto("/workspace");
  await page.getByRole("tab", { name: "路由", exact: true }).click();
  await expect(page.getByRole("heading", { name: "路由规则" })).toBeVisible();
  await expect(page.getByText("Web 规则资源绑定")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "添加绑定" })).toHaveCount(0);

  const initialCount = await page.locator(".route-rule-row").count();
  await page.getByRole("button", { name: "新增规则" }).click();
  const dialog = page.getByRole("dialog", { name: "新增规则" });
  await expect(dialog.getByText("全部规则集", { exact: true })).toBeVisible();
  await expect(dialog.locator(".ruleset-option")).toHaveCount(6);
  await expect(dialog.getByText("6 / 6", { exact: true })).toBeVisible();
  await expect(dialog.getByText("Category-Ads", { exact: true })).toBeVisible();
  await expect(dialog.getByText("GeoLocation-!CN", { exact: true })).toBeVisible();
  await dialog.locator(".ruleset-option").filter({ hasText: "Category-Ads" }).locator(".ruleset-option-content").click();
  await dialog.getByRole("button", { name: "确认添加" }).click();

  await expect(page.locator(".route-rule-row")).toHaveCount(initialCount + 1);
  await expect(page.locator(".route-rule-row").last()).toContainText("Category-Ads");
  await expect(page.locator(".save-status")).toContainText("草稿已保存");

  const persisted = await page.evaluate(async (name) => {
    const profiles = await (await fetch("/api/v1/profiles")).json() as Array<{ id: string; name: string }>;
    const profile = profiles.find((item) => item.name === name);
    return await (await fetch(`/api/v1/profiles/${profile?.id}/draft`)).json() as {
      buffers: Record<string, string>;
      bindings: Array<{ resourceId: string; tag: string; enabled: boolean }>;
    };
  }, profileName);
  const savedRoute = JSON.parse(persisted.buffers.route) as { rules: Array<Record<string, unknown>> };
  expect(savedRoute.rules.at(-1)).toMatchObject({ action: "route", rule_set: ["Category-Ads"], outbound: "🐟 漏网之鱼" });
  expect(persisted.bindings.find((binding) => binding.tag === "Category-Ads")).toMatchObject({ enabled: true });

  await page.reload();
  await page.getByRole("tab", { name: "路由", exact: true }).click();
  await expect(page.locator(".route-rule-row")).toHaveCount(initialCount + 1);

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
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
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
