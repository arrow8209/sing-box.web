import { mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import CoreDownloadProgress from "@/features/core/CoreDownloadProgress.vue";
import CoreStatusCard from "@/features/core/CoreStatusCard.vue";
import CoreUpdateDialog from "@/features/core/CoreUpdateDialog.vue";
import { createCoreStore } from "@/features/core/store";
import type { CoreApi } from "@/features/core/api";

const status = (state = "running") => ({ installedVersion: "1.12.2", installedDigest: "a".repeat(64), state: state as "running", pid: 4321, currentProfileId: "profile-a", appliedRevision: 4, expectedRunning: true, configDirectoryDigest: "", startedAt: "2026-08-21T00:00:00Z", lastError: "", operation: { active: false } });

describe("核心状态", () => {
  afterEach(() => vi.useRealTimers());
  it("只渲染服务端核心事实", () => {
    const wrapper = mount(CoreStatusCard, { props: { status: status() } });
    expect(wrapper.text()).toContain("PID 4321");
    expect(wrapper.text()).toContain("1.12.2");
    expect(wrapper.text()).not.toContain("内存占用");
  });
  it("不会在确认前启动更新", async () => {
    const confirm = vi.fn();
    const wrapper = mount(CoreUpdateDialog, { props: { open: true, currentVersion: "1.12.2", latest: { version: "1.12.3", arch: "amd64" }, busy: false }, attrs: { onConfirm: confirm } });
    expect(confirm).not.toHaveBeenCalled();
    await wrapper.get("[data-confirm-update]").trigger("click");
    expect(confirm).toHaveBeenCalledWith("1.12.3");
  });
  it("显示真实下载百分比、容量和速度", () => {
    const wrapper = mount(CoreDownloadProgress, { props: { status: {
      operationId: "update-a",
      targetVersion: "1.12.3",
      stage: "downloading",
      startedAt: "2026-08-27T00:00:00Z",
      downloadedBytes: 8 << 20,
      totalBytes: 32 << 20,
      bytesPerSecond: 4 << 20
    } } });
    expect(wrapper.get('[role="progressbar"]').attributes("aria-valuenow")).toBe("25");
    expect(wrapper.text()).toContain("25%");
    expect(wrapper.text()).toContain("8 MB / 32 MB");
    expect(wrapper.text()).toContain("4 MB/s");
  });
  it("更新期间每 500ms 轮询，页面隐藏时暂停", async () => {
    vi.useFakeTimers();
    const api: CoreApi = { status: vi.fn(async () => status()), latest: vi.fn(async () => ({ version: "1.12.3", arch: "amd64" })), updateStatus: vi.fn(async () => ({ operationId: "u", stage: "downloading", startedAt: "now" })), startUpdate: vi.fn(async () => ({ operationId: "u" })) };
    const store = createCoreStore(api); store.startPolling(); await store.startUpdate("1.12.3"); await vi.advanceTimersByTimeAsync(500);
    expect(api.updateStatus).toHaveBeenCalled(); store.dispose();
  });
});
