import { mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/api/client";
import ValidationErrors from "@/features/application/ValidationErrors.vue";
import { createApplicationStore } from "@/features/application/store";
import type { ApplicationApi } from "@/features/application/api";
import { createProfileStore } from "@/features/profiles/store";
import type { ProfileApi } from "@/features/profiles/api";
import type { Draft, ProfileSummary } from "@/features/profiles/types";

const draft = (): Draft => ({ profileId: "profile-a", revision: 4, config: {}, buffers: { log: "{}" }, bindings: [], savedAt: "2026-08-21T00:00:00Z" });
const profile: ProfileSummary = { id: "profile-a", name: "A", draftRevision: 4, appliedRevision: null, appliedDigest: "", appliedAt: null, needsApply: true, lastValidation: null };
const core = () => ({ installedVersion: "1.0", installedDigest: "a", state: "running" as const, currentProfileId: "", appliedRevision: null, lastError: "", operation: { active: false } });
function profiles(): ReturnType<typeof createProfileStore> {
  const api: ProfileApi = { list: vi.fn(async () => [profile]), create: vi.fn(), rename: vi.fn(), remove: vi.fn(), getDraft: vi.fn(async () => ({ data: draft(), etag: "x" })), saveDraft: vi.fn() };
  return createProfileStore(api);
}

describe("配置应用", () => {
  const stores: ReturnType<typeof createProfileStore>[] = [];
  afterEach(() => stores.splice(0).forEach((store) => store.dispose()));
  it("未保存草稿时禁止应用", async () => {
    const profileStore = profiles(); stores.push(profileStore); await profileStore.refresh(); await profileStore.open("profile-a"); profileStore.mutate(() => undefined);
    const api: ApplicationApi = { core: vi.fn(async () => core()), preview: vi.fn(), validate: vi.fn(), apply: vi.fn() };
    const store = createApplicationStore(profileStore, api); await store.refreshCore();
    expect(store.canApply.value).toBe(false);
  });
  it("预览仅呈现模块内容，漂移确认只重发服务器摘要", async () => {
    const profileStore = profiles(); stores.push(profileStore); await profileStore.refresh(); await profileStore.open("profile-a");
    const digest = "b".repeat(64);
    const api: ApplicationApi = { core: vi.fn(async () => core()), preview: vi.fn(async () => ({ revision: 4, files: [{ name: "80-route.json", content: { route: { rule_set: [{ path: "data/rulesets/resource-a.srs" }] } } }], digest: "a".repeat(64) })), validate: vi.fn(), apply: vi.fn(async (_id, body) => { if (!body.confirmedConfigDigest) throw new ApiError(409, { code: "config_external_drift", message: "漂移", details: { confirmationDigest: digest } }); return {}; }) };
    const store = createApplicationStore(profileStore, api); await store.refreshCore(); await store.preview();
    expect(JSON.stringify(store.previewResult.value)).toContain("data/rulesets/resource-a.srs"); expect(JSON.stringify(store.previewResult.value)).not.toContain("https://");
    await store.apply(); expect(store.driftDigest.value).toBe(digest); await store.apply(store.driftDigest.value ?? undefined);
    expect(api.apply).toHaveBeenLastCalledWith("profile-a", { revision: 4, confirmedConfigDigest: digest });
  });
  it("应用检查失败时保留核心输出和后台审核位置", async () => {
    const profileStore = profiles(); stores.push(profileStore); await profileStore.refresh(); await profileStore.open("profile-a");
    const details = {
      command: ["data/bin/sing-box", "check", "-C", "config.staging-token"],
      checkedConfigDirectory: "config.staging-token",
      reviewCommand: ["data/bin/sing-box", "check", "-C", "config.check-failed"],
      reviewConfigDirectory: "config.check-failed",
      configFiles: ["10-log.json", "60-inbounds.json"],
      profileId: "profile-a",
      revision: 4,
      configDigest: "a".repeat(64),
      exitCode: 1,
      timedOut: false,
      outputTruncated: false,
      stderr: "FATAL decode 60-inbounds.json: inbounds[1]"
    };
    const api: ApplicationApi = {
      core: vi.fn(async () => core()), preview: vi.fn(), validate: vi.fn(),
      apply: vi.fn(async () => { throw new ApiError(422, { code: "config_check_failed", message: "sing-box 配置检查失败（退出码 1）", stage: "check_config", details }); })
    };
    const store = createApplicationStore(profileStore, api); await store.refreshCore(); await store.apply();
    expect(store.error.value?.details).toMatchObject({ reviewConfigDirectory: "config.check-failed", exitCode: 1, stderr: details.stderr });

    const wrapper = mount(ValidationErrors, { props: { validation: null, error: store.error.value } });
    expect(wrapper.text()).toContain("检查配置：sing-box 配置检查失败（退出码 1）");
    expect(wrapper.text()).toContain("后台审核目录");
    expect(wrapper.text()).toContain("config.check-failed");
    expect(wrapper.text()).toContain("10-log.json, 60-inbounds.json");
    expect(wrapper.text()).toContain("档案 ID profile-a");
    expect(wrapper.text()).toContain("revision 4");
    expect(wrapper.text()).toContain("FATAL decode 60-inbounds.json: inbounds[1]");
  });
});
