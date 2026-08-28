import { describe, expect, it } from "vitest";
import type { CoreStatus } from "@/features/core/types";
import type { ProfileSummary } from "@/features/profiles/types";
import { currentConfigProfileId, profileConfigIndicators } from "@/layout/config-status";

const profile = (id = "profile-a", needsApply = false, configCurrent = false): ProfileSummary => ({
  id, name: id, draftRevision: needsApply ? 5 : 4, appliedRevision: 4,
  appliedDigest: "a", appliedAt: id === "profile-a" ? "2026-08-28T00:00:00Z" : "2026-08-28T01:00:00Z",
  configCurrent, needsApply, lastValidation: null
});
const core = (state: CoreStatus["state"], currentProfileId = "profile-a"): CoreStatus => ({
  installedVersion: "1.13.0", installedDigest: "a", state, pid: state === "running" ? 42 : 0,
  currentProfileId, appliedRevision: currentProfileId ? 4 : null, expectedRunning: state === "running",
  configDirectoryDigest: "b", startedAt: null, lastError: "", operation: { active: false }
});

describe("侧栏配置状态图标", () => {
  it("只为当前 config 档案显示应用图标", () => {
    const profiles = [profile("profile-a", false, true), profile("profile-b")];
    const current = currentConfigProfileId(profiles, core("stopped", ""));
    expect(current).toBe("profile-a");
    expect(profileConfigIndicators(profiles[0]!, core("stopped", ""), current).active).toMatchObject({ icon: "config-loaded", tone: "success" });
    expect(profileConfigIndicators(profiles[1]!, core("stopped", ""), current).active).toBeNull();
  });

  it("当前 config 同时被核心加载时显示已启动图标", () => {
    const current = profile("profile-a", false, true);
    expect(profileConfigIndicators(current, core("running"), current.id).active).toMatchObject({ icon: "power", label: expect.stringContaining("已启动") });
  });

  it("修改未同步时独立显示第二个图标", () => {
    const current = profile("profile-a", true, true);
    const indicators = profileConfigIndicators(current, core("running"), current.id);
    expect(indicators.active).toMatchObject({ icon: "power" });
    expect(indicators.pending).toMatchObject({ icon: "config-pending", tone: "warning", label: expect.stringContaining("config 目录") });
  });

  it("旧数据回退到最近应用档案且本地修改立即显示待同步", () => {
    const profiles = [profile("profile-a"), profile("profile-b")];
    const current = currentConfigProfileId(profiles, core("stopped", ""));
    expect(current).toBe("profile-b");
    expect(profileConfigIndicators(profiles[0]!, core("stopped", ""), current, true).pending?.label).toContain("尚未保存");
  });
});
