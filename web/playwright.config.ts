import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "@playwright/test";

const webDir = path.dirname(fileURLToPath(import.meta.url));
const repoDir = path.resolve(webDir, "..");

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  workers: 1,
  timeout: 90_000,
  expect: { timeout: 10_000 },
  reporter: "line",
  use: {
    baseURL: "https://127.0.0.1:18443",
    ignoreHTTPSErrors: true,
    trace: "retain-on-failure",
  },
  webServer: {
    command: "./scripts/e2e-server.sh",
    cwd: repoDir,
    url: "https://127.0.0.1:18443/healthz",
    ignoreHTTPSErrors: true,
    env: {
      http_proxy: "",
      https_proxy: "",
      HTTP_PROXY: "",
      HTTPS_PROXY: "",
      all_proxy: "",
      ALL_PROXY: "",
      no_proxy: "127.0.0.1,localhost,::1",
      NO_PROXY: "127.0.0.1,localhost,::1",
    },
    reuseExistingServer: false,
    timeout: 60_000,
  },
});
