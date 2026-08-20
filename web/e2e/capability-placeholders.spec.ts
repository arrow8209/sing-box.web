import { expect, test, type Page } from "@playwright/test";

async function login(page: Page): Promise<void> {
  await page.goto("/login");
  await page.getByLabel("密码").fill("phase1-e2e-password");
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);
}

test("phase-one capability pages are truthful and core API is absent", async ({ page }) => {
  await login(page);
  for (const item of [
    { nav: "运行状态", url: "/runtime" },
    { nav: "订阅", url: "/subscriptions" },
    { nav: "规则集", url: "/rulesets" },
  ]) {
    await page.getByRole("link", { name: item.nav }).click();
    await expect(page).toHaveURL(new RegExp(`${item.url}$`));
    await expect(page.getByText("尚未接入", { exact: true })).toBeVisible();
    await expect(page.locator("body")).not.toContainText(/MB\/s|206 ms|核心重启完成|启动核心|重启核心/);
  }
  const core = await page.request.get("/api/v1/core");
  expect(core.status()).toBe(404);
});
