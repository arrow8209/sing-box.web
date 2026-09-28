<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import AppToast from "@/components/AppToast.vue";
import { useCoreStore } from "@/features/core/store";
import { profileApi } from "@/features/profiles/api";
import { useProfileStore } from "@/features/profiles/store";
import { useCapabilitiesStore } from "@/features/capabilities/store";
import { applicationApi } from "@/features/application/api";
import type { CandidateView } from "@/features/application/types";
import type { ProfileSummary } from "@/features/profiles/types";
import AppIcon from "@/layout/AppIcon.vue";
import DashboardProxySelector from "./DashboardProxySelector.vue";
import DashboardRouteMode from "./DashboardRouteMode.vue";
import DashboardMetrics from "./DashboardMetrics.vue";
import { outboundOrderFromDraft } from "./proxy-groups";

const core = useCoreStore();
const profileStore = useProfileStore();
const capabilities = useCapabilitiesStore();
const serverResolution = computed(() => capabilities.capabilities.value?.serverReferenceResolution === true);
const stoppedCandidate = ref<CandidateView | null>(null);
const candidateError = ref("");
let candidateGeneration = 0;
const profiles = ref<ProfileSummary[]>([]);
const error = ref("");
const refreshing = ref(false);
const coreAction = ref<"start" | "stop" | "restart" | null>(null);
const notice = ref("");
const noticeTone = ref<"info" | "error">("info");
const proxySelector = ref<InstanceType<typeof DashboardProxySelector> | null>(null);
const routeModeControl = ref<InstanceType<typeof DashboardRouteMode> | null>(null);
const runtimeMetrics = ref<InstanceType<typeof DashboardMetrics> | null>(null);
const now = ref(Date.now());
let clockTimer: ReturnType<typeof setInterval> | undefined;

const serviceRunning = computed(() => ["running", "starting", "stopping"].includes(core.status.value?.state ?? ""));
const running = computed(() => serviceRunning.value);
const coreTransitioning = computed(() => Boolean(coreAction.value) || core.status.value?.state === "starting" || core.status.value?.state === "stopping" || core.status.value?.operation.active);
const coreActionLabel = computed(() => coreAction.value === "stop" || core.status.value?.state === "stopping"
  ? "正在停止"
  : coreAction.value === "start" || core.status.value?.state === "starting" ? "正在启动" : running.value ? "停止核心" : "启动核心");
const restartLabel = computed(() => coreAction.value === "restart" ? "正在重启" : "重启");
const dashboardError = computed(() => error.value || core.error.value || core.status.value?.lastError || "");
const activeProfile = computed(() => profiles.value.find((item) => item.id === core.status.value?.currentProfileId)
  ?? profiles.value.find((item) => item.id === profileStore.activeProfileId.value)
  ?? profiles.value[0]);
