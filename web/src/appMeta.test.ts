import { describe, expect, it } from "vitest";
import { appTitle } from "./appMeta";

describe("appTitle", () => {
  it("returns the production console title", () => {
    expect(appTitle()).toBe("sing-box Web 管理");
  });
});
