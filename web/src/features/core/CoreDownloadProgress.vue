<script setup lang="ts">
import { computed } from "vue";
import AppIcon from "@/layout/AppIcon.vue";
import type { CoreUpdateStatus } from "./types";

const props = defineProps<{ status: CoreUpdateStatus }>();

const stageLabels: Record<string, string> = {
  checking_release: "检查版本",
  downloading: "正在下载",
  verifying_checksum: "摘要校验",
  verifying_binary: "程序验证",
  checking_config: "配置预检",
  waiting_for_lock: "等待操作",
  stopping_core: "停止核心",
  installing: "安装核心",
  restarting_core: "重启核心",
  completed: "更新完成",
  failed: "更新失败"
};

const downloaded = computed(() => Math.max(0, props.status.downloadedBytes ?? 0));
const total = computed(() => Math.max(0, props.status.totalBytes ?? 0));
const speed = computed(() => Math.max(0, props.status.bytesPerSecond ?? 0));
const hasTotal = computed(() => total.value > 0);
const hasDownloadProgress = computed(() => props.status.stage === "downloading" || downloaded.value > 0 || hasTotal.value);
const progress = computed(() => hasTotal.value ? Math.min(100, downloaded.value / total.value * 100) : 0);
const progressNow = computed(() => hasTotal.value ? Number(progress.value.toFixed(1)) : undefined);
const progressLabel = computed(() => {
  if (!hasTotal.value) return props.status.stage === "downloading" ? "计算中" : "—";
  const digits = progress.value > 0 && progress.value < 10 ? 1 : 0;
  return `${progress.value.toFixed(digits)}%`;
});
const transferLabel = computed(() => hasTotal.value
  ? `${formatBytes(downloaded.value)} / ${formatBytes(total.value)}`
  : `已下载 ${formatBytes(downloaded.value)}`);
const speedLabel = computed(() => speed.value > 0 ? `${formatBytes(speed.value)}/s` : props.status.stage === "downloading" ? "计算中" : "—");
const stageLabel = computed(() => stageLabels[props.status.stage] ?? props.status.stage);
const progressText = computed(() => `${transferLabel.value}，当前速度 ${speedLabel.value}`);

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const unitIndex = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / 1024 ** unitIndex;
  const digits = unitIndex === 0 || value >= 100 ? 0 : value >= 10 ? 1 : 2;
  return `${Number(value.toFixed(digits))} ${units[unitIndex]}`;
}
</script>

<template>
  <section class="operation" aria-labelledby="core-operation-title">
    <header>
      <div class="operation-title">
        <span><AppIcon name="download" :size="17" /></span>
        <div>
          <h2 id="core-operation-title">更新操作</h2>
          <p>{{ status.targetVersion ? `sing-box ${status.targetVersion}` : "sing-box 核心" }}</p>
        </div>
      </div>
      <strong class="stage-badge" :class="{ failed: status.stage === 'failed', completed: status.stage === 'completed' }" aria-live="polite">{{ stageLabel }}</strong>
    </header>

    <div v-if="hasDownloadProgress" class="download-progress">
      <div class="progress-stats">
        <div><span>下载进度</span><strong data-download-percent>{{ progressLabel }}</strong></div>
        <div><span>已下载</span><strong>{{ transferLabel }}</strong></div>
        <div><span>当前速度</span><strong data-download-speed>{{ speedLabel }}</strong></div>
      </div>
      <div
        class="progress-track"
        :class="{ indeterminate: !hasTotal, complete: hasTotal && progress >= 100 }"
        role="progressbar"
        aria-label="核心下载进度"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-valuenow="progressNow"
        :aria-valuetext="progressText"
      ><span :style="hasTotal ? { width: `${progress}%` } : undefined" /></div>
    </div>

    <p v-if="status.message" class="operation-message">{{ status.message }}</p>
  </section>
</template>

<style scoped>
.operation { overflow: hidden; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }
.operation > header { display: flex; min-height: 58px; align-items: center; justify-content: space-between; gap: 14px; padding: 12px 15px; border-bottom: 1px solid var(--line); }
.operation-title { display: flex; min-width: 0; align-items: center; gap: 10px; }
.operation-title > span { display: grid; width: 34px; height: 34px; flex: 0 0 auto; place-items: center; border-radius: 10px; color: var(--primary); background: var(--primary-soft); }
.operation h2, .operation p { margin: 0; }
.operation h2 { font-size: 13px; }
.operation-title p { margin-top: 2px; color: var(--text-faint); font-size: 9px; }
.stage-badge { display: inline-flex; min-height: 24px; align-items: center; gap: 5px; padding: 3px 8px; border-radius: 999px; color: var(--primary); background: var(--primary-soft); font-size: 9px; font-weight: 700; white-space: nowrap; }
.stage-badge::before { width: 5px; height: 5px; border-radius: 50%; content: ""; background: currentColor; }
.stage-badge.completed { color: var(--green); background: var(--green-soft); }
.stage-badge.failed { color: var(--red); background: var(--red-soft); }
.download-progress { display: grid; gap: 12px; padding: 14px 15px 15px; }
.progress-stats { display: grid; grid-template-columns: .7fr 1.3fr 1fr; gap: 9px; }
.progress-stats > div { min-width: 0; padding: 9px 10px; border-radius: 9px; background: var(--surface-soft); }
.progress-stats span, .progress-stats strong { display: block; }
.progress-stats span { color: var(--text-faint); font-size: 8px; }
.progress-stats strong { overflow: hidden; margin-top: 3px; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.progress-track { position: relative; height: 7px; overflow: hidden; border-radius: 999px; background: var(--line); }
.progress-track > span { position: absolute; inset: 0 auto 0 0; width: 0; border-radius: inherit; background: linear-gradient(90deg, var(--primary), var(--cyan)); transition: width 220ms ease; }
.progress-track.complete > span { background: var(--green); }
.progress-track.indeterminate > span { width: 35%; animation: download-indeterminate 1.1s ease-in-out infinite; }
.operation-message { margin: 0 15px 14px; padding: 9px 10px; border-radius: 8px; color: var(--text-soft); background: var(--surface-soft); font-size: 9px; }
@keyframes download-indeterminate { from { transform: translateX(-110%); } to { transform: translateX(300%); } }
@media (prefers-reduced-motion: reduce) { .progress-track.indeterminate > span { animation: none; transform: translateX(90%); } }
@media (max-width: 560px) {
  .operation > header { align-items: flex-start; }
  .progress-stats { grid-template-columns: 1fr 1fr; }
  .progress-stats > div:first-child { grid-column: 1 / -1; }
}
</style>
