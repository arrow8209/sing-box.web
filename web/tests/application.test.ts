import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/api/client";
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
});
