import { expect, test } from "@playwright/test";

test("核心页只展示隔离 fake core 状态并要求人工确认", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("管理密码").fill("correct horse battery staple");
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page.getByRole("heading", { name: "仪表盘" })).toBeVisible();
  await page.goto("/core");
  await expect(page.getByRole("heading", { name: "核心管理" })).toBeVisible();
  await page.getByRole("button", { name: /手动更新|手动下载/ }).first().click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("heading", { name: "确认手动更新" })).toBeVisible();
  await expect(dialog.getByText("目标稳定版：1.0.0")).toBeVisible();
  await dialog.getByRole("button", { name: "确认更新" }).click();
  const progress = page.getByRole("progressbar", { name: "核心下载进度" });
  await expect(progress).toHaveAttribute("aria-valuenow", "25");
  await expect(page.getByText("8 MB / 32 MB")).toBeVisible();
  await expect(page.getByText("4 MB/s")).toBeVisible();
});
