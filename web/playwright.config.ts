import { defineConfig, devices } from "@playwright/test";

const e2ePort = process.env.SINGBOX_WEB_E2E_PORT ?? "50015";
const e2eURL = `http://127.0.0.1:${e2ePort}`;

export default defineConfig({
  testDir: "./e2e",
  workers: 1,
  fullyParallel: false,
  reporter: "list",
  globalSetup: "./e2e/global-setup.ts",
  use: { baseURL: e2eURL, trace: "retain-on-failure" },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }]
});
