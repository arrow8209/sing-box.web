import { expect, test } from "@playwright/test";

test("从根菜单进入核心管理并确认下载最新稳定版", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("管理密码").fill("correct horse battery staple");
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page.getByRole("heading", { name: "仪表盘" })).toBeVisible();
  const navigation = page.getByRole("navigation", { name: "主导航" });
  const coreNavigation = navigation.getByRole("link", { name: "核心管理", exact: true });
  await coreNavigation.click();
  await expect(page).toHaveURL(/\/core$/);
  await expect(coreNavigation).toHaveAttribute("aria-current", "page");
  await expect(page.getByRole("heading", { name: "核心管理" })).toBeVisible();
  const versions = page.getByLabel("版本管理");
  await expect(versions.getByRole("article", { name: "当前版本" })).toContainText("1.0.0");
  await expect(versions.getByRole("article", { name: "最新版本" })).toContainText("1.0.0");
  const download = versions.getByRole("button", { name: /重新安装最新版本|下载最新版本/ });
  await download.click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("heading", { name: "确认手动更新" })).toBeVisible();
  await expect(dialog).toContainText("1.0.0");
  await expect(dialog.getByRole("button", { name: "关闭对话框" })).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  await expect(download).toBeFocused();
});
