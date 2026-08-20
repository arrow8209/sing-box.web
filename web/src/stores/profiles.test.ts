import { beforeEach, describe, expect, it, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { apiClient } from "../api/client";
import { useProfilesStore } from "./profiles";
import { useDraftStore } from "./draft";
import type { Draft, Profile } from "../api/types";

const profileA: Profile = {
  id: "a".repeat(32),
  name: "家庭网关",
  slug: "home-gateway",
  draftRevision: 3,
  publishedRevision: 2,
  publishedDigest: "digest-a",
  publishedAt: "2026-08-18T10:00:00Z",
  lastValidation: null,
  directoryHint: "/etc/sing-box/home-gateway",
  checkCommand: "sing-box check -C /etc/sing-box/home-gateway",
  draftReadOnly: false,
  draftError: "",
};

const profileB: Profile = {
  id: "b".repeat(32),
  name: "办公室",
  slug: "office",
  draftRevision: 1,
  publishedRevision: null,
  publishedDigest: "",
  publishedAt: null,
  lastValidation: null,
  directoryHint: "/etc/sing-box/office",
  checkCommand: "sing-box check -C /etc/sing-box/office",
  draftReadOnly: false,
  draftError: "",
};

const draftBody: Draft = {
  profileId: "a".repeat(32),
  revision: 3,
  content: { config: { log: { level: "info" } }, editorBuffers: {} },
  savedAt: "2026-08-18T10:00:00Z",
};

describe("useProfilesStore", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
    apiClient.clearCSRFToken();
  });

  it("load stores profiles in server order", async () => {
    const request = vi.spyOn(apiClient, "request").mockResolvedValue({
      data: [profileA, profileB],
      etag: null,
    });
    const profiles = useProfilesStore();

    await profiles.load();

    expect(request.mock.calls[0][0]).toBe("/profiles");
    expect(profiles.items.map((p) => p.slug)).toEqual(["home-gateway", "office"]);
  });

  it("create selects and loads the new profile draft before routing to its general module", async () => {
    vi.spyOn(apiClient, "request").mockResolvedValue({ data: [profileB], etag: null });
    const profiles = useProfilesStore();
    await profiles.load();

    const request = vi
      .spyOn(apiClient, "request")
      .mockResolvedValueOnce({ data: profileA, etag: null })
      .mockResolvedValueOnce({ data: draftBody, etag: '"draft-3"' });
    const draftStore = useDraftStore();
    const router = {
      push: vi.fn(() => {
        expect(profiles.activeId).toBe(profileA.id);
        expect(draftStore.draft).toEqual(draftBody);
        expect(draftStore.etag).toBe('"draft-3"');
      }),
    };
    const created = await profiles.create({ name: "家庭网关", slug: "home-gateway" }, router as never);

    expect(request).toHaveBeenCalledWith(
      "/profiles",
      expect.objectContaining({ method: "POST", body: JSON.stringify({ name: "家庭网关", slug: "home-gateway" }) }),
    );
    expect(created).toEqual(profileA);
    expect(profiles.items.map((p) => p.slug)).toEqual(["office", "home-gateway"]);
    expect(request).toHaveBeenNthCalledWith(2, `/profiles/${profileA.id}/draft`);
    expect(profiles.activeId).toBe(profileA.id);
    expect(draftStore.etag).toBe('"draft-3"');
    expect(router.push).toHaveBeenCalledWith(`/config/${profileA.id}/general`);
  });

  it("does not create a profile until the current dirty draft is saved", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profileA];
    profiles.activeId = profileA.id;
    const draftStore = useDraftStore();
    draftStore.draft = {
      ...draftBody,
      content: { config: { log: { level: "warn" } }, editorBuffers: {} },
    };
    draftStore.etag = '"draft-3"';
    draftStore.status = "dirty";
    draftStore.mutationVersion = 1;
    const request = vi.spyOn(apiClient, "request").mockRejectedValue(new TypeError("offline"));

    await expect(
      profiles.create({ name: "办公室", slug: "office" }),
    ).rejects.toThrow("草稿保存失败");

    expect(request).toHaveBeenCalledTimes(1);
    expect(request.mock.calls[0][0]).toBe(`/profiles/${profileA.id}/draft`);
    expect(request.mock.calls[0][1]).toEqual(expect.objectContaining({ method: "PUT" }));
    expect(profiles.items).toEqual([profileA]);
    expect(profiles.activeId).toBe(profileA.id);
  });

  it("rename changes the name only", async () => {
    vi.spyOn(apiClient, "request").mockResolvedValue({ data: [profileA], etag: null });
    const profiles = useProfilesStore();
    await profiles.load();

    const request = vi.spyOn(apiClient, "request").mockResolvedValue({
      data: { ...profileA, name: "新名称" },
      etag: null,
    });
    await profiles.rename(profileA.id, "新名称");

    expect(request).toHaveBeenCalledWith(
      `/profiles/${profileA.id}`,
      expect.objectContaining({ method: "PATCH", body: JSON.stringify({ name: "新名称" }) }),
    );
    expect(profiles.items[0].name).toBe("新名称");
    expect(profiles.items[0].slug).toBe("home-gateway");
  });

  it("select fetches the active profile draft with its ETag", async () => {
    vi.spyOn(apiClient, "request").mockResolvedValueOnce({ data: [profileA], etag: null });
    const profiles = useProfilesStore();
    await profiles.load();

    const request = vi.spyOn(apiClient, "request").mockResolvedValue({ data: draftBody, etag: '"draft-3"' });
    const draftStore = useDraftStore();
    await profiles.select(profileA.id);

    expect(request).toHaveBeenCalledWith(`/profiles/${profileA.id}/draft`);
    expect(profiles.activeId).toBe(profileA.id);
    expect(draftStore.etag).toBe('"draft-3"');
    expect(draftStore.draft).toEqual(draftBody);
  });

  it("saves dirty content before loading another profile", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profileA, profileB];
    profiles.activeId = profileA.id;
    const draftStore = useDraftStore();
    draftStore.draft = {
      ...draftBody,
      content: { config: { log: { level: "warn" } }, editorBuffers: {} },
    };
    draftStore.etag = '"draft-3"';
    draftStore.status = "dirty";
    draftStore.mutationVersion = 1;

    const savedA: Draft = { ...draftStore.draft, revision: 4 };
    const draftB: Draft = {
      profileId: profileB.id,
      revision: 1,
      content: { config: { log: { level: "debug" } }, editorBuffers: {} },
      savedAt: "2026-08-18T10:01:00Z",
    };
    const request = vi
      .spyOn(apiClient, "request")
      .mockResolvedValueOnce({ data: savedA, etag: '"draft-4"' })
      .mockResolvedValueOnce({ data: draftB, etag: '"draft-1"' });

    await profiles.select(profileB.id);

    expect(request).toHaveBeenCalledTimes(2);
    expect(request.mock.calls[0][0]).toBe(`/profiles/${profileA.id}/draft`);
    expect(request.mock.calls[0][1]).toEqual(expect.objectContaining({ method: "PUT" }));
    expect(request).toHaveBeenNthCalledWith(2, `/profiles/${profileB.id}/draft`);
    expect(profiles.activeId).toBe(profileB.id);
    expect(draftStore.draft).toEqual(draftB);
  });

  it("keeps the current profile selected when its dirty draft cannot be saved", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profileA, profileB];
    profiles.activeId = profileA.id;
    const draftStore = useDraftStore();
    draftStore.draft = {
      ...draftBody,
      content: { config: { log: { level: "warn" } }, editorBuffers: {} },
    };
    draftStore.etag = '"draft-3"';
    draftStore.status = "dirty";
    draftStore.mutationVersion = 1;
    const request = vi.spyOn(apiClient, "request").mockRejectedValue(new TypeError("offline"));

    await expect(profiles.select(profileB.id)).rejects.toThrow("草稿保存失败");

    expect(request).toHaveBeenCalledTimes(1);
    expect(request.mock.calls[0][0]).toBe(`/profiles/${profileA.id}/draft`);
    expect(profiles.activeId).toBe(profileA.id);
    expect(draftStore.draft?.profileId).toBe(profileA.id);
    expect(draftStore.draft?.content.config).toEqual({ log: { level: "warn" } });
    expect(draftStore.status).toBe("offline");
  });

  it("a read-only draft shows the safe repair message and never issues a draft PUT", async () => {
    const readOnlyProfile: Profile = {
      ...profileA,
      draftReadOnly: true,
      draftError: "草稿 JSON 损坏，需要管理员在服务器修复",
    };
    vi.spyOn(apiClient, "request").mockResolvedValueOnce({ data: [readOnlyProfile], etag: null });
    const profiles = useProfilesStore();
    await profiles.load();

    const draftStore = useDraftStore();
    const save = vi.spyOn(draftStore, "save").mockResolvedValue(false);
    const request = vi.spyOn(apiClient, "request");
    await profiles.select(readOnlyProfile.id);

    expect(request).not.toHaveBeenCalled();
    await draftStore.replaceConfig({ log: { level: "debug" } });
    expect(save).not.toHaveBeenCalled();
    expect(draftStore.status).toBe("error");
    expect(profiles.items[0].draftError).toBe("草稿 JSON 损坏，需要管理员在服务器修复");
  });

  it("never labels the active editor profile as 核心当前配置", () => {
    const profiles = useProfilesStore();
    profiles.items = [profileA];
    profiles.activeId = profileA.id;

    expect(profiles.items[0].name).toBe("家庭网关");
    expect(profiles.items[0].name).not.toContain("核心当前配置");
  });
});
