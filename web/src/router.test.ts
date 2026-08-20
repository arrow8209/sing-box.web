import { beforeEach, describe, expect, it, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { apiClient } from "./api/client";
import { router } from "./router";
import { useAuthStore } from "./stores/auth";
import { useCapabilitiesStore } from "./stores/capabilities";
import { useDraftStore } from "./stores/draft";
import { useProfilesStore } from "./stores/profiles";
import type { Draft, Profile } from "./api/types";

const profileA: Profile = {
  id: "a".repeat(32),
  name: "家庭网关",
  slug: "home-gateway",
  draftRevision: 3,
  publishedRevision: null,
  publishedDigest: "",
  publishedAt: null,
  lastValidation: null,
  directoryHint: "/etc/sing-box/home-gateway",
  checkCommand: "sing-box check -C /etc/sing-box/home-gateway",
  draftReadOnly: false,
  draftError: "",
};

const profileB: Profile = {
  ...profileA,
  id: "b".repeat(32),
  name: "办公室",
  slug: "office",
  directoryHint: "/etc/sing-box/office",
  checkCommand: "sing-box check -C /etc/sing-box/office",
};

describe("config profile routing", () => {
  beforeEach(async () => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
    const auth = useAuthStore();
    auth.session = {
      username: "admin",
      csrfToken: "csrf",
      expiresAt: "2026-08-21T00:00:00Z",
    };
    vi.spyOn(auth, "restore").mockResolvedValue(undefined);
    const capabilities = useCapabilitiesStore();
    capabilities.snapshot = {
      capabilities: {
        "config.manage": true,
        "config.publish": true,
        "subscription.sync": false,
        "ruleset.sync": false,
        "core.lifecycle": false,
        "core.metrics": false,
      },
      singBox: { available: true, version: "1.13.19", targetVersion: "1.13.19", warning: "" },
    };
    await router.replace("/dashboard");
  });

  it("flushes and selects again when only the config profile route parameter changes", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profileA, profileB];
    profiles.activeId = profileA.id;
    const draft = useDraftStore();
    draft.draft = {
      profileId: profileA.id,
      revision: 3,
      content: { config: { log: { level: "info" } }, editorBuffers: {} },
      savedAt: "2026-08-20T00:00:00Z",
    };
    draft.etag = '"draft-3"';
    draft.status = "saved";
    await router.push(`/config/${profileA.id}/general`);

    draft.replaceConfig({ log: { level: "warn" } });
    const savedA: Draft = { ...draft.draft, revision: 4 };
    const draftB: Draft = {
      profileId: profileB.id,
      revision: 1,
      content: { config: { log: { level: "debug" } }, editorBuffers: {} },
      savedAt: "2026-08-20T00:01:00Z",
    };
    const request = vi
      .spyOn(apiClient, "request")
      .mockResolvedValueOnce({ data: savedA, etag: '"draft-4"' })
      .mockResolvedValueOnce({ data: draftB, etag: '"draft-1"' });

    await router.push(`/config/${profileB.id}/general`);

    expect(request).toHaveBeenCalledTimes(2);
    expect(profiles.activeId).toBe(profileB.id);
    expect(draft.draft).toEqual(draftB);
    expect(router.currentRoute.value.params.profileId).toBe(profileB.id);
  });
});
