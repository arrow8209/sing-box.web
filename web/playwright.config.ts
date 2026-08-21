import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  workers: 1,
  fullyParallel: false,
  reporter: "list",
  use: { baseURL: "http://127.0.0.1:4173", trace: "retain-on-failure" },
  webServer: {
    cwd: "..",
    command: "go run ./e2e/harness",
    env: {
      NO_PROXY: "127.0.0.1,localhost",
      no_proxy: "127.0.0.1,localhost"
    },
    url: "http://127.0.0.1:4173",
    reuseExistingServer: false,
    timeout: 120000
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }]
});
