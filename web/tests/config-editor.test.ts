import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import AdvancedPanel from "@/features/config-editor/AdvancedPanel.vue";
import ConfigTabs from "@/features/config-editor/ConfigTabs.vue";
import ModuleJsonDialog from "@/features/config-editor/ModuleJsonDialog.vue";
import OverviewPanel from "@/features/config-editor/OverviewPanel.vue";
import OutboundJsonEditor from "@/features/config-editor/OutboundJsonEditor.vue";
import type { Draft } from "@/features/profiles/types";

function overviewDraft(): Draft {
  return {
    profileId: "profile-a",
    revision: 3,
    config: {
      log: { level: "info", future: { large: 9007199254740992 } },
      dns: { strategy: "prefer_ipv6", rules: [{ action: "route" }] },
      inbounds: [{ type: "mixed", tag: "mixed-in", listen_port: 20122 }],
      outbounds: [{ type: "direct", tag: "direct" }],
      route: { default_interface: "eth0", rules: [{ action: "route", outbound: "direct" }] },
      experimental: {
        future: { enabled: true },
        cache_file: { enabled: true, path: "cache.db", store_fakeip: true },
        clash_api: { external_controller: "127.0.0.1:20124" }
      }
    },
    buffers: {
      log: '{"level":"info","future":{"large":9007199254740993}}',
      dns: '{"strategy":"prefer_ipv6","rules":[{"action":"route"}]}',
      inbounds: '[{"type":"mixed","tag":"mixed-in","listen_port":20122}]',
      outbounds: '[{"type":"direct","tag":"direct"}]',
      route: '{"default_interface":"eth0","rules":[{"action":"route","outbound":"direct"}]}',
      experimental: '{"future":{"enabled":true},"cache_file":{"enabled":true,"path":"cache.db","store_fakeip":true},"clash_api":{"external_controller":"127.0.0.1:20124"}}'
    },
    bindings: [],
    savedAt: "2026-08-28T00:00:00Z"
  };
}

