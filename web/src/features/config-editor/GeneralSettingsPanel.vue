<script setup lang="ts">
import { computed } from "vue";
import AppIcon from "@/layout/AppIcon.vue";
import type { Draft } from "@/features/profiles/types";
import { applyGeneralSettingEdits, type GeneralSettingEdit, type GeneralSettingsModule, type GeneralSettingsSave } from "./general-settings";
import { isJsonObject } from "./json";

const props = defineProps<{ draft: Draft; error?: string; experimentalError?: string }>();
const emit = defineEmits<{
  save: [payload: GeneralSettingsSave];
  editJson: [];
}>();

const defaultLogPath = "data/logs/sing-box.log";
const workModes = [
  { value: "global", label: "全局", description: "所有流量使用默认出口" },
  { value: "rule", label: "规则", description: "按路由规则决定出口" },
  { value: "direct", label: "直连", description: "所有流量直接连接" }
] as const;
type WorkMode = (typeof workModes)[number]["value"];

const log = computed(() => isJsonObject(props.draft.config.log) ? props.draft.config.log : {});
const experimental = computed(() => isJsonObject(props.draft.config.experimental) ? props.draft.config.experimental : {});
const clashApi = computed(() => isJsonObject(experimental.value.clash_api) ? experimental.value.clash_api : {});
const workMode = computed<WorkMode>(() => workModes.some((item) => item.value === clashApi.value.default_mode)
  ? clashApi.value.default_mode as WorkMode : "rule");
const apiController = computed(() => typeof clashApi.value.external_controller === "string" ? clashApi.value.external_controller : "127.0.0.1:20124");
const apiSecret = computed(() => typeof clashApi.value.secret === "string" ? clashApi.value.secret : "");
const logEnabled = computed(() => log.value.disabled !== true);
const logLevel = computed(() => typeof log.value.level === "string" ? log.value.level : "info");
const logOutput = computed(() => typeof log.value.output === "string" && log.value.output.trim() ? log.value.output : defaultLogPath);
const timestampEnabled = computed(() => log.value.timestamp === true);

function update(module: GeneralSettingsModule, edits: readonly GeneralSettingEdit[]): void {
  const result = applyGeneralSettingEdits(props.draft, module, edits);
  emit("save", { draft: result.draft, module, error: result.error?.message });
}

function changeLevel(event: Event): void {
  update("log", [{ path: ["level"], value: (event.currentTarget as HTMLSelectElement).value }]);
}

function changeOutput(event: Event): void {
  const input = event.currentTarget as HTMLInputElement;
  const value = input.value.trim() || defaultLogPath;
  input.value = value;
  update("log", [{ path: ["output"], value }]);
}

function setLogEnabled(enabled: boolean): void {
  update("log", [{ path: ["disabled"], value: !enabled }]);
}

function setTimestamp(enabled: boolean): void {
  update("log", [{ path: ["timestamp"], value: enabled }]);
}

function setWorkMode(mode: WorkMode): void {
  update("experimental", [{ path: ["clash_api", "default_mode"], value: mode }]);
}

function changeApiController(event: Event): void {
  const input = event.currentTarget as HTMLInputElement;
  const value = input.value.trim();
  input.value = value;
  update("experimental", [{ path: ["clash_api", "external_controller"], value }]);
}

function changeApiSecret(event: Event): void {
  update("experimental", [{ path: ["clash_api", "secret"], value: (event.currentTarget as HTMLInputElement).value }]);
}

function generateApiSecret(): void {
  const bytes = new Uint8Array(32);
  globalThis.crypto.getRandomValues(bytes);
  const secret = Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join("");
  update("experimental", [{ path: ["clash_api", "secret"], value: secret }]);
}
</script>

