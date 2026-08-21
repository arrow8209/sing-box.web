import { expect, test } from "@playwright/test";

test("规则页使用真实受认证 API，未提供订阅或配置导出入口", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("管理密码").fill("correct horse battery staple");
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page.getByRole("heading", { name: "仪表盘" })).toBeVisible();
  await page.goto("/rules");
  await expect(page.getByRole("heading", { name: "规则资源" })).toBeVisible();
  await expect(page.getByText("订阅", { exact: true })).toHaveCount(0);
  await expect(page.getByText("导出配置", { exact: true })).toHaveCount(0);
});
