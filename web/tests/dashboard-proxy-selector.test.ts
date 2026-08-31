import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import DashboardProxySelector from "@/features/dashboard/DashboardProxySelector.vue";

describe("仪表盘节点选择", () => {
  it("按实际引用顺序显示整个订阅的全部节点和两个单独出站", () => {
    const subscriptionTags = ["🇭🇰 香港 01", "🇭🇰 香港 02", "🇭🇰 香港 03", "🇭🇰 香港 04"];
    const individualTags = ["新加坡 01", "日本 01"];
    const runtimeTags = [...subscriptionTags, ...individualTags];
    const outbounds = [
      { type: "selector", tag: "🚀 节点选择", outbounds: runtimeTags },
      ...subscriptionTags.map((tag) => ({ type: "anytls", tag })),
      { type: "trojan", tag: individualTags[0] },
      { type: "vless", tag: individualTags[1] }
    ];
    const editorState = {
      version: 1,
      groups: [{
        id: "node-selector",
        tag: "🚀 节点选择",
        type: "selector",
        hidden: false,
        include: "",
        exclude: "",
        icon: "",
        references: [
          { kind: "subscription", subscriptionId: "subscription-a", tag: "家庭订阅", resolvedTags: subscriptionTags },
          { kind: "subscription-node", subscriptionId: "subscription-b", nodeTag: individualTags[0], resolvedTags: [individualTags[0]] },
          { kind: "subscription-node", subscriptionId: "subscription-b", nodeTag: individualTags[1], resolvedTags: [individualTags[1]] }
        ],
        runtimeTags
      }],
      generatedTags: runtimeTags
    };

    const wrapper = mount(DashboardProxySelector, { props: { outbounds, editorState } });
    const group = wrapper.get('[data-proxy-group-tag="🚀 节点选择"]');
    const members = group.findAll(".proxy-node-card");

    expect(members).toHaveLength(6);
    expect(members.map((member) => member.attributes("data-proxy-member-tag"))).toEqual(runtimeTags);
    expect(group.text()).toContain("🇭🇰 香港 04");
    expect(members[0]!.get(".proxy-node-avatar").text()).toBe("🇭🇰");
    expect(group.text()).toContain("新加坡 01");
    expect(group.text()).toContain("日本 01");
    expect(wrapper.text()).toContain("6 个节点已配置");
    expect(wrapper.text()).not.toContain("nas-ss");
  });
});
