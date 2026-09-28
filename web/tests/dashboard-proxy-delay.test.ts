import { enableAutoUnmount, flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import DashboardProxySelector from "@/features/dashboard/DashboardProxySelector.vue";
import { proxyGroupsApi, type ProxyDelayResult, type RuntimeProxyGroup } from "@/features/dashboard/api";
import { runtimeDelayGraph } from "@/features/dashboard/use-proxy-delay";
import { clearCSRFToken, request } from "@/api/client";

enableAutoUnmount(afterEach);
afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals(); clearCSRFToken(); });

function fixture(count = 3) {
  const tags = Array.from({ length: count }, (_, index) => `节点 ${index + 1}`);
  const groups: RuntimeProxyGroup[] = [{ tag: "主组", type: "Selector", runtimeKey: "run-1", selected: tags[0]!, members: tags.map((tag) => ({ tag, type: "Trojan" })) }];
  const api = {
    list: vi.fn(async () => structuredClone(groups)),
    select: vi.fn(async (tag: string, selected: string) => ({ tag, selected })),
    testDelay: vi.fn(async (tag: string, runtimeKey: string, _signal: AbortSignal): Promise<ProxyDelayResult> => ({ tag, runtimeKey, status: "ok", delay: 182, testedAt: "2026-09-28T02:00:00Z" }))
  };
  return { api, groups, tags };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}

