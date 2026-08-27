import { mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import CoreVersionPanel from "@/features/core/CoreVersionPanel.vue";
import { createCoreStore } from "@/features/core/store";
import type { CoreApi } from "@/features/core/api";
import type { CoreProcessState } from "@/features/core/types";

const status = (state: CoreProcessState = "running") => ({ installedVersion: "1.12.2", installedDigest: "a".repeat(64), state, pid: 4321, currentProfileId: "profile-a", appliedRevision: 4, expectedRunning: true, configDirectoryDigest: "", startedAt: "2026-08-21T00:00:00Z", lastError: "", operation: { active: false } });

describe("核心状态", () => {
  afterEach(() => vi.useRealTimers());
  it("按根菜单原型展示当前版本、最新稳定版和下载入口", async () => {
    const wrapper = mount(CoreVersionPanel, {
      props: {
        status: status(),
        latest: { version: "1.12.3", arch: "amd64" },
        update: null,
        checking: false,
        busy: false,
        updateAvailable: true
      }
    });
    expect(wrapper.get("[aria-label='当前版本']").text()).toContain("1.12.2");
    expect(wrapper.get("[aria-label='最新版本']").text()).toContain("1.12.3");
    expect(wrapper.text()).toContain("发现新版本");
    expect(wrapper.text()).not.toContain("内存占用");
    await wrapper.get("[data-download-core]").trigger("click");
    expect(wrapper.emitted("download")).toEqual([["1.12.3"]]);
    await wrapper.setProps({ checking: true });
    expect(wrapper.get("[data-download-core]").text()).toContain("正在检查版本");
    expect(wrapper.get("[data-download-core]").attributes()).toHaveProperty("disabled");
  });
  it("用真实更新阶段展示流程进度", async () => {
    const wrapper = mount(CoreVersionPanel, {
      props: {
        status: status(),
        latest: { version: "1.12.3", arch: "amd64" },
        update: { operationId: "update-a", targetVersion: "1.12.3", stage: "installing", startedAt: "now" },
        checking: false,
        busy: false,
        updateAvailable: true
      }
    });
    expect(wrapper.get("[role='progressbar']").attributes("aria-valuetext")).toContain("安装核心");
    expect(wrapper.text()).toContain("安装核心");
    expect(wrapper.get("[data-download-core]").attributes()).toHaveProperty("disabled");
    await wrapper.setProps({ update: { ...wrapper.props("update")!, downloadedBytes: 32 << 20, totalBytes: 32 << 20 } });
    expect(wrapper.get("[role='progressbar']").attributes("aria-valuenow")).toBe("100");
  });
  it("下载过程中显示真实百分比、容量和速度", async () => {
    const wrapper = mount(CoreVersionPanel, {
      props: {
        status: status(),
        latest: { version: "1.12.3", arch: "amd64" },
        update: {
          operationId: "update-a",
          targetVersion: "1.12.3",
          stage: "downloading",
          startedAt: "now",
          downloadedBytes: 8 << 20,
          totalBytes: 32 << 20,
          bytesPerSecond: 4 << 20
        },
        checking: false,
        busy: false,
        updateAvailable: true
      }
    });
    expect(wrapper.get("[role='progressbar']").attributes("aria-valuenow")).toBe("25");
    expect(wrapper.text()).toContain("8 MB / 32 MB");
    expect(wrapper.get("[data-download-speed]").text()).toBe("4 MB/s");
    await wrapper.setProps({ update: { ...wrapper.props("update")!, totalBytes: 0 } });
    expect(wrapper.get("[role='progressbar']").attributes("aria-valuenow")).toBeUndefined();
    expect(wrapper.get(".core-update-progress").classes()).toContain("indeterminate");
    expect(wrapper.text()).toContain("已下载 8 MB");
  });
  it("更新期间每 500ms 轮询，页面隐藏时暂停", async () => {
    vi.useFakeTimers();
    const api: CoreApi = { status: vi.fn(async () => status()), latest: vi.fn(async () => ({ version: "1.12.3", arch: "amd64" })), updateStatus: vi.fn(async () => ({ operationId: "u", stage: "downloading", startedAt: "now" })), startUpdate: vi.fn(async () => ({ operationId: "u" })) };
    const store = createCoreStore(api); store.startPolling(); await store.startUpdate("1.12.3"); await vi.advanceTimersByTimeAsync(500);
    expect(api.updateStatus).toHaveBeenCalled(); store.dispose();
  });
  it("重新进入页面时会恢复未完成更新的轮询", async () => {
    vi.useFakeTimers();
    const api: CoreApi = { status: vi.fn(async () => status()), latest: vi.fn(async () => ({ version: "1.12.3", arch: "amd64" })), updateStatus: vi.fn(async () => ({ operationId: "u", stage: "installing", startedAt: "now" })), startUpdate: vi.fn(async () => ({ operationId: "u" })) };
    const store = createCoreStore(api);
    store.startPolling();
    await vi.advanceTimersByTimeAsync(0);
    await vi.advanceTimersByTimeAsync(500);
    expect(api.updateStatus).toHaveBeenCalledTimes(2);
    store.dispose();
  });
  it("空闲状态不会启动更新轮询", async () => {
    vi.useFakeTimers();
    const api: CoreApi = { status: vi.fn(async () => status()), latest: vi.fn(async () => ({ version: "1.12.3", arch: "amd64" })), updateStatus: vi.fn(async () => ({ operationId: "", stage: "idle", startedAt: "" })), startUpdate: vi.fn(async () => ({ operationId: "u" })) };
    const store = createCoreStore(api);
    store.startPolling();
    await vi.advanceTimersByTimeAsync(0);
    await vi.advanceTimersByTimeAsync(500);
    expect(api.updateStatus).toHaveBeenCalledTimes(1);
    store.dispose();
  });
});
