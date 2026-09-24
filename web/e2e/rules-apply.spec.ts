import { expect, test } from "./fixtures";

async function login(page: import("@playwright/test").Page): Promise<void> {
  await page.goto("/");
  await page.getByLabel("管理密码").fill("correct horse battery staple");
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page.getByRole("heading", { name: "仪表盘" })).toBeVisible();
}

async function deleteRule(page: import("@playwright/test").Page, name: string): Promise<void> {
  await page.getByRole("button", { name: `编辑 ${name}` }).click();
  const editor = page.getByRole("dialog", { name: "编辑规则集" });
  await editor.getByRole("button", { name: "删除", exact: true }).click();
  const confirmation = page.getByRole("dialog", { name: "删除规则集" });
  await expect(confirmation.getByRole("button", { name: "确认删除" })).toBeEnabled();
  await confirmation.getByRole("button", { name: "确认删除" }).click();
  await expect(page.getByText(name, { exact: true })).toHaveCount(0);
}

test("规则页使用真实受认证 API，未提供配置导出入口", async ({ page }) => {
  await login(page);
  await page.goto("/rules");
  await expect(page.getByRole("heading", { name: "规则集中心" })).toBeVisible();
  await expect(page.getByText("导出配置", { exact: true })).toHaveCount(0);
});

test("可创建手工规则，并从完整 Ruleset-Hub 目录预览和导入", async ({ page }) => {
  await login(page);
  await page.goto("/rules");

  await page.getByRole("button", { name: "添加规则集" }).click();
  const create = page.getByRole("dialog", { name: "添加规则集" });
  await expect(create.getByText("Binary", { exact: true })).toBeVisible();
  await expect(create.getByText(".srs · 网络规则固定格式", { exact: true })).toBeVisible();
  await expect(create.getByText("Source", { exact: true })).toHaveCount(0);
  await create.getByText("纯手工规则", { exact: true }).click();
  await expect(create.getByText("手工规则固定使用 Source JSON", { exact: false })).toBeVisible();
  await create.getByPlaceholder("例如 geosite-cn").fill("E2E 手工规则");
  await create.locator("textarea").fill(`{"version":3,"rules":[{"domain_suffix":["e2e.example"]}]}`);
  await expect(create.getByText("JSON 有效 · 1 个规则块")).toBeVisible();
  await create.getByRole("button", { name: "添加规则集" }).click();
  await expect(page.getByText("E2E 手工规则", { exact: true })).toBeVisible();

  await page.locator(".rules-toolbar select").selectOption("manual");
  await expect(page.locator(".ruleset-data-table tbody tr")).toHaveCount(1);
  await expect(page.getByText("纯手工", { exact: true })).toBeVisible();
  await page.locator(".rules-toolbar select").selectOption("all");

  await page.getByRole("button", { name: "规则集中心", exact: true }).click();
  const catalog = page.getByRole("dialog", { name: "规则集中心" });
  const desktopViewport = page.viewportSize()!;
  const catalogBounds = await catalog.boundingBox();
  expect(catalogBounds).not.toBeNull();
  expect(catalogBounds!.width).toBe(Math.min(1440, desktopViewport.width - 32));
  await page.setViewportSize({ width: 900, height: 1024 });
  expect((await catalog.boundingBox())!.width).toBe(868);
  await page.setViewportSize({ width: 390, height: 844 });
  expect((await catalog.boundingBox())!.width).toBe(390);
  await page.setViewportSize(desktopViewport);
  await expect(catalog.getByText("2139", { exact: true }).first()).toBeVisible();
  await catalog.getByPlaceholder("搜索规则名称或类型...").fill("cloudflare");
  await catalog.getByRole("button", { name: /GeoIP/ }).click();
  await expect(catalog.locator(".catalog-card")).toHaveCount(1);
  await expect(catalog.locator(".catalog-card")).toContainText("cloudflare");
  await expect(catalog.getByText("规则数量：711", { exact: true })).toBeVisible();
  expect((await catalog.locator(".catalog-card").boundingBox())!.height).toBeLessThan(110);

  await catalog.getByRole("button", { name: "预览 cloudflare 规则" }).click();
  await expect(catalog.getByText("Source 内容结构预览（仅预览）")).toBeVisible();
  await expect(catalog.locator("pre")).toContainText("192.0.2.0/24");
  await catalog.getByRole("button", { name: "返回中心" }).click();
  await catalog.getByRole("button", { name: "更多", exact: true }).click();
  await expect(catalog.getByText("Source 内容结构预览（仅预览）")).toBeVisible();
  await catalog.getByRole("button", { name: "返回中心" }).click();
  await expect(catalog.getByRole("button", { name: /添加 Source|添加 Binary/ })).toHaveCount(0);
  await catalog.getByRole("button", { name: "添加到配置" }).click();
  await expect(catalog.getByRole("button", { name: "已添加到配置" })).toBeVisible();
  await catalog.getByRole("button", { name: "刷新" }).click();
  await expect(catalog.getByText("2139", { exact: true }).first()).toBeVisible();
  await catalog.getByRole("button", { name: "完成" }).click();

  await expect(page.getByText("cloudflare-geoip", { exact: true })).toBeVisible();
  const importedRow = page.locator(".ruleset-data-table tbody tr").filter({ hasText: "cloudflare-geoip" });
  await expect(importedRow.locator(".rule-count")).toHaveText("711");
  await deleteRule(page, "cloudflare-geoip");
  await deleteRule(page, "E2E 手工规则");
});
