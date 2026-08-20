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

  it("keeps the URL and active draft on the latest rapid profile navigation", async () => {
    const profileC: Profile = {
      ...profileB,
      id: "c".repeat(32),
      name: "备用",
      slug: "backup",
      directoryHint: "/etc/sing-box/backup",
      checkCommand: "sing-box check -C /etc/sing-box/backup",
    };
    const profiles = useProfilesStore();
    profiles.items = [profileA, profileB, profileC];
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

    const draftB: Draft = {
      profileId: profileB.id,
      revision: 1,
      content: { config: { log: { level: "warn" } }, editorBuffers: {} },
      savedAt: "2026-08-20T00:01:00Z",
    };
    const draftC: Draft = {
      profileId: profileC.id,
      revision: 1,
      content: { config: { log: { level: "debug" } }, editorBuffers: {} },
      savedAt: "2026-08-20T00:02:00Z",
    };
    let resolveB!: (value: { data: Draft; etag: string }) => void;
    let resolveC!: (value: { data: Draft; etag: string }) => void;
    const pendingB = new Promise<{ data: Draft; etag: string }>((resolve) => {
      resolveB = resolve;
    });
    const pendingC = new Promise<{ data: Draft; etag: string }>((resolve) => {
      resolveC = resolve;
    });
    const request = vi
      .spyOn(apiClient, "request")
      .mockImplementationOnce(() => pendingB)
      .mockImplementationOnce(() => pendingC);

    const navigationB = router.push(`/config/${profileB.id}/general`);
    await vi.waitFor(() => expect(request).toHaveBeenCalledTimes(1));
    const navigationC = router.push(`/config/${profileC.id}/general`);
    await vi.waitFor(() => expect(request).toHaveBeenCalledTimes(2));

    resolveC({ data: draftC, etag: '"draft-1"' });
    await navigationC;
    resolveB({ data: draftB, etag: '"draft-1"' });
    await navigationB;

    expect(router.currentRoute.value.params.profileId).toBe(profileC.id);
    expect(profiles.activeId).toBe(profileC.id);
    expect(draft.draft).toEqual(draftC);
  });

  it("cancels a pending profile load when navigation returns to the active profile", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profileA, profileB];
    profiles.activeId = profileA.id;
    const draft = useDraftStore();
    const draftA: Draft = {
      profileId: profileA.id,
      revision: 3,
      content: { config: { log: { level: "info" } }, editorBuffers: {} },
      savedAt: "2026-08-20T00:00:00Z",
    };
    draft.draft = draftA;
    draft.etag = '"draft-3"';
    draft.status = "saved";
    await router.push(`/config/${profileA.id}/general`);

    const draftB: Draft = {
      profileId: profileB.id,
      revision: 1,
      content: { config: { log: { level: "warn" } }, editorBuffers: {} },
      savedAt: "2026-08-20T00:01:00Z",
    };
    let resolveB!: (value: { data: Draft; etag: string }) => void;
    const pendingB = new Promise<{ data: Draft; etag: string }>((resolve) => {
      resolveB = resolve;
    });
    const request = vi.spyOn(apiClient, "request").mockImplementationOnce(() => pendingB);

    const navigationB = router.push(`/config/${profileB.id}/general`);
    await vi.waitFor(() => expect(request).toHaveBeenCalledTimes(1));
    await router.push(`/config/${profileA.id}/dns`);
    resolveB({ data: draftB, etag: '"draft-1"' });
    await navigationB;

    expect(request).toHaveBeenCalledTimes(1);
    expect(router.currentRoute.value.fullPath).toBe(`/config/${profileA.id}/dns`);
    expect(profiles.activeId).toBe(profileA.id);
    expect(draft.draft).toEqual(draftA);
    expect(draft.status).toBe("saved");
  });

  it("cancels a pending profile switch even while its initial flush is saving", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profileA, profileB];
    profiles.activeId = profileA.id;
    const draft = useDraftStore();
    const dirtyA: Draft = {
      profileId: profileA.id,
      revision: 3,
      content: { config: { log: { level: "warn" } }, editorBuffers: {} },
      savedAt: "2026-08-20T00:00:00Z",
    };
    draft.draft = dirtyA;
    draft.etag = '"draft-3"';
    draft.status = "dirty";
    draft.mutationVersion = 1;
    await router.push(`/config/${profileA.id}/general`);

    const savedA: Draft = { ...dirtyA, revision: 4 };
    const draftB: Draft = {
      profileId: profileB.id,
      revision: 1,
      content: { config: { log: { level: "debug" } }, editorBuffers: {} },
      savedAt: "2026-08-20T00:01:00Z",
    };
    let resolveSave!: (value: { data: Draft; etag: string }) => void;
    const pendingSave = new Promise<{ data: Draft; etag: string }>((resolve) => {
      resolveSave = resolve;
    });
    const request = vi
      .spyOn(apiClient, "request")
      .mockImplementationOnce(() => pendingSave)
      .mockResolvedValueOnce({ data: draftB, etag: '"draft-1"' });

    const navigationB = router.push(`/config/${profileB.id}/general`);
    await vi.waitFor(() => expect(request).toHaveBeenCalledTimes(1));
    await router.push(`/config/${profileA.id}/dns`);
    resolveSave({ data: savedA, etag: '"draft-4"' });
    await navigationB;

    expect(request).toHaveBeenCalledTimes(1);
    expect(router.currentRoute.value.fullPath).toBe(`/config/${profileA.id}/dns`);
    expect(profiles.activeId).toBe(profileA.id);
    expect(draft.draft).toEqual(savedA);
    expect(draft.status).toBe("saved");
  });
});