<template>
  <section class="general-settings-panel">
    <div class="general-settings-layout">
      <div class="general-settings-main">
        <article class="card runtime-settings-card">
          <header class="card-head">
            <div class="card-heading">
              <span class="heading-icon"><AppIcon name="config" :size="17" /></span>
              <div>
                <h2>工作模式与 API</h2>
                <p>设置代理行为与本地控制接口</p>
              </div>
            </div>
            <span class="module-badge">clash_api</span>
          </header>

          <div class="runtime-settings-body">
            <section class="mode-setting" aria-labelledby="work-mode-title">
              <div class="setting-intro">
                <span class="setting-icon blue"><AppIcon name="route" :size="16" /></span>
                <div class="setting-copy">
                  <strong id="work-mode-title">工作模式</strong>
                  <small>默认使用规则模式，可随时切换流量处理方式</small>
                </div>
              </div>
              <div class="mode-options" role="radiogroup" aria-label="工作模式">
                <button v-for="item in workModes" :key="item.value" class="mode-option" :class="{ active: workMode === item.value }" type="button" role="radio" :aria-label="item.label" :aria-checked="workMode === item.value" :disabled="Boolean(experimentalError)" @click="setWorkMode(item.value)"><strong>{{ item.label }}</strong><small>{{ item.description }}</small></button>
              </div>
            </section>

            <label class="api-setting-row">
              <span class="setting-icon cyan"><AppIcon name="server" :size="16" /></span>
              <span class="setting-copy">
                <strong>API 监听地址</strong>
                <small>默认仅允许本机访问控制接口</small>
              </span>
              <input :value="apiController" class="api-control" aria-label="API 监听地址" :disabled="Boolean(experimentalError)" spellcheck="false" @change="changeApiController" />
            </label>

            <div class="api-setting-row">
              <span class="setting-icon orange"><AppIcon name="lock" :size="16" /></span>
              <div class="setting-copy">
                <strong>API 密钥</strong>
                <small>用于验证控制接口请求</small>
              </div>
              <div class="secret-control api-control">
                <input :value="apiSecret" type="password" aria-label="API 密钥" :disabled="Boolean(experimentalError)" autocomplete="new-password" placeholder="点击右侧按钮生成" spellcheck="false" @change="changeApiSecret" />
                <button class="generate-button" type="button" :disabled="Boolean(experimentalError)" @click="generateApiSecret"><AppIcon name="rotate" :size="13" />随机生成</button>
              </div>
            </div>
          </div>
          <p v-if="experimentalError" class="panel-error" role="alert">{{ experimentalError }}</p>
        </article>

        <article class="card log-settings-card">
        <header class="card-head">
          <div class="card-heading">
            <span class="heading-icon"><AppIcon name="terminal" :size="17" /></span>
            <div>
              <h2>日志设置</h2>
              <p>维护当前配置的全局日志行为</p>
            </div>
          </div>
          <div class="header-actions">
            <span class="module-badge">config.log</span>
            <button class="json-button" type="button" @click="emit('editJson')"><AppIcon name="code" :size="14" />编辑完整 JSON</button>
          </div>
        </header>

        <div class="settings-list">
          <div class="setting-row">
            <span class="setting-icon"><AppIcon name="power" :size="16" /></span>
            <div class="setting-copy">
              <strong>启用日志</strong>
              <small>关闭后核心不再输出运行日志</small>
            </div>
            <div class="switch-control">
              <span>{{ logEnabled ? "已启用" : "已关闭" }}</span>
              <button class="setting-switch" :class="{ on: logEnabled }" type="button" role="switch" aria-label="启用日志" :aria-checked="logEnabled" :disabled="Boolean(error)" @click="setLogEnabled(!logEnabled)"><span /></button>
            </div>
          </div>

          <label class="setting-row">
            <span class="setting-icon blue"><AppIcon name="gauge" :size="16" /></span>
            <span class="setting-copy">
              <strong>日志等级</strong>
              <small>控制记录内容的详细程度</small>
            </span>
            <select aria-label="日志等级" :value="logLevel" :disabled="Boolean(error)" @change="changeLevel">
              <option v-for="item in ['trace', 'debug', 'info', 'warn', 'error', 'fatal', 'panic']" :key="item" :value="item">{{ item }}</option>
            </select>
          </label>

          <label class="setting-row output-row">
            <span class="setting-icon cyan"><AppIcon name="folder" :size="16" /></span>
            <span class="setting-copy">
              <strong>日志保存路径</strong>
              <small>默认保存到服务数据目录下的日志文件</small>
            </span>
            <input aria-label="日志保存路径" :value="logOutput" :disabled="Boolean(error)" placeholder="data/logs/sing-box.log" @change="changeOutput" />
          </label>

          <div class="setting-row">
            <span class="setting-icon orange"><AppIcon name="clock" :size="16" /></span>
            <div class="setting-copy">
              <strong>显示时间戳</strong>
              <small>在每条日志前记录本地时间</small>
            </div>
            <div class="switch-control">
              <span>{{ timestampEnabled ? "显示" : "隐藏" }}</span>
              <button class="setting-switch" :class="{ on: timestampEnabled }" type="button" role="switch" aria-label="显示时间戳" :aria-checked="timestampEnabled" :disabled="Boolean(error)" @click="setTimestamp(!timestampEnabled)"><span /></button>
            </div>
          </div>
        </div>
        <p v-if="error" class="panel-error" role="alert">{{ error }}</p>
        </article>
      </div>

      <aside class="settings-aside">
        <article class="card scope-card">
          <header class="aside-head"><h2>参数归属</h2><p>每项配置只在一个位置维护</p></header>
          <div class="scope-list">
            <div><span><AppIcon name="config" :size="15" /></span><p><strong>运行模式与 API</strong><small>常用控制参数在当前页维护</small></p><em>当前页</em></div>
            <div><span><AppIcon name="terminal" :size="15" /></span><p><strong>基础日志</strong><small>日志输出行为在当前页维护</small></p><em>当前页</em></div>
            <div><span><AppIcon name="route" :size="15" /></span><p><strong>网络与解析</strong><small>分别前往入站、路由和 DNS</small></p></div>
            <div><span><AppIcon name="sparkles" :size="15" /></span><p><strong>其余扩展能力</strong><small>平台相关配置位于高级</small></p></div>
          </div>
        </article>
        <p class="info-callout"><AppIcon name="info" :size="15" /><span>入站端口、出站、路由与 DNS 参数仍只在各自页面维护。</span></p>
      </aside>
    </div>
  </section>
