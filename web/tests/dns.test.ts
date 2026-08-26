import { mount } from "@vue/test-utils";
import { afterEach, describe, expect, it } from "vitest";
import DnsConfigPanel from "@/features/config-editor/DnsConfigPanel.vue";
import DnsRuleEditorDialog from "@/features/config-editor/DnsRuleEditorDialog.vue";
import DnsServerEditorDialog from "@/features/config-editor/DnsServerEditorDialog.vue";
import FakeIpDnsRuleDialog from "@/features/config-editor/FakeIpDnsRuleDialog.vue";
import {
  createDefaultFakeIpDnsRule,
  dnsServerAddress,
  ensureDefaultFakeIpDnsRuleText,
  fakeIpDnsRuleParts,
  insertDnsRuleText,
  isDefaultFakeIpDnsRule,
  parseFakeIpDomainSuffixes,
  reconcileDnsEditorState,
  removeDnsArrayItem,
  renameDnsServerTagInBuffer,
  restoreDnsEditorState,
  updateDnsBoolean,
  updateDnsString,
  upsertDnsArrayItemText,
  type DnsPanelSave,
  type DnsRuleEditorResult,
  type DnsServerEditorResult
} from "@/features/config-editor/dns";
import type { RouteRuleSetOption } from "@/features/config-editor/route";
import type { Draft } from "@/features/profiles/types";
import { useRuleResourceStore } from "@/features/rule-resources/store";
import type { RuleResource } from "@/features/rule-resources/types";

const dnsFixture = {
  servers: [
    { tag: "Fake-IP", type: "fakeip", inet4_range: "198.18.0.0/15", inet6_range: "fc00::/18" },
    { tag: "Local-DNS", type: "https", domain_resolver: "Local-DNS-Resolver", server_port: 443, server: "223.5.5.5", path: "/dns-query" },
    { tag: "Local-DNS-Resolver", type: "udp", server_port: 53, server: "223.5.5.5" },
    { tag: "Remote-DNS", type: "tls", detour: "🚀 节点选择", domain_resolver: "Remote-DNS-Resolver", server_port: 853, server: "8.8.8.8" },
    { tag: "Remote-DNS-Resolver", type: "udp", detour: "🚀 节点选择", server_port: 53, server: "8.8.8.8" }
  ],
  rules: [
    { action: "route", clash_mode: "direct", server: "Local-DNS" },
    { action: "route", clash_mode: "global", server: "Remote-DNS" },
    { action: "route", rule_set: ["GeoSite-CN"], server: "Local-DNS" },
    createDefaultFakeIpDnsRule(),
    { action: "route", rule_set: ["GeoLocation-!CN"], server: "Remote-DNS" }
  ],
  disable_cache: false,
  disable_expire: false,
  independent_cache: false,
  final: "Remote-DNS"
};

const resource: RuleResource = {
  id: "a".repeat(32), sourceType: "remote", name: "GeoSite-CN", url: "https://example.test/cn.srs", format: "binary", updateInterval: "24h", disabled: false,
  localPath: `data/rulesets/${"a".repeat(32)}.srs`, lastDigest: "1".repeat(64)
};

function draftFixture(): Draft {
  return {
    profileId: "d".repeat(32), revision: 1,
    config: {
      dns: structuredClone(dnsFixture),
      route: { default_domain_resolver: { server: "Remote-DNS" }, rules: [{ action: "resolve", domain: ["node.example"], server: "Remote-DNS" }] },
      outbounds: [{ type: "selector", tag: "🚀 节点选择", outbounds: ["direct"] }]
    },
    buffers: {
      dns: JSON.stringify(dnsFixture, null, 2),
      route: JSON.stringify({ default_domain_resolver: { server: "Remote-DNS" }, rules: [{ action: "resolve", domain: ["node.example"], server: "Remote-DNS" }] }, null, 2)
    },
    bindings: [{ resourceId: resource.id, tag: "GeoSite-CN", enabled: true }],
    savedAt: "2026-08-26T00:00:00Z"
  };
}

afterEach(() => {
  useRuleResourceStore().resources.value = [];
});

