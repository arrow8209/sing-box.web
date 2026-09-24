import { expect, test, type Page } from "./fixtures";

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
  // This flow checks outbound references without relying on downloaded rule fixtures.
  await page.evaluate(async (name) => {
    const session = await (await fetch("/api/v1/auth/session")).json();
    const profiles = await (await fetch("/api/v1/profiles")).json() as Array<{ id: string; name: string }>;
    const id = profiles.find((item) => item.name === name)!.id;
    const response = await fetch(`/api/v1/profiles/${id}/draft`);
    const draft = await response.json() as { buffers: Record<string, string>; editorState: Record<string, unknown> };
    const dns = JSON.parse(draft.buffers.dns!) as Record<string, unknown>; dns.rules = [];
    const route = JSON.parse(draft.buffers.route!) as Record<string, unknown>; route.rules = [];
    const saved = await fetch(`/api/v1/profiles/${id}/draft`, { method: "PUT", headers: { "Content-Type": "application/json", "X-CSRF-Token": session.csrfToken, "If-Match": response.headers.get("ETag")! },
      body: JSON.stringify({ buffers: { ...draft.buffers, dns: JSON.stringify(dns), route: JSON.stringify(route) }, bindings: [], editorState: draft.editorState }) });
    if (!saved.ok) throw new Error(`isolated profile save failed: ${saved.status}`);
  }, profileName);
  await page.reload();
  await page.locator(".config-name").filter({ hasText: profileName }).click();
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
  await expect(dialog.locator(".reference-badge")).toHaveCount(0);
  await dialog.getByRole("radio", { name: "自动选择" }).check();
  await dialog.locator('input[name="tag"]').fill("E2E 自动选择");
  await dialog.locator('input[name="interval"]').fill("5m");
  const reference = dialog.locator(".reference-group").first().locator(".reference-tile").filter({ hasText: "e2e-direct" });
  await reference.locator(".reference-content").click();
  await expect(reference.locator('input[type="checkbox"]')).toBeChecked();
  await expect(dialog.getByText("已选 1 项 · 1 个出站")).toBeVisible();
  await dialog.getByRole("button", { name: "确认添加" }).click();
  await expect(page.locator('[data-outbound-tag="E2E 自动选择"]')).toContainText("1 个出站 / 0 个订阅");
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
  await expect(page.locator('[data-outbound-tag="节点选择"]')).toContainText("1 个出站 / 1 个订阅");
  await expect(page.locator('[data-outbound-tag="新加坡 E2E"]')).toHaveCount(0);
  await expect(page.locator('[data-outbound-tag="香港 E2E"]')).toHaveCount(0);
}

async function expectOutboundOptions(page: Page): Promise<void> {
  const tags = await page.locator(".outbound-card").evaluateAll((cards) => cards.map((card) => card.getAttribute("data-outbound-tag") ?? ""));
  await page.getByRole("tab", { name: "路由", exact: true }).click();
  await expect(page.getByLabel("默认出站标签").locator("option")).toHaveText(["未设置", ...tags]);
  await page.getByRole("button", { name: "新增规则", exact: true }).click();
  const ruleDialog = page.getByRole("dialog", { name: "新增规则" });
  await expect(ruleDialog.getByLabel("目标出站").locator("option")).toHaveText(["请选择目标出站", ...tags]);
  await ruleDialog.getByLabel("目标出站").focus();
  await expect(ruleDialog.getByLabel("目标出站")).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(ruleDialog).toBeHidden();
  await expect(page.getByRole("button", { name: "新增规则", exact: true })).toBeFocused();

  await page.getByRole("tab", { name: "DNS", exact: true }).click();
  await page.getByRole("button", { name: "新增服务器" }).click();
  const dnsDialog = page.getByRole("dialog", { name: "新增 DNS 服务器" });
  await expect(dnsDialog.locator('select[name="detour"] option')).toHaveText(["默认出站", ...tags]);
  await dnsDialog.getByRole("button", { name: "取消" }).click();
  await page.getByRole("tab", { name: "出站", exact: true }).click();
}

