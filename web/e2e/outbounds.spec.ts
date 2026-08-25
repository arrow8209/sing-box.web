import { expect, test, type Page } from "@playwright/test";

const profileName = "出站 E2E 配置";
const subscriptionName = "出站引用 E2E";

async function login(page: Page): Promise<void> {
  await page.goto("/");
  await page.getByLabel("管理密码").fill("correct horse battery staple");
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page.getByRole("heading", { name: "仪表盘" })).toBeVisible();
}

async function createSubscription(page: Page): Promise<void> {
  await page.goto("/subscriptions");
  await page.getByRole("button", { name: "导入 JSON 订阅" }).click();
  const dialog = page.getByRole("dialog", { name: "导入 JSON 订阅" });
  await dialog.getByLabel("订阅 JSON").fill(JSON.stringify({
    name: subscriptionName,
    outbounds: [
      { type: "trojan", tag: "新加坡 E2E", server: "sg.example.test", server_port: 443, password: "outbound-e2e-secret", tls: { enabled: true } },
      { type: "vless", tag: "香港 E2E", server: "hk.example.test", server_port: 8443, uuid: "00000000-0000-0000-0000-000000000000", future: { keep: true } }
    ]
  }));
  await dialog.getByRole("button", { name: "导入全部节点" }).click();
  await expect(page.locator(".subscription-card").filter({ hasText: subscriptionName })).toBeVisible();
}

async function createProfile(page: Page): Promise<void> {
  await page.goto("/profiles");
  await page.getByRole("button", { name: "新建档案" }).click();
  const dialog = page.getByRole("dialog", { name: "新建档案" });
  await dialog.getByLabel("名称").fill(profileName);
  await dialog.getByRole("button", { name: "确认" }).click();
  await expect(page.getByRole("heading", { name: profileName })).toBeVisible();
}

async function openOutbounds(page: Page): Promise<void> {
  await page.goto("/workspace");
  await page.getByRole("tab", { name: "出站", exact: true }).click();
  await expect(page.getByRole("heading", { name: "出站与策略组" })).toBeVisible();
}

async function addDirect(page: Page): Promise<void> {
  await page.getByRole("button", { name: "新增出站" }).click();
  const dialog = page.getByRole("dialog", { name: "出站" });
  await dialog.getByRole("radio", { name: "直连" }).check();
  await dialog.locator('input[name="tag"]').fill("e2e-direct");
  await dialog.getByRole("button", { name: "确认添加" }).click();
  await expect(page.locator('[data-outbound-tag="e2e-direct"]')).toBeVisible();
}

async function addUrlTest(page: Page): Promise<void> {
  await page.getByRole("button", { name: "新增出站" }).click();
  const dialog = page.getByRole("dialog", { name: "出站" });
  await dialog.getByRole("radio", { name: "自动选择" }).check();
  await dialog.locator('input[name="tag"]').fill("E2E 自动选择");
  await dialog.locator('input[name="interval"]').fill("5m");
  await dialog.locator(".reference-group").first().locator(".reference-tile").filter({ hasText: "e2e-direct" }).locator(".reference-content").click();
  await dialog.getByRole("button", { name: "确认添加" }).click();
  await expect(page.locator('[data-outbound-tag="E2E 自动选择"]')).toContainText("1 个引用");
}

async function addSelectorWithSubscription(page: Page): Promise<void> {
  await page.getByRole("button", { name: "新增出站" }).click();
  const dialog = page.getByRole("dialog", { name: "出站" });
  await expect(dialog).not.toContainText("outbound-e2e-secret");
  await dialog.locator('input[name="tag"]').fill("节点选择");
  await dialog.locator(".reference-group").first().locator(".reference-tile").filter({ hasText: "E2E 自动选择" }).locator(".reference-content").click();
  await dialog.locator(".reference-group").nth(1).locator(".reference-tile").filter({ hasText: subscriptionName }).locator(".reference-content").click();
  await expect(dialog.getByText("已选 2 项 · 3 个出站")).toBeVisible();
  await dialog.getByRole("button", { name: "确认添加" }).click();
  await expect(page.locator('[data-outbound-tag="节点选择"]')).toContainText("3 个引用");
  await expect(page.locator('[data-outbound-tag="新加坡 E2E"]')).toBeVisible();
  await expect(page.locator('[data-outbound-tag="香港 E2E"]')).toBeVisible();
}

