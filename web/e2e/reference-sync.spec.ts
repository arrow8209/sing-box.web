import { expect, test } from "@playwright/test";

test("修改订阅后直接检测生成新候选，草稿与 revision 不变", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("管理密码").fill("correct horse battery staple");
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page.getByRole("heading", { name: "仪表盘", exact: true })).toBeVisible();
  const fixture = await page.evaluate(async () => {
    const session = await (await fetch("/api/v1/auth/session")).json();
    const headers = { "Content-Type": "application/json", "X-CSRF-Token": session.csrfToken };
    const node = { type: "ssh", tag: "sync-test-ssh", server: "old.example.test", server_port: 22, user: "test", password: "test-only", detour: "removed-group" };
    const subscriptionResponse = await fetch("/api/v1/subscriptions", { method: "POST", headers, body: JSON.stringify({ name: "同步测试订阅", outbounds: [node] }) });
    if (!subscriptionResponse.ok) throw new Error(`subscription fixture failed: ${subscriptionResponse.status}`);
    const subscription = await subscriptionResponse.json();
    const profileResponse = await fetch("/api/v1/profiles", { method: "POST", headers, body: JSON.stringify({ name: "订阅同步 E2E" }) });
    if (!profileResponse.ok) throw new Error(`profile fixture failed: ${profileResponse.status}`);
    const profile = await profileResponse.json();
    const response = await fetch(`/api/v1/profiles/${profile.id}/draft`);
    const outbounds = [{ type: "selector", tag: "sync-select", outbounds: [node.tag], interrupt_exist_connections: true }, node];
    const editorState = { outbounds: { version: 1, generatedTags: [node.tag], groups: [{
      id: "sync-test-group", type: "selector", tag: "sync-select", hidden: false, include: "", exclude: "", icon: "",
      references: [{ kind: "subscription", subscriptionId: subscription.id, tag: subscription.name, resolvedTags: [node.tag] }], runtimeTags: [node.tag]
    }] } };
    const saved = await fetch(`/api/v1/profiles/${profile.id}/draft`, { method: "PUT", headers: { ...headers, "If-Match": response.headers.get("ETag")! }, body: JSON.stringify({
      buffers: { outbounds: JSON.stringify(outbounds), inbounds: "[]", route: '{"final":"sync-select"}' }, bindings: [], editorState
    }) });
    if (!saved.ok) throw new Error(`draft fixture failed: ${saved.status}`);
    return { profileId: profile.id, subscriptionId: subscription.id };
  });

  await page.goto("/workspace");
  await page.getByRole("button", { name: /订阅同步 E2E/ }).click();
  await expect(page.getByRole("heading", { name: "订阅同步 E2E", exact: true })).toBeVisible();
  const before = await page.evaluate(async ({ profileId, subscriptionId }) => {
    const session = await (await fetch("/api/v1/auth/session")).json();
    const updated = await fetch(`/api/v1/subscriptions/${subscriptionId}`, { method: "PUT", headers: { "Content-Type": "application/json", "X-CSRF-Token": session.csrfToken }, body: JSON.stringify({
      name: "同步测试订阅", outbounds: [{ type: "ssh", tag: "sync-test-ssh", server: "updated.example.test", server_port: 22, user: "test", password: "test-only" }]
    }) });
    if (!updated.ok) throw new Error(`subscription update failed: ${updated.status}`);
    const draft = await (await fetch(`/api/v1/profiles/${profileId}/draft`)).json();
    return { revision: draft.revision, outbounds: draft.buffers.outbounds };
  }, fixture);
  expect(before.outbounds).toContain("removed-group");
  const internalSaves: string[] = [];
  page.on("request", (request) => { if (request.method() === "PUT" && request.url().endsWith(`/profiles/${fixture.profileId}/draft`)) internalSaves.push(request.url()); });
  await page.locator("[data-validate]").click();
  await expect(page.locator(".errors")).toContainText("配置检查通过");
  const after = await page.evaluate(async (id) => (await fetch(`/api/v1/profiles/${id}/draft`)).json(), fixture.profileId);
  expect(after.revision).toBe(before.revision);
  expect(after.buffers.outbounds).toBe(before.outbounds);
  expect(internalSaves).toEqual([]);
  await page.locator("[data-preview]").click();
  const preview = page.getByRole("dialog", { name: "模块预览" });
  await expect(preview).toContainText("updated.example.test");
  await expect(preview).not.toContainText("removed-group");
  await preview.getByRole("button", { name: "关闭", exact: true }).click();
  const revision = await page.evaluate(async (id) => (await (await fetch(`/api/v1/profiles/${id}/draft`)).json()).revision, fixture.profileId);
  expect(revision).toBe(after.revision);
  expect(internalSaves).toEqual([]);

  await page.evaluate(async ({ profileId, subscriptionId }) => {
    const session = await (await fetch("/api/v1/auth/session")).json();
    const headers = { "Content-Type": "application/json", "X-CSRF-Token": session.csrfToken };
    const removed = await fetch(`/api/v1/profiles/${profileId}`, { method: "DELETE", headers, body: JSON.stringify({ confirmName: "订阅同步 E2E", stopCurrent: false }) });
    if (!removed.ok) throw new Error(`profile cleanup failed: ${removed.status}`);
    const unsubscribed = await fetch(`/api/v1/subscriptions/${subscriptionId}`, { method: "DELETE", headers });
    if (!unsubscribed.ok) throw new Error(`subscription cleanup failed: ${unsubscribed.status}`);
  }, fixture);
});
