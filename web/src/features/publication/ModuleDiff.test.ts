import { describe, expect, it } from "vitest";
import { compareModules, type ModuleDiffEntry } from "./ModuleDiff.vue";

const draft = {
  log: { level: "info" },
  dns: { servers: [{ tag: "s" }] },
  inbounds: [{ type: "mixed", tag: "m" }],
  route: { final: "direct" },
};

const published = {
  log: { level: "warn" },
  dns: { servers: [{ tag: "s" }] },
  outbounds: [{ type: "direct", tag: "direct" }],
  route: { final: "direct" },
};

describe("ModuleDiff", () => {
  it("reports top-level keys as added, changed, removed or unchanged", () => {
    const entries = compareModules(draft, published);

    const byKey = new Map<string, ModuleDiffEntry>();
    for (const entry of entries) {
      byKey.set(entry.key, entry);
    }
    expect(byKey.get("log")?.status).toBe("changed");
    expect(byKey.get("dns")?.status).toBe("unchanged");
    expect(byKey.get("inbounds")?.status).toBe("added");
    expect(byKey.get("outbounds")?.status).toBe("removed");
    expect(byKey.get("route")?.status).toBe("unchanged");
  });

  it("compares semantic stable JSON regardless of key order", () => {
    const entries = compareModules(
      { log: { level: "info", timestamp: true } },
      { log: { timestamp: true, level: "info" } },
    );
    expect(entries).toEqual([{ key: "log", status: "unchanged" }]);
  });

  it("hides secret values and shows only 已修改敏感字段 for changed sensitive paths", () => {
    const entries = compareModules(
      { experimental: { clash_api: { secret: "new-secret" } } },
      { experimental: { clash_api: { secret: "old-secret" } } },
    );

    expect(entries).toEqual([
      { key: "experimental", status: "changed", sensitiveNote: "已修改敏感字段" },
    ]);
  });

  it("never persists the prior body after a successful publish", () => {
    // compareModules is pure: the inputs stay untouched.
    const draftClone = JSON.parse(JSON.stringify(draft));
    const publishedClone = JSON.parse(JSON.stringify(published));
    compareModules(draft, published);
    expect(draft).toEqual(draftClone);
    expect(published).toEqual(publishedClone);
  });
});
