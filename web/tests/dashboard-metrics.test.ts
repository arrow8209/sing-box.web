import { enableAutoUnmount, flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import DashboardMetrics from "@/features/dashboard/DashboardMetrics.vue";
import { runtimeMetricsApi, type RuntimeMetrics } from "@/features/dashboard/api";

enableAutoUnmount(afterEach);
afterEach(() => vi.useRealTimers());

function sample(milliseconds = 0, overrides: Partial<RuntimeMetrics> = {}): RuntimeMetrics {
  return {
    uploadTotal: 1000, downloadTotal: 2000, memory: 32 * 1024 * 1024, connectionCount: 3,
    runtimeKey: "42:started", observedAt: new Date(Date.parse("2026-09-24T00:00:00Z") + milliseconds).toISOString(), ...overrides
  };
}
function createApi() { return { get: vi.fn(async (_signal?: AbortSignal) => sample()) }; }
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

describe("仪表盘真实运行指标", () => {
  it("首个样本显示真实内存和连接数，两个样本按服务端采样间隔计算速率", async () => {
    vi.useFakeTimers();
    const api = createApi();
    const wrapper = mount(DashboardMetrics, { props: { state: "running", api } });
    expect(wrapper.findAll(".hero-metric strong").map((item) => item.text())).toEqual(["--", "--", "--", "--"]);
    await flushPromises();
    expect(wrapper.get('[data-metric="memory"] strong').text()).toBe("32.0 MB");
    expect(wrapper.get('[data-metric="connections"] strong').text()).toBe("3 条");
    expect(wrapper.get('[data-metric="upload"] strong').text()).toBe("--");
    expect(wrapper.text()).toContain("正在采样流量速率");
    api.get.mockResolvedValueOnce(sample(2500, { uploadTotal: 2280, downloadTotal: 4560, memory: 48 * 1024 * 1024, connectionCount: 7 }));
    await vi.advanceTimersByTimeAsync(1000);
    expect(wrapper.get('[data-metric="upload"] strong').text()).toBe("512 B/s");
    expect(wrapper.get('[data-metric="download"] strong').text()).toBe("1.00 KB/s");
    expect(wrapper.get('[data-metric="memory"] strong').text()).toBe("48.0 MB");
    expect(wrapper.get('[data-metric="connections"] strong').text()).toBe("7 条");
    expect(wrapper.text()).toContain("每秒更新");
    expect(wrapper.get('a[href="/connections"]').text()).toContain("活动连接");
  });

  it("没有流量时显示测得的零速率和零连接，而不是演示数值", async () => {
    vi.useFakeTimers();
    const api = createApi();
    api.get.mockResolvedValueOnce(sample(0, { uploadTotal: 0, downloadTotal: 0, connectionCount: 0 }));
    const wrapper = mount(DashboardMetrics, { props: { state: "running", api } });
    await flushPromises();
    api.get.mockResolvedValueOnce(sample(1000, { uploadTotal: 0, downloadTotal: 0, connectionCount: 0 }));
    await vi.advanceTimersByTimeAsync(1000);
    expect(wrapper.get('[data-metric="upload"] strong').text()).toBe("0 B/s");
    expect(wrapper.get('[data-metric="download"] strong').text()).toBe("0 B/s");
    expect(wrapper.get('[data-metric="connections"] strong').text()).toBe("0 条");
  });

  it.each([
    { name: "计数器清零", value: sample(1000, { uploadTotal: 10, downloadTotal: 20 }) },
    { name: "核心重启后计数更大", value: sample(1000, { runtimeKey: "43:new", uploadTotal: 500000, downloadTotal: 900000 }) },
    { name: "采样时间回退", value: sample(-1000) },
    { name: "采样间隔过长", value: sample(30000) }
  ])("$name 后重新建立采样基准，不显示错误速率", async ({ value }) => {
    vi.useFakeTimers();
    const api = createApi();
    const wrapper = mount(DashboardMetrics, { props: { state: "running", api } });
    await flushPromises();
    api.get.mockResolvedValueOnce(value);
    await vi.advanceTimersByTimeAsync(1000);
    expect(wrapper.get('[data-metric="upload"] strong').text()).toBe("--");
    expect(wrapper.get('[data-metric="download"] strong').text()).toBe("--");
    expect(wrapper.get('[data-metric="memory"] strong').text()).toBe("32.0 MB");
    api.get.mockResolvedValueOnce({ ...value, uploadTotal: value.uploadTotal + 2048, downloadTotal: value.downloadTotal + 1024, observedAt: new Date(Date.parse(value.observedAt) + 1000).toISOString() });
    await vi.advanceTimersByTimeAsync(1000);
    expect(wrapper.get('[data-metric="upload"] strong').text()).toBe("2.00 KB/s");
    expect(wrapper.get('[data-metric="download"] strong').text()).toBe("1.00 KB/s");
  });

  it("失败清除旧指标，重试后从新样本开始计算", async () => {
    vi.useFakeTimers();
    const api = createApi();
    const wrapper = mount(DashboardMetrics, { props: { state: "running", api } });
    await flushPromises();
    api.get.mockRejectedValueOnce(new Error("无法访问 sing-box 运行数据"));
    await vi.advanceTimersByTimeAsync(1000);
    expect(wrapper.findAll(".hero-metric strong").map((item) => item.text())).toEqual(["--", "--", "--", "--"]);
    expect(wrapper.get('[role="alert"]').text()).toContain("无法访问 sing-box");
    api.get.mockResolvedValueOnce(sample(2000, { uploadTotal: 100000, downloadTotal: 200000 }));
    await wrapper.get(".metrics-status button").trigger("click");
    await flushPromises();
    expect(wrapper.get('[data-metric="upload"] strong').text()).toBe("--");
    expect(wrapper.get('[data-metric="memory"] strong').text()).toBe("32.0 MB");
  });

  it("缺失或非法统计不能被伪装成零", async () => {
    const api = createApi();
    api.get.mockResolvedValueOnce({ memory: 1024 } as RuntimeMetrics);
    const wrapper = mount(DashboardMetrics, { props: { state: "running", api } });
    await flushPromises();
    expect(wrapper.get('[role="alert"]').text()).toContain("运行指标不完整或无效");
    expect(wrapper.findAll(".hero-metric strong").every((item) => item.text() === "--")).toBe(true);
  });

  it("停止时不读取，重启后忽略旧请求且卸载会取消请求和定时器", async () => {
    vi.useFakeTimers();
    const api = createApi();
    const wrapper = mount(DashboardMetrics, { props: { state: "stopped", api } });
    await vi.advanceTimersByTimeAsync(2000);
    expect(api.get).not.toHaveBeenCalled();
    expect(wrapper.text()).toContain("核心未运行");
    const old = deferred<RuntimeMetrics>();
    api.get.mockReturnValueOnce(old.promise);
    await wrapper.setProps({ state: "running", runtimeKey: "old" });
    const signal = api.get.mock.calls[0]![0]!;
    await wrapper.setProps({ state: "stopping", busy: true });
    expect(signal.aborted).toBe(true);
    await wrapper.setProps({ state: "running", busy: false, runtimeKey: "new" });
    await flushPromises();
    old.resolve(sample(1000, { connectionCount: 99 }));
    await flushPromises();
    expect(wrapper.get('[data-metric="connections"] strong').text()).toBe("3 条");
    const pending = deferred<RuntimeMetrics>();
    api.get.mockReturnValueOnce(pending.promise);
    await vi.advanceTimersByTimeAsync(1000);
    const lastSignal = api.get.mock.calls.at(-1)![0]!;
    wrapper.unmount();
    expect(lastSignal.aborted).toBe(true);
    expect(vi.getTimerCount()).toBe(0);
    pending.resolve(sample(2000));
    await flushPromises();
  });

  it("隐藏页面时暂停并清除采样，返回后重新读取", async () => {
    vi.useFakeTimers();
    const hidden = vi.spyOn(document, "hidden", "get").mockReturnValue(false);
    const api = createApi();
    const wrapper = mount(DashboardMetrics, { props: { state: "running", api } });
    await flushPromises();
    hidden.mockReturnValue(true);
    document.dispatchEvent(new Event("visibilitychange"));
    await vi.advanceTimersByTimeAsync(10000);
    expect(api.get).toHaveBeenCalledTimes(1);
    expect(wrapper.get('[data-metric="memory"] strong').text()).toBe("--");
    hidden.mockReturnValue(false);
    document.dispatchEvent(new Event("visibilitychange"));
    await flushPromises();
    expect(api.get).toHaveBeenCalledTimes(2);
    expect(wrapper.get('[data-metric="upload"] strong').text()).toBe("--");
  });

  it("慢请求不会重叠，超时会中止读取并清除旧指标", async () => {
    vi.useFakeTimers();
    const api = createApi();
    const wrapper = mount(DashboardMetrics, { props: { state: "running", api } });
    await flushPromises();
    api.get.mockImplementation((signal) => new Promise((_resolve, reject) => {
      signal!.addEventListener("abort", () => reject(new DOMException("Aborted", "AbortError")), { once: true });
    }));
    await vi.advanceTimersByTimeAsync(4000);
    expect(api.get).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(2000);
    expect(wrapper.get('[role="alert"]').text()).toContain("读取超时");
    expect(wrapper.findAll(".hero-metric strong").every((item) => item.text() === "--")).toBe(true);
    expect(api.get.mock.calls[1]![0]!.aborted).toBe(true);
  });

  it("使用独立只读指标接口并传递取消信号", async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify(sample()), { headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);
    const controller = new AbortController();
    await runtimeMetricsApi.get(controller.signal);
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/runtime/metrics", expect.objectContaining({ method: "GET", signal: controller.signal, credentials: "same-origin" }));
  });
});