const draftStats = computed(() => {
  const draft = profileStore.draft.value;
  if (!draft) return { inbounds: "--", outbounds: "--", routeRules: "--" };
  const config = draft.config;
  const route = config.route as Record<string, unknown> | undefined;
  return {
    inbounds: !running.value && stoppedCandidate.value ? stoppedCandidate.value.items.filter((item) => item.module === "inbounds").length : Array.isArray(config.inbounds) ? config.inbounds.length : 0,
    outbounds: !running.value && stoppedCandidate.value ? stoppedCandidate.value.items.filter((item) => item.module === "outbounds").length : Array.isArray(config.outbounds) ? config.outbounds.length : 0,
    routeRules: Array.isArray(route?.rules) ? route.rules.length : 0
  };
});
const dashboardOutbounds = computed<unknown[]>(() => {
  if (!running.value && serverResolution.value) return stoppedCandidate.value?.items.filter((item) => item.module === "outbounds")
    .map((item) => ({ type: item.type, tag: item.tag, ...(item.members ? { outbounds: item.members } : {}) })) ?? [];
  const outbounds = profileStore.draft.value?.config.outbounds;
  return Array.isArray(outbounds) ? outbounds : [];
});
const dashboardOutboundOrder = computed(() => {
  const draft = profileStore.draft.value;
  if (!draft || (running.value && draft.profileId !== core.status.value?.currentProfileId)) return [];
  return outboundOrderFromDraft(draft);
});
async function refreshStoppedCandidate(): Promise<void> {
  const generation = ++candidateGeneration;
  stoppedCandidate.value = null; candidateError.value = "";
  if (running.value || !serverResolution.value) return;
  const id = profileStore.activeProfileId.value ?? activeProfile.value?.id;
  if (!id) return;
  try {
    const revision = profileStore.draft.value?.profileId === id && profileStore.saveState.value === "saved"
      ? profileStore.draft.value.revision : (await profileApi.getDraft(id)).data.revision;
    const result = await applicationApi.preview(id, revision);
    if (generation !== candidateGeneration || running.value || id !== (profileStore.activeProfileId.value ?? activeProfile.value?.id)) return;
    if (result.candidate?.profileId === id && result.candidate.revision === revision) stoppedCandidate.value = result.candidate;
    else candidateError.value = result.errors?.[0]?.message ?? "无法读取待运行候选";
  } catch (cause) { if (generation === candidateGeneration) candidateError.value = cause instanceof Error ? cause.message : "无法读取待运行候选"; }
}
watch([running, serverResolution, () => profileStore.activeProfileId.value, () => profileStore.draft.value?.revision], () => { void refreshStoppedCandidate(); });
const dashboardOutboundEditorState = computed(() => !running.value || profileStore.draft.value?.profileId === core.status.value?.currentProfileId
  ? profileStore.draft.value?.editorState?.outbounds : undefined);
const proxyRuntimeKey = computed(() => [core.status.value?.currentProfileId, core.status.value?.pid, core.status.value?.startedAt, core.status.value?.appliedRevision].join(":"));
const uptime = computed(() => {
  if (!running.value) return "--";
  const startedAt = core.status.value?.startedAt;
  if (!startedAt) return "--";
  const started = new Date(startedAt).getTime();
  if (!Number.isFinite(started)) return "--";
  const seconds = Math.max(0, Math.floor((now.value - started) / 1000));
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const rest = seconds % 60;
  const clock = [hours, minutes, rest].map((value) => String(value).padStart(2, "0")).join(":");
  return days ? `${days} 天 ${clock}` : clock;
});
const appliedLabel = computed(() => {
  if (!activeProfile.value?.appliedAt) return "尚未应用";
  const applied = new Date(activeProfile.value.appliedAt);
  if (!Number.isFinite(applied.getTime())) return "已应用";
  const today = new Date();
  const time = applied.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", hour12: false });
  return applied.toDateString() === today.toDateString() ? `今天 ${time}` : applied.toLocaleDateString("zh-CN");
});

function showNotice(message: string, tone: "info" | "error" = "info"): void {
  notice.value = "";
  queueMicrotask(() => { noticeTone.value = tone; notice.value = message; });
}

async function refreshProfiles(): Promise<void> {
  try {
    profiles.value = await profileApi.list();
    error.value = "";
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "无法读取仪表盘摘要";
  }
}

async function refresh(): Promise<void> {
  refreshing.value = true;
  try {
    await Promise.all([core.refresh(), refreshProfiles()]);
    await refreshStoppedCandidate();
    await Promise.all([proxySelector.value?.refresh(), routeModeControl.value?.refresh(), runtimeMetrics.value?.refresh()]);
    showNotice("状态已刷新。");
  } finally {
    refreshing.value = false;
  }
}

async function restartCore(): Promise<void> {
  if (!running.value || coreAction.value) return;
  coreAction.value = "restart";
  try {
    if (await core.restartCore()) showNotice("sing-box 核心已重新启动。");
    else showNotice(core.error.value || "重启核心失败。");
  } finally {
    coreAction.value = null;
  }
}

async function toggleCore(): Promise<void> {
  if (coreAction.value) return;
  if (!running.value) {
    coreAction.value = "start";
    try {
      if (await core.startCore()) showNotice("sing-box 核心已启动。");
      else showNotice(core.error.value || "启动核心失败。");
    } finally {
      coreAction.value = null;
    }
    return;
  }
  coreAction.value = "stop";
  try {
    if (await core.stopCore()) showNotice("sing-box 进程已结束。");
    else showNotice(core.error.value || "停止核心失败。");
  } finally {
    coreAction.value = null;
  }
}

