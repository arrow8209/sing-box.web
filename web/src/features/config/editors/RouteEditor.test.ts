import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import RouteEditor from "./RouteEditor.vue";
import { useDraftStore } from "../../../stores/draft";
import type { Draft } from "../../../api/types";

const profileId = "a".repeat(32);

function makeDraft(route: Record<string, unknown>): Draft {
  return {
    profileId,
    revision: 1,
    content: { config: { route }, editorBuffers: {} },
    savedAt: "2026-08-18T10:00:00Z",
  };
}

interface RuleLike {
  action?: string;
  outbound?: string;
  invert?: boolean;
  domain_suffix?: string[];
  [key: string]: unknown;
}

describe("RouteEditor", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it("edits route.rules in order with action, outbound, invert and a condition", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft({
      rules: [
        { action: "route", outbound: "direct", domain_suffix: ["example.com"] },
        { action: "reject", domain: ["ads.test"] },
      ],
      unknown_route_field: { v: 3 },
    });
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");
    const lastRules = () => {
      const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
      return (config.route as Record<string, unknown>).rules as RuleLike[];
    };

    const wrapper = mount(RouteEditor);
    const actionSelects = wrapper.findAll('select[aria-label="规则动作"]');
    expect(actionSelects).toHaveLength(2);

    await actionSelects[0].setValue("reject");
    expect(lastRules()[0].action).toBe("reject");
    expect(lastRules()[0].domain_suffix).toEqual(["example.com"]);

    await wrapper.get('input[aria-label="规则出站"]').setValue("proxy");
    expect(lastRules()[0].outbound).toBe("proxy");

    await wrapper.get('input[aria-label="反向匹配"]').setValue(true);
    expect(lastRules()[0].invert).toBe(true);

    await wrapper.get('input[aria-label="域名后缀条件"]').setValue("a.com,b.com");
    expect(lastRules()[0].domain_suffix).toEqual(["a.com", "b.com"]);

    const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect((config.route as Record<string, unknown>).unknown_route_field).toEqual({ v: 3 });
  });

  it("adds, removes and reorders rules preserving array order", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft({
      rules: [
        { action: "route", outbound: "a" },
        { action: "reject" },
      ],
    });
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");
    const lastRules = () => {
      const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
      return (config.route as Record<string, unknown>).rules as RuleLike[];
    };

    const wrapper = mount(RouteEditor);
    await wrapper.get('button[name="添加规则"]').trigger("click");
    expect(lastRules()).toHaveLength(3);

    await wrapper.get('button[name="删除规则 1"]').trigger("click");
    expect(lastRules()).toHaveLength(2);
    expect(lastRules()[1].action).toBe("route");
    expect(lastRules()[1].outbound).toBeUndefined();

    await wrapper.get('button[name="上移规则 1"]').trigger("click");
    expect(lastRules()[0].action).toBe("route");
    expect(lastRules()[1].action).toBe("route");
    expect(lastRules()[1].outbound).toBe("a");
  });

  it("edits top-level route fields auto_detect_interface, find_process, default_interface, default_domain_resolver and final", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft({ final: "direct" });
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");
    const lastRoute = () => {
      const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
      return config.route as Record<string, unknown>;
    };

    const wrapper = mount(RouteEditor);
    await wrapper.get('input[aria-label="自动检测接口"]').setValue(true);
    expect(lastRoute().auto_detect_interface).toBe(true);

    await wrapper.get('input[aria-label="查找进程"]').setValue(true);
    expect(lastRoute().find_process).toBe(true);

    await wrapper.get('input[aria-label="默认接口"]').setValue("eth0");
    expect(lastRoute().default_interface).toBe("eth0");

    await wrapper.get('input[aria-label="默认域名解析器"]').setValue("dns");
    expect(lastRoute().default_domain_resolver).toBe("dns");

    await wrapper.get('input[aria-label="最终出站"]').setValue("block");
    expect(lastRoute().final).toBe("block");
  });

  it("edits remote rule sets with tag, type, format, url, download_detour and update_interval", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft({
      rule_set: [
        {
          tag: "geosite-cn",
          type: "remote",
          format: "binary",
          url: "https://example.test/geosite.db",
          download_detour: "proxy",
          update_interval: "24h",
        },
      ],
    });
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");
    const lastRuleSet = () => {
      const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
      return ((config.route as Record<string, unknown>).rule_set as Record<string, unknown>[])[0];
    };

    const wrapper = mount(RouteEditor);
    await wrapper.get('input[aria-label="规则集标签"]').setValue("geoip-cn");
    expect(lastRuleSet().tag).toBe("geoip-cn");

    await wrapper.get('input[aria-label="规则集 URL"]').setValue("https://example.test/geoip.db");
    expect(lastRuleSet().url).toBe("https://example.test/geoip.db");

    await wrapper.get('input[aria-label="规则集下载出站"]').setValue("direct");
    expect(lastRuleSet().download_detour).toBe("direct");

    await wrapper.get('input[aria-label="规则集更新间隔"]').setValue("12h");
    expect(lastRuleSet().update_interval).toBe("12h");
  });
});
