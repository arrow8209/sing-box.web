import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import DnsConfigPanel from "@/features/config-editor/DnsConfigPanel.vue";
import OutboundConfigPanel from "@/features/config-editor/OutboundConfigPanel.vue";
import RouteConfigPanel from "@/features/config-editor/RouteConfigPanel.vue";
import RouteRuleEditorDialog from "@/features/config-editor/RouteRuleEditorDialog.vue";
import { restoreOutboundEditorState } from "@/features/config-editor/outbound";
import type { RouteRuleEditorResult } from "@/features/config-editor/route";
import type { Draft } from "@/features/profiles/types";
import { useRuleResourceStore } from "@/features/rule-resources/store";
import { subscriptionApi } from "@/features/subscriptions/api";
import type { SubscriptionDetail } from "@/features/subscriptions/types";

const subscription: SubscriptionDetail = {
  id: "a".repeat(32),
  name: "测试订阅",
  outbounds: [
    { type: "trojan", tag: "订阅代理节点", server: "node.example.test" },
    { type: "direct", tag: "订阅直连节点", bind_interface: "lo" }
  ],
  updatedAt: "2026-09-10T00:00:00Z"
};
const outbounds = [
  { type: "selector", tag: "节点选择", outbounds: ["自动选择", "全局直连"] },
  { type: "urltest", tag: "自动选择", outbounds: ["订阅代理节点", "订阅直连节点"] },
  ...subscription.outbounds,
  { type: "direct", tag: "全局直连" },
  { type: "block", tag: "全局拦截", hidden: true },
  { type: "selector", tag: "GLOBAL", outbounds: ["节点选择"] },
  { type: "direct", tag: "direct" },
  { type: "block", tag: "block" },
  { type: "future-protocol", tag: "JSON 自定义节点" }
];
const configuredTags = ["节点选择", "自动选择", "全局直连", "全局拦截", "GLOBAL"];

beforeEach(() => {
  vi.spyOn(useRuleResourceStore(), "refresh").mockResolvedValue();
  vi.spyOn(subscriptionApi, "list").mockResolvedValue([{
    id: subscription.id, name: subscription.name, nodeCount: 2,
    protocols: ["trojan", "direct"], updatedAt: subscription.updatedAt
  }]);
  vi.spyOn(subscriptionApi, "get").mockResolvedValue(subscription);
});
afterEach(() => { vi.restoreAllMocks(); });

describe("出站选项与出站设置对应", () => {
  it.each(["导入配置", "已保存配置", "订阅读取失败"])("%s 的路由和 DNS 选项与出站卡片同名同序", async (scenario) => {
    const draft: Draft = {
      profileId: "b".repeat(32), revision: 1,
      config: { outbounds, route: { final: "节点选择" } },
      buffers: {}, bindings: [], savedAt: "2026-09-10T00:00:00Z"
    };
    if (scenario !== "导入配置") draft.editorState = { outbounds: restoreOutboundEditorState(outbounds, [subscription]) };
    if (scenario === "订阅读取失败") vi.mocked(subscriptionApi.get).mockRejectedValue(new Error("订阅暂不可用"));
    const outboundPanel = mount(OutboundConfigPanel, { props: { draft } });
    const routePanel = mount(RouteConfigPanel, { props: { draft } });
    const dnsPanel = mount(DnsConfigPanel, { props: { draft } });
    await flushPromises();

    const cardTags = outboundPanel.findAll(".outbound-card").map((card) => card.attributes("data-outbound-tag"));
    expect(cardTags).toEqual(configuredTags);
    expect(routePanel.get('[aria-label="默认出站标签"]').findAll("option").map((option) => option.attributes("value")).filter(Boolean)).toEqual(cardTags);
    await routePanel.get(".add-button").trigger("click");
    expect(routePanel.get('[aria-label="目标出站"]').findAll("option").map((option) => option.attributes("value")).filter(Boolean)).toEqual(cardTags);
    await dnsPanel.findAll(".add-button")[0]!.trigger("click");
    expect(dnsPanel.get('select[name="detour"]').findAll("option").map((option) => option.attributes("value")).filter(Boolean)).toEqual(cardTags);

    routePanel.findComponent(RouteRuleEditorDialog).vm.$emit("close");
    await flushPromises();
    const nextDraft = {
      ...draft,
      config: {
        ...draft.config,
        outbounds: [
          { type: "selector", tag: "新名称", outbounds: ["全局直连"] },
          { type: "direct", tag: "全局直连" }
        ],
        route: { final: "新名称" }
      }
    };
    for (const panel of [outboundPanel, routePanel, dnsPanel]) await panel.setProps({ draft: nextDraft });
    await routePanel.get(".add-button").trigger("click");
    expect(routePanel.get('[aria-label="目标出站"]').findAll("option").map((option) => option.attributes("value")).filter(Boolean)).toEqual(["新名称", "全局直连"]);
    expect(dnsPanel.get('select[name="detour"]').findAll("option").map((option) => option.attributes("value")).filter(Boolean)).toEqual(["新名称", "全局直连"]);
    for (const panel of [outboundPanel, routePanel, dnsPanel]) panel.unmount();
  });

  it.each([{ tags: configuredTags }, { tags: [] }])("新建规则不会把列表以外的默认出站带入选项：$tags", ({ tags }) => {
    const wrapper = mount(RouteRuleEditorDialog, {
      props: { open: true, rule: null, options: [], outbounds: tags, defaultOutbound: "订阅代理节点" }
    });
    const select = wrapper.get<HTMLSelectElement>('[aria-label="目标出站"]');
    expect(select.findAll("option").map((option) => option.attributes("value")).filter(Boolean)).toEqual(tags);
    expect(select.element.value).toBe(tags[0] ?? "");
    wrapper.unmount();
  });

  it("编辑旧规则时保留其直接引用的节点及原始值", async () => {
    const rule = { domain_suffix: ["example.test"], action: "route", outbound: "订阅代理节点" };
    const wrapper = mount(RouteRuleEditorDialog, {
      props: { open: true, rule, options: [], outbounds: configuredTags, defaultOutbound: "节点选择" }
    });
    expect(wrapper.get<HTMLSelectElement>('[aria-label="目标出站"]').element.value).toBe("订阅代理节点");
    await wrapper.get("form").trigger("submit");
    expect((wrapper.emitted("submit")![0]![0] as RouteRuleEditorResult).rule).toEqual(rule);
    wrapper.unmount();
  });

  it("订阅加载后新建规则的选项和默认值随出站列表更新", async () => {
    const wrapper = mount(RouteRuleEditorDialog, {
      props: { open: true, rule: null, options: [], outbounds: ["订阅直连节点", ...configuredTags], defaultOutbound: "订阅直连节点" }
    });
    await wrapper.setProps({ outbounds: configuredTags });
    const select = wrapper.get<HTMLSelectElement>('[aria-label="目标出站"]');
    expect(select.findAll("option").map((option) => option.attributes("value")).filter(Boolean)).toEqual(configuredTags);
    expect(select.element.value).toBe("节点选择");
    wrapper.unmount();
  });
});
