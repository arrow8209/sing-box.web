import { describe, expect, it } from "vitest";
import { collectReferences } from "./references";

describe("collectReferences", () => {
  it("collects existing tag sets without mutating config", () => {
    const config = {
      outbounds: [
        { type: "direct", tag: "direct" },
        { type: "selector", tag: "sel", outbounds: ["direct", "missing-1"] },
      ],
      route: {
        rules: [{ action: "route", outbound: "sel" }],
        final: "missing-2",
      },
    };
    const snapshot = JSON.stringify(config);

    const result = collectReferences(config);

    expect(JSON.stringify(config)).toBe(snapshot);
    expect(result.tags).toEqual(new Set(["direct", "sel"]));
    expect(result.missing).toEqual(
      expect.arrayContaining([
        { kind: "outbound", tag: "missing-1" },
        { kind: "route-final", tag: "missing-2" },
      ]),
    );
  });

  it("flags duplicate tags via client-side preview", () => {
    const config = {
      outbounds: [
        { type: "direct", tag: "dup" },
        { type: "block", tag: "dup" },
      ],
    };

    const result = collectReferences(config);

    expect(result.duplicates).toContain("dup");
  });

  it("returns empty sets for a minimal config", () => {
    const result = collectReferences({});
    expect(result.tags).toEqual(new Set());
    expect(result.missing).toEqual([]);
    expect(result.duplicates).toEqual([]);
  });
});
