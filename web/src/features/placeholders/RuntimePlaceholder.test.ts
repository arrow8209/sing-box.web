import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import RuntimePlaceholderView from "./RuntimePlaceholderView.vue";
import { useCapabilitiesStore } from "../../stores/capabilities";

describe("RuntimePlaceholderView", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it("renders 尚未接入 without fake metrics", () => {
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

    const wrapper = mount(RuntimePlaceholderView);

    expect(wrapper.text()).toContain("尚未接入");
    expect(wrapper.text()).toContain("核心状态与生命周期控制将在二期接入");
    expect(wrapper.text()).not.toContain("MB/s");
    expect(wrapper.find('button[name="启动核心"]').exists()).toBe(false);
    expect(wrapper.find('button[name="重启核心"]').exists()).toBe(false);
  });
});
