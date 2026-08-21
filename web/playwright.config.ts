import { defineConfig, devices } from "@playwright/test";

const e2ePort = process.env.SINGBOX_WEB_E2E_PORT ?? "4173";
const e2eURL = `http://127.0.0.1:${e2ePort}`;

export default defineConfig({
  testDir: "./e2e",
  workers: 1,
  fullyParallel: false,
  reporter: "list",
  use: { baseURL: e2eURL, trace: "retain-on-failure" },
  webServer: {
    cwd: "..",
    command: "go build -o /tmp/singbox-web-e2e-harness ./e2e/harness && exec /tmp/singbox-web-e2e-harness",
    env: {
      NO_PROXY: "127.0.0.1,localhost",
      no_proxy: "127.0.0.1,localhost",
      SINGBOX_WEB_E2E_PORT: e2ePort
    },
    url: e2eURL,
    reuseExistingServer: false,
    timeout: 120000
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }]
});
