<script setup lang="ts">
import { computed } from "vue";
import AppIcon from "@/layout/AppIcon.vue";
import type { CoreStatus, CoreUpdateStatus, LatestRelease } from "./types";

const props = withDefaults(defineProps<{
  status: CoreStatus | null;
  latest: LatestRelease | null;
  update: CoreUpdateStatus | null;
  checking: boolean;
  busy: boolean;
  stopping?: boolean;
  updateAvailable: boolean;
}>(), { stopping: false });

const emit = defineEmits<{ download: [version: string]; refresh: []; stop: [] }>();

const stageMeta: Record<string, { label: string; progress: number }> = {
  checking_release: { label: "检查官方版本", progress: 8 },
  downloading: { label: "下载核心", progress: 24 },
  cancelling: { label: "正在停止下载", progress: 24 },
  cancelled: { label: "下载已停止", progress: 24 },
  verifying_checksum: { label: "校验文件摘要", progress: 40 },
  verifying_binary: { label: "验证核心程序", progress: 54 },
  checking_config: { label: "检查现有配置", progress: 66 },
  waiting_for_lock: { label: "等待核心操作", progress: 74 },
  stopping_core: { label: "停止当前核心", progress: 82 },
  installing: { label: "安装核心", progress: 90 },
  restarting_core: { label: "恢复核心运行", progress: 97 },
  completed: { label: "更新完成", progress: 100 },
  failed: { label: "更新失败", progress: 100 }
};

const currentVersion = computed(() => props.status?.installedVersion || (props.status ? "尚未安装" : "读取中…"));
const latestVersion = computed(() => props.latest?.version || (props.checking ? "检查中…" : "暂不可用"));
const stage = computed(() => props.update?.stage ?? "");
const stageDetails = computed(() => stageMeta[stage.value]);
const updateFailed = computed(() => stage.value === "failed");
const updateCompleted = computed(() => stage.value === "completed");
const updateCancelled = computed(() => stage.value === "cancelled");
const cancelling = computed(() => stage.value === "cancelling");
const updating = computed(() => Boolean(stage.value && stage.value !== "idle" && !updateFailed.value && !updateCompleted.value && !updateCancelled.value));
const showProgress = computed(() => Boolean(stageDetails.value && stage.value !== "idle"));
const stageLabel = computed(() => stageDetails.value?.label ?? stage.value);
const downloading = computed(() => stage.value === "downloading");
const transferStage = computed(() => downloading.value || cancelling.value || updateCancelled.value);
const downloadedBytes = computed(() => Math.max(0, props.update?.downloadedBytes ?? 0));
const totalBytes = computed(() => Math.max(0, props.update?.totalBytes ?? 0));
const bytesPerSecond = computed(() => Math.max(0, props.update?.bytesPerSecond ?? 0));
const hasDownloadTotal = computed(() => totalBytes.value > 0);
const downloadPercent = computed(() => hasDownloadTotal.value ? Math.min(100, downloadedBytes.value / totalBytes.value * 100) : 0);
const downloadComplete = computed(() => hasDownloadTotal.value && downloadedBytes.value >= totalBytes.value);
const progressValue = computed(() => transferStage.value
  ? hasDownloadTotal.value ? downloadPercent.value : undefined
  : downloadComplete.value ? 100 : stageDetails.value?.progress);
const progressNow = computed(() => progressValue.value === undefined || updateFailed.value ? undefined : Number(progressValue.value.toFixed(1)));
const progressLabel = computed(() => {
  if (updateFailed.value) return "失败";
  if (updateCancelled.value) return "已停止";
  if (cancelling.value) return "停止中";
  if (downloading.value && !hasDownloadTotal.value) return "计算中";
  const value = progressValue.value ?? 0;
  const digits = downloading.value && value > 0 && value < 10 ? 1 : 0;
  return `${value.toFixed(digits)}%`;
});
const transferLabel = computed(() => hasDownloadTotal.value
  ? `${formatBytes(downloadedBytes.value)} / ${formatBytes(totalBytes.value)}`
  : `已下载 ${formatBytes(downloadedBytes.value)}`);
const speedLabel = computed(() => cancelling.value || updateCancelled.value ? "已停止" : bytesPerSecond.value > 0 ? `${formatBytes(bytesPerSecond.value)}/s` : "计算中");
const progressText = computed(() => transferStage.value
  ? `${transferLabel.value}，下载速度 ${speedLabel.value}`
  : props.update?.message ? `${stageLabel.value}：${props.update.message}` : stageLabel.value);

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const unitIndex = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / 1024 ** unitIndex;
  const digits = unitIndex === 0 || value >= 100 ? 0 : value >= 10 ? 1 : 2;
  return `${Number(value.toFixed(digits))} ${units[unitIndex]}`;
}

