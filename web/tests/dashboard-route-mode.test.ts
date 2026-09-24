import { enableAutoUnmount, flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import DashboardRouteMode from "@/features/dashboard/DashboardRouteMode.vue";
import { runtimeModeApi, type RouteMode, type RuntimeMode } from "@/features/dashboard/api";
import { clearCSRFToken, request } from "@/api/client";

enableAutoUnmount(afterEach);
afterEach(() => { vi.useRealTimers(); clearCSRFToken(); });

function state(mode = "rule", availableModes: RouteMode[] = ["rule", "global", "direct"]): RuntimeMode {
  return { mode, availableModes };
}

function createApi(initialMode = "rule") {
  let current = state(initialMode);
  return {
    get: vi.fn(async () => current),
    set: vi.fn(async (mode: RouteMode) => { current = state(mode); return current; })
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

describe("仪表盘真实路由模式", () => {
  it("读取核心模式、切换三种模式，重新进入页面保持核心的选择", async () => {
    const api = createApi("global");
    const wrapper = mount(DashboardRouteMode, { props: { running: true, api } });
    expect(wrapper.findAll(".mode-option.active")).toHaveLength(0);
    expect(wrapper.findAll(".mode-option").every((button) => button.attributes("disabled") !== undefined)).toBe(true);
    await flushPromises();
    expect(wrapper.get(".mode-option.active").text()).toBe("全局");
    for (const [index, mode, label] of [[0, "rule", "规则"], [2, "direct", "直连"], [1, "global", "全局"]] as const) {
      await wrapper.findAll(".mode-option")[index]!.trigger("click");
      await flushPromises();
      expect(api.set).toHaveBeenLastCalledWith(mode);
      expect(wrapper.get('.mode-option[aria-pressed="true"]').text()).toBe(label);
      expect(wrapper.emitted("notice")?.at(-1)).toEqual([`已切换到${label}模式，对新连接生效。`]);
    }
    wrapper.unmount();
    const refreshed = mount(DashboardRouteMode, { props: { running: true, api } });
    await flushPromises();
    expect(refreshed.get(".mode-option.active").text()).toBe("全局");
  });

  it("等待核心确认期间保留原选择并阻止重复点击", async () => {
    const api = createApi();
    const pending = deferred<RuntimeMode>();
    api.set.mockReturnValueOnce(pending.promise);
    const wrapper = mount(DashboardRouteMode, { props: { running: true, api } });
    await flushPromises();
    await wrapper.findAll(".mode-option")[1]!.trigger("click");
    expect(wrapper.get(".mode-option.active").text()).toBe("规则");
    expect(wrapper.text()).toContain("正在切换");
    expect(wrapper.findAll(".mode-option").every((button) => button.attributes("disabled") !== undefined)).toBe(true);
    await wrapper.findAll(".mode-option")[2]!.trigger("click");
    expect(api.set).toHaveBeenCalledTimes(1);
    expect(wrapper.emitted("notice")).toBeUndefined();
    pending.resolve(state("global"));
    await flushPromises();
    expect(wrapper.get(".mode-option.active").text()).toBe("全局");
  });

  it.each(["reject", "unconfirmed"])("切换失败会读回真实模式且不报成功：%s", async (failure) => {
    const api = createApi();
    if (failure === "reject") api.set.mockRejectedValueOnce(new Error("无法访问 sing-box 运行数据"));
    else api.set.mockResolvedValueOnce(state("rule"));
    const wrapper = mount(DashboardRouteMode, { props: { running: true, api } });
    await flushPromises();
    await wrapper.findAll(".mode-option")[1]!.trigger("click");
    await flushPromises();
    expect(api.get).toHaveBeenCalledTimes(2);
    expect(wrapper.get(".mode-option.active").text()).toBe("规则");
    expect(wrapper.get('[role="alert"]').text()).toContain(failure === "reject" ? "无法访问" : "核心未确认模式切换");
    expect(wrapper.emitted("notice")).toBeUndefined();
  });

  it("响应丢失时不假定旧模式，恢复读取失败后清除选择", async () => {
    const api = createApi();
    api.set.mockRejectedValue(new Error("请求超时"));
    const wrapper = mount(DashboardRouteMode, { props: { running: true, api } });
    await flushPromises();
    api.get.mockResolvedValueOnce(state("global"));
    await wrapper.findAll(".mode-option")[1]!.trigger("click");
    await flushPromises();
    expect(wrapper.get(".mode-option.active").text()).toBe("全局");
    expect(wrapper.emitted("notice")).toBeUndefined();
    api.get.mockRejectedValueOnce(new Error("核心不可用"));
    await wrapper.findAll(".mode-option")[2]!.trigger("click");
    await flushPromises();
    expect(wrapper.findAll(".mode-option.active")).toHaveLength(0);
    expect(wrapper.text()).toContain("状态不可用");
  });

  it("读取失败可以重试，配置不支持的模式保持禁用", async () => {
    const api = createApi();
    api.get.mockRejectedValueOnce(new Error("当前应用配置未启用可用的本机 Clash API"));
    api.get.mockResolvedValue(state("rule", ["rule", "direct"]));
    const wrapper = mount(DashboardRouteMode, { props: { running: true, api } });
    await flushPromises();
    expect(wrapper.findAll(".mode-option.active")).toHaveLength(0);
    await wrapper.get(".mode-error button").trigger("click");
    await flushPromises();
    expect(wrapper.get(".mode-option.active").text()).toBe("规则");
    expect(wrapper.text()).toContain("当前配置未启用全局模式");
    expect(wrapper.findAll(".mode-option")[1]!.attributes("disabled")).toBeDefined();
    await wrapper.findAll(".mode-option")[1]!.trigger("click");
    expect(api.set).not.toHaveBeenCalled();
  });

  it("核心停止时不请求、不显示假模式，启动后重新读取", async () => {
    const api = createApi("direct");
    const wrapper = mount(DashboardRouteMode, { props: { running: false, api } });
    await flushPromises();
    expect(api.get).not.toHaveBeenCalled();
    expect(wrapper.text()).toContain("启动核心后可切换");
    expect(wrapper.findAll(".mode-option.active")).toHaveLength(0);
    await wrapper.setProps({ running: true });
    await flushPromises();
    expect(wrapper.get(".mode-option.active").text()).toBe("直连");
    await wrapper.setProps({ running: false });
    expect(wrapper.findAll(".mode-option.active")).toHaveLength(0);
  });

  it("轮询旧响应不会覆盖刚确认的模式，后续轮询可同步外部修改", async () => {
    vi.useFakeTimers();
    const api = createApi();
    const wrapper = mount(DashboardRouteMode, { props: { running: true, api } });
    await flushPromises();
    const oldRead = deferred<RuntimeMode>();
    api.get.mockReturnValueOnce(oldRead.promise);
    await vi.advanceTimersByTimeAsync(5000);
    await wrapper.findAll(".mode-option")[1]!.trigger("click");
    await flushPromises();
    oldRead.resolve(state("rule"));
    await flushPromises();
    expect(wrapper.get(".mode-option.active").text()).toBe("全局");
    api.get.mockResolvedValueOnce(state("direct"));
    await vi.advanceTimersByTimeAsync(5000);
    expect(wrapper.get(".mode-option.active").text()).toBe("直连");
    wrapper.unmount();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("核心重启和组件卸载后忽略旧的切换结果", async () => {
    const api = createApi();
    const wrapper = mount(DashboardRouteMode, { props: { running: true, runtimeKey: "old", api } });
    await flushPromises();
    const oldWrite = deferred<RuntimeMode>();
    api.set.mockReturnValueOnce(oldWrite.promise);
    await wrapper.findAll(".mode-option")[1]!.trigger("click");
    api.get.mockResolvedValueOnce(state("direct"));
    await wrapper.setProps({ runtimeKey: "new" });
    await flushPromises();
    oldWrite.resolve(state("global"));
    await flushPromises();
    expect(wrapper.get(".mode-option.active").text()).toBe("直连");
    expect(wrapper.emitted("notice")).toBeUndefined();
    const pending = deferred<RuntimeMode>();
    api.set.mockReturnValueOnce(pending.promise);
    await wrapper.findAll(".mode-option")[0]!.trigger("click");
    wrapper.unmount();
    pending.resolve(state("rule"));
    await flushPromises();
    expect(wrapper.emitted("notice")).toBeUndefined();
  });

  it("模式 API 通过认证请求客户端发送 PUT 与 CSRF，不保存草稿", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => new Response(JSON.stringify(String(input).endsWith("/auth/session") ? { csrfToken: "test-csrf" } : state("global")), { headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);
    await request("/auth/session");
    await runtimeModeApi.get();
    await runtimeModeApi.set("global");
    expect(fetchMock).toHaveBeenLastCalledWith("/api/v1/runtime/mode", expect.objectContaining({ method: "PUT", body: JSON.stringify({ mode: "global" }), credentials: "same-origin" }));
    const options = fetchMock.mock.calls.at(-1) as unknown as [RequestInfo | URL, RequestInit];
    expect((options[1].headers as Headers).get("X-CSRF-Token")).toBe("test-csrf");
    expect(fetchMock.mock.calls.some(([path]) => String(path).includes("/draft"))).toBe(false);
  });
});
