<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { RouterLink } from "vue-router";
import AppToast from "@/components/AppToast.vue";
import AppIcon from "@/layout/AppIcon.vue";
import { coreApi, type CoreApi } from "@/features/core/api";
import type { CoreStatus, LatestRelease } from "@/features/core/types";
import { coreBehaviorSettingsApi, type CoreBehaviorSettings, type CoreBehaviorSettingsApi } from "./api";

export interface CoreSettingsApi extends Pick<CoreApi, "status" | "latest">, CoreBehaviorSettingsApi {}

const props = withDefaults(defineProps<{ api?: CoreSettingsApi }>(), {
  api: () => ({ status: coreApi.status, latest: coreApi.latest, get: coreBehaviorSettingsApi.get, update: coreBehaviorSettingsApi.update })
});

const status = ref<CoreStatus | null>(null);
const latest = ref<LatestRelease | null>(null);
const behavior = ref<CoreBehaviorSettings>({ autoStart: true, autoReload: false });
const behaviorLoaded = ref(false);
const loading = ref(true);
const saving = ref(false);
const error = ref("");
const savedMessage = ref("");

watch(savedMessage, (message, _previous, onCleanup) => {
  if (!message) return;
  const timer = setTimeout(() => { savedMessage.value = ""; }, 3000);
  onCleanup(() => clearTimeout(timer));
});

const currentVersion = computed(() => status.value?.installedVersion || "未安装");
const latestVersion = computed(() => latest.value?.version || "暂不可用");
const stableLabel = computed(() => `Stable · ${latest.value?.version || status.value?.installedVersion || "暂不可用"}`);

function errorMessage(cause: unknown, fallback: string): string {
  return cause instanceof Error ? cause.message : fallback;
}

