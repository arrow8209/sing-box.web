<script setup lang="ts">
import { computed } from "vue";
import AppIcon from "@/layout/AppIcon.vue";
import type { CoreStatus, LatestRelease } from "./types";

const props = defineProps<{
  status: CoreStatus | null;
  latest: LatestRelease | null;
  checking: boolean;
  busy: boolean;
  updateAvailable: boolean;
}>();

const emit = defineEmits<{ download: []; refresh: [] }>();

const currentVersion = computed(() => props.status?.installedVersion || "尚未安装");
const latestVersion = computed(() => props.latest?.version || (props.checking ? "正在检查…" : "暂不可用"));
const currentMeta = computed(() => {
  if (!props.status) return "正在读取本机核心信息";
  if (!props.status.installedVersion) return "本机尚未安装 sing-box 核心";
  if (props.status.state === "running") return props.status.pid ? `运行中 · PID ${props.status.pid}` : "核心正在运行";
  return "核心当前未运行";
});
const releaseLabel = computed(() => {
  if (!props.latest) return props.checking ? "检查中" : "获取失败";
  if (!props.status?.installedVersion) return "可供安装";
  return props.updateAvailable ? "发现新版本" : "已是最新版";
});
const actionLabel = computed(() => props.busy ? "正在准备…" : "下载最新版本");
</script>

<template>
  <section class="version-panel" aria-label="版本管理">
    <header class="panel-header">
      <span class="panel-icon"><AppIcon name="cpu" :size="21" /></span>
      <div>
        <h2>sing-box 核心</h2>
        <p>查看已安装版本，并获取官方最新稳定版</p>
      </div>
      <span class="release-badge" :class="{ available: updateAvailable }">{{ releaseLabel }}</span>
    </header>

    <div class="version-compare">
      <article class="version-card current-version" aria-label="当前版本">
        <span class="version-label"><AppIcon name="server" :size="14" />当前版本</span>
        <strong>{{ currentVersion }}</strong>
        <p>{{ currentMeta }}</p>
      </article>

      <span class="version-arrow" aria-hidden="true"><AppIcon name="chevron" :size="17" /></span>

      <article class="version-card latest-version" aria-label="最新版本">
        <div class="latest-heading">
          <span class="version-label"><AppIcon name="sparkles" :size="14" />最新版本</span>
          <button class="refresh-button" type="button" :disabled="checking" aria-label="重新检查最新版本" @click="emit('refresh')">
            <AppIcon name="rotate" :size="14" />
          </button>
        </div>
        <strong>{{ latestVersion }}</strong>
        <p>{{ latest ? `官方稳定版 · ${latest.arch}` : checking ? "正在连接版本源" : "暂时无法获取版本信息" }}</p>
      </article>
    </div>

    <footer class="panel-footer">
      <p><AppIcon name="shield" :size="15" /><span>下载前会再次确认版本，当前配置不会被修改。</span></p>
      <button data-download-core class="download-button" type="button" :disabled="!latest || checking || busy" @click="emit('download')">
        <AppIcon name="download" :size="16" />{{ actionLabel }}
      </button>
    </footer>
  </section>
</template>

<style scoped>
.version-panel { overflow: hidden; border: 1px solid var(--line); border-radius: var(--radius-lg); background: var(--surface); box-shadow: var(--shadow-sm); }
.panel-header { display: flex; align-items: center; gap: 11px; padding: 16px 18px; border-bottom: 1px solid var(--line); }
.panel-icon { display: grid; width: 40px; height: 40px; flex: 0 0 auto; place-items: center; border-radius: 12px; color: var(--primary); background: var(--primary-soft); }
.panel-header > div { min-width: 0; flex: 1; }
.panel-header h2, .panel-header p { margin: 0; }
.panel-header h2 { font-size: 14px; letter-spacing: -.15px; }
.panel-header p { margin-top: 2px; color: var(--text-faint); font-size: 10px; }
.release-badge { display: inline-flex; min-height: 24px; align-items: center; gap: 5px; padding: 3px 8px; border-radius: 999px; color: var(--green); background: var(--green-soft); font-size: 9px; font-weight: 720; }
.release-badge::before { width: 5px; height: 5px; border-radius: 50%; content: ""; background: currentColor; }
.release-badge.available { color: var(--primary); background: var(--primary-soft); }
.version-compare { display: grid; grid-template-columns: minmax(0, 1fr) 30px minmax(0, 1fr); align-items: center; padding: 18px; }
.version-card { min-width: 0; padding: 17px 18px; border: 1px solid var(--line); border-radius: 13px; background: var(--surface-soft); }
.latest-version { border-color: color-mix(in srgb, var(--primary) 24%, var(--line)); background: color-mix(in srgb, var(--primary-soft) 55%, var(--surface)); }
.version-label { display: flex; align-items: center; gap: 6px; color: var(--text-soft); font-size: 10px; font-weight: 650; }
.version-card strong { display: block; margin-top: 10px; overflow: hidden; color: var(--text); font-size: clamp(22px, 3vw, 29px); letter-spacing: -.7px; line-height: 1.15; text-overflow: ellipsis; white-space: nowrap; }
.latest-version strong { color: var(--primary-strong); }
.version-card p { margin: 6px 0 0; color: var(--text-faint); font-size: 9px; }
.version-arrow { display: grid; place-items: center; color: var(--text-faint); }
.latest-heading { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.refresh-button { display: inline-grid; width: 25px; min-height: 25px; place-items: center; padding: 0; color: var(--text-faint); background: transparent; }
.refresh-button:hover { color: var(--primary); background: var(--surface); }
.panel-footer { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 13px 18px; border-top: 1px solid var(--line); background: var(--surface-soft); }
.panel-footer p { display: flex; min-width: 0; align-items: center; gap: 7px; margin: 0; color: var(--text-faint); font-size: 9px; }
.panel-footer p :deep(.icon) { color: var(--green); }
.download-button { display: inline-flex; min-height: 36px; flex: 0 0 auto; align-items: center; justify-content: center; gap: 7px; padding: 7px 13px; font-size: 10px; }

@media (max-width: 680px) {
  .panel-header { align-items: flex-start; padding: 15px; }
  .release-badge { margin-left: auto; white-space: nowrap; }
  .version-compare { grid-template-columns: 1fr; gap: 9px; padding: 13px; }
  .version-arrow { transform: rotate(90deg); }
  .version-card { padding: 15px; }
  .panel-footer { align-items: stretch; flex-direction: column; padding: 13px; }
  .download-button { width: 100%; min-height: 40px; }
}
</style>
