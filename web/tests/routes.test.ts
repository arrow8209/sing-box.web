import { mount } from "@vue/test-utils";
import { afterEach, describe, expect, it } from "vitest";
import RouteConfigPanel from "@/features/config-editor/RouteConfigPanel.vue";
import RouteRuleEditorDialog from "@/features/config-editor/RouteRuleEditorDialog.vue";
import {
  buildRouteRuleSetOptions,
  mergeSelectedRuleBindings,
  removeRouteRule,
  upsertRouteRule
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
  final: "节点选择",
  rule_set: [{ type: "inline", tag: "Inline-Private", rules: [] }],
  rules: [{ action: "route", rule_set: ["GeoSite-CN-Bound", "Legacy-Tag"], outbound: "节点选择" }]
};

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

  it("增删单条规则时保留未编辑的大整数 token", () => {
    expect(JSON.parse(upsertRouteRule("{}", 0, { action: "route", rule_set: ["GeoSite-CN"], outbound: "节点选择" }))).toEqual({
      rules: [{ action: "route", rule_set: ["GeoSite-CN"], outbound: "节点选择" }]
    });
    const source = `{"future":9007199254740993,"rules":[{"protocol":"dns","future":9007199254740995}]}`;
    const added = upsertRouteRule(source, 1, { action: "route", rule_set: ["GeoSite-CN"], outbound: "节点选择" });

    expect(added).toContain("9007199254740993");
    expect(added).toContain("9007199254740995");
    expect((JSON.parse(added) as { rules: unknown[] }).rules).toHaveLength(2);

    const removed = removeRouteRule(added, 1);
    expect(removed).toContain("9007199254740993");
    expect(removed).toContain("9007199254740995");
    expect((JSON.parse(removed) as { rules: unknown[] }).rules).toHaveLength(1);
  });

  it("新增规则弹窗默认展示所有规则集并提交资源引用", async () => {
    const options = buildRouteRuleSetOptions(route, bindings, resources);
    const wrapper = mount(RouteRuleEditorDialog, {
      props: { open: true, rule: null, options, outbounds: ["节点选择", "direct"], defaultOutbound: "节点选择" }
    });

    expect(wrapper.findAll(".ruleset-option")).toHaveLength(options.length);
    expect(wrapper.text()).toContain("全部规则集");
    expect(wrapper.text()).toContain("Disabled-Rules");
    expect(wrapper.get('[aria-label="选择规则集 Disabled-Rules"]').attributes("disabled")).toBeDefined();

    await wrapper.get('[aria-label="选择规则集 Private-Domains"]').setValue(true);
    await wrapper.get("form").trigger("submit");
    expect(wrapper.emitted("submit")?.[0]?.[0]).toEqual({
      rule: { action: "route", rule_set: ["Private-Domains"], outbound: "节点选择" },
      resourceIds: [resources[1]!.id]
    });
  });

  it("路由页移除独立规则集绑定区域，规则集只在新增规则中出现", async () => {
    useRuleResourceStore().resources.value = resources;
    const draft: Draft = {
      profileId: "d".repeat(32), revision: 1,
      config: { route, outbounds: [{ type: "selector", tag: "节点选择", outbounds: ["direct"] }] },
      buffers: { route: JSON.stringify(route, null, 2) }, bindings, savedAt: "2026-08-26T00:00:00Z"
    };
    const wrapper = mount(RouteConfigPanel, { props: { draft } });

    expect(wrapper.text()).not.toContain("Web 规则资源绑定");
    expect(wrapper.text()).not.toContain("添加绑定");
    expect(wrapper.find(".bindings").exists()).toBe(false);
    await wrapper.get(".add-button").trigger("click");
    expect(wrapper.get("dialog").attributes("aria-label")).toBe("新增规则");
    expect(wrapper.findAll(".ruleset-option")).toHaveLength(5);
  });
});
