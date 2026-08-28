import { mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/api/client";
import ApplicationBar from "@/features/application/ApplicationBar.vue";
import ValidationErrors from "@/features/application/ValidationErrors.vue";
import { createApplicationStore } from "@/features/application/store";
import type { ApplicationApi } from "@/features/application/api";
import { createProfileStore } from "@/features/profiles/store";
import type { ProfileApi } from "@/features/profiles/api";
import type { Draft, ProfileSummary } from "@/features/profiles/types";

const profileId = "0123456789abcdef0123456789abcdef";
const draft = (): Draft => ({ profileId, revision: 4, config: {}, buffers: { log: "{}" }, bindings: [], savedAt: "2026-08-21T00:00:00Z" });
const profile: ProfileSummary = { id: profileId, name: "A", draftRevision: 4, appliedRevision: null, appliedDigest: "", appliedAt: null, needsApply: true, lastValidation: null };
function profiles(): ReturnType<typeof createProfileStore> {
  const api: ProfileApi = { list: vi.fn(async () => [profile]), create: vi.fn(), duplicate: vi.fn(), rename: vi.fn(), remove: vi.fn(), getDraft: vi.fn(async () => ({ data: draft(), etag: "x" })), saveDraft: vi.fn() };
  return createProfileStore(api);
}

describe("配置应用", () => {
  const stores: ReturnType<typeof createProfileStore>[] = [];
  afterEach(() => stores.splice(0).forEach((store) => store.dispose()));
  it("预览和检查组件不混入档案操作", async () => {
    const profileStore = profiles(); stores.push(profileStore); await profileStore.refresh(); await profileStore.open(profileId);
    const wrapper = mount(ApplicationBar);
    expect(wrapper.find("[data-apply]").exists()).toBe(false);
    expect(wrapper.findAll("button").map((button) => button.text())).toEqual(["预览", "检查"]);
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
