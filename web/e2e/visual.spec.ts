import { expect, test, type Page } from "@playwright/test";

const scenarios = [
  { name: "desktop-light", width: 1440, height: 900, theme: "light" },
  { name: "desktop-dark", width: 1440, height: 900, theme: "dark" },
  { name: "tablet-light", width: 900, height: 1024, theme: "light" },
  { name: "tablet-dark", width: 900, height: 1024, theme: "dark" },
  { name: "mobile-light", width: 390, height: 844, theme: "light" },
  { name: "mobile-dark", width: 390, height: 844, theme: "dark" }
] as const;

async function loginAndOpenProfile(page: Page): Promise<void> {
  await page.goto("/");
  const loginButton = page.getByRole("button", { name: "登录" });
  const dashboardHeading = page.getByRole("heading", { name: "仪表盘" });
  await expect(loginButton.or(dashboardHeading)).toBeVisible();
  if (await loginButton.isVisible()) {
    await page.getByLabel("管理密码").fill("correct horse battery staple");
    await Promise.all([
      page.waitForURL((url) => url.pathname === "/"),
      loginButton.click()
    ]);
  }
  await expect(dashboardHeading).toBeVisible();
}

async function setTheme(page: Page, theme: "light" | "dark"): Promise<void> {
  await page.evaluate((value) => { document.documentElement.dataset.theme = value; }, theme);
}

for (const scenario of scenarios) {
  test(`${scenario.name} configuration workspace`, async ({ page }) => {
    await page.setViewportSize({ width: scenario.width, height: scenario.height });
    await loginAndOpenProfile(page);
    await setTheme(page, scenario.theme);
    await expect(page.locator("main")).toBeVisible();
    await expect(page.locator("body")).not.toHaveCSS("overflow-x", "scroll");
    await expect(page).toHaveScreenshot(`${scenario.name}-config.png`, { fullPage: true });
  });
}
