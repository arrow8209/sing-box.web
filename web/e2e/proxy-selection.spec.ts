import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { expect, test } from "./fixtures";

test("节点与模式切换经服务端送达核心，刷新保持选择且失败不会假报成功", async ({ page }, testInfo) => {
  test.setTimeout(90000);
  const tag = "🚀 节点/选择 #%";
  const first = "🇭🇰 香港 01";
  const target = "🇯🇵 日本 02 [0.3X]";
  let selected = first;
  let failWrite = false;
  let writes = 0;
  let mode = "rule";
  let failModeWrite = false;
  let timeoutDelay = false;
  const delayCalls: string[] = [];
  const history = new Map<string, Array<{ time: string; delay: number }>>();
  let delayGate: Promise<void> | undefined;
  let releaseDelay: (() => void) | undefined;
  const browserErrors: string[] = [];
  page.on("pageerror", (error) => browserErrors.push(error.message));
  const controller = createServer(async (request, response) => {
    if (request.headers.authorization !== "Bearer isolated-controller-secret") {
      response.writeHead(401).end();
      return;
    }
    const requestURL = new URL(request.url!, "http://127.0.0.1");
    const path = requestURL.pathname;
    if (path === "/connections") {
      response.setHeader("Content-Type", "application/json");
      response.end(JSON.stringify({ memory: 32 * 1024 * 1024, uploadTotal: 0, downloadTotal: 0, connections: [] }));
      return;
    }
    if (path === "/configs") {
      if (request.method === "PATCH") {
        if (failModeWrite) { response.writeHead(400).end(); return; }
        const chunks: Buffer[] = [];
        for await (const chunk of request) chunks.push(Buffer.from(chunk));
        mode = (JSON.parse(Buffer.concat(chunks).toString()) as { mode: string }).mode;
        response.writeHead(204).end();
      } else {
        response.setHeader("Content-Type", "application/json");
        response.end(JSON.stringify({ mode, "mode-list": ["rule", "global", "direct"] }));
      }
      return;
    }
    if (path.startsWith("/proxies/")) {
      const delay = path.endsWith("/delay");
      const node = decodeURIComponent(path.slice("/proxies/".length, delay ? -"/delay".length : undefined));
      if ([first, target].includes(node)) {
        response.setHeader("Content-Type", "application/json");
        if (!delay) { response.end(JSON.stringify({ type: "Direct", history: history.get(node) ?? [] })); return; }
        expect(requestURL.searchParams.get("url")).toBe("https://www.gstatic.com/generate_204");
        expect(requestURL.searchParams.get("timeout")).toBe("5000");
        delayCalls.push(node);
        await delayGate;
        if (timeoutDelay && node === target) {
          history.delete(node);
          response.writeHead(504).end(JSON.stringify({ message: "Timeout" }));
        } else {
          const measured = node === target ? 217 : 136;
          history.set(node, [{ time: new Date().toISOString(), delay: measured }]);
          response.end(JSON.stringify({ delay: measured }));
        }
        return;
      }
    }
    if (path !== "/proxies" && decodeURIComponent(path.slice("/proxies/".length)) !== tag) {
      response.writeHead(404).end();
      return;
    }
    if (request.method === "PUT") {
      writes++;
      if (failWrite) { response.writeHead(400).end(); return; }
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const input = JSON.parse(Buffer.concat(chunks).toString()) as { name: string };
      if (![first, target].includes(input.name)) { response.writeHead(400).end(); return; }
      selected = input.name;
      response.writeHead(204).end();
      return;
    }
    const group = { type: "Selector", now: selected, all: [first, target] };
    response.setHeader("Content-Type", "application/json");
    response.end(JSON.stringify(path === "/proxies" ? { proxies: {
      GLOBAL: { type: "Fallback", now: tag, all: [tag, "自动选择", first, target] },
      [tag]: group,
      "自动选择": { type: "URLTest", now: target, all: [first, target] },
      [first]: { type: "Direct", history: history.get(first) ?? [] },
      [target]: { type: "Direct", history: history.get(target) ?? [] }
    } } : group));
  });
  await new Promise<void>((resolve) => controller.listen(0, "127.0.0.1", resolve));
  const controllerAddress = `127.0.0.1:${(controller.address() as AddressInfo).port}`;
  let profileId = "";
  let headers: Record<string, string> = {};
  try {
    await page.goto("/");
    await page.getByLabel("管理密码").fill("correct horse battery staple");
    await page.getByRole("button", { name: "登录", exact: true }).click();
    await expect(page.getByRole("heading", { name: "仪表盘", exact: true })).toBeVisible();
    const session = await (await page.request.get("/api/v1/auth/session")).json() as { csrfToken: string };
    headers = { "X-CSRF-Token": session.csrfToken };
    const created = await page.request.post("/api/v1/profiles", { headers, data: { name: "E2E 节点切换" } });
    expect(created.ok()).toBe(true);
    profileId = (await created.json() as { id: string }).id;
    const draftResponse = await page.request.get(`/api/v1/profiles/${profileId}/draft`);
    const draft = await draftResponse.json() as { buffers: Record<string, string> };
    const input = { buffers: {
      ...draft.buffers,
      inbounds: "[]", dns: "{}", route: JSON.stringify({ final: tag }),
      outbounds: JSON.stringify([
        { type: "selector", tag, default: first, outbounds: [first, target] },
        { type: "urltest", tag: "自动选择", outbounds: [first, target] },
        { type: "direct", tag: first }, { type: "direct", tag: target }
      ]),
      experimental: JSON.stringify({ clash_api: { external_controller: controllerAddress, secret: "isolated-controller-secret" } })
    }, editorState: {}, bindings: [] };
    const saved = await page.request.put(`/api/v1/profiles/${profileId}/draft`, { headers: { ...headers, "If-Match": draftResponse.headers().etag! }, data: input });
    expect(saved.ok()).toBe(true);
    const revision = (await saved.json() as { revision: number }).revision;
    const applied = await page.request.post(`/api/v1/profiles/${profileId}/apply`, { headers, data: { revision } });
    expect(applied.ok(), await applied.text()).toBe(true);
    await page.goto("/");
    const stoppedDelay = page.getByRole("button", { name: `测试 ${target} 延迟`, exact: true }).first();
    await expect(page.locator(".proxy-test-status")).toHaveText("启动核心后可测速");
    await expect(stoppedDelay).toBeDisabled();
    await expect(stoppedDelay).toHaveAttribute("aria-busy", "false");
    await expect(stoppedDelay).toHaveAttribute("title", "启动核心后可测速");
    await stoppedDelay.hover();
    await expect(stoppedDelay).toHaveCSS("cursor", "not-allowed");
    expect((await page.request.post("/api/v1/core", { headers })).ok()).toBe(true);
    await page.goto("/");
    await expect(page.getByText("快速控制", { exact: true })).toHaveCount(0);
    const modeControls = page.getByRole("group", { name: "路由模式", exact: true });
    const globalMode = modeControls.getByRole("button", { name: "全局", exact: true });
    const directMode = modeControls.getByRole("button", { name: "直连", exact: true });
    expect((await page.request.put("/api/v1/runtime/mode", { data: { mode: "global" } })).status()).toBe(403);
    expect(mode).toBe("rule");
    for (const [name, value] of [["全局", "global"], ["直连", "direct"], ["规则", "rule"]]) {
      const button = modeControls.getByRole("button", { name, exact: true });
      await button.click();
      await expect(button).toHaveAttribute("aria-pressed", "true");
      expect(mode).toBe(value);
    }
    await globalMode.click();
    await expect(globalMode).toHaveAttribute("aria-pressed", "true");
    await page.reload();
    await expect(globalMode).toHaveAttribute("aria-pressed", "true");
    failModeWrite = true;
    await directMode.click();
    await expect(page.locator(".mode-error")).toContainText("核心未确认模式切换");
    await expect(globalMode).toHaveAttribute("aria-pressed", "true");
    expect(mode).toBe("global");
    failModeWrite = false;
    await page.reload();
    const main = page.locator(".proxy-group").filter({ has: page.locator(".proxy-group-copy strong", { hasText: "节点/选择 #%" }) });
    const firstButton = main.getByRole("button", { name: `选择 ${first}`, exact: true });
    const targetButton = main.getByRole("button", { name: `选择 ${target}`, exact: true });
    await expect(firstButton).toHaveAttribute("aria-pressed", "true");
    expect((await page.request.put("/api/v1/runtime/proxy-groups", { data: { tag, selected: target } })).status()).toBe(403);
    expect(writes).toBe(0);
    const sent = page.waitForResponse((response) => response.request().method() === "PUT" && response.url().endsWith("/api/v1/runtime/proxy-groups"));
    await targetButton.click();
    const changed = await sent;
    expect(changed.status()).toBe(200);
    expect(changed.request().headers()["x-csrf-token"]).toBeTruthy();
    expect(changed.request().postDataJSON()).toEqual({ tag, selected: target });
    await expect(targetButton).toHaveAttribute("aria-pressed", "true");
    expect(selected).toBe(target);
    expect(writes).toBe(1);
    await page.reload();
    await expect(targetButton).toHaveAttribute("aria-pressed", "true");
    await expect(firstButton).toHaveAttribute("aria-pressed", "false");
    const draftAfterSwitch = await page.request.get(`/api/v1/profiles/${profileId}/draft`);
    expect(draftAfterSwitch.headers().etag).toBe(saved.headers().etag);

    const runtimeGroups = await (await page.request.get("/api/v1/runtime/proxy-groups")).json() as Array<{ runtimeKey: string }>;
    expect((await page.request.post("/api/v1/runtime/proxies/delay", { data: { tag: target, runtimeKey: runtimeGroups[0]!.runtimeKey } })).status()).toBe(403);
    expect(delayCalls).toHaveLength(0);
    const targetDelay = main.getByRole("button", { name: `测试 ${target} 延迟`, exact: true });
    await expect(targetDelay).toBeEnabled();
    await targetDelay.hover();
    await expect(targetDelay).toHaveCSS("cursor", "pointer");
    delayGate = new Promise<void>((resolve) => { releaseDelay = resolve; });
    await targetDelay.click();
    await expect(targetDelay).toHaveText("测速中");
    await expect(targetDelay).toBeDisabled();
    await expect(targetDelay).toHaveCSS("cursor", "wait");
    await expect(main.getByRole("button", { name: `测试 ${first} 延迟`, exact: true })).toHaveCSS("cursor", "not-allowed");
    releaseDelay!();
    delayGate = undefined;
    await expect(targetDelay).toHaveText("217 ms");
    await expect(targetDelay).toBeEnabled();
    await expect(targetDelay).toHaveCSS("cursor", "pointer");
    await expect(page.locator('[data-proxy-group-tag="自动选择"]').getByRole("button", { name: `测试 ${target} 延迟`, exact: true })).toHaveText("217 ms");
    expect(delayCalls).toEqual([target]);
    expect(writes).toBe(1);
    await targetDelay.focus();
    await page.keyboard.press("Space");
    await expect.poll(() => delayCalls.length).toBe(2);
    await expect(targetDelay).toHaveText("217 ms");
    await expect(targetButton).toHaveAttribute("aria-pressed", "true");
    await page.reload();
    await expect(targetDelay).toHaveText("217 ms");
    timeoutDelay = true;
    delayCalls.length = 0;
    await page.getByRole("button", { name: "全部测速", exact: true }).click();
    await expect(targetDelay).toHaveText("超时");
    await expect(main.getByRole("button", { name: `测试 ${first} 延迟`, exact: true })).toHaveText("136 ms");
    expect(delayCalls.sort()).toEqual([first, target].sort());
    expect((await page.request.get(`/api/v1/profiles/${profileId}/draft`)).headers().etag).toBe(saved.headers().etag);
    timeoutDelay = false;

    // An unapplied draft can no longer replace the running group's members.
    const changedDraft = await page.request.put(`/api/v1/profiles/${profileId}/draft`, {
      headers: { ...headers, "If-Match": draftAfterSwitch.headers().etag! },
      data: { ...input, buffers: { ...input.buffers, outbounds: JSON.stringify([{ type: "direct", tag: "草稿节点" }]), route: "{}" } }
    });
    expect(changedDraft.ok()).toBe(true);
    await page.reload();
    await expect(targetButton).toHaveAttribute("aria-pressed", "true");
    await expect(page.getByRole("button", { name: "选择 草稿节点" })).toHaveCount(0);
    const automatic = page.locator('[data-proxy-group-tag="自动选择"]');
    await expect(automatic.getByRole("button", { name: `选择 ${first}`, exact: true })).toBeDisabled();
    await expect(automatic.getByRole("button", { name: `选择 ${target}`, exact: true })).toHaveAttribute("aria-pressed", "true");

    failWrite = true;
    await firstButton.click();
    await expect(page.getByRole("alert")).toContainText("核心未确认节点切换");
    await expect(targetButton).toHaveAttribute("aria-pressed", "true");
    expect(selected).toBe(target);
    failWrite = false;
    await page.reload();
    await expect(targetButton).toHaveAttribute("aria-pressed", "true");
    for (const viewport of [{ name: "desktop", width: 1440, height: 900 }, { name: "tablet", width: 900, height: 1024 }, { name: "mobile", width: 390, height: 844 }]) {
      await page.setViewportSize(viewport);
      for (const theme of ["light", "dark"]) {
        await page.evaluate((theme) => { document.documentElement.dataset.theme = theme; }, theme);
        await targetButton.focus();
        await expect(targetButton).toBeFocused();
        await page.keyboard.press("Tab");
        await expect(main.getByRole("button", { name: `测试 ${target} 延迟`, exact: true })).toBeFocused();
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
        await page.locator(".proxy-selector-card").screenshot({ path: testInfo.outputPath(`node-selection-${viewport.name}-${theme}.png`), animations: "disabled" });
      }
    }
    expect(browserErrors).toEqual([]);
  } finally {
    releaseDelay?.();
    if (profileId) {
      await page.request.delete("/api/v1/core", { headers });
      await page.request.delete(`/api/v1/profiles/${profileId}`, { headers, data: { confirmName: "E2E 节点切换", stopCurrent: true } });
    }
    await new Promise<void>((resolve, reject) => controller.close((error) => error ? reject(error) : resolve()));
  }
});