describe("真实节点延迟", () => {
  it("最多同时测试五个节点，逐个显示结果和排队进度", async () => {
    const { api, tags } = fixture(8);
    const requests = tags.map(() => deferred<ProxyDelayResult>());
    api.testDelay.mockImplementation((tag) => requests[tags.indexOf(tag)]!.promise);
    const wrapper = mount(DashboardProxySelector, { props: { api } });
    await flushPromises();
    await wrapper.get(".proxy-selector-actions button").trigger("click");
    expect(api.testDelay).toHaveBeenCalledTimes(5);
    expect(wrapper.findAll(".proxy-latency.testing")).toHaveLength(5);
    expect(wrapper.findAll(".proxy-latency.queued")).toHaveLength(3);
    expect(wrapper.get('[role="status"]').text()).toContain("0 / 8");
    requests[2]!.resolve({ tag: tags[2]!, runtimeKey: "run-1", status: "ok", delay: 346, testedAt: "2026-09-28T02:00:00Z" });
    await flushPromises();
    expect(api.testDelay).toHaveBeenCalledTimes(6);
    expect(wrapper.get('[data-proxy-member-tag="节点 3"] .proxy-latency').text()).toBe("346 ms");
    expect(wrapper.get('[role="status"]').text()).toContain("1 / 8");
    for (const [index, value] of requests.entries()) value.resolve({ tag: tags[index]!, runtimeKey: "run-1", status: "ok", delay: 100 + index, testedAt: "2026-09-28T02:00:00Z" });
    await flushPromises();
    expect(api.testDelay).toHaveBeenCalledTimes(8);
    expect(wrapper.text()).toContain("最近测速成功 8 个");
    expect(wrapper.emitted("notice")).toEqual([["全部节点测速完成：成功 8，超时 0，失败 0。"]]);
  });

  it("嵌套和隐藏组按真实成员展开，Direct 可测，循环引用和 Block 不重复测试", async () => {
    const { api, groups } = fixture(2);
    groups[0]!.members = [{ tag: "隐藏组", type: "URLTest" }, { tag: "节点 1", type: "Trojan" }, { tag: "direct", type: "Direct" }];
    groups[0]!.selected = "隐藏组";
    groups.push({ tag: "隐藏组", type: "URLTest", runtimeKey: "run-1", selected: "节点 2", members: [
      { tag: "节点 2", type: "Trojan", history: { delay: 42, testedAt: "2026-09-28T01:00:00Z" } },
      { tag: "主组", type: "Selector" }, { tag: "block", type: "Reject" }
    ] });
    const wrapper = mount(DashboardProxySelector, { props: { api, editorState: { groups: [{ tag: "隐藏组", hidden: true }] } } });
    await flushPromises();
    expect(wrapper.findAll(".proxy-group")).toHaveLength(1);
    const nested = wrapper.get('[data-proxy-member-tag="隐藏组"] .proxy-latency');
    expect(nested.element.tagName).toBe("SPAN");
    expect(nested.text()).toBe("42 ms");
    await wrapper.get('[aria-label="测试 主组 延迟"]').trigger("click");
    await flushPromises();
    expect(api.testDelay.mock.calls.map(([tag]) => tag)).toEqual(["节点 2", "节点 1", "direct"]);
    expect(nested.text()).toBe("182 ms");
    expect(api.select).not.toHaveBeenCalled();
    groups[1]!.selected = "主组";
    expect(runtimeDelayGraph(groups).selectedLeaf("主组")).toBeUndefined();
  });

  it("刷新显示真实历史，旧轮询不会覆盖新失败，新历史可以恢复结果", async () => {
    const { api, groups } = fixture(1);
    groups[0]!.members[0]!.history = { delay: 75, testedAt: "2026-09-28T01:00:00Z" };
    const wrapper = mount(DashboardProxySelector, { props: { api } });
    await flushPromises();
    expect(wrapper.get(".proxy-latency").text()).toBe("75 ms");
    api.testDelay.mockResolvedValueOnce({ tag: "节点 1", runtimeKey: "run-1", status: "timeout", testedAt: "2026-09-28T02:00:00Z" });
    await wrapper.get(".proxy-latency").trigger("click");
    await flushPromises();
    await wrapper.vm.refresh();
    await flushPromises();
    expect(wrapper.get(".proxy-latency").text()).toBe("超时");
    groups[0]!.members[0]!.history = { delay: 209, testedAt: "2026-09-28T03:00:00Z" };
    await wrapper.vm.refresh();
    await flushPromises();
    expect(wrapper.get(".proxy-latency").text()).toBe("209 ms");
    groups[0]!.runtimeKey = "run-2";
    delete groups[0]!.members[0]!.history;
    await wrapper.vm.refresh();
    await flushPromises();
    expect(wrapper.get(".proxy-latency").text()).toBe("未测速");
  });

  it("停止核心时不接受草稿中的 latency，并禁用测速", async () => {
    const { api } = fixture(1);
    const wrapper = mount(DashboardProxySelector, { props: { api, running: false, outbounds: [
      { type: "selector", tag: "主组", outbounds: ["节点 1"] }, { type: "trojan", tag: "节点 1", latency: 35 }
    ] } });
    expect(wrapper.get(".proxy-latency").text()).toBe("未测速");
    expect(wrapper.get(".proxy-selector-actions button").attributes("disabled")).toBeDefined();
    await wrapper.get(".proxy-latency").trigger("click");
    expect(api.testDelay).not.toHaveBeenCalled();
    await wrapper.setProps({ running: true, busy: true });
    expect(api.list).not.toHaveBeenCalled();
  });

  it("控制接口故障停止排队，不假报节点超时，也不发出成功提示", async () => {
    const { api } = fixture(7);
    const pending = deferred<ProxyDelayResult>();
    api.testDelay.mockImplementation(() => pending.promise);
    const wrapper = mount(DashboardProxySelector, { props: { api } });
    await flushPromises();
    await wrapper.get(".proxy-selector-actions button").trigger("click");
    pending.reject(new Error("控制 API 不可用"));
    await flushPromises();
    expect(api.testDelay).toHaveBeenCalledTimes(5);
    expect(api.testDelay.mock.calls.every(([, , signal]) => signal.aborted)).toBe(true);
    expect(wrapper.get('[role="alert"]').text()).toContain("控制 API 不可用");
    expect(wrapper.findAll(".proxy-latency.timeout")).toHaveLength(0);
    expect(wrapper.emitted("notice")).toBeUndefined();
    expect(wrapper.emitted("error")).toEqual([["控制 API 不可用"]]);
  });

  it("运行标识变化会取消排队和在途请求，晚到的结果不能更新页面", async () => {
    const { api, groups, tags } = fixture(7);
    const calls = tags.map(() => deferred<ProxyDelayResult>());
    api.testDelay.mockImplementation((tag) => calls[tags.indexOf(tag)]!.promise);
    const wrapper = mount(DashboardProxySelector, { props: { api } });
    await flushPromises();
    await wrapper.get(".proxy-selector-actions button").trigger("click");
    groups[0]!.runtimeKey = "run-2";
    await wrapper.vm.refresh();
    await flushPromises();
    for (const [index, value] of calls.entries()) value.resolve({ tag: tags[index]!, runtimeKey: "run-1", status: "ok", delay: 85, testedAt: "2026-09-28T02:00:00Z" });
    await flushPromises();
    expect(api.testDelay).toHaveBeenCalledTimes(5);
    expect(api.testDelay.mock.calls.every(([, , signal]) => signal.aborted)).toBe(true);
    expect(wrapper.findAll(".proxy-latency").every((node) => node.text() === "未测速")).toBe(true);
    expect(wrapper.emitted("notice")).toBeUndefined();
  });

  it("浏览器等待上限到达后停止任务，不把请求超时冒充节点测量", async () => {
    vi.useFakeTimers();
    const { api } = fixture(1);
    api.testDelay.mockImplementation((_tag, _key, signal) => new Promise((_resolve, reject) => {
      signal.addEventListener("abort", () => reject(new Error("aborted")), { once: true });
    }));
    const wrapper = mount(DashboardProxySelector, { props: { api } });
    await flushPromises();
    await wrapper.get(".proxy-latency").trigger("click");
    await vi.advanceTimersByTimeAsync(9000);
    await flushPromises();
    expect(wrapper.get('[role="alert"]').text()).toContain("测速请求未完成");
    expect(wrapper.get(".proxy-latency").text()).toBe("未测速");
    expect(wrapper.emitted("notice")).toBeUndefined();
  });

  it.each([
    { status: "ok", delay: 0 }, { status: "ok", delay: -5 }, { status: "ok", delay: 1.5 },
    { status: "ok" }, { status: "made-up", delay: 30 }, { status: "ok", delay: 30, runtimeKey: "old" }
  ])("无效测量结果不会成为可用节点：%o", async (patch) => {
    const { api } = fixture(1);
    api.testDelay.mockResolvedValueOnce({ tag: "节点 1", runtimeKey: "run-1", testedAt: "2026-09-28T02:00:00Z", ...patch } as ProxyDelayResult);
    const wrapper = mount(DashboardProxySelector, { props: { api } });
    await flushPromises();
    await wrapper.get(".proxy-latency").trigger("click");
    await flushPromises();
    expect(wrapper.get('[role="alert"]').text()).toContain("核心测速结果无效");
    expect(wrapper.emitted("notice")).toBeUndefined();
  });

  it("测速 API 使用 POST、CSRF 和取消信号，不向浏览器传控制器参数", async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ csrfToken: "test-csrf" }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await request("/auth/session");
    fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ tag: "节点 /?#", runtimeKey: "run-1", status: "ok", delay: 123, testedAt: "2026-09-28T02:00:00Z" })));
    const controller = new AbortController();
    const result = await proxyGroupsApi.testDelay("节点 /?#", "run-1", controller.signal);
    const [url, options] = (fetchMock.mock.calls as unknown as Array<[string, RequestInit]>)[1]!;
    expect(url).toBe("/api/v1/runtime/proxies/delay");
    expect(options.method).toBe("POST");
    expect(options.signal).toBe(controller.signal);
    expect(new Headers(options.headers).get("X-CSRF-Token")).toBe("test-csrf");
    expect(JSON.parse(String(options.body))).toEqual({ tag: "节点 /?#", runtimeKey: "run-1" });
    expect(result.delay).toBe(123);
  });
});
