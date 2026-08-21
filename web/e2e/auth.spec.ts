import { expect, test } from "@playwright/test";

test("仅密码登录且会话 Cookie 受保护", async ({ page }) => {
  const response = await page.goto("/");
  expect(response?.status()).toBe(200);
  const sessionResponse = await page.request.get("/api/v1/auth/session");
  expect(sessionResponse.headers()["x-content-type-options"]).toBe("nosniff");
  await expect(page.locator('input[name="username"]')).toHaveCount(0);
  await page.getByLabel("管理密码").fill("correct horse battery staple");
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page.getByRole("heading", { name: "仪表盘" })).toBeVisible();
  const cookies = await page.context().cookies();
  const session = cookies.find((cookie) => cookie.name === "singbox_session");
  expect(session?.httpOnly).toBe(true);
  expect(session?.sameSite).toBe("Strict");
});
