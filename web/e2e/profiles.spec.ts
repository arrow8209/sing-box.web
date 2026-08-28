import { expect, test } from "@playwright/test";

async function login(page: import("@playwright/test").Page): Promise<void> {
  await page.goto("/");
  await page.getByLabel("管理密码").fill("correct horse battery staple");
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page.getByRole("heading", { name: "仪表盘" })).toBeVisible();
}

async function clearRuleBindings(page: import("@playwright/test").Page, name: string): Promise<void> {
  await page.evaluate(async (profileName) => {
    const session = await (await fetch("/api/v1/auth/session")).json() as { csrfToken: string };
    const profiles = await (await fetch("/api/v1/profiles")).json() as Array<{ id: string; name: string }>;
    const profile = profiles.find((item) => item.name === profileName);
    if (!profile) throw new Error("profile not found");
    const response = await fetch(`/api/v1/profiles/${profile.id}/draft`);
    const draft = await response.json() as { buffers: Record<string, string>; editorState?: Record<string, unknown> };
    const buffers = { ...draft.buffers };
    const route = JSON.parse(buffers.route ?? "{}") as { rules?: Array<Record<string, unknown>> };
    route.rules = route.rules?.filter((rule) => rule.rule_set === undefined) ?? [];
    buffers.route = JSON.stringify(route);
    const saved = await fetch(`/api/v1/profiles/${profile.id}/draft`, {
      method: "PUT",
      headers: { "Content-Type": "application/json", "If-Match": response.headers.get("ETag") ?? "", "X-CSRF-Token": session.csrfToken },
      body: JSON.stringify({ buffers, editorState: draft.editorState ?? {}, bindings: [] })
    });
    if (!saved.ok) throw new Error(`save failed: ${saved.status}`);
  }, name);
}

test("创建、复制、改名和删除档案", async ({ page }) => {
  await login(page);
  await page.goto("/profiles");
  await page.getByRole("link", { name: "新建档案" }).click();
  await page.getByRole("dialog", { name: "新建档案" }).getByLabel("名称").fill("E2E 家庭出口");
  await page.getByRole("dialog", { name: "新建档案" }).getByRole("button", { name: "确认" }).click();
  await expect(page.getByRole("heading", { name: "E2E 家庭出口" })).toBeVisible();
  await expect(page.getByRole("img", { name: "sing-box 未启动，也未加载配置" })).toBeVisible();
  await expect(page.getByRole("group", { name: "档案操作" })).toHaveCount(0);
  await clearRuleBindings(page, "E2E 家庭出口");
  await page.goto("/workspace");
  await expect(page.locator(".config-context-bar").getByRole("link", { name: "管理档案" })).toHaveCount(0);
  await expect(page.getByRole("link", { name: /管理 E2E 家庭出口/ })).toHaveCount(0);
  const contextBar = page.locator(".config-context-bar");
  const profileActions = page.getByRole("group", { name: "档案操作" });
  const applicationActions = page.getByRole("group", { name: "配置操作" });
  await expect(contextBar.getByRole("button")).toHaveText(["重命名", "复制", "删除", "预览", "检测", "应用"]);
  await expect(applicationActions.getByRole("button", { name: "应用", exact: true })).toBeEnabled();
  await applicationActions.getByRole("button", { name: "应用", exact: true }).click();
  await expect(page.getByText("配置检测通过，已复制配置文件。")).toBeVisible();
  await expect(page.getByRole("img", { name: "sing-box 未启动，也未加载配置" })).toBeVisible();
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