onMounted(() => {
  core.startPolling();
  void refreshProfiles();
  clockTimer = setInterval(() => { now.value = Date.now(); }, 1000);
});

onBeforeUnmount(() => {
  candidateGeneration++;
  core.stopPolling();
  if (clockTimer) clearInterval(clockTimer);
});
</script>

<template>
  <section class="dashboard-page">
    <h1 class="sr-only">仪表盘</h1>
    <header class="page-intro">
      <div class="page-intro-copy">
        <h2>早上好，管理员</h2>
        <p v-if="running">运行指标来自 sing-box 核心，流量统计覆盖经核心转发的连接。</p>
        <p v-else>核心服务当前未运行，启动后将恢复流量转发与运行状态监控。</p>
      </div>
      <div class="page-actions">
        <button class="btn" type="button" :disabled="refreshing" @click="refresh"><AppIcon name="rotate" :size="15" />{{ refreshing ? "刷新中" : "刷新状态" }}</button>
        <a class="btn btn-primary" href="/workspace"><AppIcon name="config" :size="15" />管理配置</a>
      </div>
    </header>

    <p v-if="dashboardError" class="page-error" role="alert"><AppIcon name="info" :size="15" />{{ dashboardError }}</p>

    <div class="dashboard-grid">
      <article class="card hero-status">
        <div class="hero-content">
          <div class="hero-header">
            <div class="hero-state">
              <div class="hero-eyebrow"><span class="pulse-dot" :class="{ stopped: !running }" />系统实时状态</div>
              <h2>{{ running ? "sing-box 正在运行" : "sing-box 已停止" }}</h2>
            </div>
            <div class="hero-actions">
              <button class="hero-btn" data-core-restart type="button" :disabled="coreTransitioning || !running" @click="restartCore"><AppIcon name="rotate" :size="14" />{{ restartLabel }}</button>
              <button class="hero-btn" :class="{ 'hero-btn-danger': running }" data-core-toggle type="button" :disabled="coreTransitioning" @click="toggleCore"><AppIcon :name="coreAction ? 'rotate' : 'power'" :size="14" />{{ coreActionLabel }}</button>
            </div>
          </div>
          <DashboardMetrics ref="runtimeMetrics" :state="core.status.value?.state" :busy="Boolean(coreTransitioning)" :runtime-key="proxyRuntimeKey" />
          <div class="hero-profile">
            <span class="hero-profile-label">活动配置</span>
            <strong class="hero-profile-name" :title="activeProfile?.name || '未选择档案'">{{ activeProfile?.name || "未选择档案" }}</strong>
            <span class="hero-profile-applied">最后应用于{{ appliedLabel }}</span>
            <div class="hero-profile-stats"><span>入站 <strong>{{ draftStats.inbounds }}</strong></span><span>出站 <strong>{{ draftStats.outbounds }}</strong></span><span>路由规则 <strong>{{ draftStats.routeRules }}</strong></span></div>
          </div>
          <div class="hero-meta">
            <div class="hero-meta-item"><span>运行时间</span><strong>{{ uptime }}</strong></div>
            <div class="hero-meta-item"><span>核心版本</span><strong>{{ core.status.value?.installedVersion || "--" }}</strong></div>
            <div class="hero-meta-item"><span>进程 ID</span><strong>{{ core.status.value?.pid || "--" }}</strong></div>
          </div>
        </div>
      </article>

      <DashboardRouteMode ref="routeModeControl" :running="core.status.value?.state === 'running' && !core.error.value" :busy="Boolean(coreTransitioning)" :runtime-key="proxyRuntimeKey" @notice="showNotice" />

      <p v-if="!running && serverResolution" class="candidate-source" role="status">{{ candidateError || (stoppedCandidate ? '以下为选定档案的待运行候选；核心尚未运行' : '正在读取选定档案的待运行候选') }}</p>
      <DashboardProxySelector ref="proxySelector" class="dashboard-proxy" :outbounds="dashboardOutbounds" :outbound-order="dashboardOutboundOrder" :editor-state="dashboardOutboundEditorState" :loading="!core.status.value || (!running && serverResolution && !stoppedCandidate && !candidateError)" :running="core.status.value?.state === 'running' && !core.error.value" :busy="Boolean(coreTransitioning)" :runtime-key="proxyRuntimeKey" @notice="showNotice" @error="showNotice($event, 'error')" />
    </div>

    <AppToast v-if="notice" :message="notice" :tone="noticeTone" />
  </section>
