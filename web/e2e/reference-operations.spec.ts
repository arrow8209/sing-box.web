import { expect, test } from "@playwright/test";

test("自动重载开启时预览和检测不保存草稿，手动应用只提交一次", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("管理密码").fill("correct horse battery staple");
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page.getByRole("heading", { name: "仪表盘" })).toBeVisible();
  const fixture = await page.evaluate(async () => {
    const session = await (await fetch("/api/v1/auth/session")).json();
    const headers = { "Content-Type": "application/json", "X-CSRF-Token": session.csrfToken };
    const created = await fetch("/api/v1/profiles", { method: "POST", headers, body: JSON.stringify({ name: "操作副作用 E2E" }) });
    if (!created.ok) throw new Error("create failed");
    const profile = await created.json() as { id: string };
    const path = "/api/v1/profiles/" + profile.id;
    const response = await fetch(path + "/draft");
    const draft = await response.json() as { buffers: Record<string, string>; editorState: Record<string, unknown> };
    const buffers = Object.fromEntries(Object.keys(draft.buffers).map((key) => [key, ""]));
    buffers.inbounds = "[]"; buffers.outbounds = "[]"; buffers.route = "{}";
    const saved = await fetch(path + "/draft", { method: "PUT", headers: { ...headers, "If-Match": response.headers.get("ETag")! },
      body: JSON.stringify({ buffers, editorState: draft.editorState, bindings: [] }) });
    if (!saved.ok) throw new Error("save failed");
    const revision = (await saved.json() as { revision: number }).revision;
    const firstApply = await fetch(path + "/apply", { method: "POST", headers, body: JSON.stringify({ revision }) });
    if (!firstApply.ok || !(await firstApply.json() as { applied: boolean }).applied) throw new Error("initial apply failed");
    const behavior = await fetch("/api/v1/settings/core", { method: "PUT", headers, body: JSON.stringify({ autoStart: false, autoReload: true }) });
    if (!behavior.ok) throw new Error("settings failed");
    return { id: profile.id, revision };
  });
  await page.goto("/workspace");
  await page.locator(".config-name").filter({ hasText: "操作副作用 E2E" }).click();
  await expect(page.locator(".save-status")).toContainText("草稿已保存");
  const draftPuts: string[] = [];
  const applies: string[] = [];
  page.on("request", (request) => {
    if (request.url().endsWith("/profiles/" + fixture.id + "/draft") && request.method() === "PUT") draftPuts.push(request.url());
    if (request.url().endsWith("/profiles/" + fixture.id + "/apply") && request.method() === "POST") applies.push(request.url());
  });
  const state = async () => page.evaluate(async (id) => ({
    draft: await (await fetch("/api/v1/profiles/" + id + "/draft")).json() as { revision: number },
    meta: (await (await fetch("/api/v1/profiles")).json() as Array<{ id: string; appliedDigest: string }>).find((item) => item.id === id),
    core: await (await fetch("/api/v1/core")).json() as { state: string; pid: number }
  }), fixture.id);
  const before = await state();
  await page.locator("[data-preview]").click();
  await expect(page.getByRole("dialog", { name: "模块预览" })).toBeVisible();
  await page.getByRole("dialog", { name: "模块预览" }).getByRole("button", { name: "关闭", exact: true }).click();
  await page.locator("[data-validate]").click();
  await expect(page.locator(".errors")).toContainText("配置检查通过");
  const checked = await state();
  expect(draftPuts).toEqual([]);
  expect(applies).toEqual([]);
  expect(checked.draft.revision).toBe(before.draft.revision);
  expect(checked.meta?.appliedDigest).toBe(before.meta?.appliedDigest);
  expect(checked.core.state).toBe(before.core.state);
  expect(checked.core.pid).toBe(before.core.pid);
  await page.locator("[data-apply]").click();
  await expect(page.getByText("配置检测通过，已复制配置文件。")).toBeVisible();
  expect(draftPuts).toEqual([]);
  expect(applies).toHaveLength(1);
  const after = await state();
  expect(after.draft.revision).toBe(fixture.revision);
  expect(after.core.state).toBe(before.core.state);
  expect(after.core.pid).toBe(before.core.pid);
  await page.evaluate(async (id) => {
    const session = await (await fetch("/api/v1/auth/session")).json();
    const headers = { "Content-Type": "application/json", "X-CSRF-Token": session.csrfToken };
    await fetch("/api/v1/settings/core", { method: "PUT", headers, body: JSON.stringify({ autoStart: false, autoReload: false }) });
    await fetch("/api/v1/profiles/" + id, { method: "DELETE", headers, body: JSON.stringify({ confirmName: "操作副作用 E2E", stopCurrent: true }) });
  }, fixture.id);
});
