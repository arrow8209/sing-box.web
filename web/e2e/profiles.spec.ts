import { expect, test } from "@playwright/test";

async function login(page: import("@playwright/test").Page): Promise<void> {
  await page.goto("/");
  await page.getByLabel("管理密码").fill("correct horse battery staple");
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page.getByRole("heading", { name: "仪表盘" })).toBeVisible();
}

test("创建、改名和删除档案", async ({ page }) => {
  await login(page);
  await page.goto("/profiles");
  await page.getByRole("button", { name: "新建档案" }).click();
  await page.getByRole("dialog", { name: "新建档案" }).getByLabel("名称").fill("E2E 家庭出口");
  await page.getByRole("dialog", { name: "新建档案" }).getByRole("button", { name: "确认" }).click();
  await expect(page.getByRole("heading", { name: "E2E 家庭出口" })).toBeVisible();
  await page.getByRole("button", { name: "重命名" }).click();
  await page.getByRole("dialog", { name: "重命名档案" }).getByLabel("名称").fill("E2E 备用出口");
  await page.getByRole("dialog", { name: "重命名档案" }).getByRole("button", { name: "确认" }).click();
  await expect(page.getByRole("heading", { name: "E2E 备用出口" })).toBeVisible();
  await page.getByRole("button", { name: "删除" }).click();
  await page.getByLabel("确认名称").fill("E2E 备用出口");
  await page.getByRole("dialog", { name: "删除档案" }).getByRole("button", { name: "删除" }).click();
  await expect(page.getByRole("button", { name: /E2E 备用出口/ })).toHaveCount(0);
});
