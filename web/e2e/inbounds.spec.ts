import { expect, test, type Page } from "@playwright/test";

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
  await dialog.getByLabel("名称").fill("入站 E2E 配置");
  await dialog.getByRole("button", { name: "确认" }).click();
  await expect(page.getByRole("heading", { name: "入站 E2E 配置" })).toBeVisible();
}

async function openInboundTab(page: Page): Promise<void> {
  await page.goto("/workspace");
  await expect(page.locator(".config-context-bar")).toBeVisible();
  await page.getByRole("tab", { name: "入站", exact: true }).click();
  await expect(page.getByRole("heading", { name: "当前自定义配置的入站" })).toBeVisible();
}

test("入站资源导入后成为与源配置无关的深拷贝", async ({ page }) => {
  await login(page);
  await createProfile(page);

  await page.goto("/inbounds");
  await expect(page.getByRole("heading", { name: "入站配置", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "新建入站配置", exact: true }).click();
  const createDialog = page.getByRole("dialog", { name: "新建入站配置" });
  await createDialog.locator('input[name="tag"]').fill("mixed-in");
  await createDialog.locator('input[type="number"]').first().fill("19090");
  await expect(createDialog.getByText("协议嗅探", { exact: true })).toHaveCount(0);
  await expect(createDialog.getByText("覆盖目标地址", { exact: true })).toHaveCount(0);
  await createDialog.getByRole("button", { name: "新建入站配置", exact: true }).click();
  await expect(page.locator(".resource-row")).toContainText("127.0.0.1:19090");

  await openInboundTab(page);
  await page.getByRole("button", { name: "导入入站配置" }).click();
  const importDialog = page.getByRole("dialog", { name: "导入入站配置" });
  await expect(importDialog.getByText("将改名为 mixed-in-2")).toBeVisible();
  await importDialog.locator('.import-option input[type="checkbox"]').check();
  await importDialog.getByRole("button", { name: "导入所选 1 项" }).click();
  await expect(page.locator(".entry-row").filter({ hasText: "mixed-in-2" })).toContainText("127.0.0.1:19090");
  await expect(page.locator(".save-status")).toContainText("草稿已保存");

  await page.getByRole("button", { name: "编辑 mixed-in-2" }).click();
  const profileEditor = page.getByRole("dialog", { name: "编辑独立入站副本" });
  await expect(profileEditor.getByText("协议嗅探", { exact: true })).toHaveCount(0);
  await expect(profileEditor.getByText("覆盖目标地址", { exact: true })).toHaveCount(0);
  await profileEditor.getByRole("button", { name: "保存入站配置" }).click();
  await expect(page.locator(".save-status")).toContainText("草稿已保存");

  const persisted = await page.evaluate(async () => {
    const resources = await (await fetch("/api/v1/inbound-resources")).json() as Array<{ id: string }>;
    const profiles = await (await fetch("/api/v1/profiles")).json() as Array<{ id: string; name: string }>;
    const profile = profiles.find((item) => item.name === "入站 E2E 配置");
    const draft = await (await fetch(`/api/v1/profiles/${profile?.id}/draft`)).json() as { buffers: Record<string, string> };
    return { resourceId: resources[0]?.id ?? "", inbounds: draft.buffers.inbounds, route: draft.buffers.route };
  });
  expect(persisted.inbounds).not.toContain(persisted.resourceId);
  expect(persisted.inbounds).not.toContain("sourceId");
  expect(persisted.inbounds).not.toContain('"sniff"');
  expect((JSON.parse(persisted.route) as { rules: Array<Record<string, unknown>> }).rules.some((rule) => (
    rule.inbound === "mixed-in-2" && rule.action === "sniff"
  ))).toBe(false);

  await page.goto("/inbounds");
  await page.getByRole("button", { name: "编辑 mixed-in" }).click();
  const editDialog = page.getByRole("dialog", { name: "编辑入站配置" });
  await editDialog.locator('input[type="number"]').first().fill("19091");
  await editDialog.getByRole("button", { name: "保存入站配置" }).click();
  await expect(page.locator(".resource-row")).toContainText("127.0.0.1:19091");

  await openInboundTab(page);
  await expect(page.locator(".entry-row").filter({ hasText: "mixed-in-2" })).toContainText("127.0.0.1:19090");

  await page.goto("/inbounds");
  await page.getByRole("button", { name: "删除 mixed-in" }).click();
  await page.getByRole("dialog", { name: "删除入站配置" }).getByRole("button", { name: "确认删除" }).click();
  await expect(page.locator(".resource-row")).toHaveCount(0);

  await openInboundTab(page);
  await page.reload();
  await page.getByRole("tab", { name: "入站", exact: true }).click();
  await expect(page.locator(".entry-row").filter({ hasText: "mixed-in-2" })).toContainText("127.0.0.1:19090");

  await page.setViewportSize({ width: 390, height: 844 });
  await page.evaluate(() => { document.documentElement.dataset.theme = "dark"; });
  await expect(page.locator("body")).not.toHaveCSS("overflow-x", "scroll");
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);

  await page.goto("/workspace");
  await page.getByRole("group", { name: "档案操作" }).getByRole("button", { name: "删除", exact: true }).click();
  await page.getByLabel("确认名称").fill("入站 E2E 配置");
  await page.getByRole("dialog", { name: "删除档案" }).getByRole("button", { name: "删除" }).click();
  await expect(page.getByRole("button", { name: /入站 E2E 配置/ })).toHaveCount(0);
});
