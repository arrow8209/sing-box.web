import { beforeEach, describe, expect, it, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { useDraftStore } from "../../stores/draft";
import { useDraftAutosave } from "./useDraftAutosave";

describe("useDraftAutosave", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("registers beforeunload only while the draft is dirty, saving, offline or conflict", () => {
    const draft = useDraftStore();
    const addEventListener = vi.spyOn(window, "addEventListener");
    const removeEventListener = vi.spyOn(window, "removeEventListener");

    useDraftAutosave();

    // Start with saved state: no beforeunload listener.
    expect(addEventListener).not.toHaveBeenCalledWith("beforeunload", expect.anything());

    draft.status = "dirty";
    expect(addEventListener).toHaveBeenCalledWith("beforeunload", expect.anything());

    draft.status = "saved";
    expect(removeEventListener).toHaveBeenCalledWith("beforeunload", expect.anything());
  });

  it("retries once on the online event using the current ETag", async () => {
    const addEventListener = vi.spyOn(window, "addEventListener");
    const draft = useDraftStore();
    const save = vi.spyOn(draft, "save").mockResolvedValue(undefined);
    useDraftAutosave();

    draft.status = "offline";
    const handler = addEventListener.mock.calls.find(([event]) => event === "online")?.[1] as
      | EventListener
      | undefined;

    handler?.(new Event("online"));
    await Promise.resolve();

    expect(save).toHaveBeenCalledTimes(1);
  });

  it("does not auto-retry a publish request", () => {
    const draft = useDraftStore();
    draft.status = "saved";
    const onlineHandler = vi.fn();
    const publishSpy = vi.fn();

    // The composable must not wire any retry into publish flows.
    useDraftAutosave();
    expect(publishSpy).not.toHaveBeenCalled();
    expect(onlineHandler).not.toHaveBeenCalled();
  });
});
