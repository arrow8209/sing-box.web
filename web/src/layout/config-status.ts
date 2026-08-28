import type { CoreStatus } from "@/features/core/types";
import type { ProfileSummary } from "@/features/profiles/types";

export type ProfileIndicatorTone = "success" | "warning";

export interface ProfileIndicator {
  icon: "power" | "config-loaded" | "config-pending";
  tone: ProfileIndicatorTone;
  label: string;
}

export interface ProfileConfigIndicators {
  active: ProfileIndicator | null;
  pending: ProfileIndicator | null;
}

// configCurrent is written with the atomic config copy. The fallbacks keep
// profiles created before that field existed useful until their next apply,
// while still selecting at most one profile.
export function currentConfigProfileId(profiles: readonly ProfileSummary[], core: CoreStatus | null): string | null {
  const marked = profiles.find((profile) => profile.configCurrent);
  if (marked) return marked.id;

  let latest: ProfileSummary | undefined;
  let latestTime = Number.NEGATIVE_INFINITY;
  for (const profile of profiles) {
    if (profile.appliedRevision === null) continue;
    const appliedTime = profile.appliedAt ? new Date(profile.appliedAt).getTime() : Number.NEGATIVE_INFINITY;
    if (!latest || (Number.isFinite(appliedTime) && appliedTime > latestTime)) {
      latest = profile;
      latestTime = appliedTime;
    }
  }
  if (latest) return latest.id;
  return profiles.some((profile) => profile.id === core?.currentProfileId) ? core?.currentProfileId ?? null : null;
}

export function profileConfigIndicators(
  profile: ProfileSummary,
  core: CoreStatus | null,
  currentProfileId: string | null,
  locallyModified = false
): ProfileConfigIndicators {
  const current = profile.id === currentProfileId;
  const started = current && core?.state === "running" && core.currentProfileId === profile.id;
  const modified = locallyModified || profile.needsApply || profile.appliedRevision !== profile.draftRevision;
  return {
    active: !current ? null : started
      ? { icon: "power", tone: "success", label: "已启动：sing-box 正在使用此配置" }
      : { icon: "config-loaded", tone: "success", label: "应用中：此配置已同步到 config 目录" },
    pending: !modified ? null : {
      icon: "config-pending",
      tone: "warning",
      label: locallyModified ? "有修改尚未保存，也未同步到 config 目录" : "有修改尚未同步到 config 目录"
    }
  };
}