describe("配置编辑器", () => {
  it("按固定顺序显示七个标签且不提供端点模式", () => {
    const wrapper = mount(ConfigTabs, { props: { active: "overview" } });
    expect(wrapper.findAll("button").map((button) => button.text())).toEqual([
      "配置概览",
      "入站",
      "出站",
      "路由",
      "DNS",
      "服务",
      "高级"
    ]);
    expect(wrapper.text()).not.toContain("端点");
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

  it("配置概览按原型显示十二个模块、常用参数和核心功能", () => {
    const wrapper = mount(OverviewPanel, {
      props: { draft: overviewDraft(), profileName: "家庭出口", errors: {} }
    });

    expect(wrapper.findAll(".config-map-item")).toHaveLength(12);
    expect(wrapper.text()).toContain("完整配置结构");
    expect(wrapper.text()).toContain("常用参数");
    expect(wrapper.text()).toContain("核心功能");
    expect(wrapper.text()).toContain("6/12");
    expect(wrapper.text()).not.toContain("Schema");
    expect((wrapper.get('[aria-label="配置名称"]').element as HTMLInputElement).value).toBe("家庭出口");
    expect((wrapper.get('[aria-label="Mixed 入站端口"]').element as HTMLInputElement).value).toBe("20122");
    expect((wrapper.get('[aria-label="Clash API 端口"]').element as HTMLInputElement).value).toBe("20124");
    expect((wrapper.get('[aria-label="缓存文件"]').element as HTMLInputElement).value).toBe("cache.db");
    expect(wrapper.get('[role="switch"][aria-label="启用 IPv6"]').attributes("aria-checked")).toBe("true");
    expect(wrapper.get('[role="switch"][aria-label="Clash API"]').attributes("aria-checked")).toBe("true");
    expect(wrapper.get('[role="switch"][aria-label="FakeIP 持久化"]').attributes("aria-checked")).toBe("true");
  });

  it("配置概览只改目标字段并保留未知字段和大整数文本", async () => {
    const wrapper = mount(OverviewPanel, {
      props: { draft: overviewDraft(), profileName: "家庭出口", errors: {} }
    });

    await wrapper.get('[aria-label="日志等级"]').setValue("debug");
    const logUpdate = wrapper.emitted("update")?.[0];
    expect(logUpdate?.[0]).toBe("log");
    const next = logUpdate?.[1] as Draft;
    expect(next.buffers.log).toContain('"level": "debug"');
    expect(next.buffers.log).toContain('"large":9007199254740993');

    const apiWrapper = mount(OverviewPanel, {
      props: { draft: overviewDraft(), profileName: "家庭出口", errors: {} }
    });
    await apiWrapper.get('[aria-label="Clash API 端口"]').setValue("20909");
    const apiUpdate = apiWrapper.emitted("update")?.[0];
    expect(apiUpdate?.[0]).toBe("experimental");
    const apiDraft = apiUpdate?.[1] as Draft;
    expect(apiDraft.buffers.experimental).toContain('"external_controller": "127.0.0.1:20909"');
    expect(apiDraft.buffers.experimental).toContain('"future":{"enabled":true}');
  });

  it("配置概览把端口、接口、缓存和开关写入对应模块", async () => {
    const mounted = () => mount(OverviewPanel, {
      props: { draft: overviewDraft(), profileName: "家庭出口", errors: {} }
    });

    const mixed = mounted();
    await mixed.get('[aria-label="Mixed 入站端口"]').setValue("22022");
    const mixedDraft = mixed.emitted("update")?.[0]?.[1] as Draft;
    expect(JSON.parse(mixedDraft.buffers.inbounds)[0].listen_port).toBe(22022);

    const route = mounted();
    await route.get('[aria-label="网络接口"]').setValue("wlan0");
    const routeDraft = route.emitted("update")?.[0]?.[1] as Draft;
    expect(JSON.parse(routeDraft.buffers.route).default_interface).toBe("wlan0");

    const cache = mounted();
    await cache.get('[aria-label="缓存文件"]').setValue("state/cache.db");
    const cacheDraft = cache.emitted("update")?.[0]?.[1] as Draft;
    expect(JSON.parse(cacheDraft.buffers.experimental).cache_file.path).toBe("state/cache.db");

    const ipv6 = mounted();
    await ipv6.get('[role="switch"][aria-label="启用 IPv6"]').trigger("click");
    const dnsDraft = ipv6.emitted("update")?.[0]?.[1] as Draft;
    expect(JSON.parse(dnsDraft.buffers.dns).strategy).toBe("ipv4_only");

    const clash = mounted();
    await clash.get('[role="switch"][aria-label="Clash API"]').trigger("click");
    const clashDraft = clash.emitted("update")?.[0]?.[1] as Draft;
    const experimental = JSON.parse(clashDraft.buffers.experimental);
    expect(experimental.clash_api).toBeUndefined();
    expect(experimental.future).toEqual({ enabled: true });

    const fakeIp = mounted();
    await fakeIp.get('[role="switch"][aria-label="FakeIP 持久化"]').trigger("click");
    const fakeIpDraft = fakeIp.emitted("update")?.[0]?.[1] as Draft;
    expect(JSON.parse(fakeIpDraft.buffers.experimental).cache_file.store_fakeip).toBe(false);
  });

  it("配置地图跳转到对应标签，日志入口打开完整 JSON", async () => {
    const wrapper = mount(OverviewPanel, {
      props: { draft: overviewDraft(), profileName: "家庭出口", errors: {} }
    });
    const buttons = wrapper.findAll(".config-map-item");
    await buttons.find((button) => button.text().includes("DNS"))!.trigger("click");
    await buttons.find((button) => button.text().includes("日志"))!.trigger("click");
    expect(wrapper.emitted("selectTab")?.[0]).toEqual(["dns"]);
    expect(wrapper.emitted("editJson")?.[0]).toEqual(["log"]);
  });

  it("高级页按原型显示六个模块且不呈现 Schema", async () => {
    const draft = overviewDraft();
    draft.config.ntp = { enabled: true, server: "time.apple.com" };
    draft.buffers.ntp = '{"enabled":true,"server":"time.apple.com"}';
    const wrapper = mount(AdvancedPanel, { props: { draft, errors: {} } });

    expect(wrapper.findAll(".advanced-section-row")).toHaveLength(6);
    expect(wrapper.text()).toContain("高级与共享模块");
    expect(wrapper.text()).toContain("6 modules");
    expect(wrapper.text()).toContain("2 个字段");
    expect(wrapper.text()).not.toContain("Schema");
    await wrapper.findAll(".advanced-section-row")[0]!.trigger("click");
    expect(wrapper.emitted("editJson")?.[0]).toEqual(["ntp"]);
  });

  it("高级模块编辑器校验顶层容器并提供清空入口", () => {
    const wrapper = mount(ModuleJsonDialog, {
      props: { open: true, module: "ntp", text: "[]", configured: true }
    });
    expect(wrapper.text()).toContain("完整维护 config.ntp");
    expect(wrapper.text()).toContain("ntp 必须是对象");
    expect(wrapper.get("[data-save-module]").attributes()).toHaveProperty("disabled");
    expect(wrapper.get("button.clear-button").text()).toContain("清空模块");
  });
});
