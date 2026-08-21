import { mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import RuleBindingEditor from "@/features/rule-resources/RuleBindingEditor.vue";
import RuleResourceDialog from "@/features/rule-resources/RuleResourceDialog.vue";
import { createRuleResourceStore } from "@/features/rule-resources/store";
import type { RuleResource, RuleResourceInput } from "@/features/rule-resources/types";

const resource: RuleResource = {
  id: "a".repeat(32), name: "GeoSite CN", url: "https://example.test/cn.srs", format: "binary", updateInterval: "6h", disabled: false,
  localPath: "data/rulesets/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.srs", lastDigest: "b".repeat(64)
};

describe("规则资源", () => {
  it("创建输入永远不包含服务端只读 localPath", async () => {
    const wrapper = mount(RuleResourceDialog, { props: { open: true } });
    const inputs = wrapper.findAll("input");
    await inputs[0].setValue("GeoSite CN"); await inputs[1].setValue("https://example.test/cn.srs");
    await wrapper.get("form").trigger("submit");
    const submitted = wrapper.emitted("submit")?.[0]?.[0] as RuleResourceInput;
    expect(submitted).toEqual({ name: "GeoSite CN", url: "https://example.test/cn.srs", format: "binary", updateInterval: "", disabled: false });
    expect(JSON.stringify(submitted)).not.toContain("localPath");
  });

  it("更新期间锁定同一资源，并刷新服务端周期状态", async () => {
    let release: (() => void) | undefined;
    const api = {
      list: vi.fn(async () => [resource]), create: vi.fn(), patch: vi.fn(), remove: vi.fn(), references: vi.fn(),
      update: vi.fn(async () => await new Promise<{ changed: boolean; digest: string; message: string }>((resolve) => { release = () => resolve({ changed: true, digest: resource.lastDigest!, message: "规则资源已更新，请点击“应用" }); }))
    };
    const store = createRuleResourceStore(api); await store.refresh(); const pending = store.update(resource.id);
    expect(store.updatingId.value).toBe(resource.id); release?.(); await pending;
    expect(store.updatingId.value).toBeNull(); expect(api.list).toHaveBeenCalledTimes(2);
  });

  it("绑定只产出 resourceId、tag、enabled，路径只读", async () => {
    const wrapper = mount(RuleBindingEditor, { props: { bindings: [], resources: [resource] } });
    await wrapper.get("button").trigger("click");
    expect(wrapper.emitted("change")?.[0]?.[0]).toEqual([{ resourceId: resource.id, tag: "GeoSite CN", enabled: true }]);
    expect(wrapper.text()).not.toContain("download_detour");
  });
});
