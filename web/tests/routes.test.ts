import { mount } from "@vue/test-utils";
import { afterEach, describe, expect, it } from "vitest";
import RouteConfigPanel from "@/features/config-editor/RouteConfigPanel.vue";
import RouteRuleEditorDialog from "@/features/config-editor/RouteRuleEditorDialog.vue";
import {
  buildRouteRuleSetOptions,
  insertRouteRuleText,
  mergeSelectedRuleBindings,
  parseRouteRuleText,
  removeRouteRule,
  reorderRouteRules,
  restoreRouteEditorState,
  updateRouteBoolean,
  updateRouteDefaultDomainResolver,
  updateRouteFinal,
  upsertRouteRule,
  type RoutePanelSave,
  type RouteRuleEditorResult
} from "@/features/config-editor/route";
import type { Draft, RuleBinding } from "@/features/profiles/types";
import { useRuleResourceStore } from "@/features/rule-resources/store";
import type { RuleResource } from "@/features/rule-resources/types";

const resources: RuleResource[] = [
  {
    id: "a".repeat(32), sourceType: "remote", name: "GeoSite-CN", url: "https://example.test/cn.srs", format: "binary", updateInterval: "24h", disabled: false,
    localPath: `data/rulesets/${"a".repeat(32)}.srs`, lastDigest: "1".repeat(64)
  },
  {
    id: "b".repeat(32), sourceType: "manual", name: "Private-Domains", url: "", format: "source", updateInterval: "", disabled: false,
    localPath: `data/rulesets/${"b".repeat(32)}.json`, lastDigest: "2".repeat(64)
  },
  {
    id: "c".repeat(32), sourceType: "remote", name: "Disabled-Rules", url: "https://example.test/disabled.srs", format: "binary", updateInterval: "24h", disabled: true,
    localPath: `data/rulesets/${"c".repeat(32)}.srs`
  }
];

const bindings: RuleBinding[] = [{ resourceId: resources[0]!.id, tag: "GeoSite-CN-Bound", enabled: false }];
const route = {
  auto_detect_interface: true,
  default_domain_resolver: { server: "Local-DNS" },
  final: "节点选择",
  rule_set: [{ type: "inline", tag: "Inline-Private", rules: [] }],
  rules: [{ action: "route", rule_set: ["GeoSite-CN-Bound", "Legacy-Tag"], outbound: "节点选择" }]
};

function draftFixture(routeValue: Record<string, unknown> = route): Draft {
  return {
    profileId: "d".repeat(32), revision: 1,
    config: {
      route: routeValue,
      dns: { servers: [{ type: "udp", tag: "Local-DNS", server: "223.5.5.5" }] },
      inbounds: [{ type: "mixed", tag: "mixed-in" }],
      outbounds: [{ type: "selector", tag: "节点选择", outbounds: ["direct"] }, { type: "direct", tag: "direct" }]
    },
    buffers: { route: JSON.stringify(routeValue, null, 2) }, bindings, savedAt: "2026-08-26T00:00:00Z"
  };
}

afterEach(() => {
  useRuleResourceStore().resources.value = [];
});

