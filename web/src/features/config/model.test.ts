import { describe, expect, it } from "vitest";
import { deleteAtPath, getAtPath, setAtPath } from "./model";

describe("setAtPath", () => {
  it("changes one known field without dropping unknown fields", () => {
    const source = {
      log: { level: "info", future_option: { enabled: true } },
      future_top_level: { value: 7 },
    };
    const result = setAtPath(source, ["log", "level"], "debug");
    expect(result).toEqual({
      log: { level: "debug", future_option: { enabled: true } },
      future_top_level: { value: 7 },
    });
    expect(result).not.toBe(source);
  });

  it("replaces an array index", () => {
    const source = { dns: { servers: [{ tag: "a" }, { tag: "b" }] } };
    const result = setAtPath(source, ["dns", "servers", "1"], { tag: "c" });
    expect(result).toEqual({ dns: { servers: [{ tag: "a" }, { tag: "c" }] } });
  });

  it("deletes an optional field when set to undefined", () => {
    const source = { log: { level: "info", output: "console" } };
    const result = setAtPath(source, ["log", "output"], undefined);
    expect(result).toEqual({ log: { level: "info" } });
  });

  it("creates a missing object container", () => {
    const source = {};
    const result = setAtPath(source, ["a", "b"], 1);
    expect(result).toEqual({ a: { b: 1 } });
  });

  it("rejects __proto__, prototype and constructor path segments", () => {
    const source = {};
    for (const segment of ["__proto__", "prototype", "constructor"]) {
      expect(() => setAtPath(source, [segment], "x")).toThrow();
      expect(() => setAtPath(source, ["a", segment], "x")).toThrow();
    }
  });
});

describe("getAtPath", () => {
  it("reads nested values and returns undefined for missing paths", () => {
    const source = { log: { level: "info" } };
    expect(getAtPath(source, ["log", "level"])).toBe("info");
    expect(getAtPath(source, ["log", "missing"])).toBeUndefined();
    expect(getAtPath(source, ["missing"])).toBeUndefined();
  });
});

describe("deleteAtPath", () => {
  it("removes a field without mutating the source", () => {
    const source = { log: { level: "info", output: "console" } };
    const result = deleteAtPath(source, ["log", "output"]);
    expect(result).toEqual({ log: { level: "info" } });
    expect(source.log.output).toBe("console");
  });

  it("removes an array element at an index", () => {
    const source = { dns: { servers: [{ tag: "a" }, { tag: "b" }] } };
    const result = deleteAtPath(source, ["dns", "servers", "0"]);
    expect(result).toEqual({ dns: { servers: [{ tag: "b" }] } });
  });
});
