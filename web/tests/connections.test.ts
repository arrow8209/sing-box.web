import { enableAutoUnmount, flushPromises, mount } from "@vue/test-utils";
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

enableAutoUnmount(afterEach);
afterEach(() => { localStorage.clear(); vi.useRealTimers(); vi.restoreAllMocks(); });

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

  it("默认同时展示活动和已关闭连接，并可筛选和清除历史", async () => {
    const connectionApi = api();
    const wrapper = mount(ConnectionsView, { props: { api: connectionApi, pollInterval: 60_000 } });
    await flushPromises();

    await wrapper.get('[aria-label="关闭 api.example.test 的连接"]').trigger("click");
    await flushPromises();
    expect(connectionApi.close).toHaveBeenCalledWith("connection-1");
    expect(wrapper.findAll(".connections-table tbody tr")).toHaveLength(2);
    expect(wrapper.findAll(".connection-state.closed")).toHaveLength(1);
    expect(wrapper.find('[aria-label="关闭 api.example.test 的连接"]').exists()).toBe(false);

    await wrapper.findAll('[role="tab"]')[1]!.trigger("click");
    expect(wrapper.findAll(".connections-table tbody tr")).toHaveLength(1);
    expect(wrapper.text()).toContain("dns.example.test");
    await wrapper.findAll('[role="tab"]')[2]!.trigger("click");
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
    expect(wrapper.findAll(".connection-state.closed")).toHaveLength(2);
    expect(wrapper.findAll('[role="tab"]')[2]!.text()).toContain("2");
    await wrapper.findAll('[role="tab"]')[1]!.trigger("click");
    expect(wrapper.text()).toContain("当前没有活动连接");
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
    expect(wrapper.findAll(".connection-state.closed")).toHaveLength(1);
    expect(wrapper.findAll('[role="tab"]')[1]!.text()).toContain("0");
    expect(wrapper.findAll('[role="tab"]')[2]!.text()).toContain("1");
    wrapper.unmount();
  });

  it("保存并恢复显示字段偏好", async () => {
    const wrapper = mount(ConnectionsView, { props: { api: api(), pollInterval: 60_000 } });
    await flushPromises();
    await wrapper.get('[aria-label="连接列表设置"]').trigger("click");
    const dialog = wrapper.get('dialog[aria-label="连接列表设置"]');
    await dialog.get('[aria-label="显示类型"]').setValue(false);
    await dialog.get(".app-dialog__footer button:last-child").trigger("click");
    expect(JSON.parse(localStorage.getItem("singbox-connections-columns") ?? "[]")).not.toContain("type");
    expect(wrapper.findAll(".connections-table th button").map((button) => button.text())).not.toContain("类型");
    wrapper.unmount();
  });

  it("从断开时起保留三分钟，暂停刷新期间仍会过期并关闭历史详情", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-09-10T08:00:00Z"));
    const connectionApi = api(snapshot());
    vi.mocked(connectionApi.list).mockResolvedValueOnce(snapshot([first])).mockResolvedValue(snapshot([second]));
    const wrapper = mount(ConnectionsView, { props: { api: connectionApi, pollInterval: 600_000 } });
    await flushPromises();
    await wrapper.get('[aria-label="立即刷新连接"]').trigger("click");
    await flushPromises();
    expect(wrapper.findAll(".connections-table tbody tr")).toHaveLength(2);
    expect(wrapper.get('[data-connection-id="connection-1"] .connection-state').text()).toBe("已关闭");
    expect(wrapper.get('[data-connection-id="connection-1"] .upload-speed').text()).toBe("0 B/s");

    await wrapper.get('[aria-label="暂停实时刷新"]').trigger("click");
    await wrapper.get('[aria-label="查看 api.example.test 的连接详情"]').trigger("click");
    await vi.advanceTimersByTimeAsync(179_999);
    expect(wrapper.find('[data-connection-id="connection-1"]').exists()).toBe(true);
    expect(wrapper.get('dialog[aria-label="连接详情"]').attributes()).toHaveProperty("open");
    await vi.advanceTimersByTimeAsync(1);
    expect(wrapper.find('[data-connection-id="connection-1"]').exists()).toBe(false);
    expect(wrapper.find('[data-connection-id="connection-2"]').exists()).toBe(true);
    expect(wrapper.get('dialog[aria-label="连接详情"]').attributes()).not.toHaveProperty("open");
    expect(connectionApi.list).toHaveBeenCalledTimes(2);
  });

  it("保留时间可保存和恢复，缩短后立即清理过期历史，取消不会应用修改", async () => {
    vi.useFakeTimers();
    const connectionApi = api(snapshot([]));
    vi.mocked(connectionApi.list).mockResolvedValueOnce(snapshot([first]));
    const wrapper = mount(ConnectionsView, { props: { api: connectionApi, pollInterval: 600_000 } });
    await flushPromises();
    await wrapper.get('[aria-label="立即刷新连接"]').trigger("click");
    await flushPromises();
    await vi.advanceTimersByTimeAsync(90_000);
    await wrapper.get('[aria-label="连接列表设置"]').trigger("click");
    const dialog = wrapper.get('dialog[aria-label="连接列表设置"]');
    const duration = dialog.get('input[aria-label="历史保留时间（分钟）"]');
    const save = dialog.get(".app-dialog__footer button:last-child");
    for (const invalid of ["", "0", "1.5", "61"]) {
      await duration.setValue(invalid);
      expect(save.attributes()).toHaveProperty("disabled");
    }
    await duration.setValue("1");
    await save.trigger("click");
    expect(wrapper.find('[data-connection-id="connection-1"]').exists()).toBe(false);
    expect(localStorage.getItem("singbox-connections-history-minutes")).toBe("1");
    expect(wrapper.get(".history-summary").text()).toContain("1 分钟");

    await wrapper.get('[aria-label="连接列表设置"]').trigger("click");
    await duration.setValue("5");
    await dialog.get('[aria-label="显示类型"]').setValue(false);
    await dialog.findAll(".app-dialog__footer button")[1]!.trigger("click");
    expect(wrapper.get(".history-summary").text()).toContain("1 分钟");
    await wrapper.get('[aria-label="连接列表设置"]').trigger("click");
    expect((duration.element as HTMLInputElement).value).toBe("1");
    expect((dialog.get('[aria-label="显示类型"]').element as HTMLInputElement).checked).toBe(true);
    wrapper.unmount();

    const reopened = mount(ConnectionsView, { props: { api: api(snapshot([])), pollInterval: 600_000 } });
    await flushPromises();
    expect(reopened.get(".history-summary").text()).toContain("1 分钟");
    expect(reopened.findAll(".connections-table tbody tr")).toHaveLength(0);
    expect(JSON.stringify(localStorage)).not.toContain("api.example.test");
  });

  it("全部列表批量关闭时只请求仍活动的连接", async () => {
    const connectionApi = api(snapshot([first]));
    vi.mocked(connectionApi.list).mockResolvedValueOnce(snapshot());
    const wrapper = mount(ConnectionsView, { props: { api: connectionApi, pollInterval: 600_000 } });
    await flushPromises();
    await wrapper.get('[aria-label="立即刷新连接"]').trigger("click");
    await flushPromises();
    await wrapper.get('[aria-label="搜索连接"]').setValue("example.test");
    await wrapper.get('[aria-label="关闭筛选出的连接"]').trigger("click");
    await flushPromises();
    expect(connectionApi.close).toHaveBeenCalledExactlyOnceWith(first.id);
    expect(connectionApi.closeAll).not.toHaveBeenCalled();
    expect(wrapper.findAll(".connection-state.closed")).toHaveLength(2);
    expect(wrapper.get('[aria-label="关闭筛选出的连接"]').attributes()).toHaveProperty("disabled");
  });

  it("同一连接重新出现在快照中时去除旧历史，避免重复展示", async () => {
    const connectionApi = api(snapshot([first]));
    vi.mocked(connectionApi.list).mockResolvedValueOnce(snapshot([first])).mockResolvedValueOnce(snapshot([]));
    const wrapper = mount(ConnectionsView, { props: { api: connectionApi, pollInterval: 600_000 } });
    await flushPromises();
    await wrapper.get('[aria-label="立即刷新连接"]').trigger("click");
    await flushPromises();
    expect(wrapper.findAll(".connection-state.closed")).toHaveLength(1);
    await wrapper.get('[aria-label="立即刷新连接"]').trigger("click");
    await flushPromises();
    expect(wrapper.findAll(".connections-table tbody tr")).toHaveLength(1);
    expect(wrapper.findAll(".connection-state.closed")).toHaveLength(0);
  });

  it("页面卸载后停止轮询并丢弃延迟返回的快照，重新进入不恢复历史", async () => {
    vi.useFakeTimers();
    let finish: (value: ConnectionsSnapshot) => void = () => undefined;
    const connectionApi = api(snapshot([]));
    vi.mocked(connectionApi.list).mockResolvedValueOnce(snapshot([first])).mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
    const wrapper = mount(ConnectionsView, { props: { api: connectionApi, pollInterval: 1000 } });
    await flushPromises();
    await wrapper.get('[aria-label="立即刷新连接"]').trigger("click");
    wrapper.unmount();
    expect(vi.getTimerCount()).toBe(0);
    finish(snapshot([]));
    await flushPromises();
    document.dispatchEvent(new Event("visibilitychange"));
    await vi.advanceTimersByTimeAsync(5000);
    expect(connectionApi.list).toHaveBeenCalledTimes(2);

    const reopened = mount(ConnectionsView, { props: { api: connectionApi, pollInterval: 1000 } });
    await flushPromises();
    expect(reopened.findAll(".connections-table tbody tr")).toHaveLength(0);
    expect(reopened.findAll('[role="tab"]')[2]!.text()).toContain("0");
  });

  it("三分钟以内的历史不会因超过五百条而提前丢弃", async () => {
    localStorage.setItem("singbox-connections-columns", JSON.stringify(["status"]));
    const connections = Array.from({ length: 501 }, (_, index) => ({ ...first, id: `history-${index}` }));
    const connectionApi = api(snapshot([]));
    vi.mocked(connectionApi.list).mockResolvedValueOnce(snapshot(connections));
    const wrapper = mount(ConnectionsView, { props: { api: connectionApi, pollInterval: 600_000 } });
    await flushPromises();
    await wrapper.get('[aria-label="立即刷新连接"]').trigger("click");
    await flushPromises();
    expect(wrapper.findAll(".connection-state.closed")).toHaveLength(501);
  });

  it("从浏览器页面缓存恢复时重新开始记录，忽略离开前未完成的请求", async () => {
    vi.useFakeTimers();
    let finish: (value: ConnectionsSnapshot) => void = () => undefined;
    const connectionApi = api(snapshot([]));
    vi.mocked(connectionApi.list)
      .mockResolvedValueOnce(snapshot([first]))
      .mockResolvedValueOnce(snapshot([]))
      .mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
    const wrapper = mount(ConnectionsView, { props: { api: connectionApi, pollInterval: 1000 } });
    await flushPromises();
    await wrapper.get('[aria-label="立即刷新连接"]').trigger("click");
    await flushPromises();
    expect(wrapper.findAll(".connection-state.closed")).toHaveLength(1);
    await wrapper.get('[aria-label="立即刷新连接"]').trigger("click");

    window.dispatchEvent(new PageTransitionEvent("pagehide", { persisted: true }));
    await flushPromises();
    expect(wrapper.findAll(".connections-table tbody tr")).toHaveLength(0);
    expect(vi.getTimerCount()).toBe(0);
    window.dispatchEvent(new PageTransitionEvent("pageshow", { persisted: true }));
    await flushPromises();
    expect(connectionApi.list).toHaveBeenCalledTimes(4);
    finish(snapshot([first]));
    await flushPromises();
    expect(wrapper.findAll(".connections-table tbody tr")).toHaveLength(0);
    await vi.advanceTimersByTimeAsync(1000);
    expect(connectionApi.list).toHaveBeenCalledTimes(5);
  });
});