describe("自定义配置路由", () => {
  it("新增规则默认提供全部受管、配置内及当前引用规则集", () => {
    const options = buildRouteRuleSetOptions(route, bindings, resources);

    expect(options.map((option) => option.name)).toEqual([
      "Inline-Private",
      "GeoSite-CN",
      "Private-Domains",
      "Disabled-Rules",
      "Legacy-Tag"
    ]);
    expect(options.find((option) => option.name === "GeoSite-CN")?.tag).toBe("GeoSite-CN-Bound");
    expect(options.find((option) => option.name === "Disabled-Rules")?.disabled).toBe(true);
  });

  it("选择未绑定规则集时自动新增绑定，已有绑定会直接启用", () => {
    const options = buildRouteRuleSetOptions(route, bindings, resources);
    const next = mergeSelectedRuleBindings(bindings, options, [resources[0]!.id, resources[1]!.id]);

    expect(next).toEqual([
      { resourceId: resources[0]!.id, tag: "GeoSite-CN-Bound", enabled: true },
      { resourceId: resources[1]!.id, tag: "Private-Domains", enabled: true }
    ]);
  });

  it("增删和重排规则时保留未编辑的大整数 token", () => {
    expect(JSON.parse(upsertRouteRule("{}", 0, { action: "route", rule_set: ["GeoSite-CN"], outbound: "节点选择" }))).toEqual({
      rules: [{ action: "route", rule_set: ["GeoSite-CN"], outbound: "节点选择" }]
    });
    const source = `{"future":9007199254740993,"rules":[{"protocol":"dns","future":9007199254740995}]}`;
    const added = upsertRouteRule(source, 1, { action: "route", rule_set: ["GeoSite-CN"], outbound: "节点选择" });

    expect(added).toContain("9007199254740993");
    expect(added).toContain("9007199254740995");
    expect((JSON.parse(added) as { rules: unknown[] }).rules).toHaveLength(2);

    const reordered = reorderRouteRules(added, [1, 0]);
    expect(reordered).toContain("9007199254740993");
    expect(reordered).toContain("9007199254740995");
    expect((JSON.parse(reordered) as { rules: Array<Record<string, unknown>> }).rules[0]?.rule_set).toEqual(["GeoSite-CN"]);

    const removed = removeRouteRule(reordered, 0);
    expect(removed).toContain("9007199254740993");
    expect(removed).toContain("9007199254740995");
    expect((JSON.parse(removed) as { rules: unknown[] }).rules).toHaveLength(1);

    const inserted = insertRouteRuleText(source, 0, '{"domain_suffix":["first.example"]}');
    const insertedRules = (JSON.parse(inserted) as { rules: Array<Record<string, unknown>> }).rules;
    expect(insertedRules).toHaveLength(2);
    expect(insertedRules[0]?.domain_suffix).toEqual(["first.example"]);
    expect(insertedRules[1]?.protocol).toBe("dns");
    expect(inserted).toContain("9007199254740995");
  });

  it("通用设置只修改目标路径并保留解析器对象中的未知字段", () => {
    const source = `{"future":9007199254740993,"default_domain_resolver":{"server":"dns-a","future":9007199254740995}}`;
    const dnsChanged = updateRouteDefaultDomainResolver(source, { default_domain_resolver: { server: "dns-a", future: 1 } }, "dns-b");
    const enabled = updateRouteBoolean(dnsChanged, "find_process", true);
    const disabled = updateRouteBoolean(enabled, "find_process", false);
    const finalChanged = updateRouteFinal(disabled, "direct");

    expect(finalChanged).toContain("9007199254740993");
    expect(finalChanged).toContain("9007199254740995");
    expect(JSON.parse(finalChanged)).toMatchObject({ default_domain_resolver: { server: "dns-b" }, final: "direct" });
    expect(finalChanged).not.toContain("find_process");
  });

  it("恢复规则编辑状态时校正插入点和停用规则位置", () => {
    const state = restoreRouteEditorState({
      version: 1,
      insertionPosition: 99,
      disabledRules: [{ id: "disabled-a", position: 1, text: '{"protocol":"quic","action":"reject"}' }]
    }, 2);

    expect(state).toEqual({
      version: 1,
      insertionPosition: 3,
      disabledRules: [{ id: "disabled-a", position: 1, text: '{"protocol":"quic","action":"reject"}' }]
    });
  });

  it("新增规则弹窗默认展示所有规则集并提交资源引用", async () => {
    const options = buildRouteRuleSetOptions(route, bindings, resources);
    const wrapper = mount(RouteRuleEditorDialog, {
      props: { open: true, rule: null, options, inbounds: ["mixed-in"], outbounds: ["节点选择", "direct"], dnsServers: ["Local-DNS"], defaultOutbound: "节点选择" }
    });

    expect(wrapper.findAll(".ruleset-option")).toHaveLength(options.length);
    expect(wrapper.findAll('input[name="route-action"]')).toHaveLength(6);
    expect(wrapper.get('[aria-label="规则类型"]').findAll("option")).toHaveLength(20);
    expect(wrapper.text()).toContain("全部规则集");
    expect(wrapper.text()).toContain("Disabled-Rules");
    expect(wrapper.get('[aria-label="选择规则集 Disabled-Rules"]').attributes("disabled")).toBeDefined();

    await wrapper.get('[aria-label="选择规则集 Private-Domains"]').setValue(true);
    await wrapper.get("form").trigger("submit");
    const result = wrapper.emitted("submit")?.[0]?.[0] as RouteRuleEditorResult | undefined;
    expect(result).toMatchObject({
      rule: { action: "route", rule_set: ["Private-Domains"], outbound: "节点选择" },
      resourceIds: [resources[1]!.id]
    });
    expect(JSON.parse(String(result?.text))).toEqual(result?.rule);
  });

  it("规则弹窗支持原型中的匹配类型、拒绝动作并保留原始大整数", async () => {
    const raw = `{"future":9007199254740993,"protocol":"dns","action":"route","outbound":"direct"}`;
    const wrapper = mount(RouteRuleEditorDialog, {
      props: {
        open: true,
        rule: parseRouteRuleText(raw),
        ruleText: raw,
        options: [],
        inbounds: ["mixed-in"],
        outbounds: ["direct", "block"],
        dnsServers: ["Local-DNS"],
        defaultOutbound: "direct"
      }
    });

    await wrapper.get('[aria-label="规则类型"]').setValue("domain_suffix");
    await wrapper.get('[aria-label="匹配内容"]').setValue("example.com,example.org");
    await wrapper.get('input[name="route-action"][value="reject"]').setValue(true);
    await wrapper.get('input[value="drop"]').setValue(true);
    await wrapper.get("form").trigger("submit");

    const result = wrapper.emitted("submit")?.[0]?.[0] as RouteRuleEditorResult | undefined;
    expect(result?.rule).toMatchObject({ domain_suffix: ["example.com", "example.org"], action: "reject", method: "drop" });
    expect(result?.rule).not.toHaveProperty("protocol");
    expect(result?.rule).not.toHaveProperty("outbound");
    expect(result?.text).toContain("9007199254740993");
  });

  it("规则弹窗按 sing-box 字段映射路由选项、劫持、嗅探和解析动作", async () => {
    const cases: Array<Record<string, unknown>> = [
      { domain_suffix: ["options.example"], action: "route-options", override_address: "1.1.1.1" },
      { protocol: "dns", action: "hijack-dns" },
      { inbound: "mixed-in", action: "sniff", sniffer: ["http", "tls"] },
      { domain: ["resolve.example"], action: "resolve", strategy: "prefer_ipv4", server: "Local-DNS" }
    ];

    for (const expected of cases) {
      const raw = JSON.stringify(expected);
      const wrapper = mount(RouteRuleEditorDialog, {
        props: {
          open: true,
          rule: parseRouteRuleText(raw),
          ruleText: raw,
          options: [],
          inbounds: ["mixed-in"],
          outbounds: ["direct"],
          dnsServers: ["Local-DNS"],
          defaultOutbound: "direct"
        }
      });

      await wrapper.get("form").trigger("submit");
      const result = wrapper.emitted("submit")?.[0]?.[0] as RouteRuleEditorResult | undefined;
      expect(result?.rule).toMatchObject(expected);
      wrapper.unmount();
    }
  });

  it("路由页呈现通用设置、插入点和整合后的规则集入口", async () => {
    useRuleResourceStore().resources.value = resources;
    const wrapper = mount(RouteConfigPanel, { props: { draft: draftFixture() } });

    expect(wrapper.text()).toContain("通用设置");
    expect(wrapper.text()).not.toContain("Web 规则资源绑定");
    expect(wrapper.text()).not.toContain("添加绑定");
    expect(wrapper.find(".bindings").exists()).toBe(false);
    expect(wrapper.findAll(".route-rule-row")).toHaveLength(1);
    expect(wrapper.find(".route-insertion-point").exists()).toBe(true);
    await wrapper.get(".add-button").trigger("click");
    expect(wrapper.get("dialog").attributes("aria-label")).toBe("新增规则");
    expect(wrapper.findAll(".ruleset-option")).toHaveLength(5);
  });

  it("停用规则会移出实际 JSON 并保存在编辑状态", async () => {
    useRuleResourceStore().resources.value = resources;
    const wrapper = mount(RouteConfigPanel, { props: { draft: draftFixture() } });

    await wrapper.get(".rule-switch input").setValue(false);
    const result = wrapper.emitted("save")?.[0]?.[0] as RoutePanelSave | undefined;
    expect((JSON.parse(String(result?.text)) as { rules: unknown[] }).rules).toHaveLength(0);
    expect(result?.text).not.toContain("enabled");
    expect(result?.editorState.disabledRules).toHaveLength(1);
    expect(result?.editorState.disabledRules[0]?.text).toContain("GeoSite-CN-Bound");
  });

  it.each([false, true])("切换一条规则后相邻规则的开关保持原状态（重复内容：%s）", async (duplicate) => {
    useRuleResourceStore().resources.value = resources;
    const rules = [
      { protocol: "dns", action: "hijack-dns" },
      duplicate ? { protocol: "dns", action: "hijack-dns" } : { clash_mode: "direct", action: "route", outbound: "direct" },
      { protocol: "quic", action: "reject" }
    ];
    let draft = draftFixture({
      final: "direct",
      rules
    });
    const wrapper = mount(RouteConfigPanel, { props: { draft } });
    const checked = () => wrapper.findAll<HTMLInputElement>(".rule-switch input").map((input) => input.element.checked);

    async function toggle(index: number, enabled: boolean): Promise<void> {
      await wrapper.findAll(".rule-switch input")[index]!.setValue(enabled);
      const result = wrapper.emitted("save")!.at(-1)![0] as RoutePanelSave;
      draft = {
        ...draft,
        config: { ...draft.config, route: JSON.parse(result.text) },
        buffers: { ...draft.buffers, route: result.text },
        editorState: { ...draft.editorState, route: result.editorState }
      };
      await wrapper.setProps({ draft });
    }

    expect(checked()).toEqual([true, true, true]);
    await toggle(0, false);
    expect(checked()).toEqual([false, true, true]);
    expect(draft.config.route).toMatchObject({ rules: rules.slice(1) });
    await toggle(1, false);
    expect(checked()).toEqual([false, false, true]);
    expect(draft.config.route).toMatchObject({ rules: rules.slice(2) });
    await toggle(0, true);
    expect(checked()).toEqual([true, false, true]);
    await wrapper.setProps({ draft: structuredClone(draft) });
    expect(checked()).toEqual([true, false, true]);
    await toggle(1, true);
    expect(checked()).toEqual([true, true, true]);
    expect(draft.config.route).toMatchObject({ rules });
    wrapper.unmount();
  });

  it("上下方向键重排规则并保持插入点", async () => {
    const routeValue = {
      final: "direct",
      rules: [
        { domain_suffix: "first.example", action: "route", outbound: "direct" },
        { domain_suffix: "second.example", action: "route", outbound: "direct" }
      ]
    };
    const draft = draftFixture(routeValue);
    draft.editorState = { route: { version: 1, insertionPosition: 1, disabledRules: [] } };
    const wrapper = mount(RouteConfigPanel, { props: { draft } });

    await wrapper.findAll("[data-route-rule-drag-handle]")[1]!.trigger("keydown", { key: "ArrowUp" });
    const result = wrapper.emitted("save")?.[0]?.[0] as RoutePanelSave | undefined;
    const saved = JSON.parse(String(result?.text)) as { rules: Array<Record<string, unknown>> };
    expect(saved.rules.map((rule) => rule.domain_suffix)).toEqual(["second.example", "first.example"]);
    expect(result?.editorState.insertionPosition).toBe(1);
  });
});
