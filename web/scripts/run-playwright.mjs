import { spawn } from "node:child_process";

// Keep visual tests on the local harness even when a developer uses a global
// HTTP proxy. This is implemented in Node rather than shell syntax so it works
// the same on Windows, macOS, and Linux.
const environment = { ...process.env };
for (const name of ["HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"]) {
  delete environment[name];
}
environment.NO_PROXY = "127.0.0.1,localhost";
environment.no_proxy = "127.0.0.1,localhost";

const child = spawn(process.platform === "win32" ? "npx.cmd" : "npx", ["playwright", "test", ...process.argv.slice(2)], {
  env: environment,
  stdio: "inherit"
});
child.on("exit", (code, signal) => process.exitCode = code ?? (signal ? 1 : 0));