describe("自定义配置 DNS", () => {
  it("按默认配置格式展示 Fake-IP、HTTPS、UDP 与 TLS 地址", () => {
    expect(dnsFixture.servers.map(dnsServerAddress)).toEqual([
      "fake-ip://198.18.0.0/15,fc00::/18",
      "https://223.5.5.5:443/dns-query",
      "udp://223.5.5.5:53",
      "tls://8.8.8.8:853",
      "udp://8.8.8.8:53"
    ]);
  });

  it("识别、规范化并补全原型内置 Fake-IP 规则", () => {
    const rule = createDefaultFakeIpDnsRule([".lan", ".example"], ["A"], "Fake-IP");
    expect(isDefaultFakeIpDnsRule(rule)).toBe(true);
    expect(fakeIpDnsRuleParts(rule)).toEqual({ suffixes: [".lan", ".example"], queryTypes: ["A"] });
    expect(parseFakeIpDomainSuffixes("LAN\n Example.COM,example.com ")).toEqual([".lan", ".example.com"]);

    const source = `{"future":9007199254740993,"rules":[{"clash_mode":"direct","action":"route","server":"Local-DNS"},{"rule_set":["GeoLocation-!CN"],"action":"route","server":"Remote-DNS"}]}`;
    const changed = ensureDefaultFakeIpDnsRuleText(source);
    const rules = (JSON.parse(changed) as { rules: Array<Record<string, unknown>> }).rules;
    expect(changed).toContain("9007199254740993");
    expect(rules).toHaveLength(3);
    expect(isDefaultFakeIpDnsRule(rules[1])).toBe(true);

    const disabled = { version: 1, insertionPosition: 1, disabledRules: [{ id: "fakeip-off", position: 1, text: JSON.stringify(rule) }] };
    expect(ensureDefaultFakeIpDnsRuleText(source, disabled)).toBe(source);
    expect(reconcileDnsEditorState(disabled, { rules: [rule] }).disabledRules).toEqual([]);
  });

  it("增删 DNS 项目时保留其他字段和大整数 token", () => {
    const source = `{"future":9007199254740993,"servers":[{"tag":"dns-a","type":"udp","server":"1.1.1.1","future":9007199254740995}],"rules":[{"clash_mode":"direct","action":"route","server":"dns-a","future":9007199254740997}]}`;
    const server = upsertDnsArrayItemText(source, "servers", 1, `{"tag":"dns-b","type":"tls","server":"8.8.8.8","server_port":853}`);
    const inserted = insertDnsRuleText(server, 0, `{"domain_suffix":["example.test"],"action":"route","server":"dns-b"}`);
    const removed = removeDnsArrayItem(inserted, "rules", 1);

    expect(removed).toContain("9007199254740993");
    expect(removed).toContain("9007199254740995");
    expect(removed).not.toContain("9007199254740997");
    expect((JSON.parse(removed) as { servers: unknown[]; rules: unknown[] }).servers).toHaveLength(2);
    expect((JSON.parse(removed) as { rules: Array<Record<string, unknown>> }).rules[0]?.domain_suffix).toEqual(["example.test"]);
  });

  it("通用设置与服务器改名只修改已知引用路径", () => {
    const source = `{"future":9007199254740993,"servers":[{"tag":"dns-a","type":"udp","server":"1.1.1.1"},{"tag":"dns-resolver","type":"tls","domain_resolver":"dns-a","server":"8.8.8.8"}],"rules":[{"domain":["example.test"],"action":"route","server":"dns-a"}],"final":"dns-a"}`;
    let changed = updateDnsBoolean(source, "disable_cache", true);
    changed = updateDnsString(changed, "strategy", "prefer_ipv4");
    changed = renameDnsServerTagInBuffer(changed, "dns", "dns-a", "dns-new");
    const value = JSON.parse(changed) as { final: string; servers: Array<Record<string, unknown>>; rules: Array<Record<string, unknown>>; disable_cache: boolean; strategy: string };

    expect(changed).toContain("9007199254740993");
    expect(value).toMatchObject({ final: "dns-new", disable_cache: true, strategy: "prefer_ipv4" });
    expect(value.servers[0]?.server).toBe("1.1.1.1");
    expect(value.servers[1]?.domain_resolver).toBe("dns-new");
    expect(value.rules[0]?.server).toBe("dns-new");

    const route = renameDnsServerTagInBuffer(`{"default_domain_resolver":{"server":"dns-a","future":9007199254740995},"rules":[{"action":"resolve","server":"dns-a"}]}`, "route", "dns-a", "dns-new");
    expect(route).toContain("9007199254740995");
    expect(JSON.parse(route)).toMatchObject({ default_domain_resolver: { server: "dns-new" }, rules: [{ server: "dns-new" }] });
  });

  it("默认把 DNS 新规则插入点放在前两条规则之后并恢复停用规则", () => {
    expect(restoreDnsEditorState(undefined, 5)).toEqual({ version: 1, insertionPosition: 2, disabledRules: [] });
    expect(restoreDnsEditorState({
      version: 1,
      insertionPosition: 99,
      disabledRules: [{ id: "disabled-a", position: 2, text: `{"domain_suffix":["off.example"],"action":"route","server":"dns-a"}` }]
    }, 5)).toEqual({
      version: 1,
      insertionPosition: 6,
      disabledRules: [{ id: "disabled-a", position: 2, text: `{"domain_suffix":["off.example"],"action":"route","server":"dns-a"}` }]
    });
  });

  it("服务器弹窗保留高级字段并输出结构化端点", async () => {
    const raw = `{"tag":"dns-a","type":"udp","server":"1.1.1.1","server_port":53,"future":9007199254740993}`;
    const wrapper = mount(DnsServerEditorDialog, {
      props: {
        open: true,
        server: JSON.parse(raw),
        serverText: raw,
        reservedTags: ["dns-b"],
        outbounds: ["direct"],
        dnsServers: ["dns-a", "dns-b"]
      }
    });

    await wrapper.get('input[name="tag"]').setValue("dns-renamed");
    await wrapper.get('input[name="server"]').setValue("8.8.8.8");
    await wrapper.get('input[name="server-port"]').setValue("5353");
    await wrapper.get("form").trigger("submit");
    const result = wrapper.emitted("submit")?.[0]?.[0] as DnsServerEditorResult | undefined;
    expect(result?.server).toMatchObject({ tag: "dns-renamed", type: "udp", server: "8.8.8.8", server_port: 5353 });
    expect(result?.text).toContain("9007199254740993");
  });

  it("规则弹窗支持匹配类型、规则集与目标 DNS 服务器", async () => {
    const raw = `{"future":9007199254740993,"clash_mode":"direct","action":"route","server":"Local-DNS"}`;
    const options: RouteRuleSetOption[] = [{ id: "managed:a", tag: "GeoSite-CN", name: "GeoSite-CN", kind: "managed", detail: "网络 · Binary", resourceId: resource.id, disabled: false }];
    const wrapper = mount(DnsRuleEditorDialog, { props: { open: true, rule: JSON.parse(raw), ruleText: raw, servers: ["Local-DNS", "Remote-DNS"], options } });

    await wrapper.get('[aria-label="DNS 规则类型"]').setValue("domain_suffix");
    await wrapper.get('[aria-label="DNS 匹配内容"]').setValue("example.com,example.org");
    await wrapper.get('[aria-label="目标 DNS 服务器"]').setValue("Remote-DNS");
    await wrapper.get("form").trigger("submit");
    const result = wrapper.emitted("submit")?.[0]?.[0] as DnsRuleEditorResult | undefined;
    expect(result?.rule).toMatchObject({ domain_suffix: ["example.com", "example.org"], action: "route", server: "Remote-DNS" });
    expect(result?.rule).not.toHaveProperty("clash_mode");
    expect(result?.text).toContain("9007199254740993");
  });

  it("内置 Fake-IP 编辑器只开放安全字段并重建固定逻辑结构", async () => {
    const wrapper = mount(FakeIpDnsRuleDialog, {
      props: {
        open: true,
        rule: createDefaultFakeIpDnsRule(),
        enabled: true,
        servers: dnsFixture.servers
      }
    });

    expect(wrapper.text()).toContain("系统内置 · 当前启用");
    expect(wrapper.findAll('[aria-label="目标 Fake-IP 服务器"] option')).toHaveLength(1);
    expect(wrapper.text()).toContain("198.18.0.0/15");
    await wrapper.get('[aria-label="不进入 Fake-IP 的域名后缀"]').setValue("LAN\nExample.COM\nexample.com");
    await wrapper.get('[aria-label="Fake-IP 查询类型 AAAA"]').setValue(false);
    await wrapper.get("form").trigger("submit");

    const result = wrapper.emitted("submit")?.[0]?.[0] as DnsRuleEditorResult | undefined;
    expect(result?.rule).toEqual(createDefaultFakeIpDnsRule([".lan", ".example.com"], ["A"], "Fake-IP"));
    expect(result?.resourceIds).toEqual([]);
  });

  it("页面按最新版原型在同一页呈现通用设置、五个服务器和五条默认规则", async () => {
    useRuleResourceStore().resources.value = [resource];
    const wrapper = mount(DnsConfigPanel, { props: { draft: draftFixture() } });

    expect(wrapper.get('[aria-label="回退 DNS"]').element).toHaveProperty("value", "Remote-DNS");
    expect(wrapper.findAll('[role="switch"]')).toHaveLength(3);
    expect(wrapper.findAll('[role="switch"]').every((item) => item.attributes("aria-checked") === "false")).toBe(true);
    expect(wrapper.find(".dns-section-tabs").exists()).toBe(false);
    expect(wrapper.findAll(".dns-section-card")).toHaveLength(3);
    expect(wrapper.findAll(".server-row")).toHaveLength(5);
    expect(wrapper.text()).toContain("https://223.5.5.5:443/dns-query");
    expect(wrapper.text()).toContain("🚀 节点选择");
    expect(wrapper.findAll(".dns-rule-row")).toHaveLength(5);
    expect(wrapper.find(".dns-insertion-point").exists()).toBe(true);
    expect(wrapper.text()).toContain("clash_mode, direct, route, Local-DNS");
    const fakeIpRule = wrapper.get('[data-dns-rule-kind="fakeip"]');
    expect(fakeIpRule.classes()).toContain("is-protected");
    expect(fakeIpRule.text()).toContain("Fake-IP 地址映射");
    expect(fakeIpRule.text()).toContain("排除 10 个后缀 · A / AAAA → Fake-IP");
    expect(fakeIpRule.get('[aria-label="内置 Fake-IP 规则不能删除"]').attributes("disabled")).toBeDefined();

    await wrapper.get(".rule-switch input").setValue(false);
    const result = wrapper.emitted("save")?.at(-1)?.[0] as DnsPanelSave | undefined;
    expect((JSON.parse(String(result?.text)) as { rules: unknown[] }).rules).toHaveLength(4);
    expect(result?.editorState.disabledRules).toHaveLength(1);
    expect(result?.editorState.disabledRules[0]?.text).toContain("clash_mode");
  });

  it("内置 Fake-IP 规则可停用但不可删除，并使用专用编辑器", async () => {
    useRuleResourceStore().resources.value = [resource];
    const wrapper = mount(DnsConfigPanel, { props: { draft: draftFixture() } });
    const fakeIpRule = wrapper.get('[data-dns-rule-kind="fakeip"]');

    await fakeIpRule.get('[aria-label="编辑内置 Fake-IP 规则"]').trigger("click");
    const dialog = wrapper.get('dialog[aria-label="编辑内置 Fake-IP 规则"]');
    expect(dialog.attributes("open")).toBeDefined();
    await dialog.get('[aria-label="关闭对话框"]').trigger("click");
    await fakeIpRule.get('[aria-label="停用内置 Fake-IP 规则"]').setValue(false);

    const result = wrapper.emitted("save")?.at(-1)?.[0] as DnsPanelSave | undefined;
    const saved = JSON.parse(String(result?.text)) as { rules: unknown[] };
    expect(saved.rules).toHaveLength(4);
    expect(saved.rules.some(isDefaultFakeIpDnsRule)).toBe(false);
    expect(result?.editorState.disabledRules).toHaveLength(1);
    expect(isDefaultFakeIpDnsRule(JSON.parse(result?.editorState.disabledRules[0]?.text ?? "{}"))).toBe(true);
  });

  it("编辑服务器标签时同步 DNS 内部引用并报告改名", async () => {
    useRuleResourceStore().resources.value = [resource];
    const wrapper = mount(DnsConfigPanel, { props: { draft: draftFixture() } });
    await wrapper.findAll('.server-row [aria-label^="编辑 DNS 服务器"]')[3]!.trigger("click");
    await wrapper.get('input[name="tag"]').setValue("Remote-DNS-New");
    await wrapper.get("#dns-server-form").trigger("submit");

    const result = wrapper.emitted("save")?.at(-1)?.[0] as DnsPanelSave | undefined;
    const saved = JSON.parse(String(result?.text)) as typeof dnsFixture;
    expect(result?.renamed).toEqual({ from: "Remote-DNS", to: "Remote-DNS-New" });
    expect(saved.final).toBe("Remote-DNS-New");
    expect(saved.rules[1]?.server).toBe("Remote-DNS-New");
    expect(saved.rules[4]?.server).toBe("Remote-DNS-New");
  });
});
