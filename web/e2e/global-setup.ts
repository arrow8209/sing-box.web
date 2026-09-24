import { execFileSync } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

export default function globalSetup(): () => void {
  const directory = mkdtempSync(join(tmpdir(), "singbox-web-e2e-build-"));
  const binary = join(directory, "harness");
  try {
    execFileSync("go", ["build", "-o", binary, "./e2e/harness"], {
      cwd: fileURLToPath(new URL("../../", import.meta.url)), stdio: "inherit"
    });
    process.env.SINGBOX_WEB_E2E_HARNESS = binary;
  } catch (error) {
    rmSync(directory, { recursive: true, force: true });
    throw error;
  }
  return () => rmSync(directory, { recursive: true, force: true });
}
