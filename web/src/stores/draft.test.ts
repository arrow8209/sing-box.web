import { beforeEach, describe, expect, it, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { apiClient, APIClientError } from "../api/client";
import { useDraftStore } from "./draft";
import { useProfilesStore } from "./profiles";
import type { Draft, Profile } from "../api/types";

const profile: Profile = {
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

const draftBody: Draft = {
  profileId: profile.id,
  revision: 3,
  content: { config: { log: { level: "info" } }, editorBuffers: {} },
  savedAt: "2026-08-18T10:00:00Z",
};

describe("useDraftStore", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    apiClient.clearCSRFToken();
  });

  it("load fetches the draft and records the ETag", async () => {
    const request = vi.spyOn(apiClient, "request").mockResolvedValue({
      data: draftBody,
      etag: '"draft-3"',
    });
    const draft = useDraftStore();

    await draft.load(profile.id);

    expect(request).toHaveBeenCalledWith(`/profiles/${profile.id}/draft`);
    expect(draft.draft).toEqual(draftBody);
    expect(draft.etag).toBe('"draft-3"');
    expect(draft.status).toBe("saved");
  });

  it("debounces mutations for 600 ms and coalesces them into one PUT", async () => {
    vi.useFakeTimers();
    try {
      vi.spyOn(apiClient, "request").mockResolvedValueOnce({
        data: draftBody,
        etag: '"draft-3"',
      });
      const draft = useDraftStore();
      await draft.load(profile.id);

      const savedDraft: Draft = { ...draftBody, revision: 4 };
      const request = vi.spyOn(apiClient, "request").mockResolvedValue({
        data: savedDraft,
        etag: '"draft-4"',
      });

      draft.replaceConfig({ log: { level: "warn" } });
      draft.replaceConfig({ log: { level: "debug" }, dns: { servers: [] } });
      expect(draft.status).toBe("dirty");

      await vi.advanceTimersByTimeAsync(599);
      expect(request).not.toHaveBeenCalled();

      await vi.advanceTimersByTimeAsync(1);
      expect(request).toHaveBeenCalledTimes(1);

      const [path, init] = request.mock.calls[0];
      expect(path).toBe(`/profiles/${profile.id}/draft`);
      expect(init).toEqual(
        expect.objectContaining({
          method: "PUT",
          headers: expect.objectContaining({ "If-Match": '"draft-3"' }),
          body: expect.stringContaining('"debug"'),
        }),
      );
      expect(draft.draft?.revision).toBe(4);
      expect(draft.etag).toBe('"draft-4"');
      expect(draft.status).toBe("saved");
    } finally {
      vi.useRealTimers();
    }
  });

  it("on 409 enters the conflict state without retrying", async () => {
    vi.useFakeTimers();
    try {
      vi.spyOn(apiClient, "request").mockResolvedValueOnce({
        data: draftBody,
        etag: '"draft-3"',
      });
      const draft = useDraftStore();
      await draft.load(profile.id);

      const conflictError = new APIClientError(409, {
        code: "draft_conflict",
        message: "草稿已被其他页面更新",
        currentRevision: 5,
      });
      const request = vi.spyOn(apiClient, "request").mockRejectedValue(conflictError);

      draft.replaceConfig({ log: { level: "error" } });
      await vi.advanceTimersByTimeAsync(600);

      expect(request).toHaveBeenCalledTimes(1);
      expect(draft.status).toBe("conflict");
      expect(draft.conflict).toEqual({
        localContent: { config: { log: { level: "error" } }, editorBuffers: {} },
        serverRevision: 5,
      });

      await vi.advanceTimersByTimeAsync(5000);
      expect(request).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it("reloadAfterConflict immediately discards local changes and loads the server draft", async () => {
    vi.spyOn(apiClient, "request").mockResolvedValueOnce({ data: draftBody, etag: '"draft-3"' });
    const draft = useDraftStore();
    await draft.load(profile.id);

    draft.replaceConfig({ log: { level: "error" } });
    draft.conflict = {
      localContent: { config: { log: { level: "error" } }, editorBuffers: {} },
      serverRevision: 5,
    };
    draft.status = "conflict";

    const confirm = vi.fn().mockReturnValue(false);
    vi.stubGlobal("window", { confirm });
    const serverDraft: Draft = { ...draftBody, revision: 5 };
    const request = vi.spyOn(apiClient, "request").mockResolvedValue({
      data: serverDraft,
      etag: '"draft-5"',
    });
    await draft.reloadAfterConflict();

    expect(confirm).not.toHaveBeenCalled();
    expect(request).toHaveBeenCalledWith(`/profiles/${profile.id}/draft`);
    expect(draft.draft?.revision).toBe(5);
    expect(draft.etag).toBe('"draft-5"');
    expect(draft.conflict).toBeNull();
    expect(draft.status).toBe("saved");
  });

  it("overwriteAfterConflict GETs the latest draft then PUTs the preserved local content", async () => {
    vi.spyOn(apiClient, "request").mockResolvedValueOnce({ data: draftBody, etag: '"draft-3"' });
    const draft = useDraftStore();
    await draft.load(profile.id);

    const localContent = { config: { log: { level: "error" } }, editorBuffers: {} };
    draft.conflict = { localContent, serverRevision: 5 };
    draft.status = "conflict";

    const serverDraft: Draft = { ...draftBody, revision: 5 };
    const overwritten: Draft = { ...draftBody, revision: 6, content: localContent };
    const request = vi
      .spyOn(apiClient, "request")
      .mockResolvedValueOnce({ data: serverDraft, etag: '"draft-5"' })
      .mockResolvedValueOnce({ data: overwritten, etag: '"draft-6"' });

    await draft.overwriteAfterConflict();

    expect(request).toHaveBeenNthCalledWith(1, `/profiles/${profile.id}/draft`);
    const [path, init] = request.mock.calls[1];
    expect(path).toBe(`/profiles/${profile.id}/draft`);
    expect(init).toEqual(
      expect.objectContaining({
        method: "PUT",
        headers: expect.objectContaining({ "If-Match": '"draft-5"' }),
      }),
    );
    const body = JSON.parse(init?.body as string);
    expect(body.config).toEqual({ log: { level: "error" } });
    expect(draft.etag).toBe('"draft-6"');
    expect(draft.conflict).toBeNull();
  });

  it("keeps the local content when the network fails during save", async () => {
    vi.useFakeTimers();
    try {
      vi.spyOn(apiClient, "request").mockResolvedValueOnce({
        data: draftBody,
        etag: '"draft-3"',
      });
      const draft = useDraftStore();
      await draft.load(profile.id);

      vi.spyOn(apiClient, "request").mockRejectedValue(new TypeError("network down"));
      draft.replaceConfig({ log: { level: "trace" } });
      await vi.advanceTimersByTimeAsync(600);

      expect(draft.status).toBe("offline");
      expect(draft.draft?.content.config).toEqual({ log: { level: "trace" } });
    } finally {
      vi.useRealTimers();
    }
  });

  it("rejects local mutation for a read-only draft", async () => {
    const readOnlyProfile: Profile = {
      ...profile,
      draftReadOnly: true,
      draftError: "草稿 JSON 损坏，需要管理员在服务器修复",
    };
    vi.spyOn(apiClient, "request").mockResolvedValueOnce({ data: [readOnlyProfile], etag: null });
    const profiles = useProfilesStore();
    await profiles.load();

    const draft = useDraftStore();
    await draft.load(readOnlyProfile.id);

    draft.replaceConfig({ log: { level: "debug" } });
    expect(draft.status).toBe("error");
    expect(draft.lastError?.code).toBe("draft_read_only");
  });

  it("wraps applied top-level buffers after structured config changes", () => {
    const draft = useDraftStore();
    draft.draft = {
      ...draftBody,
      content: {
        config: { log: { level: "info" } },
        editorBuffers: {
          log: { text: '{"log":{"level":"info"}}', valid: true, applied: true },
        },
      },
    };
    draft.status = "saved";

    draft.replaceConfig({ log: { level: "warn" } });

    expect(draft.draft?.content.editorBuffers.log).toEqual({
      text: JSON.stringify({ log: { level: "warn" } }, null, 2),
      valid: true,
      applied: true,
    });
  });

  it("restores an unapplied top-level buffer in the wrapped module format", () => {
    const draft = useDraftStore();
    draft.draft = {
      ...draftBody,
      content: {
        config: { log: { level: "info" } },
        editorBuffers: {
          log: { text: '{"log":', valid: false, applied: false },
        },
      },
    };
    draft.status = "saved";

    draft.discardUnappliedBuffer("log");

    expect(draft.draft?.content.editorBuffers.log).toEqual({
      text: JSON.stringify({ log: { level: "info" } }, null, 2),
      valid: true,
      applied: true,
    });
  });

  it("keeps collection-row buffers formatted as row objects", () => {
    const draft = useDraftStore();
    draft.draft = {
      ...draftBody,
      content: {
        config: { outbounds: [{ type: "direct", tag: "direct" }] },
        editorBuffers: {
          "outbounds/0": { text: '{"type":"direct"}', valid: true, applied: true },
        },
      },
    };
    draft.status = "saved";

    draft.replaceConfig({ outbounds: [{ type: "block", tag: "blocked" }] });

    expect(draft.draft?.content.editorBuffers["outbounds/0"]?.text).toBe(
      JSON.stringify({ type: "block", tag: "blocked" }, null, 2),
    );
  });

  it("regenerates nested collection-path buffers from their full canonical path", () => {
    const draft = useDraftStore();
    draft.draft = {
      ...draftBody,
      content: {
        config: { route: { rules: [{ action: "route", outbound: "old" }] } },
        editorBuffers: {
          "route/rules/0": { text: '{"action":"route"}', valid: true, applied: true },
        },
      },
    };
    draft.status = "saved";

    draft.replaceConfig({ route: { rules: [{ action: "route", outbound: "new" }] } });

    expect(draft.draft?.content.editorBuffers["route/rules/0"]?.text).toBe(
      JSON.stringify({ action: "route", outbound: "new" }, null, 2),
    );
  });

  it("restores nested collection-path buffers from their full canonical path", () => {
    const draft = useDraftStore();
    draft.draft = {
      ...draftBody,
      content: {
        config: { route: { rules: [{ action: "route", outbound: "direct" }] } },
        editorBuffers: {
          "route/rules/0": { text: "{", valid: false, applied: false },
        },
      },
    };
    draft.status = "saved";

    draft.discardUnappliedBuffer("route/rules/0");

    expect(draft.draft?.content.editorBuffers["route/rules/0"]).toEqual({
      text: JSON.stringify({ action: "route", outbound: "direct" }, null, 2),
      valid: true,
      applied: true,
    });
  });

  it("removes buffers whose top-level canonical key no longer exists", () => {
    const draft = useDraftStore();
    draft.draft = {
      ...draftBody,
      content: {
        config: { log: { level: "info" } },
        editorBuffers: {
          dns: { text: '{"dns":{"servers":[]}}', valid: true, applied: true },
        },
      },
    };
    draft.status = "saved";

    draft.replaceConfig({ log: { level: "warn" } });

    expect(draft.draft?.content.editorBuffers.dns).toBeUndefined();
  });

  it("removes an unapplied buffer whose canonical path no longer exists", () => {
    const draft = useDraftStore();
    draft.draft = {
      ...draftBody,
      content: {
        config: { log: { level: "info" } },
        editorBuffers: {
          dns: { text: "{", valid: false, applied: false },
        },
      },
    };
    draft.status = "saved";

    draft.discardUnappliedBuffer("dns");

    expect(draft.draft?.content.editorBuffers.dns).toBeUndefined();
  });

  it("keeps an explicitly null top-level canonical value as an applied buffer", () => {
    const draft = useDraftStore();
    draft.draft = {
      ...draftBody,
      content: {
        config: { log: { level: "info" } },
        editorBuffers: {
          log: { text: '{"log":{"level":"info"}}', valid: true, applied: true },
        },
      },
    };
    draft.status = "saved";

    draft.replaceConfig({ log: null });

    expect(draft.draft?.content.editorBuffers.log).toEqual({
      text: JSON.stringify({ log: null }, null, 2),
      valid: true,
      applied: true,
    });
  });

  it("treats a sparse array index with a polluted prototype as a missing path", () => {
    const draft = useDraftStore();
    const rules: unknown[] = [];
    rules.length = 1;
    Object.defineProperty(Array.prototype, "0", {
      configurable: true,
      value: { action: "route", outbound: "inherited" },
      writable: true,
    });
    try {
      expect(draft.formatModule("route/rules/0", { route: { rules } })).toBeUndefined();
    } finally {
      delete (Array.prototype as unknown as Record<string, unknown>)["0"];
    }
  });

  it("removes regenerated buffers for sparse array indexes with polluted prototypes", () => {
    const draft = useDraftStore();
    const rules: unknown[] = [];
    rules.length = 1;
    draft.draft = {
      ...draftBody,
      content: {
        config: { route: { rules: [] } },
        editorBuffers: {
          "route/rules/0": { text: '{"action":"route"}', valid: true, applied: true },
        },
      },
    };
    draft.status = "saved";
    Object.defineProperty(Array.prototype, "0", {
      configurable: true,
      value: { action: "route", outbound: "inherited" },
      writable: true,
    });
    try {
      draft.replaceConfig({ route: { rules } });

      expect(draft.draft?.content.editorBuffers["route/rules/0"]).toBeUndefined();
    } finally {
      delete (Array.prototype as unknown as Record<string, unknown>)["0"];
    }
  });
});
