import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import ConfigTabs from "@/features/config-editor/ConfigTabs.vue";
import OutboundJsonEditor from "@/features/config-editor/OutboundJsonEditor.vue";

describe("配置编辑器", () => {
  it("始终显示八个固定标签", () => {
    const wrapper = mount(ConfigTabs, { props: { active: "overview" } });
    expect(wrapper.findAll("button")).toHaveLength(8);
    expect(wrapper.text()).toContain("高级");
  });

  it("手工粘贴出站 JSON 原样回传，不遮罩也不丢字段", async () => {
    const node = [{ type: "hysteria2", tag: "新加坡 01", server: "sg.example.test", server_port: 443, password: "visible-test-value", future: { x: 1 } }];
    const wrapper = mount(OutboundJsonEditor, { props: { text: "[]" } });
    await wrapper.get("[data-outbound-json]").setValue(JSON.stringify(node, null, 2));
    await wrapper.get("[data-save-module]").trigger("click");
    expect(wrapper.emitted("save")?.[0]).toEqual([JSON.stringify(node, null, 2)]);
    expect((wrapper.get("[data-outbound-json]").element as HTMLTextAreaElement).value).toContain("visible-test-value");
    expect(wrapper.text()).not.toContain("订阅");
  });
});
