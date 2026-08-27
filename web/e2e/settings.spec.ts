import { expect, test } from "@playwright/test";

test("可保存下载代理并留空恢复直连", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("管理密码").fill("correct horse battery staple");
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page.getByRole("heading", { name: "仪表盘" })).toBeVisible();
  await page.goto("/settings");

  const coreSettings = page.getByRole("region", { name: "核心设置" });
  await expect(coreSettings).toBeVisible();
  await expect(coreSettings.getByLabel("核心分支")).toHaveValue("Stable · 1.0.0");
  await expect(coreSettings.getByRole("link", { name: "管理版本" })).toHaveAttribute("href", "/core");

  const proxyUrl = page.getByLabel("代理服务器 URL");
  await expect(proxyUrl).toHaveValue("");
  await proxyUrl.fill("http://127.0.0.1:7890");
  await page.getByRole("button", { name: "保存代理设置" }).click();
  await expect(page.getByText("代理设置已保存并即时生效")).toBeVisible();

  await page.reload();
  await expect(proxyUrl).toHaveValue("http://127.0.0.1:7890");
  await proxyUrl.fill("");
  await page.getByRole("button", { name: "保存代理设置" }).click();
  await expect(page.getByText("代理设置已保存并即时生效")).toBeVisible();
  await expect(page.getByText("直连", { exact: true })).toBeVisible();

  const proxyNavigation = page.getByRole("link", { name: "代理设置" });
  await proxyNavigation.focus();
  await expect(proxyNavigation).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(/#proxy$/);
  await proxyUrl.focus();
  await expect(proxyUrl).toBeFocused();

  for (const viewport of [{ width: 900, height: 1024 }, { width: 390, height: 844 }]) {
    await page.setViewportSize(viewport);
    await page.evaluate(() => { document.documentElement.dataset.theme = "dark"; });
    await expect(page.getByRole("heading", { name: "代理设置" })).toBeVisible();
    expect(await page.locator("body").evaluate((body) => body.scrollWidth <= body.clientWidth)).toBe(true);
  }
});
