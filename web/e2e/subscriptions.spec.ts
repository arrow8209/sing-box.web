import { expect, test } from "@playwright/test";

async function login(page: import("@playwright/test").Page): Promise<void> {
  await page.goto("/");
  await page.getByLabel("管理密码").fill("correct horse battery staple");
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page.getByRole("heading", { name: "仪表盘" })).toBeVisible();
}

test("可通过真实 API 导入、查看、整体编辑和删除 JSON 订阅", async ({ page }) => {
  await login(page);
  await page.goto("/subscriptions");
  await expect(page.getByRole("heading", { name: "订阅配置" })).toBeVisible();
  await expect(page.locator(".subscription-card")).toHaveCount(0);

  await page.getByRole("button", { name: "导入 JSON 订阅" }).click();
  const create = page.getByRole("dialog", { name: "导入 JSON 订阅" });
  await create.getByLabel("订阅 JSON").fill(JSON.stringify({
    name: "E2E 节点集合",
    experimental: { ignored: true },
    outbounds: [
      { type: "trojan", tag: "新加坡 01", server: "sg.example.test", server_port: 443, password: "e2e-secret" },
      { type: "vless", tag: "香港 01", server: "hk.example.test", server_port: 8443, uuid: "00000000-0000-0000-0000-000000000000" }
    ]
  }));
  await expect(create.getByText("JSON 有效 · 将导入 2 个节点 · 2 种协议")).toBeVisible();
  await create.getByRole("button", { name: "导入全部节点" }).click();

  const card = page.locator(".subscription-card").filter({ hasText: "E2E 节点集合" });
  await expect(card).toBeVisible();
  await expect(card).toContainText("2 个出站");
  await expect(card).toContainText("2 种");
  await page.reload();
  await expect(card).toBeVisible();

  await card.press("Enter");
  const nodes = page.getByRole("dialog", { name: "全部订阅节点" });
  await expect(nodes.getByText("新加坡 01", { exact: true })).toBeVisible();
  await expect(nodes.getByText("sg.example.test:443", { exact: true })).toBeVisible();
  await nodes.getByRole("button", { name: "完成" }).click();

  await card.getByRole("button", { name: "编辑 JSON" }).click();
  const editor = page.getByRole("dialog", { name: "编辑 JSON 订阅" });
  await expect(editor.getByLabel("订阅 JSON")).toHaveValue(/e2e-secret/);
  await editor.getByLabel("订阅 JSON").fill(JSON.stringify({
    name: "E2E 已更新",
    nodes: [{ type: "hysteria2", tag: "东京 01", server: "jp.example.test", server_port: 443 }]
  }));
  await editor.getByRole("button", { name: "保存全部节点" }).click();
  await expect(page.getByText("E2E 已更新", { exact: true })).toBeVisible();
  await expect(page.locator(".subscription-card")).toContainText("1 个出站");

  await page.locator(".subscription-card").getByRole("button", { name: "编辑 JSON" }).click();
  await page.getByRole("dialog", { name: "编辑 JSON 订阅" }).getByRole("button", { name: "删除订阅" }).click();
  const confirmation = page.getByRole("dialog", { name: "删除 JSON 订阅" });
  await confirmation.getByRole("button", { name: "确认删除" }).click();
  await expect(page.locator(".subscription-card")).toHaveCount(0);
});

test("重复节点标签会阻止提交", async ({ page }) => {
  await login(page);
  await page.goto("/subscriptions");
  await page.getByRole("button", { name: "导入 JSON 订阅" }).click();
  const dialog = page.getByRole("dialog", { name: "导入 JSON 订阅" });
  await dialog.getByLabel("订阅 JSON").fill(`[{"type":"trojan","tag":"香港"},{"type":"vless","tag":" 香港 "}]`);
  await expect(dialog.getByText("节点标签不能重复：香港")).toBeVisible();
  await expect(dialog.getByRole("button", { name: "导入全部节点" })).toBeDisabled();
});
