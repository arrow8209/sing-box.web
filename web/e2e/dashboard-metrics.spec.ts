import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { expect, test } from "./fixtures";

test("仪表盘实时指标经服务端采样，失败和停止时清除旧数据", async ({ page }, testInfo) => {
  test.setTimeout(90000);
  let uploadTotal = 100000;
  let downloadTotal = 200000;
  let memory = 32 * 1024 * 1024;
  let connectionCount = 2;
  let unavailable = false;
  let reads = 0;
  const browserErrors: string[] = [];
  page.on("pageerror", (error) => browserErrors.push(error.message));
  const controller = createServer((request, response) => {
    if (request.headers.authorization !== "Bearer metrics-controller-only") { response.writeHead(401).end(); return; }
    response.setHeader("Content-Type", "application/json");
    if (request.url === "/connections") {
      reads++;
      if (unavailable) { response.writeHead(503).end(); return; }
      uploadTotal += 4096;
      downloadTotal += 8192;
      response.end(JSON.stringify({ memory, uploadTotal, downloadTotal,
        connections: Array.from({ length: connectionCount }, (_, index) => ({ id: `connection-${index}`, metadata: { host: "metrics.example.test" } })) }));
    } else if (request.url === "/configs") response.end(JSON.stringify({ mode: "rule", "mode-list": ["rule", "global", "direct"] }));
    else if (request.url === "/proxies") response.end(JSON.stringify({ proxies: {} }));
    else response.writeHead(404).end();
  });
  await new Promise<void>((resolve) => controller.listen(0, "127.0.0.1", resolve));
  const controllerAddress = `127.0.0.1:${(controller.address() as AddressInfo).port}`;
  let profileId = "";
  let headers: Record<string, string> = {};
  try {
    expect((await page.request.get("/api/v1/runtime/metrics")).status()).toBe(401);
    await page.goto("/");
    await page.getByLabel("管理密码").fill("correct horse battery staple");
    await page.getByRole("button", { name: "登录", exact: true }).click();
    await expect(page.getByRole("heading", { name: "仪表盘", exact: true })).toBeVisible();
    const session = await (await page.request.get("/api/v1/auth/session")).json() as { csrfToken: string };
    headers = { "X-CSRF-Token": session.csrfToken };
    const created = await page.request.post("/api/v1/profiles", { headers, data: { name: "E2E 运行指标" } });
    expect(created.ok()).toBe(true);
    profileId = (await created.json() as { id: string }).id;
    const draftResponse = await page.request.get(`/api/v1/profiles/${profileId}/draft`);
    const draft = await draftResponse.json() as { buffers: Record<string, string> };
    const saved = await page.request.put(`/api/v1/profiles/${profileId}/draft`, { headers: { ...headers, "If-Match": draftResponse.headers().etag! }, data: {
      buffers: { ...draft.buffers, inbounds: "[]", dns: "{}", route: JSON.stringify({ final: "direct" }), outbounds: JSON.stringify([{ type: "direct", tag: "direct" }]),
        experimental: JSON.stringify({ clash_api: { external_controller: controllerAddress, secret: "metrics-controller-only" } }) },
      editorState: {}, bindings: []
    } });
    expect(saved.ok()).toBe(true);
    const revision = (await saved.json() as { revision: number }).revision;
    expect((await page.request.post(`/api/v1/profiles/${profileId}/apply`, { headers, data: { revision } })).ok()).toBe(true);
    expect((await page.request.post("/api/v1/core", { headers })).ok()).toBe(true);
    await page.goto("/");
    const upload = page.locator('[data-metric="upload"] strong');
    const download = page.locator('[data-metric="download"] strong');
    const usedMemory = page.locator('[data-metric="memory"] strong');
    const connections = page.locator('[data-metric="connections"] strong');
    await expect(usedMemory).toHaveText("32.0 MB");
    await expect(connections).toHaveText("2 条");
    await expect(upload).toContainText("KB/s");
    await expect(download).toContainText("KB/s");
    expect(reads).toBeGreaterThanOrEqual(2);
    const metricsResponse = await page.request.get("/api/v1/runtime/metrics");
    const metrics = await metricsResponse.json() as Record<string, unknown>;
    expect(metrics.connectionCount).toBe(2);
    expect(metrics.runtimeKey).toBeTruthy();
    expect(metrics.observedAt).toBeTruthy();
    expect(metrics).not.toHaveProperty("connections");
    expect(JSON.stringify(metrics)).not.toContain("metrics-controller-only");
    expect(JSON.stringify(metrics)).not.toContain("metrics.example.test");

    for (const viewport of [{ name: "desktop", width: 1440, height: 900 }, { name: "tablet", width: 900, height: 1024 }, { name: "mobile", width: 390, height: 844 }]) {
      await page.setViewportSize(viewport);
      for (const theme of ["light", "dark"]) {
        await page.evaluate((theme) => { document.documentElement.dataset.theme = theme; }, theme);
        const link = page.locator('a.hero-metric-link[href="/connections"]');
        await link.focus();
        await expect(link).toBeFocused();
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
        await page.locator(".hero-status").screenshot({ path: testInfo.outputPath(`metrics-${viewport.name}-${theme}.png`), animations: "disabled" });
      }
    }
    memory = 40 * 1024 * 1024;
    connectionCount = 0;
    await page.getByRole("button", { name: "刷新状态", exact: true }).click();
    await expect(usedMemory).toHaveText("40.0 MB");
    await expect(connections).toHaveText("0 条");
    unavailable = true;
    await expect(page.locator(".metrics-status")).toContainText("无法访问 sing-box 运行数据");
    for (const metric of [upload, download, usedMemory, connections]) await expect(metric).toHaveText("--");
    unavailable = false;
    // The one-second poll may recover and remove the inline retry button
    // before Playwright clicks it. The persistent refresh control is stable.
    await page.getByRole("button", { name: "刷新状态", exact: true }).click();
    await expect(usedMemory).toHaveText("40.0 MB");
    await expect(upload).toContainText("KB/s");
    await page.getByRole("button", { name: "停止核心", exact: true }).click();
    await expect(page.getByRole("heading", { name: "sing-box 已停止", exact: true })).toBeVisible();
    await expect(page.locator(".metrics-status")).toContainText("核心未运行");
    for (const metric of [upload, download, usedMemory, connections]) await expect(metric).toHaveText("--");
    const stoppedReads = reads;
    await page.clock.install();
    await page.clock.fastForward(3000);
    expect(reads).toBe(stoppedReads);
    await page.locator('a.hero-metric-link[href="/connections"]').click();
    await expect(page.getByRole("heading", { name: "活动连接", exact: true })).toBeVisible();
    expect(browserErrors).toEqual([]);
  } finally {
    try {
      if (profileId) {
        await page.request.delete("/api/v1/core", { headers });
        await page.request.delete(`/api/v1/profiles/${profileId}`, { headers, data: { confirmName: "E2E 运行指标", stopCurrent: true } });
      }
    } finally {
      controller.closeAllConnections();
      await new Promise<void>((resolve, reject) => controller.close((error) => error ? reject(error) : resolve()));
    }
  }
});
