<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { RouterLink } from "vue-router";
import AppIcon from "@/layout/AppIcon.vue";
import { coreApi, type CoreApi } from "@/features/core/api";
import type { CoreStatus, LatestRelease } from "@/features/core/types";

export type CoreSettingsApi = Pick<CoreApi, "status" | "latest">;

const props = withDefaults(defineProps<{ api?: CoreSettingsApi }>(), {
  api: () => coreApi
});

const status = ref<CoreStatus | null>(null);
const latest = ref<LatestRelease | null>(null);
const loading = ref(true);
const error = ref("");

const currentVersion = computed(() => status.value?.installedVersion || "未安装");
const latestVersion = computed(() => latest.value?.version || "暂不可用");
const stableLabel = computed(() => `Stable · ${latest.value?.version || status.value?.installedVersion || "暂不可用"}`);
const autoStart = computed(() => Boolean(status.value?.expectedRunning));

function errorMessage(cause: unknown, fallback: string): string {
  return cause instanceof Error ? cause.message : fallback;
}

onMounted(async () => {
  const [statusResult, latestResult] = await Promise.allSettled([
    props.api.status(),
    props.api.latest()
  ]);
  const failures: string[] = [];
  if (statusResult.status === "fulfilled") {
    status.value = statusResult.value;
  } else {
    failures.push(errorMessage(statusResult.reason, "读取核心状态失败"));
  }
  if (latestResult.status === "fulfilled") {
    latest.value = latestResult.value;
  } else {
    failures.push(errorMessage(latestResult.reason, "读取最新稳定版失败"));
  }
  error.value = failures.join("；");
  loading.value = false;
});
</script>

<template>
  <section class="core-settings-card panel" aria-labelledby="core-settings-title">
    <header class="card-head">
      <div>
        <h2 id="core-settings-title">核心设置</h2>
        <p>sing-box 服务启动与运行行为</p>
      </div>
      <span class="version-badge" :class="{ installed: status?.installedVersion }">
        {{ loading ? "读取中" : status?.installedVersion ? `v${status.installedVersion}` : "未安装" }}
      </span>
    </header>

    <p v-if="error" class="load-error" role="alert">{{ error }}</p>

    <div class="setting-list" :aria-busy="loading">
      <div class="setting-row">
        <div class="setting-copy">
          <strong>开机自动运行</strong>
          <span>控制台重新启动后，恢复最近已应用的 sing-box 核心</span>
        </div>
        <div class="setting-control switch-control">
          <span>{{ loading ? "读取中" : autoStart ? "已启用" : "未启用" }}</span>
          <button
            class="setting-switch"
            :class="{ on: autoStart }"
            type="button"
            role="switch"
            aria-label="开机自动运行（只读）"
            :aria-checked="autoStart"
            disabled
            title="该状态由最近一次成功应用的配置决定"
          ><span /></button>
        </div>
      </div>

      <div class="setting-row">
        <div class="setting-copy">
          <strong>配置变更自动重载</strong>
          <span>草稿保存后不会直接影响运行核心，请在配置工作台手动应用</span>
        </div>
        <div class="setting-control switch-control">
          <span>手动应用</span>
          <button
            class="setting-switch"
            type="button"
            role="switch"
            aria-label="配置变更自动重载（只读）"
            aria-checked="false"
            disabled
            title="当前仅支持确认后手动应用配置"
          ><span /></button>
        </div>
      </div>

      <div class="setting-row">
        <div class="setting-copy">
          <strong>核心分支</strong>
          <span>当前只使用经过校验的 sing-box 官方稳定版本</span>
        </div>
        <div class="setting-control">
          <select aria-label="核心分支" :value="stableLabel" disabled>
            <option :value="stableLabel">{{ stableLabel }}</option>
          </select>
        </div>
      </div>

      <div class="setting-row">
        <div class="setting-copy">
          <strong>核心版本</strong>
          <span>当前 {{ currentVersion }} · 最新 {{ latestVersion }}</span>
        </div>
        <div class="setting-control">
          <RouterLink class="manage-button" to="/core">
            管理版本<AppIcon name="chevron" :size="13" />
          </RouterLink>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.core-settings-card { overflow: hidden; }
.card-head { display: flex; min-height: 58px; align-items: center; justify-content: space-between; gap: 16px; padding: 12px 16px; border-bottom: 1px solid var(--line); }
.card-head h2, .card-head p { margin: 0; }
.card-head h2 { font-size: 14px; letter-spacing: -.15px; }
.card-head p { margin-top: 2px; color: var(--text-faint); font-size: 10px; }
.version-badge { display: inline-flex; min-height: 23px; align-items: center; gap: 5px; padding: 3px 8px; border-radius: 999px; color: var(--text-soft); background: var(--surface-soft); font-size: 9px; font-weight: 690; white-space: nowrap; }
.version-badge::before { width: 5px; height: 5px; border-radius: 50%; content: ""; background: currentColor; }
.version-badge.installed { color: var(--green); background: var(--green-soft); }
.load-error { margin: 10px 16px 0; padding: 9px 10px; border-radius: 8px; color: var(--red); background: var(--red-soft); font-size: 10px; }
.setting-list { padding: 4px 18px; }
.setting-row { display: flex; min-height: 66px; align-items: center; justify-content: space-between; gap: 20px; padding: 12px 0; border-bottom: 1px solid var(--line); }
.setting-row:last-child { border-bottom: 0; }
.setting-copy { min-width: 0; }
.setting-copy strong, .setting-copy span { display: block; }
.setting-copy strong { font-size: 11px; font-weight: 680; }
.setting-copy span { margin-top: 2px; color: var(--text-faint); font-size: 9px; }
.setting-control { min-width: 175px; text-align: right; }
.setting-control select { width: 175px; min-height: 36px; padding: 7px 9px; font-size: 10px; }
.setting-control select:disabled { cursor: default; opacity: 1; color: var(--text-soft); background: var(--surface-soft); }
.switch-control { display: flex; align-items: center; justify-content: flex-end; gap: 9px; }
.switch-control > span { color: var(--text-faint); font-size: 9px; font-weight: 620; }
.setting-switch { position: relative; width: 34px; min-height: 20px; flex: 0 0 auto; padding: 0; border-radius: 7px; background: var(--line-strong); }
.setting-switch:disabled { cursor: default; opacity: 1; }
.setting-switch > span { position: absolute; top: 4px; left: 4px; width: 12px; height: 12px; border-radius: 4px; background: #fff; transition: transform 150ms ease; }
.setting-switch.on { background: var(--primary); }
.setting-switch.on > span { transform: translateX(14px); }
.manage-button { display: inline-flex; min-height: 34px; align-items: center; justify-content: center; gap: 4px; padding: 6px 10px; border: 1px solid var(--line); border-radius: 8px; color: var(--text-soft); background: var(--surface-soft); font-size: 10px; font-weight: 650; text-decoration: none; }
.manage-button:hover { color: var(--primary); border-color: color-mix(in srgb, var(--primary) 24%, var(--line)); background: var(--primary-soft); }

@media (max-width: 560px) {
  .card-head { align-items: flex-start; }
  .setting-list { padding-inline: 15px; }
  .setting-row { align-items: flex-start; flex-direction: column; gap: 10px; }
  .setting-control, .setting-control select { width: 100%; min-width: 0; text-align: left; }
  .switch-control { justify-content: flex-start; }
  .manage-button { width: 100%; }
}
</style>
