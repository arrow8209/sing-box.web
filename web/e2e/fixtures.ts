import { spawn } from "node:child_process";
import { createServer } from "node:net";
import { setTimeout as delay } from "node:timers/promises";
import { test as base } from "@playwright/test";

export { expect, type Page } from "@playwright/test";

// A new harness owns a new data directory and fake core for every test,
// including retries. A failed test cannot change the next test's fixtures.
export const test = base.extend<{ isolatedHarness: void }>({
  isolatedHarness: [async ({ baseURL }, use, testInfo) => {
    const binary = process.env.SINGBOX_WEB_E2E_HARNESS;
    if (!binary || !baseURL) throw new Error("E2E harness configuration is missing");
    const address = new URL(baseURL);
    const probe = createServer();
    await new Promise<void>((resolve, reject) => {
      probe.once("error", reject);
      probe.listen(Number(address.port), address.hostname, resolve);
    });
    await new Promise<void>((resolve, reject) => probe.close((error) => error ? reject(error) : resolve()));
    const child = spawn(binary, [], {
      env: { ...process.env, SINGBOX_WEB_E2E_PORT: address.port },
      stdio: ["ignore", "pipe", "pipe"]
    });
    let output = "";
    child.stdout.on("data", (chunk) => { output += chunk.toString(); });
    child.stderr.on("data", (chunk) => { output += chunk.toString(); });
    let failure: Error | undefined;
    child.on("error", (error) => { failure = error; });
    const exited = new Promise<void>((resolve) => child.once("close", () => resolve()));
    try {
      const deadline = Date.now() + 20_000;
      while (true) {
        if (failure) throw failure;
        if (child.exitCode !== null || child.signalCode !== null) throw new Error(`E2E harness exited: ${output}`);
        try {
          const response = await fetch(`${baseURL}/api/v1/auth/session`, { signal: AbortSignal.timeout(500) });
          if (response.ok && (await response.json() as { valid: boolean }).valid === false) break;
        } catch { /* The new listener may not be ready yet. */ }
        if (Date.now() >= deadline) throw new Error(`E2E harness did not start: ${output}`);
        await delay(50);
      }
      await use();
    } finally {
      child.kill("SIGTERM");
      const stopped = await Promise.race([exited.then(() => true), delay(10_000, false, { ref: false })]);
      if (!stopped) {
        child.kill("SIGKILL");
        await exited;
      }
      if (testInfo.status !== testInfo.expectedStatus || !stopped) {
        await testInfo.attach("harness.log", { body: output, contentType: "text/plain" });
      }
      if (!stopped) throw new Error("E2E harness did not shut down cleanly");
    }
  }, { auto: true }]
});
