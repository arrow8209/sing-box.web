import { flushPromises, mount } from "@vue/test-utils";
import { ref } from "vue";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/api/client";
import ApplicationBar from "@/features/application/ApplicationBar.vue";
import ValidationErrors from "@/features/application/ValidationErrors.vue";
import { createApplicationStore } from "@/features/application/store";
import type { ApplicationApi } from "@/features/application/api";
import type { PreviewResult } from "@/features/application/types";
import { createProfileStore } from "@/features/profiles/store";
import type { ProfileApi } from "@/features/profiles/api";
import type { Draft, ProfileSummary } from "@/features/profiles/types";
import type { ReferenceSources } from "@/features/application/references";
import type { SubscriptionDetail } from "@/features/subscriptions/types";

const profileId = "0123456789abcdef0123456789abcdef";
const draft = (): Draft => ({ profileId, revision: 4, config: {}, buffers: { log: "{}" }, bindings: [], savedAt: "2026-08-21T00:00:00Z" });
const profile: ProfileSummary = { id: profileId, name: "A", draftRevision: 4, appliedRevision: null, appliedDigest: "", appliedAt: null, needsApply: true, lastValidation: null };
function profiles(): ReturnType<typeof createProfileStore> {
  const api: ProfileApi = { list: vi.fn(async () => [profile]), create: vi.fn(), duplicate: vi.fn(), rename: vi.fn(), remove: vi.fn(), getDraft: vi.fn(async () => ({ data: draft(), etag: "x" })), saveDraft: vi.fn() };
  return createProfileStore(api);
}

function linkedProfiles() {
  const outbounds = [{ type: "selector", tag: "选择", interrupt_exist_connections: true, outbounds: ["test-node"] }, { type: "ssh", tag: "test-node", detour: "removed-group" }];
  let saved: Draft = {
    ...draft(), config: { outbounds }, buffers: { outbounds: JSON.stringify(outbounds) },
    editorState: { outbounds: { version: 1, generatedTags: ["test-node"], groups: [{
      id: "group-a", tag: "选择", type: "selector", hidden: false, include: "", exclude: "", icon: "",
      references: [{ kind: "subscription", subscriptionId: "a".repeat(32), tag: "测试订阅", resolvedTags: ["test-node"] }], runtimeTags: ["test-node"]
    }] } }
  };
  const api: ProfileApi = {
    list: vi.fn(async () => [{ ...profile, draftRevision: saved.revision }]), create: vi.fn(), duplicate: vi.fn(), rename: vi.fn(), remove: vi.fn(),
    getDraft: vi.fn(async () => ({ data: structuredClone(saved), etag: `"rev-${saved.revision}"` })),
    saveDraft: vi.fn(async (_id, _etag, input) => {
      saved = { ...structuredClone(input), revision: saved.revision + 1 };
      return { data: structuredClone(saved), etag: `"rev-${saved.revision}"` };
    })
  };
  const latest: SubscriptionDetail = { id: "a".repeat(32), name: "测试订阅", outbounds: [{ type: "ssh", tag: "test-node", server: "updated.example.test" }], updatedAt: "2026-09-23T00:00:00Z" };
  const sources: ReferenceSources = { subscription: vi.fn(async () => latest), inbound: vi.fn() };
  return { profiles: createProfileStore(api), profileApi: api, sources, latest };
}