</template>

<style scoped>
.candidate-source { grid-column: 1 / -1; margin: 0; padding: 8px 12px; border-radius: var(--radius-sm); color: var(--text-soft); background: var(--surface-soft); font-size: var(--font-size-sm); }
.page-intro { display: flex; align-items: flex-end; justify-content: space-between; gap: var(--section-gap); margin: 0 0 var(--section-gap); }
.page-intro-copy h2 { margin: 0; color: var(--text); font-size: 22px; font-weight: 750; letter-spacing: -.6px; }
.page-intro-copy p { max-width: 680px; margin: 3px 0 0; color: var(--text-soft); font-size: var(--font-size-base); }
.page-actions { display: flex; align-items: center; gap: 9px; }
.btn { display: inline-flex; min-height: 34px; align-items: center; justify-content: center; gap: 7px; padding: 7px 11px; border: 1px solid var(--line); border-radius: 10px; color: var(--text-soft); background: var(--surface); box-shadow: 0 1px 2px rgb(23 32 51 / 3%); font-size: var(--font-size-base); font-weight: 650; text-decoration: none; transition: color 150ms ease, border-color 150ms ease, background 150ms ease, transform 150ms ease, box-shadow 150ms ease; }
.btn:hover { color: var(--text); border-color: var(--line-strong); background: var(--surface-soft); transform: translateY(-1px); }
.btn-primary { color: #fff; border-color: var(--primary); background: linear-gradient(135deg, var(--primary), var(--primary-strong)); box-shadow: 0 10px 22px -10px color-mix(in srgb, var(--primary) 65%, transparent); }
.btn-primary:hover { color: #fff; border-color: var(--primary-strong); background: linear-gradient(135deg, color-mix(in srgb, var(--primary) 88%, #fff), var(--primary-strong)); box-shadow: 0 13px 26px -11px color-mix(in srgb, var(--primary) 76%, transparent); }
.page-error { display: flex; align-items: center; gap: 8px; margin: 0 0 12px; padding: 10px 12px; border-radius: 9px; color: var(--red); background: var(--red-soft); font-size: var(--font-size-sm); }
.dashboard-grid { display: grid; grid-template-columns: minmax(0, 2fr) minmax(180px, 1fr); gap: var(--section-gap); }
.dashboard-proxy { grid-column: 1 / -1; min-width: 0; }
.card { border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }
.hero-status { position: relative; min-width: 0; overflow: hidden; border: 0; color: #fff; background: linear-gradient(124deg, #2d45ab 0%, #4a6cf2 54%, #6480ff 100%); box-shadow: 0 22px 48px -22px rgb(57 81 184 / 60%); }
.hero-status::before { position: absolute; top: -110px; right: -50px; width: 330px; height: 330px; border: 1px solid rgb(255 255 255 / 11%); border-radius: 50%; content: ""; box-shadow: 0 0 0 42px rgb(255 255 255 / 4%), 0 0 0 92px rgb(255 255 255 / 3%); }
.hero-status::after { position: absolute; right: 74px; bottom: -94px; width: 190px; height: 190px; border-radius: 50%; content: ""; background: radial-gradient(circle at 32% 28%, rgb(255 255 255 / 26%), rgb(255 255 255 / 2%) 62%); }
.hero-content { position: relative; z-index: 2; display: grid; align-content: start; gap: 5px; padding: 8px 11px; }
.hero-header { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.hero-state { min-width: 0; }
.hero-eyebrow { display: flex; align-items: center; gap: 8px; color: rgb(255 255 255 / 78%); font-size: var(--font-size-xs); font-weight: 740; letter-spacing: .5px; line-height: 1.15; }
.hero-state h2 { overflow: hidden; margin: 2px 0 0; color: #fff; font-size: 17px; font-weight: 730; letter-spacing: -.35px; line-height: 1.15; text-overflow: ellipsis; white-space: nowrap; }
.pulse-dot { position: relative; width: 7px; height: 7px; border-radius: 50%; background: #6cf0bb; box-shadow: 0 0 0 5px rgb(108 240 187 / 15%); }
.pulse-dot::after { position: absolute; inset: -6px; border: 1px solid #6cf0bb; border-radius: 50%; content: ""; animation: pulse 1.8s ease-out infinite; }
.pulse-dot.stopped { background: #ffd084; box-shadow: 0 0 0 5px rgb(255 208 132 / 15%); }
.pulse-dot.stopped::after { border-color: #ffd084; }
.hero-actions { display: flex; flex: 0 0 auto; gap: 5px; }
.hero-btn { display: inline-flex; min-height: 31px; align-items: center; gap: 5px; padding: 5px 8px; border: 1px solid rgb(255 255 255 / 20%); border-radius: 8px; color: #fff; background: rgb(255 255 255 / 10%); font-size: var(--font-size-xs); font-weight: 650; backdrop-filter: blur(8px); transition: background 150ms ease, transform 150ms ease; }
.hero-btn:hover { background: rgb(255 255 255 / 18%); transform: translateY(-1px); }
.hero-btn-danger { color: #ffeff0; border-color: rgb(255 225 228 / 20%); background: rgb(107 20 35 / 22%); }
.hero-profile { display: flex; min-width: 0; align-items: baseline; flex-wrap: wrap; gap: 3px 8px; padding-top: 5px; border-top: 1px solid rgb(255 255 255 / 18%); font-size: var(--font-size-xs); line-height: 1.2; }
.hero-profile-label, .hero-profile-applied { color: rgb(255 255 255 / 75%); white-space: nowrap; }
.hero-profile-name { max-width: 135px; overflow: hidden; color: #fff; text-overflow: ellipsis; white-space: nowrap; }
.hero-profile-stats { display: flex; flex-wrap: wrap; gap: 4px 9px; margin-left: auto; color: rgb(255 255 255 / 75%); white-space: nowrap; }
.hero-profile-stats strong { color: #fff; font-weight: 700; }
.hero-meta { display: flex; align-items: baseline; flex-wrap: wrap; gap: 4px 12px; padding-top: 5px; border-top: 1px solid rgb(255 255 255 / 18%); line-height: 1.2; }
.hero-meta-item { display: flex; min-width: 0; align-items: baseline; gap: 4px; }
.hero-meta-item span { color: rgb(255 255 255 / 65%); font-size: var(--font-size-xs); white-space: nowrap; }
.hero-meta-item strong { overflow: hidden; color: #fff; font-size: var(--font-size-xs); font-weight: 650; text-overflow: ellipsis; white-space: nowrap; }
@keyframes pulse { from { opacity: .7; transform: scale(.45); } to { opacity: 0; transform: scale(1.45); } }
@media (max-width: 980px) {
  .dashboard-grid { grid-template-columns: minmax(0, 1fr); }
}
@media (max-width: 760px) {
  .page-intro { align-items: flex-start; flex-direction: column; }
  .page-intro-copy h2 { font-size: 21px; }
  .page-actions { width: 100%; flex-wrap: wrap; }
  .page-actions .btn { flex: 1; }
}
@media (max-width: 430px) {
  .hero-header { align-items: flex-start; flex-wrap: wrap; }
  .hero-actions { width: 100%; }
  .hero-btn { flex: 1; justify-content: center; }
  .hero-profile-stats { width: 100%; margin-left: 0; }
}
@media (prefers-reduced-motion: reduce) { *, *::before, *::after { animation-duration: .01ms !important; animation-iteration-count: 1 !important; transition-duration: .01ms !important; } }
</style>
