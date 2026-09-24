import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/api/client";
import { createProfileStore } from "@/features/profiles/store";
import type { ProfileApi } from "@/features/profiles/api";
import type { Draft, ProfileSummary } from "@/features/profiles/types";

function draftFixture(revision = 3): Draft {
  return { profileId: "profile-a", revision, config: { inbounds: [] }, buffers: { log: '{"level":"info"}', outbounds: "[]" }, bindings: [], savedAt: "2026-08-21T00:00:00Z" };
}
const profile: ProfileSummary = { id: "profile-a", name: "家庭出口", draftRevision: 3, appliedRevision: null, appliedDigest: "", appliedAt: null, needsApply: true, lastValidation: null };

type FakeProfileApi = ProfileApi & {
  list: ReturnType<typeof vi.fn>;
  duplicate: ReturnType<typeof vi.fn>;
  getDraft: ReturnType<typeof vi.fn>;
  saveDraft: ReturnType<typeof vi.fn>;
  clearSaveError: () => void;
};

function fakeApi(options: { saveError?: unknown } = {}): FakeProfileApi {
  let failure = options.saveError;
  const api = {
    list: vi.fn(async () => [profile]), create: vi.fn(), duplicate: vi.fn(), rename: vi.fn(), remove: vi.fn(),
    getDraft: vi.fn(async () => ({ data: draftFixture(), etag: '"rev-3"' })),
    saveDraft: vi.fn(async (_id: string, _etag: string, input: Draft) => { if (failure) throw failure; return { data: { ...input, revision: 4, savedAt: "2026-08-21T00:01:00Z" }, etag: '"rev-4"' }; }),
    clearSaveError: () => { failure = undefined; }
  };
  return api as FakeProfileApi;
}