describe("配置应用", () => {
  const stores: ReturnType<typeof createProfileStore>[] = [];
  afterEach(() => stores.splice(0).forEach((store) => store.dispose()));
  it.each(["preview", "validate", "apply"] as const)("服务端解析时 %s 不同步保存引用", async (action) => {
    const env = linkedProfiles(); stores.push(env.profiles);
    await env.profiles.refresh(); await env.profiles.open(profileId);
    const success = { revision: 4, files: [], digest: "a".repeat(64), valid: true, stage: "core_check", exitCode: 0 };
    const api: ApplicationApi = { preview: vi.fn(async () => success), validate: vi.fn(async () => success), apply: vi.fn(async () => ({ applied: true, profileId, revision: 4, digest: success.digest, validation: success })) };
    const store = createApplicationStore(env.profiles, api, env.sources, ref(true));
    await store[action]();
    expect(env.profileApi.saveDraft).not.toHaveBeenCalled();
    expect(env.sources.subscription).not.toHaveBeenCalled();
    expect(env.profiles.draft.value?.revision).toBe(4);
    expect(api[action]).toHaveBeenCalledWith(profileId, action === "apply" ? { revision: 4 } : 4);
  });

  it("服务端解析时未保存编辑阻止操作，迟到候选不覆盖新输入", async () => {
    const env = linkedProfiles(); stores.push(env.profiles);
    await env.profiles.refresh(); await env.profiles.open(profileId);
    let finish: (result: { revision: number; files: []; digest: string }) => void = () => undefined;
    const api: ApplicationApi = { preview: vi.fn(() => new Promise<PreviewResult>((resolve) => { finish = resolve; })), validate: vi.fn(), apply: vi.fn() };
    const store = createApplicationStore(env.profiles, api, env.sources, ref(true));
    const pending = store.preview(); await flushPromises();
    env.profiles.mutate((current) => { current.buffers.log = '{"level":"debug"}'; });
    finish({ revision: 4, files: [], digest: "late" }); await pending;
    expect(store.previewResult.value).toBeNull();
    await store.validate();
    expect(api.validate).not.toHaveBeenCalled();
    expect(env.profileApi.saveDraft).not.toHaveBeenCalled();
  });
  it.each(["preview", "validate", "apply"] as const)("%s 前读取最新订阅并保存草稿，随后使用新修订执行", async (action) => {
    const env = linkedProfiles(); stores.push(env.profiles);
    await env.profiles.refresh(); await env.profiles.open(profileId);
    const success = { revision: 5, files: [], digest: "a".repeat(64), valid: true, stage: "core_check", exitCode: 0 };
    const api: ApplicationApi = {
      preview: vi.fn(async () => success), validate: vi.fn(async () => success),
      apply: vi.fn(async () => ({ applied: true, profileId, revision: 5, digest: success.digest, validation: success }))
    };
    const store = createApplicationStore(env.profiles, api, env.sources);
    await store[action]();
    expect(env.profileApi.saveDraft).toHaveBeenCalledOnce();
    expect(env.sources.subscription).toHaveBeenCalledWith("a".repeat(32));
    expect(env.profiles.draft.value?.buffers.outbounds).not.toContain("removed-group");
    expect(env.profiles.draft.value?.buffers.outbounds).toContain("updated.example.test");
    expect(api[action]).toHaveBeenCalledWith(profileId, action === "apply" ? { revision: 5 } : 5);
    expect(store.preparing.value).toBe(false);
    expect(store.error.value).toBeNull();
    await store[action]();
    expect(env.profileApi.saveDraft).toHaveBeenCalledOnce();
    expect(env.sources.subscription).toHaveBeenCalledTimes(2);
  });

  it("引用读取失败时不保存部分同步结果，也不继续检测", async () => {
    const env = linkedProfiles(); stores.push(env.profiles);
    await env.profiles.refresh(); await env.profiles.open(profileId);
    vi.mocked(env.sources.subscription).mockRejectedValue(new Error("订阅已删除"));
    const api: ApplicationApi = { preview: vi.fn(), validate: vi.fn(), apply: vi.fn() };
    const store = createApplicationStore(env.profiles, api, env.sources);
    await store.validate();
    expect(env.profileApi.saveDraft).not.toHaveBeenCalled();
    expect(api.validate).not.toHaveBeenCalled();
    expect(store.error.value).toMatchObject({ stage: "sync_references", fieldPath: "outbounds" });
    expect(store.error.value?.message).toContain("订阅已删除");
    expect(env.profiles.draft.value?.buffers.outbounds).toContain("removed-group");
    store.previewResult.value = { revision: 4, files: [], digest: "old-preview" };
    await store.preview();
    expect(store.previewResult.value).toBeNull();
    expect(api.preview).not.toHaveBeenCalled();
  });

  it("同步期间的本地编辑不会被覆盖", async () => {
    const env = linkedProfiles(); stores.push(env.profiles);
    await env.profiles.refresh(); await env.profiles.open(profileId);
    let finish: (value: SubscriptionDetail) => void = () => undefined;
    vi.mocked(env.sources.subscription).mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
    const api: ApplicationApi = { preview: vi.fn(), validate: vi.fn(), apply: vi.fn() };
    const store = createApplicationStore(env.profiles, api, env.sources);
    const pending = store.validate();
    await flushPromises();
    expect(store.preparing.value).toBe(true);
    env.profiles.mutate((current) => { current.buffers.log = '{"level":"debug"}'; });
    finish(env.latest);
    await pending;
    expect(store.error.value?.message).toContain("同步期间草稿已变更");
    expect(env.profiles.draft.value?.buffers.log).toBe('{"level":"debug"}');
    expect(env.profileApi.saveDraft).not.toHaveBeenCalled();
    expect(api.validate).not.toHaveBeenCalled();
  });

  it("同步草稿保存冲突时不会检测旧修订", async () => {
    const env = linkedProfiles(); stores.push(env.profiles);
    await env.profiles.refresh(); await env.profiles.open(profileId);
    vi.mocked(env.profileApi.saveDraft).mockRejectedValue(new ApiError(409, { code: "draft_revision_conflict", message: "版本冲突", latestRevision: 6 }));
    const api: ApplicationApi = { preview: vi.fn(), validate: vi.fn(), apply: vi.fn() };
    const store = createApplicationStore(env.profiles, api, env.sources);
    await store.validate();
    expect(env.profiles.saveState.value).toBe("conflict");
    expect(api.validate).not.toHaveBeenCalled();
    expect(store.error.value?.stage).toBe("sync_references");
  });

  it("配置操作组依次提供预览、检测和应用", async () => {
    const profileStore = profiles(); stores.push(profileStore); await profileStore.refresh(); await profileStore.open(profileId);
    const wrapper = mount(ApplicationBar);
    const buttons = wrapper.findAll("button");
    expect(buttons.map((button) => button.text())).toEqual(["预览", "检测", "应用"]);
    expect(buttons[2].attributes("title")).toBe("检测通过后复制配置文件");
  });
  it("预览仅呈现模块内容", async () => {
    const profileStore = profiles(); stores.push(profileStore); await profileStore.refresh(); await profileStore.open(profileId);
    const api: ApplicationApi = { preview: vi.fn(async () => ({ revision: 4, files: [{ name: "80-route.json", content: { route: { rule_set: [{ path: "data/rulesets/resource-a.srs" }] } } }], digest: "a".repeat(64) })), validate: vi.fn(), apply: vi.fn() };
    const store = createApplicationStore(profileStore, api); await store.preview();
    expect(JSON.stringify(store.previewResult.value)).toContain("data/rulesets/resource-a.srs"); expect(JSON.stringify(store.previewResult.value)).not.toContain("https://");
  });
  it("检查成功时显示成功结果", () => {
    const validation = { revision: 4, files: [], digest: "a".repeat(64), valid: true, stage: "core_check", message: "配置检查通过", exitCode: 0 };
    const wrapper = mount(ValidationErrors, { props: { validation, error: null } });
    expect(wrapper.get('[role="status"]').text()).toContain("核心检查：配置检查通过");
  });
  it("检查失败时显示按档案 ID 固定且保留的送检目录", async () => {
    const profileStore = profiles(); stores.push(profileStore); await profileStore.refresh(); await profileStore.open(profileId);
    const validation = {
      revision: 4,
      files: [{ name: "10-log.json", content: { log: {} } }, { name: "60-inbounds.json", content: { inbounds: [] } }],
      digest: "a".repeat(64),
      valid: false,
      stage: "core_check",
      message: "sing-box 配置检查失败",
      code: "singbox_check_failed",
      exitCode: 1,
      stderr: "FATAL decode 60-inbounds.json: inbounds[1]",
      command: ["data/bin/sing-box", "check", "-C", `config.staging-${profileId}`],
      configDirectory: `config.staging-${profileId}`
    };
    const api: ApplicationApi = { preview: vi.fn(), validate: vi.fn(async () => validation), apply: vi.fn() };
    const store = createApplicationStore(profileStore, api); await store.validate();

    const wrapper = mount(ValidationErrors, { props: { validation: store.validation.value, error: null } });
    expect(wrapper.text()).toContain("核心检查：sing-box 配置检查失败");
    expect(wrapper.text()).toContain("送检配置目录");
    expect(wrapper.text()).toContain(`config.staging-${profileId}`);
    expect(wrapper.text()).toContain("该目录会保留");
    expect(wrapper.text()).toContain("10-log.json, 60-inbounds.json");
    expect(wrapper.text()).toContain("revision 4");
    expect(wrapper.text()).toContain("FATAL decode 60-inbounds.json: inbounds[1]");
  });

  it("应用提交当前修订，并直接呈现检测失败", async () => {
    const profileStore = profiles(); stores.push(profileStore); await profileStore.refresh(); await profileStore.open(profileId);
    const failed = {
      revision: 4, files: [], digest: "a".repeat(64), valid: false, stage: "core_check",
      message: "sing-box 配置检查失败", code: "singbox_check_failed", exitCode: 7, stderr: "FATAL invalid"
    };
    const api: ApplicationApi = {
      preview: vi.fn(), validate: vi.fn(),
      apply: vi.fn(async () => ({ applied: false, profileId, revision: 4, digest: failed.digest, validation: failed }))
    };
    const store = createApplicationStore(profileStore, api);
    const result = await store.apply();
    expect(api.apply).toHaveBeenCalledWith(profileId, { revision: 4 });
    expect(result?.applied).toBe(false);
    expect(store.validation.value).toEqual(failed);
    expect(store.error.value).toBeNull();
  });

  it("应用成功后刷新档案状态", async () => {
    const profileStore = profiles(); stores.push(profileStore); await profileStore.refresh(); await profileStore.open(profileId);
    const success = { revision: 4, files: [], digest: "b".repeat(64), valid: true, stage: "core_check", message: "配置检查通过", exitCode: 0 };
    const api: ApplicationApi = {
      preview: vi.fn(), validate: vi.fn(),
      apply: vi.fn(async () => ({ applied: true, profileId, revision: 4, digest: success.digest, validation: success }))
    };
    const refresh = vi.spyOn(profileStore, "refresh");
    const store = createApplicationStore(profileStore, api);
    const result = await store.apply();
    expect(result?.applied).toBe(true);
    expect(refresh).toHaveBeenCalled();
    expect(store.validation.value?.valid).toBe(true);
  });

  it("配置复制冲突时显示服务端错误", async () => {
    const profileStore = profiles(); stores.push(profileStore); await profileStore.refresh(); await profileStore.open(profileId);
    const api: ApplicationApi = {
      preview: vi.fn(), validate: vi.fn(),
      apply: vi.fn(async () => { throw new ApiError(409, { code: "config_copy_conflict", message: "配置目录在复制期间发生变更，请重试" }); })
    };
    const store = createApplicationStore(profileStore, api);
    expect(await store.apply()).toBeNull();
    expect(store.error.value).toEqual({ code: "config_copy_conflict", message: "配置目录在复制期间发生变更，请重试", stage: undefined, fieldPath: undefined, output: "" });
  });
});
