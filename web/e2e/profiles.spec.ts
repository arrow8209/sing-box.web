import { expect, test } from "@playwright/test";

async function login(page: import("@playwright/test").Page): Promise<void> {
  await page.goto("/");
  await page.getByLabel("管理密码").fill("correct horse battery staple");
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page.getByRole("heading", { name: "仪表盘" })).toBeVisible();
}

test("创建、复制、改名和删除档案", async ({ page }) => {
  await login(page);
  await page.goto("/profiles");
  await page.getByRole("button", { name: "新建档案" }).click();
  await page.getByRole("dialog", { name: "新建档案" }).getByLabel("名称").fill("E2E 家庭出口");
  await page.getByRole("dialog", { name: "新建档案" }).getByRole("button", { name: "确认" }).click();
  await expect(page.getByRole("heading", { name: "E2E 家庭出口" })).toBeVisible();
  await expect(page.getByRole("group", { name: "档案操作" })).toHaveCount(0);
  await page.getByRole("link", { name: "打开配置工作台" }).click();
  await expect(page.locator(".config-context-bar").getByRole("link", { name: "管理档案" })).toHaveCount(0);
  await expect(page.getByRole("link", { name: /管理 E2E 家庭出口/ })).toHaveCount(0);
  const profileActions = page.getByRole("group", { name: "档案操作" });
  await expect(profileActions.getByRole("button")).toHaveText(["应用", "复制", "重命名", "删除"]);
  await expect(profileActions.getByRole("button", { name: "应用", exact: true })).toBeDisabled();
  await expect(profileActions.getByRole("button", { name: "应用", exact: true })).toHaveAttribute("title", "应用逻辑尚未实现");
  await profileActions.getByRole("button", { name: "复制" }).click();
  await expect(page.getByRole("heading", { name: "E2E 家庭出口 副本" })).toBeVisible();
  await expect(page.getByTitle("切换到 E2E 家庭出口", { exact: true })).toBeVisible();
  await expect(page.getByTitle("切换到 E2E 家庭出口 副本", { exact: true })).toBeVisible();
  await profileActions.getByRole("button", { name: "重命名" }).click();
  await page.getByRole("dialog", { name: "重命名档案" }).getByLabel("名称").fill("E2E 备用出口");
  await page.getByRole("dialog", { name: "重命名档案" }).getByRole("button", { name: "确认" }).click();
  await expect(page.getByRole("heading", { name: "E2E 备用出口" })).toBeVisible();
  await profileActions.getByRole("button", { name: "删除" }).click();
  await page.getByLabel("确认名称").fill("E2E 备用出口");
  await page.getByRole("dialog", { name: "删除档案" }).getByRole("button", { name: "删除" }).click();
  await expect(page.getByTitle("切换到 E2E 备用出口", { exact: true })).toHaveCount(0);
  await page.getByTitle("切换到 E2E 家庭出口", { exact: true }).click();
  await expect(page.getByRole("heading", { name: "E2E 家庭出口" })).toBeVisible();
  await profileActions.getByRole("button", { name: "删除" }).click();
  await page.getByLabel("确认名称").fill("E2E 家庭出口");
  await page.getByRole("dialog", { name: "删除档案" }).getByRole("button", { name: "删除" }).click();
  await expect(page.locator(".config-name")).toHaveCount(0);
});
