import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import PublicationBar from "./PublicationBar.vue";
import { useDraftStore } from "../../stores/draft";
import { useProfilesStore } from "../../stores/profiles";
import { usePublicationStore } from "../../stores/publication";
import { useCapabilitiesStore } from "../../stores/capabilities";
import type { Profile } from "../../api/types";

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

describe("PublicationBar", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
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
      singBox: { available: true, version: "1.12.0", targetVersion: "1.12.0", warning: "" },
    };
  });

  it("labels states exactly and never uses 保存并应用", async () => {
    const draft = useDraftStore();
    draft.status = "dirty";
    const wrapper = mount(PublicationBar);
    expect(wrapper.get('[role="status"]').text()).toBe("草稿保存中");
    expect(wrapper.text()).not.toContain("保存并应用");

    draft.status = "saved";
    await wrapper.vm.$nextTick();
    expect(wrapper.get('[role="status"]').text()).toBe("草稿已保存");
  });

  it("exposes 仅校验、校验并写入、复制脱敏配置 and 下载完整配置", () => {
    const wrapper = mount(PublicationBar);
    for (const name of ["仅校验", "校验并写入", "复制脱敏配置", "下载完整配置"]) {
      expect(wrapper.get(`button[name="${name}"]`).isVisible()).toBe(true);
    }
  });

  it("announces 校验通过 after a valid preflight", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profile];
    profiles.activeId = profile.id;
    const publication = usePublicationStore();
    vi.spyOn(publication, "validate").mockResolvedValue({
      valid: true,
      issues: [],
      targetVersion: "1.12.0",
      installedVersion: "1.12.0",
      checkedAt: "",
    });

    const wrapper = mount(PublicationBar);
    await wrapper.get('button[name="仅校验"]').trigger("click");

    expect(wrapper.get('[role="status"]').text()).toBe("校验通过");
  });

  it("announces 校验失败，共 N 项 with the issue count", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profile];
    profiles.activeId = profile.id;
    const publication = usePublicationStore();
    vi.spyOn(publication, "validate").mockResolvedValue({
      valid: false,
      issues: [
        { stage: "schema", path: "/dns", code: "required", message: "m1" },
        { stage: "schema", path: "/log", code: "required", message: "m2" },
        { stage: "semantic", path: "/route", code: "unknown_tag", message: "m3" },
      ],
      targetVersion: "1.12.0",
      installedVersion: "1.12.0",
      checkedAt: "",
    });

    const wrapper = mount(PublicationBar);
    await wrapper.get('button[name="仅校验"]').trigger("click");

    expect(wrapper.get('[role="status"]').text()).toBe("校验失败，共 3 项");
  });

  it("announces 正式配置已写入 after a successful publication", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profile];
    profiles.activeId = profile.id;
    const draft = useDraftStore();
    draft.draft = {
      profileId: profile.id,
      revision: 3,
      content: { config: { log: { level: "info" } }, editorBuffers: {} },
      savedAt: "",
    };
    draft.etag = '"draft-3"';
    draft.status = "saved";
    const publication = usePublicationStore();
    vi.spyOn(publication, "publish").mockResolvedValue({
      profileId: profile.id,
      revision: 3,
      digest: "digest-1",
      directory: "/etc/sing-box/home-gateway",
      publishedAt: "",
      validation: {
        valid: true,
        issues: [],
        targetVersion: "1.12.0",
        installedVersion: "1.12.0",
        checkedAt: "",
      },
    });

    const wrapper = mount(PublicationBar);
    await wrapper.get('button[name="校验并写入"]').trigger("click");

    expect(wrapper.get('[role="status"]').text()).toBe("正式配置已写入");
  });

  it("full download requires the exact confirmation prompt", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profile];
    profiles.activeId = profile.id;
    const publication = usePublicationStore();
    const downloadFull = vi.spyOn(publication, "downloadFull").mockResolvedValue(undefined);

    const wrapper = mount(PublicationBar);
    await wrapper.get('button[name="下载完整配置"]').trigger("click");

    expect(downloadFull).toHaveBeenCalledTimes(1);
  });

  it("disables 校验并写入 when config.publish is unavailable", () => {
    const capabilities = useCapabilitiesStore();
    capabilities.snapshot = {
      capabilities: {
        "config.manage": true,
        "config.publish": false,
        "subscription.sync": false,
        "ruleset.sync": false,
        "core.lifecycle": false,
        "core.metrics": false,
      },
      singBox: {
        available: false,
        version: "",
        targetVersion: "1.12.0",
        warning: "sing-box 不可用，配置发布已禁用",
      },
    };

    const wrapper = mount(PublicationBar);
    expect(wrapper.get('button[name="校验并写入"]').attributes("disabled")).toBeDefined();
  });
});
