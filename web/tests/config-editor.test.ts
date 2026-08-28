import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import AdvancedPanel from "@/features/config-editor/AdvancedPanel.vue";
import ConfigTabs from "@/features/config-editor/ConfigTabs.vue";
import GeneralSettingsPanel from "@/features/config-editor/GeneralSettingsPanel.vue";
import ModuleJsonDialog from "@/features/config-editor/ModuleJsonDialog.vue";
import OutboundJsonEditor from "@/features/config-editor/OutboundJsonEditor.vue";
import type { Draft } from "@/features/profiles/types";

function generalSettingsDraft(): Draft {
  return {
    profileId: "profile-a",
    revision: 3,
    config: {
      log: { disabled: false, level: "info", output: "", timestamp: false, future: { large: 9007199254740992 } },
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
      log: '{"disabled":false,"level":"info","output":"","timestamp":false,"future":{"large":9007199254740993}}',
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
    const wrapper = mount(ConfigTabs, { props: { active: "general" } });
    expect(wrapper.findAll("button").map((button) => button.text())).toEqual([
      "通用设置",
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

  it("通用设置显示工作模式、API 与基础日志参数", () => {
    const wrapper = mount(GeneralSettingsPanel, { props: { draft: generalSettingsDraft() } });

    expect(wrapper.text()).toContain("工作模式与 API");
    expect(wrapper.text()).toContain("日志设置");
    expect(wrapper.text()).toContain("参数归属");
    expect(wrapper.findAll(".setting-row")).toHaveLength(4);
    expect(wrapper.findAll('[role="radio"]')).toHaveLength(3);
    expect(wrapper.findAll(".config-map-item")).toHaveLength(0);
    expect(wrapper.get('[role="radio"][aria-label="规则"]').attributes("aria-checked")).toBe("true");
    expect((wrapper.get('[aria-label="API 监听地址"]').element as HTMLInputElement).value).toBe("127.0.0.1:20213");
    expect((wrapper.get('[aria-label="API 密钥"]').element as HTMLInputElement).value).toBe("");
    expect((wrapper.get('[aria-label="日志等级"]').element as HTMLSelectElement).value).toBe("info");
    expect((wrapper.get('[aria-label="日志保存路径"]').element as HTMLInputElement).value).toBe("data/logs/sing-box.log");
    expect(wrapper.get('[role="switch"][aria-label="启用日志"]').attributes("aria-checked")).toBe("true");
    expect(wrapper.get('[role="switch"][aria-label="显示时间戳"]').attributes("aria-checked")).toBe("false");
    expect(wrapper.text()).not.toContain("配置概览");
    expect(wrapper.text()).not.toContain("Mixed 入站端口");
    expect(wrapper.text()).not.toContain("Clash API 端口");
    expect(wrapper.text()).not.toContain("网络接口");
    expect(wrapper.text()).not.toContain("启用 IPv6");
  });

  it("工作模式和随机 API 密钥只更新当前原型状态", async () => {
    const wrapper = mount(GeneralSettingsPanel, { props: { draft: generalSettingsDraft() } });

    await wrapper.get('[role="radio"][aria-label="全局"]').trigger("click");
    expect(wrapper.get('[role="radio"][aria-label="全局"]').attributes("aria-checked")).toBe("true");
    expect(wrapper.get('[role="radio"][aria-label="规则"]').attributes("aria-checked")).toBe("false");

    await wrapper.get(".generate-button").trigger("click");
    expect((wrapper.get('[aria-label="API 密钥"]').element as HTMLInputElement).value).toMatch(/^[0-9a-f]{64}$/);
    expect(wrapper.emitted("update")).toBeUndefined();
  });

  it("通用设置只改日志目标字段并保留未知字段和大整数文本", async () => {
    const wrapper = mount(GeneralSettingsPanel, { props: { draft: generalSettingsDraft() } });

    await wrapper.get('[aria-label="日志等级"]').setValue("debug");
    const logUpdate = wrapper.emitted("update")?.[0];
    const next = logUpdate?.[0] as Draft;
    expect(next.buffers.log).toContain('"level": "debug"');
    expect(next.buffers.log).toContain('"large":9007199254740993');
    expect(next.buffers.inbounds).toBe(generalSettingsDraft().buffers.inbounds);
    expect(next.buffers.route).toBe(generalSettingsDraft().buffers.route);
    expect(next.buffers.dns).toBe(generalSettingsDraft().buffers.dns);
  });

  it("通用设置把输出路径和两个开关写入日志模块", async () => {
    const mounted = () => mount(GeneralSettingsPanel, { props: { draft: generalSettingsDraft() } });

    const output = mounted();
    await output.get('[aria-label="日志保存路径"]').setValue("logs/sing-box.log");
    const outputDraft = output.emitted("update")?.[0]?.[0] as Draft;
    expect(JSON.parse(outputDraft.buffers.log).output).toBe("logs/sing-box.log");

    const enabled = mounted();
    await enabled.get('[role="switch"][aria-label="启用日志"]').trigger("click");
    const enabledDraft = enabled.emitted("update")?.[0]?.[0] as Draft;
    expect(JSON.parse(enabledDraft.buffers.log).disabled).toBe(true);

    const timestamp = mounted();
    await timestamp.get('[role="switch"][aria-label="显示时间戳"]').trigger("click");
    const timestampDraft = timestamp.emitted("update")?.[0]?.[0] as Draft;
    expect(JSON.parse(timestampDraft.buffers.log).timestamp).toBe(true);
  });

  it("通用设置保留日志完整 JSON 入口", async () => {
    const wrapper = mount(GeneralSettingsPanel, { props: { draft: generalSettingsDraft() } });
    await wrapper.get(".json-button").trigger("click");
    expect(wrapper.emitted("editJson")?.[0]).toEqual([]);
  });

  it("高级页按原型显示六个模块且不呈现 Schema", async () => {
    const draft = generalSettingsDraft();
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
