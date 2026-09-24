import { expect, test } from "./fixtures";

test("核心设置保存反馈不会挤动表单和后续卡片", async ({ page }) => {
  test.setTimeout(60000);
  let behavior = { autoStart: true, autoReload: false };
  let finishSave: (() => void) | undefined;
  let failSave = false;
  await page.route("**/api/v1/settings/core", async (route) => {
    if (route.request().method() === "GET") {
      await route.fulfill({ json: behavior });
      return;
    }
    await new Promise<void>((resolve) => { finishSave = resolve; });
    finishSave = undefined;
    if (failSave) {
      await route.fulfill({ status: 500, json: { error: { code: "save_failed", message: "模拟保存失败" } } });
    } else {
      behavior = route.request().postDataJSON() as typeof behavior;
      await route.fulfill({ json: behavior });
    }
  });
  await page.goto("/");
  await page.getByLabel("管理密码").fill("correct horse battery staple");
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page.getByRole("heading", { name: "仪表盘" })).toBeVisible();
  await page.clock.install();

  const layout = () => page.evaluate(() => [...document.querySelectorAll(
    ".core-settings-card, .core-settings-card .setting-row, .core-settings-card .setting-control, #proxy, #security"
  )].map((element) => {
    const box = element.getBoundingClientRect();
    return { x: box.x + scrollX, y: box.y + scrollY, width: box.width, height: box.height };
  }));

  try {
    for (const width of [1440, 900, 390, 320]) {
      await page.setViewportSize({ width, height: 900 });
      for (const theme of ["light", "dark"]) {
        await page.goto("/settings");
        const card = page.getByRole("region", { name: "核心设置" });
        const autoStart = card.getByRole("switch", { name: "开机自动运行" });
        await expect(autoStart).toBeEnabled();
        await page.evaluate((value) => { document.documentElement.dataset.theme = value; }, theme);
        const before = await layout();

        for (const name of ["开机自动运行", "配置变更自动重载"]) {
          const control = card.getByRole("switch", { name });
          await control.click();
          await expect.poll(() => Boolean(finishSave)).toBe(true);
          expect(await layout(), `${width}/${theme}: 保存中`).toEqual(before);
          finishSave!();
          await expect(control).toBeEnabled();
          expect(await layout(), `${width}/${theme}: 保存完成`).toEqual(before);
          await expect(page.locator(".app-toast")).toBeVisible();
          await page.clock.fastForward(3000);
          await expect(page.locator(".app-toast")).toHaveCount(0);
          expect(await layout(), `${width}/${theme}: 提示消失`).toEqual(before);
        }
      }
    }

    failSave = true;
    const control = page.getByRole("switch", { name: "开机自动运行" });
    const original = await control.getAttribute("aria-checked");
    const before = await layout();
    await control.click();
    await expect.poll(() => Boolean(finishSave)).toBe(true);
    finishSave!();
    await expect(page.getByRole("alert")).toHaveText("模拟保存失败");
    await expect(control).toHaveAttribute("aria-checked", original!);
    expect(await layout(), "保存失败回滚").toEqual(before);
  } finally {
    finishSave?.();
  }
});

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

  const autoStart = coreSettings.getByRole("switch", { name: "开机自动运行" });
  const autoReload = coreSettings.getByRole("switch", { name: "配置变更自动重载" });
  await expect(autoStart).toBeChecked();
  await expect(autoReload).not.toBeChecked();
  await autoStart.click();
  await autoReload.click();
  await expect(autoStart).not.toBeChecked();
  await expect(autoReload).toBeChecked();
  await page.reload();
  await expect(autoStart).not.toBeChecked();
  await expect(autoReload).toBeChecked();
  await autoStart.click();
  await autoReload.click();
  await expect(autoStart).toBeChecked();
  await expect(autoReload).not.toBeChecked();

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

  await proxyUrl.focus();
  await expect(proxyUrl).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(page.getByRole("button", { name: "保存代理设置" })).toBeFocused();

  for (const viewport of [{ width: 900, height: 1024 }, { width: 390, height: 844 }]) {
    await page.setViewportSize(viewport);
    await page.evaluate(() => { document.documentElement.dataset.theme = "dark"; });
    await expect(page.getByRole("heading", { name: "代理设置" })).toBeVisible();
    expect(await page.locator("body").evaluate((body) => body.scrollWidth <= body.clientWidth)).toBe(true);
  }
});
