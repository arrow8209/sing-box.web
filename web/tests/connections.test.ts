import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/api/client";
import ConnectionsView from "@/features/connections/ConnectionsView.vue";
import type { ConnectionsApi } from "@/features/connections/api";
import type { ConnectionsSnapshot, RuntimeConnection } from "@/features/connections/types";

const first: RuntimeConnection = {
  id: "connection-1",
  metadata: {
    type: "tun",
    network: "tcp",
    host: "api.example.test",
    sourceIP: "172.16.1.20",
    sourcePort: "51820",
    destinationIP: "203.0.113.10",
    destinationPort: "443",
    process: "browser",
    processPath: "/usr/bin/browser"
  },
  upload: 2048,
  download: 8192,
  start: "2026-08-31T06:00:00Z",
  chains: ["香港 01", "🚀 节点选择"],
  rule: "rule_set",
  rulePayload: "GeoLocation-!CN"
};
const second: RuntimeConnection = {
  id: "connection-2",
  metadata: {
    type: "mixed",
    network: "udp",
    host: "dns.example.test",
    sourceIP: "127.0.0.1",
    sourcePort: "53000",
    destinationIP: "198.51.100.53",
    destinationPort: "53"
  },
  upload: 512,
  download: 1024,
  start: "2026-08-31T06:01:00Z",
  chains: ["direct"],
  rule: "final",
  rulePayload: ""
};

function snapshot(connections: RuntimeConnection[] = [first, second]): ConnectionsSnapshot {
  return { memory: 12 * 1024 * 1024, uploadTotal: 4096, downloadTotal: 16384, connections };
}

function api(value = snapshot()): ConnectionsApi {
  return {
    list: vi.fn(async () => value),
    close: vi.fn(async () => undefined),
    closeAll: vi.fn(async () => undefined)
  };
}

afterEach(() => { localStorage.clear(); });

describe("活动连接页面", () => {
  it("展示、搜索并查看连接的完整详情", async () => {
    const connectionApi = api();
    const wrapper = mount(ConnectionsView, { props: { api: connectionApi, pollInterval: 60_000 } });
    await flushPromises();

    expect(wrapper.findAll(".connections-table tbody tr")).toHaveLength(2);
    expect(wrapper.text()).toContain("api.example.test");
    expect(wrapper.text()).toContain("GeoLocation-!CN");
    expect(wrapper.text()).toContain("香港 01");
    expect(wrapper.text()).toContain("12.0 MB");

    await wrapper.get('input[aria-label="搜索连接"]').setValue("browser");
    expect(wrapper.findAll(".connections-table tbody tr")).toHaveLength(1);
    expect(wrapper.text()).not.toContain("dns.example.test");

    await wrapper.get('[aria-label="查看 api.example.test 的连接详情"]').trigger("click");
    await flushPromises();
    const dialog = wrapper.get('dialog[aria-label="连接详情"]');
    expect(dialog.attributes()).toHaveProperty("open");
    expect(dialog.text()).toContain("203.0.113.10:443");
    expect(dialog.text()).toContain("/usr/bin/browser");
    expect(dialog.text()).toContain("查看原始 JSON");
    expect(dialog.text()).toContain('"id": "connection-1"');
    wrapper.unmount();
  });

  it("关闭连接后保留到已关闭列表，并可清除历史", async () => {
    const connectionApi = api();
    const wrapper = mount(ConnectionsView, { props: { api: connectionApi, pollInterval: 60_000 } });
    await flushPromises();

    await wrapper.get('[aria-label="关闭 api.example.test 的连接"]').trigger("click");
    await flushPromises();
    expect(connectionApi.close).toHaveBeenCalledWith("connection-1");
    expect(wrapper.findAll(".connections-table tbody tr")).toHaveLength(1);

    await wrapper.findAll('[role="tab"]')[1]!.trigger("click");
    expect(wrapper.text()).toContain("api.example.test");
    expect(wrapper.findAll(".connections-table tbody tr")).toHaveLength(1);
    await wrapper.get('[aria-label="清除已关闭记录"]').trigger("click");
    expect(wrapper.text()).toContain("尚无已关闭记录");
    wrapper.unmount();
  });

  it("无搜索条件时一次关闭全部活动连接", async () => {
    const connectionApi = api();
    const wrapper = mount(ConnectionsView, { props: { api: connectionApi, pollInterval: 60_000 } });
    await flushPromises();

    await wrapper.get('[aria-label="关闭筛选出的连接"]').trigger("click");
    await flushPromises();
    expect(connectionApi.closeAll).toHaveBeenCalledOnce();
    expect(wrapper.text()).toContain("当前没有活动连接");
    expect(wrapper.findAll('[role="tab"]')[1]!.text()).toContain("2");
    wrapper.unmount();
  });

  it("核心停止后不继续把旧快照标记为活动", async () => {
    const connectionApi = api();
    vi.mocked(connectionApi.list)
      .mockResolvedValueOnce(snapshot([first]))
      .mockRejectedValueOnce(new ApiError(409, { code: "core_not_running", message: "sing-box 核心当前未运行" }));
    const wrapper = mount(ConnectionsView, { props: { api: connectionApi, pollInterval: 60_000 } });
    await flushPromises();
    expect(wrapper.findAll(".connections-table tbody tr")).toHaveLength(1);

    await wrapper.get('[aria-label="立即刷新连接"]').trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("sing-box 核心当前未运行");
    expect(wrapper.text()).toContain("当前没有活动连接");
    expect(wrapper.findAll('[role="tab"]')[1]!.text()).toContain("1");
    wrapper.unmount();
  });

  it("保存并恢复显示字段偏好", async () => {
    const wrapper = mount(ConnectionsView, { props: { api: api(), pollInterval: 60_000 } });
    await flushPromises();
    await wrapper.get('[aria-label="设置显示字段"]').trigger("click");
    const dialog = wrapper.get('dialog[aria-label="连接列表设置"]');
    const checkboxes = dialog.findAll('input[type="checkbox"]');
    await checkboxes[0]!.setValue(false);
    await dialog.get("button:not(.secondary)").trigger("click");
    expect(JSON.parse(localStorage.getItem("singbox-connections-columns") ?? "[]")).not.toContain("type");
    expect(wrapper.findAll(".connections-table th button").map((button) => button.text())).not.toContain("类型");
    wrapper.unmount();
  });
});
