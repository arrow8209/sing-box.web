import { expect, test } from "@playwright/test";

test("核心页展示当前版本、最新版本和下载入口", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("管理密码").fill("correct horse battery staple");
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page.getByRole("heading", { name: "仪表盘" })).toBeVisible();
  await page.goto("/core");
  await expect(page.getByRole("heading", { name: "核心管理" })).toBeVisible();
  const versions = page.getByLabel("版本管理");
  await expect(versions.getByRole("article", { name: "当前版本" })).toContainText("1.0.0");
  await expect(versions.getByRole("article", { name: "最新版本" })).toContainText("1.0.0");
  await versions.getByRole("button", { name: "下载最新版本" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("heading", { name: "下载最新版本" })).toBeVisible();
  await expect(dialog.locator(".target")).toContainText("1.0.0");
});
