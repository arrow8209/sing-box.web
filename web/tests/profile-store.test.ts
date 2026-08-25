import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/api/client";
import { createProfileStore } from "@/features/profiles/store";
import type { ProfileApi } from "@/features/profiles/api";
import type { Draft, ProfileSummary } from "@/features/profiles/types";

function draftFixture(revision = 3): Draft {
  return { profileId: "profile-a", revision, config: { inbounds: [] }, buffers: { log: '{"level":"info"}', outbounds: "[]" }, bindings: [], savedAt: "2026-08-21T00:00:00Z" };
}
const profile: ProfileSummary = { id: "profile-a", name: "家庭出口", draftRevision: 3, appliedRevision: null, appliedDigest: "", appliedAt: null, needsApply: true, lastValidation: null };

function fakeApi(options: { saveError?: unknown } = {}): ProfileApi & { saveDraft: ReturnType<typeof vi.fn>; clearSaveError: () => void } {
  let failure = options.saveError;
  const api = {
    list: vi.fn(async () => [profile]), create: vi.fn(), rename: vi.fn(), remove: vi.fn(),
    getDraft: vi.fn(async () => ({ data: draftFixture(), etag: '"rev-3"' })),
    saveDraft: vi.fn(async (_id: string, _etag: string, input: Draft) => { if (failure) throw failure; return { data: { ...input, revision: 4, savedAt: "2026-08-21T00:01:00Z" }, etag: '"rev-4"' }; }),
    clearSaveError: () => { failure = undefined; }
  };
  return api as ProfileApi & { saveDraft: ReturnType<typeof vi.fn>; clearSaveError: () => void };
}

describe("档案草稿保存", () => {
  const stores: ReturnType<typeof createProfileStore>[] = [];
  afterEach(() => stores.splice(0).forEach((store) => store.dispose()));

  it("防抖保存，仅发送 buffers 和 bindings", async () => {
    vi.useFakeTimers();
    const api = fakeApi(); const store = createProfileStore(api); stores.push(store);
    await store.refresh(); await store.open("profile-a");
    store.mutate((value) => { value.buffers.log = '{"level":"debug"}'; });
    store.mutate((value) => { value.buffers.log = '{"level":"warn"}'; });
    await vi.advanceTimersByTimeAsync(600);
    expect(api.saveDraft).toHaveBeenCalledTimes(1);
    expect(api.saveDraft.mock.calls[0]?.[1]).toBe('"rev-3"');
    expect(api.saveDraft.mock.calls[0]?.[2]).toMatchObject({ buffers: { log: '{"level":"warn"}' }, bindings: [] });
    expect(store.saveState.value).toBe("saved"); expect(store.etag.value).toBe('"rev-4"');
    vi.useRealTimers();
  });

  it("409 时保留本地内容直到用户选择", async () => {
    const api = fakeApi({ saveError: new ApiError(409, { code: "draft_revision_conflict", message: "冲突", latestRevision: 4 }) });
    const store = createProfileStore(api); stores.push(store);
    await store.refresh(); await store.open("profile-a");
    store.mutate((value) => { value.buffers.outbounds = '[{"tag":"local"}]'; }); await store.flush();
    expect(store.saveState.value).toBe("conflict"); expect(store.draft.value?.buffers.outbounds).toContain("local"); expect(store.conflict.value?.latestRevision).toBe(4);
  });

  it("断线后保留内存修改，并在 online 时重试", async () => {
    const api = fakeApi({ saveError: new TypeError("Failed to fetch") }); const store = createProfileStore(api); stores.push(store);
    await store.refresh(); await store.open("profile-a"); store.mutate((value) => { value.buffers.log = '{"level":"error"}'; }); await store.flush();
    expect(store.saveState.value).toBe("offline"); expect(localStorage.length).toBe(0);
    api.clearSaveError(); window.dispatchEvent(new Event("online")); await new Promise((resolve) => setTimeout(resolve));
    expect(store.saveState.value).toBe("saved");
  });

  it("递归解包更新过程写回的深层 Vue 代理", async () => {
    const api = fakeApi(); const store = createProfileStore(api); stores.push(store);
    await store.refresh(); await store.open("profile-a");
    store.mutate((value) => {
      const current = value.config.inbounds as unknown[];
      value.config = { ...value.config, inbounds: [...current, { type: "mixed", tag: "mixed-in" }] };
      value.buffers.inbounds = JSON.stringify(value.config.inbounds);
    });
    await store.flush();
    expect(store.saveState.value).toBe("saved");
    expect(api.saveDraft).toHaveBeenCalledTimes(1);
    expect(api.saveDraft.mock.calls[0]?.[2].buffers.inbounds).toContain("mixed-in");
  });
});