const processLabel = computed(() => {
  if (!props.status) return "正在读取本机核心信息";
  if (!props.status.installedVersion || props.status.state === "not_installed") return "本机尚未安装 sing-box 核心";
  const labels = {
    stopped: "核心当前已停止",
    starting: "核心正在启动",
    running: props.status.pid ? `核心正在运行 · PID ${props.status.pid}` : "核心正在运行",
    stopping: "核心正在停止",
    start_failed: "核心启动失败",
    exited: "核心已经退出",
    not_installed: "本机尚未安装 sing-box 核心"
  };
  return `已安装 · ${labels[props.status.state]}`;
});

const releaseState = computed(() => {
  if (props.busy) return { label: "准备更新", tone: "downloading" };
  if (props.stopping || cancelling.value) return { label: "正在停止", tone: "downloading" };
  if (updateCancelled.value) return { label: "下载已停止", tone: "cancelled" };
  if (updating.value) return { label: stageLabel.value, tone: "downloading" };
  if (updateFailed.value) return { label: "更新失败", tone: "failed" };
  if (updateCompleted.value) return { label: "更新完成", tone: "downloaded" };
  if (props.checking) return { label: "检查中", tone: "downloading" };
  if (!props.latest) return { label: "获取失败", tone: "failed" };
  if (!props.status?.installedVersion) return { label: "可供安装", tone: "" };
  if (props.updateAvailable) return { label: "发现新版本", tone: "" };
  return { label: "已是最新版", tone: "downloaded" };
});

const downloadTitle = computed(() => {
  if (props.stopping || cancelling.value) return "正在停止下载";
  if (updateCancelled.value) return "下载已停止";
  if (updating.value || props.busy) return "正在更新核心";
  if (updateFailed.value) return "核心更新失败";
  if (updateCompleted.value) return "核心更新完成";
  if (!props.latest) return props.checking ? "正在检查官方版本" : "暂时无法获取最新版本";
  if (!props.status?.installedVersion) return "下载官方核心";
  return props.updateAvailable ? "获取官方更新" : "当前已是最新版本";
});

const downloadDescription = computed(() => {
  if (props.update?.message) return props.update.message;
  if (updating.value) return `${props.update?.targetVersion ? `sing-box ${props.update.targetVersion} · ` : ""}${stageLabel.value}`;
  if (props.latest) return `Linux · ${props.latest.arch} · 安装前校验 SHA-256`;
  return props.checking ? "正在连接官方稳定版本源" : "请重新检查官方稳定版本";
});

const actionLabel = computed(() => {
  if (props.busy) return "正在准备更新";
  if (updating.value) return "正在更新核心";
  if (props.checking) return "正在检查版本";
  if (!props.latest) return "重新检查版本";
  if (updateCancelled.value) return "重新下载最新版本";
  if (updateFailed.value) return "重试下载";
  if (!props.status?.installedVersion || props.updateAvailable) return "下载最新版本";
  return "重新安装最新版本";
});

const actionDisabled = computed(() => props.busy || updating.value || props.checking);

function triggerAction(): void {
  if (!props.latest) emit("refresh");
  else emit("download", props.latest.version);
}
</script>