</template>

<style scoped>
.general-settings-panel { display: grid; gap: 10px; }
.general-settings-layout { display: grid; grid-template-columns: minmax(0, 1fr) 240px; gap: var(--section-gap); align-items: start; }
.general-settings-main, .settings-aside { display: grid; min-width: 0; gap: var(--section-gap); }
.card { overflow: hidden; }
.card-head { display: flex; min-height: 48px; align-items: center; justify-content: space-between; gap: var(--section-gap); padding: 8px var(--card-padding); border-bottom: 1px solid var(--line); }
.card-heading { display: flex; min-width: 0; align-items: center; gap: 10px; }
.heading-icon { display: grid; width: 30px; height: 30px; flex: 0 0 auto; place-items: center; border-radius: 10px; color: var(--primary); background: var(--primary-soft); }
.card-head h2, .card-head p, .aside-head h2, .aside-head p { margin: 0; }
.card-head h2, .aside-head h2 { font-size: 13px; }
.card-head p, .aside-head p { margin-top: 2px; color: var(--text-faint); font-size: 9px; }
.header-actions { display: flex; flex: 0 0 auto; align-items: center; gap: 7px; }
.module-badge { display: inline-flex; min-height: 25px; align-items: center; padding: 4px 8px; border-radius: 999px; color: var(--primary); background: var(--primary-soft); font: 680 8px ui-monospace, SFMono-Regular, Consolas, monospace; }
.json-button { display: inline-flex; min-height: 31px; align-items: center; gap: 5px; padding: 5px 8px; color: var(--text-soft); border: 1px solid var(--line); background: var(--surface-soft); font-size: 9px; }
.runtime-settings-body { display: grid; padding: 2px var(--card-padding); }
.mode-setting { display: grid; gap: 8px; padding: 10px 0; border-bottom: 1px solid var(--line); }
.setting-intro { display: flex; min-width: 0; align-items: center; gap: var(--section-gap); }
.mode-options { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; padding-left: 40px; }
.mode-option { display: grid; min-height: 44px; align-content: center; gap: 2px; padding: 8px 10px; color: var(--text-soft); border: 1px solid var(--line); border-radius: 9px; background: var(--surface-soft); text-align: left; }
.mode-option:hover { color: var(--primary); border-color: color-mix(in srgb, var(--primary) 26%, var(--line)); }
.mode-option.active { color: var(--primary-strong); border-color: color-mix(in srgb, var(--primary) 35%, var(--line)); background: var(--primary-soft); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--primary) 8%, transparent); }
.mode-option strong, .mode-option small { display: block; }
.mode-option strong { font-size: 10px; }
.mode-option small { color: var(--text-faint); font-size: 8px; font-weight: 500; }
.api-setting-row { display: grid; min-width: 0; min-height: 54px; grid-template-columns: auto minmax(0, 1fr) minmax(260px, .72fr); align-items: center; gap: var(--section-gap); padding: 8px 0; border-bottom: 1px solid var(--line); }
.api-setting-row:last-child { border-bottom: 0; }
.api-control { width: 100%; min-width: 0; min-height: 36px; font-size: 9px; }
.secret-control { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 6px; }
.secret-control input { min-width: 0; font-family: ui-monospace, SFMono-Regular, Consolas, monospace; }
.generate-button { display: inline-flex; min-height: 36px; align-items: center; justify-content: center; gap: 5px; padding: 6px 9px; color: var(--primary); border: 1px solid color-mix(in srgb, var(--primary) 22%, var(--line)); background: var(--primary-soft); font-size: 8px; white-space: nowrap; }
.settings-list { display: grid; padding: 2px var(--card-padding); }
.setting-row { display: grid; min-width: 0; min-height: 54px; grid-template-columns: auto minmax(0, 1fr) minmax(170px, .52fr); align-items: center; gap: var(--section-gap); padding: 8px 0; border-bottom: 1px solid var(--line); }
.setting-row:last-child { border-bottom: 0; }
.setting-icon { display: grid; width: 30px; height: 30px; place-items: center; border-radius: 9px; color: var(--green); background: var(--green-soft); }
.setting-icon.blue { color: var(--primary); background: var(--primary-soft); }
.setting-icon.cyan { color: var(--cyan); background: var(--cyan-soft); }
.setting-icon.orange { color: var(--orange); background: var(--orange-soft); }
.setting-copy { min-width: 0; }
.setting-copy strong, .setting-copy small { display: block; }
.setting-copy strong { color: var(--text); font-size: 10px; font-weight: 670; }
.setting-copy small { margin-top: 3px; color: var(--text-faint); font-size: 8px; font-weight: 500; }
.setting-row select, .setting-row input { width: 100%; min-width: 0; min-height: 36px; font-size: 9px; }
.switch-control { display: flex; align-items: center; justify-content: flex-end; gap: 9px; }
.switch-control > span { color: var(--text-faint); font-size: 8px; font-weight: 620; }
.setting-switch { position: relative; width: 34px; min-height: 20px; flex: 0 0 auto; padding: 0; border-radius: 7px; background: var(--line-strong); }
.setting-switch > span { position: absolute; top: 4px; left: 4px; width: 12px; height: 12px; border-radius: 4px; background: #fff; transition: transform 150ms ease; }
.setting-switch.on { background: var(--primary); }
.setting-switch.on > span { transform: translateX(14px); }
.panel-error { margin: 0 14px 12px; padding: 9px 10px; border-radius: 9px; color: var(--red); background: var(--red-soft); font-size: 9px; }
.aside-head { padding: 8px var(--card-padding); border-bottom: 1px solid var(--line); }
.scope-list { display: grid; padding: 5px 13px 10px; }
.scope-list > div { display: grid; min-width: 0; min-height: 46px; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 9px; border-bottom: 1px solid var(--line); }
.scope-list > div:last-child { border-bottom: 0; }
.scope-list > div > span { display: grid; width: 30px; height: 30px; place-items: center; border-radius: 8px; color: var(--primary); background: var(--primary-soft); }
.scope-list p { min-width: 0; margin: 0; }
.scope-list strong, .scope-list small { display: block; }
.scope-list strong { font-size: 9px; }
.scope-list small { margin-top: 2px; color: var(--text-faint); font-size: 8px; }
.scope-list em { padding: 3px 6px; border-radius: 6px; color: var(--green); background: var(--green-soft); font-size: 7px; font-style: normal; font-weight: 680; white-space: nowrap; }
.info-callout { display: flex; gap: 8px; margin: 0; padding: 11px 12px; border: 1px solid color-mix(in srgb, var(--primary) 14%, var(--line)); border-radius: 10px; color: var(--primary); background: var(--primary-soft); font-size: 9px; }
.info-callout span { color: var(--text-soft); }

@media (min-width: 1440px) {
  .general-settings-main { grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: start; }
}
@media (max-width: 1080px) {
  .general-settings-layout { grid-template-columns: minmax(0, 1fr); }
  .settings-aside { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 720px) {
  .card-head { flex-wrap: wrap; }
  .header-actions { margin-left: auto; }
  .setting-row { grid-template-columns: auto minmax(0, 1fr); gap: 9px; }
  .setting-row > :last-child { grid-column: 2; width: 100%; }
  .setting-row:has(.switch-control) { grid-template-columns: auto minmax(0, 1fr) auto; }
  .setting-row > .switch-control { grid-column: 3; grid-row: 1; }
  .api-setting-row { grid-template-columns: auto minmax(0, 1fr); gap: 9px; }
  .api-control { grid-column: 2; }
  .mode-options { padding-left: 0; }
  .switch-control { justify-content: flex-start; }
  .settings-aside { grid-template-columns: 1fr; }
}
@media (max-width: 420px) {
  .settings-list { padding-inline: 12px; }
  .module-badge { display: none; }
  .json-button { width: 100%; justify-content: center; }
  .mode-option { min-height: 48px; padding-inline: 8px; }
  .mode-option small { font-size: 7px; }
  .setting-copy strong { font-size: 11px; }
  .setting-copy small { font-size: 9px; }
}
</style>
