import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import DashboardView from "./DashboardView.vue";
import { useProfilesStore } from "../../stores/profiles";
import { useDraftStore } from "../../stores/draft";
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
  lastValidation: {
    revision: 3,
    status: "valid",
    stage: "check",
    message: "",
    checkedAt: "2026-08-18T10:00:00Z",
  },
  directoryHint: "/etc/sing-box/home-gateway",
  checkCommand: "sing-box check -C /etc/sing-box/home-gateway",
  draftReadOnly: false,
  draftError: "",
};

describe("DashboardView", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it("renders health cards without fake runtime data", () => {
    const profiles = useProfilesStore();
    profiles.items = [profile];
    profiles.activeId = profile.id;
    const draft = useDraftStore();
    draft.status = "saved";
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
      singBox: {
        available: true,
        version: "1.12.0",
        targetVersion: "1.12.0",
        warning: "",
      },
    };

    const wrapper = mount(DashboardView);

    expect(wrapper.text()).toContain("家庭网关");
    expect(wrapper.text()).toContain("草稿已保存");
    expect(wrapper.text()).toContain("1.12.0");
    // 无模拟运行数据。
    expect(wrapper.text()).not.toContain("MB/s");
    expect(wrapper.text()).not.toContain("连接数");
    expect(wrapper.find('button[name="启动核心"]').exists()).toBe(false);
    expect(wrapper.find('button[name="重启核心"]').exists()).toBe(false);
  });

  it("shows target and installed versions from capabilities", () => {
    const profiles = useProfilesStore();
    profiles.items = [profile];
    profiles.activeId = profile.id;
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
      singBox: {
        available: true,
        version: "1.11.9",
        targetVersion: "1.12.0",
        warning: "sing-box 版本不匹配：当前 1.11.9，目标 1.12.0",
      },
    };

    const wrapper = mount(DashboardView);
    expect(wrapper.text()).toContain("1.11.9");
    expect(wrapper.text()).toContain("1.12.0");
  });

  it("shows an empty state without profiles", () => {
    const wrapper = mount(DashboardView);
    expect(wrapper.text()).toContain("尚未创建配置档案");
    expect(wrapper.text()).not.toContain("MB/s");
  });
});
