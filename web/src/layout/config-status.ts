import type { CoreStatus } from "@/features/core/types";
import type { ProfileSummary } from "@/features/profiles/types";

export type ProfileConfigState = "stopped" | "transition" | "unloaded" | "loaded" | "modified";
export type ProfileConfigTone = "danger" | "neutral" | "success" | "warning";

export interface ProfileConfigStatus {
  state: ProfileConfigState;
  icon: "power-off" | "core-transition" | "config-unloaded" | "config-loaded" | "config-pending";
  tone: ProfileConfigTone;
  label: string;
}

export function profileConfigStatus(profile: ProfileSummary, core: CoreStatus | null): ProfileConfigStatus {
  if (core?.state === "starting" || core?.state === "stopping") {
    return { state: "transition", icon: "core-transition", tone: "neutral", label: "sing-box 正在切换运行状态，尚未确认加载配置" };
  }
  if (core?.state !== "running") {
    return { state: "stopped", icon: "power-off", tone: "danger", label: "sing-box 未启动，也未加载配置" };
  }
  if (core.currentProfileId !== profile.id) {
    return { state: "unloaded", icon: "config-unloaded", tone: "neutral", label: "sing-box 已启动，但未加载此配置" };
  }
  if (profile.needsApply || core.appliedRevision !== profile.draftRevision) {
    return { state: "modified", icon: "config-pending", tone: "warning", label: "sing-box 已启动并加载了此配置；配置有修改，尚未同步" };
  }
  return { state: "loaded", icon: "config-loaded", tone: "success", label: "sing-box 已启动，并已加载最新配置" };
}
