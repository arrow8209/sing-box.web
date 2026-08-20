import { readFile } from "node:fs/promises";
import { expect, test, type Page } from "@playwright/test";

const fixturePassword = "phase1-e2e-password";
const temporaryPassword = "phase1-e2e-password-2";

async function login(page: Page, password = fixturePassword): Promise<void> {
  await page.goto("/login");
  await expect(page.getByLabel("用户名")).toHaveValue("admin");
  await page.getByLabel("密码").fill(password);
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);
}

async function waitForDraft(page: Page): Promise<void> {
  await expect(page.getByText("草稿已保存", { exact: true }).last()).toBeVisible();
}

test("login, draft conflict, validation, publication and secret-safe export", async ({ page, context }) => {
  test.setTimeout(120_000);
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await login(page);

  const cookies = await context.cookies();
  const session = cookies.find((cookie) => cookie.name === "sb_session");
  expect(session).toMatchObject({ secure: true, httpOnly: true, sameSite: "Strict" });

  const health = await page.request.get("/healthz");
  expect(health.headers()["strict-transport-security"]).toContain("max-age=31536000");
  expect(health.headers()["content-security-policy"]).toBeTruthy();

  await page.getByRole("link", { name: "配置" }).click();
  await page.getByRole("button", { name: "新建档案" }).click();
  const createDialog = page.getByRole("dialog", { name: "创建配置档案" });
  await createDialog.getByLabel("显示名称").fill("主配置");
  await createDialog.getByLabel("slug").fill("primary");
  await createDialog.getByRole("button", { name: "创建" }).click();
  await expect(page).toHaveURL(/\/config\/[a-f0-9]{32}\/general$/);

  await page.getByRole("tab", { name: "高级" }).click();
  await expect(page.getByLabel("Clash API Secret")).toHaveAttribute("type", "password");
  await page.getByRole("tab", { name: "概览" }).click();
  await expect(page.getByLabel("日志级别")).toHaveValue("info");

  const secondPage = await context.newPage();
  await secondPage.goto(page.url());
  await expect(secondPage.getByLabel("日志级别")).toHaveValue("info");
  await page.getByLabel("日志级别").selectOption("debug");
  await waitForDraft(page);
  await secondPage.getByLabel("日志级别").selectOption("warn");
  const conflict = secondPage.getByRole("dialog", { name: "草稿冲突" });
  await expect(conflict).toBeVisible();
  await conflict.getByRole("button", { name: "重新载入服务器版本" }).click();
  await expect(secondPage.getByLabel("日志级别")).toHaveValue("debug");
  await secondPage.close();

  await page.reload();
  await expect(page.getByLabel("日志级别")).toHaveValue("debug");
  await page.getByRole("button", { name: "编辑配置概览 JSON" }).click();
  const rawGeneral = page.getByRole("textbox", { name: "配置概览 JSON" });
  await rawGeneral.fill('{"log":');
  await waitForDraft(page);
  await page.reload();
  await page.getByRole("button", { name: "编辑配置概览 JSON" }).click();
  await expect(page.getByRole("textbox", { name: "配置概览 JSON" })).toHaveValue('{"log":');
  await page.getByRole("textbox", { name: "配置概览 JSON" }).fill('{"log":{"level":"invalid"}}');
  await waitForDraft(page);
  await page.getByRole("button", { name: "仅校验" }).click();
  const issue = page.getByRole("link", { name: "转到字段 /log/level" });
  await expect(issue).toBeVisible();
  await issue.click();
  await expect(page.getByLabel("日志级别")).toBeFocused();
  await page.getByLabel("日志级别").selectOption("info");
  await waitForDraft(page);
  await page.getByRole("button", { name: "仅校验" }).click();
  await expect(page.getByText("校验通过", { exact: true })).toBeVisible();

  await page.getByRole("button", { name: "校验并写入" }).click();
  await expect(page.getByText("正式配置已写入", { exact: true })).toBeVisible();

  let fullExportRequests = 0;
  page.on("request", (request) => {
    if (request.url().includes("/export?mode=full")) fullExportRequests += 1;
  });
  page.once("dialog", (dialog) => dialog.dismiss());
  await page.getByRole("button", { name: "下载完整配置" }).click();
  await page.waitForTimeout(700);
  expect(fullExportRequests).toBe(0);

  page.once("dialog", (dialog) => dialog.accept());
  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.getByRole("button", { name: "下载完整配置" }).click(),
  ]);
  expect(download.suggestedFilename()).toBe("primary-config.json");
  const fullConfigPath = await download.path();
  if (!fullConfigPath) throw new Error("download has no local path");
  const fullConfig = JSON.parse(await readFile(fullConfigPath, "utf8"));
  const knownSecret = fullConfig.experimental.clash_api.secret as string;
  expect(knownSecret).toHaveLength(64);

  await page.getByRole("button", { name: "复制脱敏配置" }).click();
  await expect(page.getByText("脱敏配置已复制", { exact: true })).toBeVisible();
  const redacted = await page.evaluate(() => navigator.clipboard.readText());
  expect(redacted).toContain("***REDACTED***");
  expect(redacted).not.toContain(knownSecret);

  await page.getByRole("link", { name: "设置" }).click();
  await page.getByLabel("新密码", { exact: true }).fill(temporaryPassword);
  await page.getByLabel("确认新密码").fill(temporaryPassword);
  await page.getByRole("button", { name: "修改管理员密码" }).click();
  await expect(page).toHaveURL(/\/login$/);
  await page.getByLabel("密码").fill(fixturePassword);
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page.getByRole("alert")).toHaveText("用户名或密码错误");
  await expect(page).toHaveURL(/\/login$/);
  await page.getByLabel("密码").fill(temporaryPassword);
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);
  await page.getByRole("link", { name: "设置" }).click();
  await page.getByLabel("新密码", { exact: true }).fill(fixturePassword);
  await page.getByLabel("确认新密码").fill(fixturePassword);
  await page.getByRole("button", { name: "修改管理员密码" }).click();
  await expect(page).toHaveURL(/\/login$/);
  await login(page);
  await page.getByRole("button", { name: "退出登录" }).click();
  await expect(page).toHaveURL(/\/login$/);
  expect((await context.cookies()).some((cookie) => cookie.name === "sb_session")).toBe(false);
});
