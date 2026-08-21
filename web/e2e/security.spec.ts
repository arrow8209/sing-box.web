import { expect, test } from "@playwright/test";

test("未知 API 不回退为 SPA，深层 SPA 路由可用", async ({ page, request }) => {
  const api = await request.get("/api/v1/not-found");
  expect(api.status()).toBe(404);
  expect(api.headers()["content-type"]).toContain("application/json");
  await page.goto("/workspace");
  await expect(page).toHaveURL(/\/login\?redirect=/);
});
