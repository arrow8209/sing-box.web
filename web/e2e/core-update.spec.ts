import { expect, test } from "@playwright/test";

test("从根菜单进入核心管理并直接下载最新稳定版", async ({ page }) => {
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
  const updateRequest = page.waitForRequest((request) => request.method() === "POST" && new URL(request.url()).pathname === "/api/v1/core/update");
  await download.click();
  const request = await updateRequest;
  expect(request.postDataJSON()).toEqual({ confirmedVersion: "1.0.0" });
  await expect(page.getByRole("dialog")).toHaveCount(0);
  const progress = page.getByRole("progressbar", { name: "核心下载进度" });
  await expect(progress).toHaveAttribute("aria-valuenow", "25");
  await expect(versions.getByText("8 MB / 32 MB")).toBeVisible();
  await expect(versions.getByText("4 MB/s")).toBeVisible();
});
