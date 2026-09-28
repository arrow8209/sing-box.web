import { readdirSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const sourceRoot = resolve(import.meta.dirname, "../src");

function readAllSourceFiles(directory: string): Array<{ path: string; content: string }> {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = resolve(directory, entry.name);
    return entry.isDirectory() ? readAllSourceFiles(path) : [{ path, content: readFileSync(path, "utf8") }];
  });
}

describe("未开放功能守卫", () => {
  it("不包含被禁止的产品路由或调用", () => {
    const sources = readAllSourceFiles(sourceRoot);
    const joined = sources.map((source) => source.content).join("\n");
    for (const value of ["/config/export", "downloadConfig", "copyFullConfig", "mockTraffic", "mockLatency"]) {
      expect(joined).not.toContain(value);
    }
  });

  it("运行功能文案只允许出现在运行能力说明与仪表盘实现中", () => {
    const sources = readAllSourceFiles(sourceRoot);
    for (const source of sources) {
      if (["RuntimeUnavailableView.vue", "DashboardView.vue", "DashboardProxySelector.vue", "use-proxy-delay.ts"].some((name) => source.path.endsWith(name))) continue;
      for (const value of ["导出配置", "复制完整配置", "测速", "切换模式"]) expect(source.content).not.toContain(value);
    }
  });
});
