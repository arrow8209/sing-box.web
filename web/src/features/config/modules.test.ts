import { describe, expect, it } from "vitest";
import { jsonPointerToModule } from "./modules";

describe("jsonPointerToModule", () => {
  it("does not URL-decode JSON Pointer segments", () => {
    expect(jsonPointerToModule("/%6Cog/level")).toBeNull();
  });
});