<template>
  <article class="core-version-shell" aria-label="版本管理">
    <header class="core-version-shell-head">
      <span class="core-shell-icon"><AppIcon name="cpu" :size="20" /></span>
      <div class="core-shell-copy"><strong>sing-box 核心</strong><span>版本信息</span></div>
      <span class="core-release-state" :class="releaseState.tone" role="status"><i />{{ releaseState.label }}</span>
    </header>

    <div class="core-version-grid">
      <article class="core-version-tile" aria-label="当前版本">
        <div class="core-version-label"><AppIcon name="server" :size="15" /><span>当前版本</span></div>
        <strong class="core-version-value">{{ currentVersion }}</strong>
        <p><span v-if="status?.installedVersion" class="core-running-dot" :class="status.state" />{{ processLabel }}</p>
      </article>

      <span class="core-version-connector" aria-hidden="true"><AppIcon name="chevron" :size="17" /></span>

      <article class="core-version-tile latest" aria-label="最新版本">
        <div class="core-version-label"><AppIcon name="sparkles" :size="15" /><span>最新版本</span><b>Stable</b></div>
        <strong class="core-version-value">{{ latestVersion }}</strong>
        <p>{{ latest ? `官方最新稳定版本 · Linux ${latest.arch}` : checking ? "正在连接官方版本源" : "暂时无法获取官方版本" }}</p>
      </article>
    </div>

    <div class="core-download-bar">
      <div class="core-download-copy">
        <span :class="{ failed: updateFailed, updating, cancelled: updateCancelled }"><AppIcon :name="updateCompleted ? 'check' : updateCancelled ? 'stop' : updating ? 'rotate' : 'download'" :size="16" /></span>
        <div><strong>{{ downloadTitle }}</strong><small>{{ downloadDescription }}</small></div>
      </div>
      <div class="core-download-actions">
        <button v-if="downloading || cancelling" class="core-stop-button" data-stop-core-update type="button" :disabled="stopping || cancelling" @click="emit('stop')">
          <AppIcon :name="stopping || cancelling ? 'rotate' : 'stop'" :size="15" />{{ stopping || cancelling ? "正在停止" : "停止下载" }}
        </button>
        <button v-else class="core-download-button" data-download-core type="button" :disabled="actionDisabled" @click="triggerAction">
          <AppIcon :name="updating || busy || checking ? 'rotate' : 'download'" :size="15" />{{ actionLabel }}
        </button>
      </div>
    </div>

    <div v-if="showProgress" class="core-update-progress" :class="{ complete: updateCompleted, failed: updateFailed, cancelled: updateCancelled, indeterminate: progressNow === undefined && !updateFailed && !updateCancelled }" aria-live="polite">
      <div class="core-progress-head"><span>{{ stageLabel }}</span><strong>{{ progressLabel }}</strong></div>
      <div v-if="transferStage" class="core-download-metrics">
        <span><small>下载进度</small><strong>{{ transferLabel }}</strong></span>
        <span><small>下载速度</small><strong data-download-speed>{{ speedLabel }}</strong></span>
      </div>
      <span
        class="core-progress-track"
        role="progressbar"
        aria-label="核心下载进度"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-valuenow="progressNow"
        :aria-valuetext="progressText"
      ><i :style="progressValue === undefined ? undefined : { width: `${progressValue}%` }" /></span>
    </div>
  </article>
</template>

