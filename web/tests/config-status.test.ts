import { describe, expect, it } from "vitest";
import type { CoreStatus } from "@/features/core/types";
import type { ProfileSummary } from "@/features/profiles/types";
import { profileConfigStatus } from "@/layout/config-status";

const profile = (needsApply = false): ProfileSummary => ({
  id: "profile-a", name: "A", draftRevision: needsApply ? 5 : 4, appliedRevision: 4,
  appliedDigest: "a", appliedAt: "2026-08-28T00:00:00Z", needsApply, lastValidation: null
});
const core = (state: CoreStatus["state"], currentProfileId = "profile-a"): CoreStatus => ({
  installedVersion: "1.13.0", installedDigest: "a", state, pid: state === "running" ? 42 : 0,
  currentProfileId, appliedRevision: currentProfileId ? 4 : null, expectedRunning: state === "running",
  configDirectoryDigest: "b", startedAt: null, lastError: "", operation: { active: false }
});

describe("侧栏配置状态图标", () => {
  it("未启动且未加载配置时使用断电图标", () => {
    expect(profileConfigStatus(profile(), core("stopped", ""))).toMatchObject({ icon: "power-off", tone: "danger", state: "stopped" });
  });

  it("运行并加载最新配置时使用勾选配置图标", () => {
    expect(profileConfigStatus(profile(), core("running"))).toMatchObject({ icon: "config-loaded", tone: "success", state: "loaded" });
  });

  it("运行配置存在未同步修改时使用编辑待同步图标", () => {
    expect(profileConfigStatus(profile(true), core("running"))).toMatchObject({ icon: "config-pending", tone: "warning", state: "modified" });
  });

  it("核心运行但未加载该档案时使用未连接配置图标", () => {
    expect(profileConfigStatus(profile(), core("running", "profile-b"))).toMatchObject({ icon: "config-unloaded", tone: "neutral", state: "unloaded" });
  });
});