test("按原型创建、引用、编辑和持久化自定义配置出站", async ({ page }) => {
  await login(page);
  await createSubscription(page);
  await createProfile(page);
  await openOutbounds(page);
  const initialOutboundCount = await page.locator(".outbound-card").count();
  expect(initialOutboundCount).toBeGreaterThan(0);

  await addDirect(page);
  await addUrlTest(page);
  await addSelectorWithSubscription(page);

  await page.getByRole("button", { name: "新增出站" }).click();
  const custom = page.getByRole("dialog", { name: "出站" });
  await custom.getByRole("radio", { name: "完整 JSON" }).check();
  await custom.getByLabel("出站 JSON").fill(JSON.stringify({
    type: "future-protocol",
    tag: "未来节点",
    server: "future.example.test",
    token: "visible-custom-value",
    future: { nested: [1, "x", true] }
  }, null, 2));
  await custom.getByRole("button", { name: "确认添加" }).click();
  await expect(page.locator('[data-outbound-tag="未来节点"]')).toContainText("完整 JSON");
  await expect(page.getByLabel("测试间隔")).toHaveValue("5m");
  await expect(page.locator(".save-status")).toContainText("草稿已保存");

  const persisted = await page.evaluate(async (name) => {
    const profiles = await (await fetch("/api/v1/profiles")).json() as Array<{ id: string; name: string }>;
    const profile = profiles.find((item) => item.name === name);
    return await (await fetch(`/api/v1/profiles/${profile?.id}/draft`)).json() as { buffers: Record<string, string> };
  }, profileName);
  const saved = JSON.parse(persisted.buffers.outbounds) as Array<Record<string, unknown>>;
  expect(saved.find((item) => item.tag === "节点选择")?.outbounds).toEqual(["E2E 自动选择", "新加坡 E2E", "香港 E2E"]);
  expect(saved.find((item) => item.tag === "新加坡 E2E")).toMatchObject({ password: "outbound-e2e-secret", tls: { enabled: true } });
  expect(saved.find((item) => item.tag === "香港 E2E")).toMatchObject({ future: { keep: true } });
  expect(saved.find((item) => item.tag === "未来节点")).toMatchObject({ token: "visible-custom-value", future: { nested: [1, "x", true] } });
  expect(persisted.buffers.outbounds).not.toContain("subscription-node");
  expect(persisted.buffers.outbounds).not.toContain("sourceId");

  await page.reload();
  await page.getByRole("tab", { name: "出站", exact: true }).click();
  await expect(page.locator(".outbound-card")).toHaveCount(initialOutboundCount + 6);
  await page.getByRole("button", { name: "编辑出站 节点选择" }).click();
  const editor = page.getByRole("dialog", { name: "出站" });
  await expect(editor.getByText("已选 3 项 · 3 个出站")).toBeVisible();
  await editor.getByLabel("隐藏出站").check();
  await editor.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.locator('[data-outbound-tag="节点选择"]')).toContainText("隐藏");

  await page.setViewportSize({ width: 390, height: 844 });
  await page.evaluate(() => { document.documentElement.dataset.theme = "dark"; });
  await page.getByRole("button", { name: "编辑出站 E2E 自动选择" }).click();
  const mobileDialog = page.getByRole("dialog", { name: "出站" });
  await expect(mobileDialog).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
  expect(await mobileDialog.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
  await mobileDialog.getByRole("button", { name: "取消" }).click();

  for (const scenario of [
    { width: 390, height: 844, theme: "light" },
    { width: 900, height: 1024, theme: "light" },
    { width: 900, height: 1024, theme: "dark" },
    { width: 1440, height: 900, theme: "dark" }
  ] as const) {
    await page.setViewportSize({ width: scenario.width, height: scenario.height });
    await page.evaluate((theme) => { document.documentElement.dataset.theme = theme; }, scenario.theme);
    await expect(page.locator(".outbound-list-card")).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
  }

  await page.goto("/profiles");
  await page.getByRole("button", { name: "删除" }).click();
  await page.getByLabel("确认名称").fill(profileName);
  await page.getByRole("dialog", { name: "删除档案" }).getByRole("button", { name: "删除" }).click();
  await expect(page.getByRole("button", { name: new RegExp(profileName) })).toHaveCount(0);

  await page.goto("/subscriptions");
  const card = page.locator(".subscription-card").filter({ hasText: subscriptionName });
  await card.getByRole("button", { name: "编辑 JSON" }).click();
  await page.getByRole("dialog", { name: "编辑 JSON 订阅" }).getByRole("button", { name: "删除订阅" }).click();
  await page.getByRole("dialog", { name: "删除 JSON 订阅" }).getByRole("button", { name: "确认删除" }).click();
  await expect(card).toHaveCount(0);
});