<style scoped>
.core-version-shell { overflow: hidden; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }
.core-version-shell-head { display: flex; min-height: 54px; align-items: center; gap: 12px; padding: 10px var(--card-padding); border-bottom: 1px solid var(--line); }
.core-shell-icon { display: grid; width: 32px; height: 32px; flex: 0 0 auto; place-items: center; border-radius: 12px; color: var(--primary); background: var(--primary-soft); }
.core-shell-copy { min-width: 0; flex: 1; }
.core-shell-copy strong, .core-shell-copy span { display: block; }
.core-shell-copy strong { font-size: 14px; font-weight: 740; }
.core-shell-copy span { margin-top: 1px; color: var(--text-faint); font-size: var(--font-size-xs); }
.core-release-state { display: inline-flex; min-height: 25px; align-items: center; gap: 6px; padding: 4px 9px; border-radius: 999px; color: var(--primary); background: var(--primary-soft); font-size: var(--font-size-xs); font-weight: 720; white-space: nowrap; }
.core-release-state i { width: 5px; height: 5px; border-radius: 50%; background: currentColor; }
.core-release-state.downloading { color: var(--orange); background: var(--orange-soft); }
.core-release-state.downloaded { color: var(--green); background: var(--green-soft); }
.core-release-state.failed { color: var(--red); background: var(--red-soft); }
.core-release-state.cancelled { color: var(--text-soft); background: var(--surface-soft); }
.core-version-grid { display: grid; grid-template-columns: minmax(0, 1fr) 34px minmax(0, 1fr); align-items: center; padding: var(--card-padding); }
.core-version-tile { min-width: 0; min-height: 112px; padding: var(--card-padding); border: 1px solid var(--line); border-radius: 14px; background: var(--surface-soft); }
.core-version-tile.latest { border-color: color-mix(in srgb, var(--primary) 30%, var(--line)); background: linear-gradient(145deg, color-mix(in srgb, var(--primary-soft) 75%, var(--surface)), var(--surface)); }
.core-version-label { display: flex; flex-wrap: wrap; align-items: center; gap: 7px; color: var(--text-soft); font-size: var(--font-size-base); font-weight: 680; }
.core-version-label b { margin-left: auto; padding: 3px 7px; border-radius: 6px; color: var(--primary); background: var(--primary-soft); font-size: var(--font-size-sm); letter-spacing: .25px; }
.core-version-value { display: block; margin-top: 10px; overflow: hidden; color: var(--text); font-size: clamp(27px, 4vw, 36px); font-weight: 780; letter-spacing: -1.2px; line-height: 1; text-overflow: ellipsis; white-space: nowrap; }
.core-version-tile.latest .core-version-value { color: var(--primary-strong); }
.core-version-tile p { display: flex; align-items: center; gap: 6px; margin: 10px 0 0; color: var(--text-faint); font-size: var(--font-size-xs); }
.core-running-dot { width: 6px; height: 6px; flex: 0 0 auto; border-radius: 50%; background: var(--text-faint); box-shadow: 0 0 0 3px color-mix(in srgb, var(--text-faint) 13%, transparent); }
.core-running-dot.running, .core-running-dot.starting { background: var(--green); box-shadow: 0 0 0 3px color-mix(in srgb, var(--green) 13%, transparent); }
.core-running-dot.start_failed, .core-running-dot.exited { background: var(--red); box-shadow: 0 0 0 3px color-mix(in srgb, var(--red) 13%, transparent); }
.core-version-connector { display: grid; place-items: center; color: var(--text-faint); }
.core-download-bar { display: flex; min-height: 56px; align-items: center; justify-content: space-between; gap: var(--section-gap); padding: 10px var(--card-padding); border-top: 1px solid var(--line); background: var(--surface-soft); }
.core-download-copy { display: flex; min-width: 0; align-items: center; gap: 10px; }
.core-download-copy > span { display: grid; width: 34px; height: 34px; flex: 0 0 auto; place-items: center; border-radius: 10px; color: var(--green); background: var(--green-soft); }
.core-download-copy > span.updating { color: var(--orange); background: var(--orange-soft); }
.core-download-copy > span.failed { color: var(--red); background: var(--red-soft); }
.core-download-copy > span.cancelled { color: var(--text-soft); background: var(--line); }
.core-download-copy strong, .core-download-copy small { display: block; }
.core-download-copy strong { font-size: var(--font-size-base); }
.core-download-copy small { margin-top: 2px; color: var(--text-faint); font-size: var(--font-size-sm); }
.core-download-actions { display: flex; flex: 0 0 auto; }
.core-download-button, .core-stop-button { display: inline-flex; min-width: 178px; min-height: 36px; align-items: center; justify-content: center; gap: 7px; padding: 7px 13px; font-size: var(--font-size-sm); }
.core-stop-button { color: var(--red); border: 1px solid color-mix(in srgb, var(--red) 30%, var(--line)); background: var(--red-soft); }
.core-update-progress { padding: var(--card-padding); border-top: 1px solid var(--line); background: var(--surface); }
.core-progress-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 7px; color: var(--text-soft); font-size: var(--font-size-xs); }
.core-update-progress strong { color: var(--primary); font-size: var(--font-size-base); }
.core-download-metrics { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; margin-bottom: 10px; }
.core-download-metrics > span { min-width: 0; padding: 8px 10px; border-radius: 8px; background: var(--surface-soft); }
.core-download-metrics small, .core-download-metrics strong { display: block; }
.core-download-metrics small { color: var(--text-faint); font-size: var(--font-size-sm); }
.core-download-metrics strong { overflow: hidden; margin-top: 2px; color: var(--text); font-size: var(--font-size-base); text-overflow: ellipsis; white-space: nowrap; }
.core-progress-track { display: block; height: 6px; overflow: hidden; border-radius: 999px; background: var(--line); }
.core-progress-track i { display: block; height: 100%; border-radius: inherit; background: linear-gradient(90deg, var(--primary), var(--cyan)); transition: width 240ms ease; }
.core-update-progress.indeterminate .core-progress-track i { width: 35%; animation: core-progress-indeterminate 1.1s ease-in-out infinite; }
.core-update-progress.complete strong { color: var(--green); }
.core-update-progress.complete .core-progress-track i { background: var(--green); }
.core-update-progress.failed strong { color: var(--red); }
.core-update-progress.failed .core-progress-track i { background: var(--red); }
.core-update-progress.cancelled strong { color: var(--text-soft); }
.core-update-progress.cancelled .core-progress-track i { background: var(--text-faint); }
@keyframes core-progress-indeterminate { from { transform: translateX(-110%); } to { transform: translateX(300%); } }

@media (max-width: 768px) {

  .core-version-grid { grid-template-columns: minmax(0, 1fr) 20px minmax(0, 1fr); }
  .core-version-tile { padding: 10px; }
  .core-download-bar { align-items: stretch; flex-direction: column; gap: var(--section-gap); padding: var(--card-padding); }
  .core-download-actions, .core-download-button, .core-stop-button { width: 100%; }
  .core-download-button, .core-stop-button { min-height: 42px; }
  .core-update-progress { padding: var(--card-padding); }
}

@media (max-width: 420px) {
  .core-release-state { max-width: 112px; justify-content: center; overflow: hidden; text-align: center; text-overflow: ellipsis; }
  .core-shell-copy span { display: none; }
  .core-version-value { font-size: 28px; }
}

@media (prefers-reduced-motion: reduce) {
  .core-progress-track i { transition: none; }
  .core-update-progress.indeterminate .core-progress-track i { animation: none; transform: translateX(90%); }
}
</style>