test("按原型创建、引用、编辑和持久化自定义配置出站", async ({ page }) => {
  test.setTimeout(60_000);
  await login(page);
  await createSubscription(page);
  await createProfile(page);
  await openOutbounds(page);
  const initialOutboundCount = await page.locator(".outbound-card").count();
  expect(initialOutboundCount).toBeGreaterThan(0);

  await addDirect(page);
  await addUrlTest(page);
  await addSelectorWithSubscription(page);
  await expect(page.locator(".save-status")).toContainText("草稿已保存");
  await expectOutboundOptions(page);

  await expect(page.locator(".outbound-members")).toHaveCount(0);
  await expect(page.getByLabel("测试间隔")).toHaveValue("5m");
  await expect(page.locator(".save-status")).toContainText("草稿已保存");

  const persisted = await page.evaluate(async (name) => {
    const profiles = await (await fetch("/api/v1/profiles")).json() as Array<{ id: string; name: string }>;
    const profile = profiles.find((item) => item.name === name);
    return { id: profile?.id, draft: await (await fetch(`/api/v1/profiles/${profile?.id}/draft`)).json() as { revision: number; schemaVersion: number; buffers: Record<string, string>; references: { outbounds: Array<Record<string, unknown>> }; editorState: { outbounds: { groups: Array<Record<string, unknown>> } } } };
  }, profileName);
  const saved = JSON.parse(persisted.draft.buffers.outbounds) as Array<Record<string, unknown>>;
  expect(persisted.draft.schemaVersion).toBe(2);
  expect(saved.find((item) => item.tag === "节点选择")).not.toHaveProperty("outbounds");
  expect(saved.find((item) => item.tag === "新加坡 E2E")).toBeUndefined();
  expect(saved.find((item) => item.tag === "香港 E2E")).toBeUndefined();
  expect(saved.find((item) => item.tag === "节点选择")).not.toHaveProperty("hidden");
  expect(saved.find((item) => item.tag === "节点选择")).not.toHaveProperty("include");
  expect(persisted.draft.buffers.outbounds).not.toContain("subscription-node");
  expect(persisted.draft.buffers.outbounds).not.toContain("sourceId");
  expect(persisted.draft.references.outbounds.find((item) => item.tag === "节点选择")?.references).toMatchObject([
    { kind: "outbound" },
    { kind: "subscription" }
  ]);
  expect(persisted.draft.editorState.outbounds.groups.find((item) => item.tag === "节点选择")).not.toHaveProperty("references");
  const candidate = await page.evaluate(async ({ id, revision }) => {
    const session = await (await fetch("/api/v1/auth/session")).json();
    const response = await fetch(`/api/v1/profiles/${id}/preview`, { method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": session.csrfToken }, body: JSON.stringify({ revision }) });
    return await response.json() as { files: Array<{ name: string; content: { outbounds?: Array<Record<string, unknown>> } }> };
  }, { id: persisted.id, revision: persisted.draft.revision });
  const generated = candidate.files.find((item) => item.name === "70-outbounds.json")?.content.outbounds ?? [];
  expect(generated.find((item) => item.tag === "节点选择")?.outbounds).toEqual(["E2E 自动选择", "新加坡 E2E", "香港 E2E"]);
  expect(generated.find((item) => item.tag === "新加坡 E2E")).toMatchObject({ password: "outbound-e2e-secret", tls: { enabled: true } });
  expect(generated.find((item) => item.tag === "香港 E2E")).toMatchObject({ future: { keep: true } });

  await page.goto("/");
  const dashboardSelector = page.locator('[data-proxy-group-tag="节点选择"]');
  await expect(dashboardSelector).toBeVisible();
  const expandSelector = dashboardSelector.locator(".proxy-group-main");
  if (await expandSelector.getAttribute("aria-expanded") !== "true") await expandSelector.click();
  const dashboardMembers = dashboardSelector.locator(".proxy-node-card");
  await expect(dashboardMembers).toHaveCount(3);
  expect(await dashboardMembers.evaluateAll((members) => members.map((member) => member.getAttribute("data-proxy-member-tag")))).toEqual(["E2E 自动选择", "新加坡 E2E", "香港 E2E"]);
  const firstSelection = dashboardMembers.first().getByRole("button", { name: "选择 E2E 自动选择", exact: true });
  const nodeSelection = dashboardMembers.nth(1).getByRole("button", { name: "选择 新加坡 E2E", exact: true });
  const nodeDelay = dashboardMembers.nth(1).getByRole("button", { name: "测试 新加坡 E2E 延迟", exact: true });
  await nodeDelay.click();
  await expect(nodeDelay).toHaveText(/\d+ ms/);
  await expect(firstSelection).toHaveAttribute("aria-pressed", "false");
  await expect(nodeSelection).toHaveAttribute("aria-pressed", "false");
  await expect(firstSelection).toBeDisabled();
  await expect(nodeSelection).toBeDisabled();
  await expect(dashboardMembers.nth(2).locator(".proxy-latency")).toHaveText("未测速");
  await nodeDelay.focus();
  await expect(nodeDelay).toBeFocused();
  await page.keyboard.press("Space");
  await expect(nodeDelay).toHaveText(/\d+ ms/);
  await expect(nodeSelection).toHaveAttribute("aria-pressed", "false");

  await openOutbounds(page);
  await page.reload();
  await page.getByRole("tab", { name: "出站", exact: true }).click();
  await expect(page.locator(".outbound-card")).toHaveCount(initialOutboundCount + 3);
  const selectorCard = page.locator('[data-outbound-tag="节点选择"]');
  await expect(selectorCard.getByRole("button", { name: "编辑出站 节点选择" })).toBeVisible();
  await expect(selectorCard.getByRole("button", { name: "删除出站 节点选择" })).toBeVisible();
  await page.getByRole("button", { name: "编辑出站 节点选择" }).click();
  const editor = page.getByRole("dialog", { name: "出站" });
  await expect(editor.getByRole("button", { name: "删除出站" })).toHaveCount(0);
  await expect(editor.getByText("已选 2 项 · 3 个出站")).toBeVisible();
  await editor.getByLabel("隐藏出站").check();
  await editor.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.locator('[data-outbound-tag="节点选择"]')).toContainText("隐藏");

  await page.getByRole("button", { name: "编辑出站 E2E 自动选择" }).click();
  const renameDialog = page.getByRole("dialog", { name: "出站" });
  await renameDialog.locator('input[name="tag"]').fill("E2E 自动选择-已改");
  await renameDialog.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.locator('[data-outbound-tag="E2E 自动选择-已改"]')).toBeVisible();
  await page.getByRole("button", { name: "编辑出站 节点选择" }).click();
  const selectorDialog = page.getByRole("dialog", { name: "出站" });
  const renamedReference = selectorDialog.locator(".reference-tile").filter({ hasText: "E2E 自动选择-已改" });
  await expect(renamedReference.locator('input[type="checkbox"]')).toBeChecked();
  await selectorDialog.getByRole("button", { name: "取消" }).click();
  await expectOutboundOptions(page);

  await page.setViewportSize({ width: 390, height: 844 });
  await page.evaluate(() => { document.documentElement.dataset.theme = "dark"; });
  await page.getByRole("button", { name: "编辑出站 E2E 自动选择-已改" }).click();
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

  await page.goto("/workspace");
  await page.getByRole("group", { name: "档案操作" }).getByRole("button", { name: "删除", exact: true }).click();
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