describe("档案草稿保存", () => {
  const stores: ReturnType<typeof createProfileStore>[] = [];
  afterEach(() => {
    stores.splice(0).forEach((store) => store.dispose());
    vi.useRealTimers();
  });

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

  it("首次迁移保存期间继续编辑时，接收新格式引用并保留本地修改", async () => {
    vi.useFakeTimers();
    const api = fakeApi(); const store = createProfileStore(api); stores.push(store);
    const legacy = draftFixture();
    legacy.config.outbounds = [{ type: "selector", tag: "proxy", outbounds: ["direct"] }, { type: "direct", tag: "direct" }];
    legacy.buffers.outbounds = JSON.stringify(legacy.config.outbounds);
    api.getDraft.mockResolvedValue({ data: legacy, etag: '"rev-3"' });
    let acknowledge!: (value: Awaited<ReturnType<ProfileApi["saveDraft"]>>) => void;
    api.saveDraft.mockImplementationOnce(() => new Promise((resolve) => { acknowledge = resolve; }));
    await store.refresh(); await store.open("profile-a");
    store.mutate((value) => { value.buffers.log = '{"level":"debug"}'; });
    const saving = store.flush();
    const sent = api.saveDraft.mock.calls[0]![2] as Draft;
    store.mutate((value) => {
      value.buffers.log = '{"level":"warn"}';
      value.buffers.route = '{"final":"direct"}';
      value.config.route = { final: "direct" };
      value.editorState = { route: { version: 1, insertionPosition: 0, disabledRules: [] } };
    });
    const migrated: Draft = {
      ...sent, revision: 4, schemaVersion: 2,
      references: { version: 2, inbounds: [], outbounds: [{ id: "proxy", tag: "proxy", type: "selector", include: "", exclude: "", references: [{ kind: "outbound", tag: "direct" }] }] },
      config: { ...sent.config, outbounds: [{ type: "selector", tag: "proxy" }, { type: "direct", tag: "direct" }] },
      buffers: { ...sent.buffers, outbounds: '[{"type":"selector","tag":"proxy"},{"type":"direct","tag":"direct"}]' },
      editorState: { outbounds: { version: 2, groups: [] } }
    };
    acknowledge({ data: migrated, etag: '"rev-4"' });
    await saving;
    expect(store.saveState.value).toBe("dirty");
    expect(store.draft.value).toMatchObject({
      revision: 4, schemaVersion: 2, references: migrated.references,
      buffers: { log: '{"level":"warn"}', route: '{"final":"direct"}', outbounds: migrated.buffers.outbounds },
      config: { route: { final: "direct" }, outbounds: migrated.config.outbounds },
      editorState: { route: { version: 1, insertionPosition: 0 }, outbounds: { version: 2 } }
    });
    await store.flush();
    expect(api.saveDraft).toHaveBeenCalledTimes(2);
    expect(api.saveDraft.mock.calls[1]![1]).toBe('"rev-4"');
    expect(api.saveDraft.mock.calls[1]![2]).toMatchObject({ schemaVersion: 2, references: migrated.references, buffers: { log: '{"level":"warn"}' } });
    expect(store.saveState.value).toBe("saved");
  });

  it("断线后保留内存修改，并在 online 时重试", async () => {
    const api = fakeApi({ saveError: new TypeError("Failed to fetch") }); const store = createProfileStore(api); stores.push(store);
    await store.refresh(); await store.open("profile-a"); store.mutate((value) => { value.buffers.log = '{"level":"error"}'; }); await store.flush();
    expect(store.saveState.value).toBe("offline"); expect(localStorage.length).toBe(0);
    api.clearSaveError(); window.dispatchEvent(new Event("online")); await new Promise((resolve) => setTimeout(resolve));
    expect(store.saveState.value).toBe("saved");
  });

  it("保存响应不能覆盖期间新编辑的引用、展示状态和规则绑定", async () => {
    vi.useFakeTimers();
    const api = fakeApi(); const store = createProfileStore(api); stores.push(store);
    const initial: Draft = { ...draftFixture(), schemaVersion: 2, references: { version: 2, inbounds: [], outbounds: [] } };
    api.getDraft.mockResolvedValue({ data: initial, etag: '"rev-3"' });
    let acknowledge!: (value: Awaited<ReturnType<ProfileApi["saveDraft"]>>) => void;
    api.saveDraft.mockImplementationOnce(() => new Promise((resolve) => { acknowledge = resolve; }));
    await store.refresh(); await store.open("profile-a");
    store.mutate((value) => { value.buffers.log = '{"level":"debug"}'; });
    const saving = store.flush();
    const sent = api.saveDraft.mock.calls[0]![2] as Draft;
    const references: Draft["references"] = { version: 2, inbounds: [], outbounds: [{ id: "proxy", tag: "proxy", type: "selector", references: [{ kind: "subscription", subscriptionId: "subscription-a" }] }] };
    const editorState = { outbounds: { version: 2, groups: [{ id: "proxy", tag: "proxy", hidden: true }] } };
    const bindings = [{ resourceId: "rule-a", tag: "ads", enabled: true }];
    store.mutate((value) => {
      value.references = references;
      value.editorState = editorState;
      value.bindings = bindings;
      value.buffers.outbounds = '[{"type":"selector","tag":"proxy"}]';
    });
    acknowledge({ data: { ...sent, revision: 4 }, etag: '"rev-4"' });
    await saving;
    await store.flush();
    expect(api.saveDraft.mock.calls[1]![2]).toMatchObject({ references, editorState, bindings, buffers: { outbounds: '[{"type":"selector","tag":"proxy"}]' } });
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

  it("保存后公开服务端自动重载结果", async () => {
    const api = fakeApi(); const store = createProfileStore(api); stores.push(store);
    api.saveDraft.mockImplementation(async (_id: string, _etag: string, input: Draft) => ({
      data: { ...input, revision: 4, savedAt: "2026-08-21T00:01:00Z" },
      etag: '"rev-4"',
      autoReload: { attempted: true, applied: true, reloaded: true, message: "配置已自动应用并重新加载运行核心" }
    }));
    await store.refresh(); await store.open("profile-a");
    store.mutate((value) => { value.buffers.log = '{"level":"debug"}'; });
    await store.flush();
    expect(store.autoReloadOutcome.value).toEqual({ attempted: true, applied: true, reloaded: true, message: "配置已自动应用并重新加载运行核心" });
    store.mutate((value) => { value.buffers.log = '{"level":"warn"}'; });
    expect(store.autoReloadOutcome.value).toBeNull();
  });

  it("复制前保存当前草稿并打开新档案", async () => {
    const copied = { ...profile, id: "profile-copy", name: "家庭出口 副本", draftRevision: 1 };
    const api = fakeApi();
    let profiles = [profile];
    api.list.mockImplementation(async () => profiles);
    api.duplicate.mockImplementation(async () => { profiles = [profile, copied]; return copied; });
    api.getDraft.mockImplementation(async (id: string) => ({ data: { ...draftFixture(id === "profile-copy" ? 1 : 3), profileId: id }, etag: id === "profile-copy" ? '"rev-1"' : '"rev-3"' }));
    const store = createProfileStore(api); stores.push(store);
    await store.refresh(); await store.open("profile-a");
    store.mutate((value) => { value.buffers.log = '{"level":"debug"}'; });
    await store.duplicate("profile-a");
    expect(api.saveDraft).toHaveBeenCalledTimes(1);
    expect(api.duplicate).toHaveBeenCalledWith("profile-a");
    expect(store.activeProfileId.value).toBe("profile-copy");
    expect(store.draft.value?.revision).toBe(1);
  });
});
