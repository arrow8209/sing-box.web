import { beforeEach, describe, expect, it, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { apiClient } from "../api/client";
import { useAuthStore } from "./auth";
import { useCapabilitiesStore } from "./capabilities";
import type { CapabilitySnapshot } from "../api/types";

const snapshot: CapabilitySnapshot = {
  capabilities: {
    "config.manage": true,
    "config.publish": true,
    "subscription.sync": false,
    "ruleset.sync": false,
    "core.lifecycle": false,
    "core.metrics": false,
  },
  singBox: {
    available: true,
    version: "1.12.0",
    targetVersion: "1.12.0",
    warning: "",
  },
};

describe("useCapabilitiesStore", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it("load fetches /capabilities only after authentication", async () => {
    const request = vi.spyOn(apiClient, "request").mockResolvedValue({ data: snapshot, etag: null });
    const auth = useAuthStore();
    const capabilities = useCapabilitiesStore();

    // Not authenticated yet: load must not reach the network.
    auth.session = null;
    await capabilities.load();
    expect(request).not.toHaveBeenCalled();

    auth.session = {
      username: "admin",
      csrfToken: "csrf-1",
      expiresAt: "2030-01-01T00:00:00Z",
    };
    await capabilities.load();

    expect(request).toHaveBeenCalledTimes(1);
    expect(request.mock.calls[0][0]).toBe("/capabilities");
    expect(capabilities.snapshot).toEqual(snapshot);
  });

  it("exposes feature flags for phase-one unavailable features", async () => {
    vi.spyOn(apiClient, "request").mockResolvedValue({ data: snapshot, etag: null });
    const auth = useAuthStore();
    auth.session = {
      username: "admin",
      csrfToken: "csrf-1",
      expiresAt: "2030-01-01T00:00:00Z",
    };
    const capabilities = useCapabilitiesStore();

    await capabilities.load();

    expect(capabilities.can("config.manage")).toBe(true);
    expect(capabilities.can("config.publish")).toBe(true);
    expect(capabilities.can("subscription.sync")).toBe(false);
    expect(capabilities.snapshot?.singBox.available).toBe(true);
  });
});