onMounted(async () => {
  const [statusResult, latestResult, behaviorResult] = await Promise.allSettled([
    props.api.status(),
    props.api.latest(),
    props.api.get()
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
  if (behaviorResult.status === "fulfilled") {
    behavior.value = behaviorResult.value;
    behaviorLoaded.value = true;
  } else {
    failures.push(errorMessage(behaviorResult.reason, "读取核心行为设置失败"));
  }
  error.value = failures.join("；");
  loading.value = false;
});

async function toggle(key: keyof CoreBehaviorSettings): Promise<void> {
  if (!behaviorLoaded.value || saving.value) return;
  const previous = { ...behavior.value };
  behavior.value = { ...previous, [key]: !previous[key] };
  saving.value = true;
  error.value = "";
  savedMessage.value = "";
  try {
    behavior.value = await props.api.update(behavior.value);
    const label = key === "autoStart" ? "开机自动运行" : "配置自动重载";
    savedMessage.value = `${label}已${behavior.value[key] ? "开启" : "关闭"}`;
  } catch (cause) {
    behavior.value = previous;
    error.value = errorMessage(cause, "保存核心行为设置失败");
  } finally {
    saving.value = false;
  }
}
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

    <div class="setting-list" :aria-busy="loading">
      <div class="setting-row">
        <div class="setting-copy">
          <strong>开机自动运行</strong>
          <span>开启后，随控制台启动已应用配置；没有配置则保持停止。</span>
        </div>
        <div class="setting-control switch-control">
          <span>{{ loading ? "读取中" : saving ? "保存中" : behavior.autoStart ? "已启用" : "未启用" }}</span>
          <button
            class="setting-switch"
            :class="{ on: behavior.autoStart }"
            type="button"
            role="switch"
            aria-label="开机自动运行"
            :aria-checked="behavior.autoStart"
            :disabled="!behaviorLoaded || saving"
            title="切换控制台启动时是否自动运行已应用配置"
            @click="toggle('autoStart')"
          ><span /></button>
        </div>
      </div>

      <div class="setting-row">
        <div class="setting-copy">
          <strong>配置变更自动重载</strong>
          <span>开启后，保存当前已应用档案会自动检查并应用，并重启正在运行的核心。</span>
        </div>
        <div class="setting-control switch-control">
          <span>{{ loading ? "读取中" : saving ? "保存中" : behavior.autoReload ? "自动重载" : "手动应用" }}</span>
          <button
            class="setting-switch"
            :class="{ on: behavior.autoReload }"
            type="button"
            role="switch"
            aria-label="配置变更自动重载"
            :aria-checked="behavior.autoReload"
            :disabled="!behaviorLoaded || saving"
            title="仅自动处理当前已应用档案，其他档案仍保持草稿状态"
            @click="toggle('autoReload')"
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
    <AppToast v-if="error || savedMessage" :message="error || savedMessage" :tone="error ? 'error' : 'success'" />
  </section>
</template>

<style scoped>
.core-settings-card { overflow: hidden; }
.card-head { display: flex; min-height: 48px; align-items: center; justify-content: space-between; gap: var(--section-gap); padding: 8px var(--card-padding); border-bottom: 1px solid var(--line); }
.card-head h2, .card-head p { margin: 0; }
.card-head h2 { font-size: var(--font-size-heading); letter-spacing: -.15px; }
.card-head p { margin-top: 2px; color: var(--text-faint); font-size: var(--font-size-sm); }
.version-badge { display: inline-flex; min-height: 23px; align-items: center; gap: 5px; padding: 3px 8px; border-radius: 999px; color: var(--text-soft); background: var(--surface-soft); font-size: var(--font-size-xs); font-weight: 690; white-space: nowrap; }
.version-badge::before { width: 5px; height: 5px; border-radius: 50%; content: ""; background: currentColor; }
.version-badge.installed { color: var(--green); background: var(--green-soft); }
.setting-list { padding: 2px var(--card-padding); }
.setting-row { display: flex; min-height: 52px; align-items: center; justify-content: space-between; gap: var(--section-gap); padding: 8px 0; border-bottom: 1px solid var(--line); }
.setting-row:last-child { border-bottom: 0; }
.setting-copy { min-width: 0; }
.setting-copy strong, .setting-copy span { display: block; }
.setting-copy strong { font-size: var(--font-size-base); font-weight: 680; }
.setting-copy span { margin-top: 2px; color: var(--text-faint); font-size: var(--font-size-xs); }
.setting-control { min-width: 175px; text-align: right; }
.setting-control select { width: 175px; min-height: 36px; padding: 7px 9px; font-size: var(--font-size-base); }
.setting-control select:disabled { cursor: default; opacity: 1; color: var(--text-soft); background: var(--surface-soft); }
.switch-control { display: flex; align-items: center; justify-content: flex-end; gap: 9px; }
.switch-control > span { display: inline-block; min-width: 4em; color: var(--text-faint); font-size: var(--font-size-xs); font-weight: 620; }
.setting-switch { position: relative; width: 34px; min-height: 20px; flex: 0 0 auto; padding: 0; border-radius: 7px; cursor: pointer; background: var(--line-strong); }
.setting-switch:disabled { cursor: wait; opacity: .7; }
.setting-switch:focus-visible { outline: 2px solid color-mix(in srgb, var(--primary) 65%, transparent); outline-offset: 2px; }
.setting-switch > span { position: absolute; top: 4px; left: 4px; width: 12px; height: 12px; border-radius: 4px; background: #fff; transition: transform 150ms ease; }
.setting-switch.on { background: var(--primary); }
.setting-switch.on > span { transform: translateX(14px); }
.manage-button { display: inline-flex; min-height: 34px; align-items: center; justify-content: center; gap: 4px; padding: 6px 10px; border: 1px solid var(--line); border-radius: 8px; color: var(--text-soft); background: var(--surface-soft); font-size: var(--font-size-sm); font-weight: 650; text-decoration: none; }
.manage-button:hover { color: var(--primary); border-color: color-mix(in srgb, var(--primary) 24%, var(--line)); background: var(--primary-soft); }

@media (max-width: 560px) {
  .card-head { align-items: flex-start; }
  .setting-list { padding-inline: var(--card-padding); }
  .setting-row { align-items: flex-start; flex-direction: column; gap: 10px; }
  .setting-control, .setting-control select { width: 100%; min-width: 0; text-align: left; }
  .setting-row:has(.switch-control) { flex-direction: row; align-items: center; }
  .switch-control { width: auto; flex: 0 0 auto; }
  .manage-button { width: 100%; }
}
</style>
