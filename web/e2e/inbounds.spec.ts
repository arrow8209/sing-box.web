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

test("入站由服务端生成候选，关联转独立与旧副本恢复需明确操作", async ({ page }) => {
  test.setTimeout(90_000);
  await login(page);
  await createProfile(page);
  // This test concerns inbound dependencies, without downloading rule fixtures.
  await page.evaluate(async () => {
    const session = await (await fetch("/api/v1/auth/session")).json();
    const profiles = await (await fetch("/api/v1/profiles")).json() as Array<{ id: string; name: string }>;
    const profile = profiles.find((item) => item.name === "入站 E2E 配置")!;
    const response = await fetch(`/api/v1/profiles/${profile.id}/draft`);
    const draft = await response.json();
    const dns = JSON.parse(draft.buffers.dns);
    dns.rules = [];
    const saved = await fetch(`/api/v1/profiles/${profile.id}/draft`, {
      method: "PUT", headers: { "Content-Type": "application/json", "X-CSRF-Token": session.csrfToken, "If-Match": response.headers.get("ETag")! },
      body: JSON.stringify({ buffers: { ...draft.buffers, dns: JSON.stringify(dns), route: '{"final":"direct"}' }, bindings: [], editorState: draft.editorState })
    });
    if (!saved.ok) throw new Error(`fixture save failed: ${saved.status}`);
  });

  await page.goto("/inbounds");
  await expect(page.getByRole("heading", { name: "入站配置", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "新建入站配置", exact: true }).click();
  const createDialog = page.getByRole("dialog", { name: "新建入站配置" });
  const serverSettings = await page.evaluate(async () => (await fetch("/api/v1/settings/server")).json() as Promise<{ webPort: number; defaultMixedPort: number; defaultApiPort: number; defaultApiController: string }>);
  expect(serverSettings).toEqual({ webPort: 50015, defaultMixedPort: 50016, defaultApiPort: 50024, defaultApiController: "127.0.0.1:50024" });
  await expect(createDialog.locator('input[type="number"]').first()).toHaveValue("50016");
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
  await expect(page.getByText("关联入站的生成结果只读；请先解除关联", { exact: false })).toBeVisible();

  const persisted = await page.evaluate(async () => {
    const resources = await (await fetch("/api/v1/inbound-resources")).json() as Array<{ id: string }>;
    const profiles = await (await fetch("/api/v1/profiles")).json() as Array<{ id: string; name: string }>;
    const profile = profiles.find((item) => item.name === "入站 E2E 配置");
    const draft = await (await fetch(`/api/v1/profiles/${profile?.id}/draft`)).json() as { schemaVersion: number; buffers: Record<string, string>; references: { inbounds: unknown[] } };
    return { resourceId: resources[0]?.id ?? "", schemaVersion: draft.schemaVersion, inbounds: draft.buffers.inbounds, route: draft.buffers.route, bindings: draft.references.inbounds };
  });
  expect(persisted.inbounds).not.toContain(persisted.resourceId);
  expect(persisted.inbounds).not.toContain("sourceId");
  expect(persisted.inbounds).not.toContain('"sniff"');
  expect(persisted.schemaVersion).toBe(2);
  expect(persisted.bindings).toEqual([{ tag: "mixed-in-2", resourceId: persisted.resourceId, sourceTag: "mixed-in", position: 1 }]);
  expect(((JSON.parse(persisted.route) as { rules?: Array<Record<string, unknown>> }).rules ?? []).some((rule) => (
    rule.inbound === "mixed-in-2" && rule.action === "sniff"
  ))).toBe(false);

  await page.goto("/inbounds");
  await page.getByRole("button", { name: "编辑 mixed-in" }).click();
  const editDialog = page.getByRole("dialog", { name: "编辑入站配置" });
  await editDialog.locator('input[type="number"]').first().fill("19091");
  await editDialog.getByRole("button", { name: "保存入站配置" }).click();
  await expect(page.locator(".resource-row")).toContainText("127.0.0.1:19091");

  await openInboundTab(page);
  await expect(page.locator(".entry-row").filter({ hasText: "mixed-in-2" })).toContainText("127.0.0.1:19091");
  await page.locator("[data-validate]").click();
  await expect(page.locator(".entry-row").filter({ hasText: "mixed-in-2" })).toContainText("127.0.0.1:19091");
  await expect(page.locator(".errors")).toContainText("配置检查通过");

  await page.goto("/inbounds");
  await page.getByRole("button", { name: "删除 mixed-in" }).click();
  await page.getByRole("dialog", { name: "删除入站配置" }).getByRole("button", { name: "确认删除" }).click();
  await expect(page.locator(".resource-row")).toHaveCount(0);

  await openInboundTab(page);
  await page.reload();
  await page.getByRole("tab", { name: "入站", exact: true }).click();
  await expect(page.locator(".entry-row").filter({ hasText: "mixed-in-2" })).toContainText("候选未就绪");
  await page.locator("[data-validate]").click();
  await expect(page.locator(".errors")).toContainText("解析引用");
  await expect(page.locator(".errors")).toContainText("请重新关联或明确恢复此前副本");
  await page.getByRole("button", { name: "恢复此前副本 mixed-in-2", exact: true }).click();
  await expect(page.locator(".profile-error")).toContainText("没有可恢复的旧副本");
  await expect(page.locator(".entry-row").filter({ hasText: "mixed-in-2" }).locator("em")).toHaveText("已关联源资源");
  await expect(page.locator(".save-status")).toContainText("草稿已保存");

  await page.goto("/inbounds");
  await page.getByRole("button", { name: "新建入站配置", exact: true }).click();
  const replacement = page.getByRole("dialog", { name: "新建入站配置" });
  await replacement.locator('input[name="tag"]').fill("shared-source");
  await replacement.locator('input[type="number"]').first().fill("19100");
  await replacement.getByRole("button", { name: "新建入站配置", exact: true }).click();
  await expect(page.locator(".resource-row")).toContainText("shared-source");
  await openInboundTab(page);
  await page.getByRole("button", { name: "关联源资源 mixed-in-2", exact: true }).click();
  const relink = page.getByRole("dialog", { name: "关联入站资源", exact: true });
  await relink.getByRole("radio", { name: "关联 shared-source" }).check();
  await relink.getByRole("button", { name: "确认关联", exact: true }).click();
  await expect(page.locator(".entry-row").filter({ hasText: "mixed-in-2" })).toContainText("127.0.0.1:19100");
  await page.getByRole("button", { name: "关联源资源 mixed-in", exact: true }).click();
  const association = page.getByRole("dialog", { name: "关联入站资源", exact: true });
  await association.getByRole("radio", { name: "关联 shared-source" }).check();
  await association.getByRole("button", { name: "确认关联", exact: true }).click();
  const linked = page.locator(".entry-row").filter({ has: page.getByRole("button", { name: "编辑 mixed-in", exact: true }) });
  await expect(linked).toContainText("127.0.0.1:19100");
  await expect(linked.locator("strong")).toHaveText("mixed-in");
  await expect(linked.locator("em")).toHaveText("已关联源资源");
  await expect(page.locator(".save-status")).toContainText("草稿已保存");

  await page.setViewportSize({ width: 390, height: 844 });
  await page.evaluate(() => { document.documentElement.dataset.theme = "dark"; });
  await expect(page.locator("body")).not.toHaveCSS("overflow-x", "scroll");
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);

  await page.goto("/workspace");
  await page.getByRole("group", { name: "档案操作" }).getByRole("button", { name: "删除", exact: true }).click();
  await page.getByLabel("确认名称").fill("入站 E2E 配置");
  await page.getByRole("dialog", { name: "删除档案" }).getByRole("button", { name: "删除" }).click();
  await expect(page.getByRole("button", { name: /入站 E2E 配置/ })).toHaveCount(0);
  await page.goto("/inbounds");
  await page.getByRole("button", { name: "删除 shared-source" }).click();
  await page.getByRole("dialog", { name: "删除入站配置" }).getByRole("button", { name: "确认删除" }).click();
  await expect(page.locator(".resource-row")).toHaveCount(0);
});
