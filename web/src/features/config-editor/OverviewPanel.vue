<script setup lang="ts">
import { computed } from "vue";
import AppIcon from "@/layout/AppIcon.vue";
import type { Draft } from "@/features/profiles/types";
import { configSections, type ConfigModule, type ConfigSection, type ConfigTabId } from "./module-map";
import {
  applyOverviewEdits,
  externalControllerPort,
  isJsonObject,
  mixedInboundIndex,
  replaceExternalControllerPort,
  type OverviewEdit
} from "./overview";

const props = defineProps<{
  draft: Draft;
  profileName: string;
  errors?: Record<string, string>;
}>();
const emit = defineEmits<{
  update: [module: ConfigModule, draft: Draft, error?: string];
  editJson: [module: ConfigModule];
  selectTab: [tab: ConfigTabId];
  rename: [name: string];
}>();

const log = computed(() => isJsonObject(props.draft.config.log) ? props.draft.config.log : {});
const dns = computed(() => isJsonObject(props.draft.config.dns) ? props.draft.config.dns : {});
const route = computed(() => isJsonObject(props.draft.config.route) ? props.draft.config.route : {});
const experimental = computed(() => isJsonObject(props.draft.config.experimental) ? props.draft.config.experimental : {});
const inbounds = computed(() => Array.isArray(props.draft.config.inbounds) ? props.draft.config.inbounds : []);
const mixedIndex = computed(() => mixedInboundIndex(props.draft.config));
const mixedInbound = computed(() => {
  const value = inbounds.value[mixedIndex.value];
  return isJsonObject(value) ? value : {};
});
const clashApi = computed(() => isJsonObject(experimental.value.clash_api) ? experimental.value.clash_api : null);
const cacheFile = computed(() => isJsonObject(experimental.value.cache_file) ? experimental.value.cache_file : null);

const logLevel = computed(() => typeof log.value.level === "string" ? log.value.level : "");
const mixedPort = computed(() => Number.isInteger(mixedInbound.value.listen_port) ? String(mixedInbound.value.listen_port) : "");
const apiPort = computed(() => externalControllerPort(clashApi.value?.external_controller) || "9090");
const cachePath = computed(() => typeof cacheFile.value?.path === "string" ? cacheFile.value.path : "cache.db");
const defaultInterface = computed(() => typeof route.value.default_interface === "string" ? route.value.default_interface : "");
const interfaceOptions = computed(() => [...new Set([defaultInterface.value, "eth0", "wlan0", "en0"].filter(Boolean))]);
const ipv6Enabled = computed(() => dns.value.strategy !== "ipv4_only");
const clashApiEnabled = computed(() => clashApi.value !== null);
const fakeIpPersisted = computed(() => cacheFile.value?.store_fakeip === true);

const configurationSummary = computed(() => ({
  configured: configSections.filter((section) => isConfigured(props.draft.config[section.key])).length,
  inbounds: inbounds.value.length,
  outbounds: Array.isArray(props.draft.config.outbounds) ? props.draft.config.outbounds.length : 0,
  rules: Array.isArray(route.value.rules) ? route.value.rules.length : 0
}));

const commonErrors = computed(() => [...new Set(
  ["log", "inbounds", "experimental", "route", "dns"]
    .map((module) => props.errors?.[module])
    .filter((message): message is string => Boolean(message))
)]);

function isConfigured(value: unknown): boolean {
  if (Array.isArray(value)) return value.length > 0;
  if (isJsonObject(value)) return Object.keys(value).length > 0;
  return value !== undefined && value !== null && value !== "";
}

function sectionSummary(section: ConfigSection): string {
  const value = props.draft.config[section.key];
  if (!isConfigured(value)) return "未配置";
  if (Array.isArray(value)) return `${value.length} 个项目`;
  if (isJsonObject(value)) return `${Object.keys(value).length} 个字段`;
  return "已配置";
}

function openSection(section: ConfigSection): void {
  if (section.key === "log") emit("editJson", "log");
  else emit("selectTab", section.group);
}

function update(module: ConfigModule, edits: readonly OverviewEdit[], emptyRoot: Record<string, never> | unknown[] = {}): void {
  const result = applyOverviewEdits(props.draft, module, edits, emptyRoot);
  emit("update", module, result.draft, result.error?.message);
}

function portValue(event: Event): number | null {
  const value = Number((event.currentTarget as HTMLInputElement).value);
  return Number.isInteger(value) && value >= 1 && value <= 65535 ? value : null;
}

function changeProfileName(event: Event): void {
  const input = event.currentTarget as HTMLInputElement;
  const name = input.value.trim();
  if (!name) {
    input.value = props.profileName;
    return;
  }
  if (name !== props.profileName) emit("rename", name);
}

function changeLogLevel(event: Event): void {
  const value = (event.currentTarget as HTMLSelectElement).value;
  update("log", [{ path: ["level"], value: value || undefined }]);
}

function uniqueMixedTag(): string {
  const tags = new Set(inbounds.value.flatMap((item) => isJsonObject(item) && typeof item.tag === "string" ? [item.tag] : []));
  if (!tags.has("mixed-in")) return "mixed-in";
  let suffix = 2;
  while (tags.has(`mixed-in-${suffix}`)) suffix += 1;
  return `mixed-in-${suffix}`;
}

function changeMixedPort(event: Event): void {
  const port = portValue(event);
  if (port === null) return;
  if (mixedIndex.value >= 0) {
    update("inbounds", [{ path: [mixedIndex.value, "listen_port"], value: port }], []);
    return;
  }
  update("inbounds", [{
    path: [inbounds.value.length],
    value: { type: "mixed", tag: uniqueMixedTag(), listen: "127.0.0.1", listen_port: port }
  }], []);
}

function changeApiPort(event: Event): void {
  const port = portValue(event);
  if (port === null) return;
  const controller = replaceExternalControllerPort(clashApi.value?.external_controller, port);
  if (clashApi.value) update("experimental", [{ path: ["clash_api", "external_controller"], value: controller }]);
  else update("experimental", [{ path: ["clash_api"], value: { external_controller: controller } }]);
}

function changeCachePath(event: Event): void {
  const value = (event.currentTarget as HTMLInputElement).value.trim();
  const edits: OverviewEdit[] = [{ path: ["cache_file", "path"], value: value || undefined }];
  if (value && !cacheFile.value) edits.unshift({ path: ["cache_file"], value: { enabled: true, path: value } });
  update("experimental", edits);
}

function changeInterface(event: Event): void {
  const value = (event.currentTarget as HTMLSelectElement).value;
  update("route", [{ path: ["default_interface"], value: value || undefined }]);
}

function setIPv6(enabled: boolean): void {
  update("dns", [{ path: ["strategy"], value: enabled ? undefined : "ipv4_only" }]);
}

function setClashApi(enabled: boolean): void {
  update("experimental", [{
    path: ["clash_api"],
    value: enabled ? { external_controller: `127.0.0.1:${apiPort.value}` } : undefined
  }]);
}

function setFakeIpPersistence(enabled: boolean): void {
  const edits: OverviewEdit[] = [];
  if (enabled && !cacheFile.value) edits.push({ path: ["cache_file"], value: { enabled: true, path: cachePath.value } });
  edits.push({ path: ["cache_file", "store_fakeip"], value: enabled });
  update("experimental", edits);
}
</script>

<template>
  <section class="overview-panel">
    <div class="config-health-strip" aria-label="配置摘要">
      <div><span><AppIcon name="check" :size="17" /></span><strong>{{ configurationSummary.configured }}<small>/{{ configSections.length }}</small></strong><em>已配置模块</em></div>
      <div><span><AppIcon name="inbound" :size="17" /></span><strong>{{ configurationSummary.inbounds }}</strong><em>入站</em></div>
      <div><span><AppIcon name="outbound" :size="17" /></span><strong>{{ configurationSummary.outbounds }}</strong><em>出站</em></div>
      <div><span><AppIcon name="route" :size="17" /></span><strong>{{ configurationSummary.rules }}</strong><em>路由规则</em></div>
    </div>

    <div class="config-layout config-overview-layout">
      <div class="config-main">
        <article class="card config-map-card">
          <header class="card-head">
            <div><h2 class="card-title">完整配置结构</h2><p class="card-subtitle">依据当前产品范围 · {{ configSections.length }} 个顶层模块</p></div>
            <span class="type-badge">config.json</span>
          </header>
          <div class="config-map">
            <button
              v-for="section in configSections"
              :key="section.key"
              type="button"
              class="config-map-item"
              :class="{ 'is-configured': isConfigured(draft.config[section.key]) }"
              :aria-label="`${section.label}：${sectionSummary(section)}`"
              @click="openSection(section)"
            >
              <span class="config-map-icon"><AppIcon :name="section.icon" :size="14" /></span>
              <span class="config-map-copy"><strong>{{ section.label }}</strong><code>{{ section.key }}</code></span>
              <span class="config-map-state">{{ sectionSummary(section) }}</span>
              <AppIcon name="chevron" class="config-map-chevron" :size="12" />
            </button>
          </div>
        </article>

        <article class="card common-settings-card">
          <header class="card-head">
            <div><h2 class="card-title">常用参数</h2><p class="card-subtitle">名称、日志、监听与控制端口</p></div>
            <span class="type-badge">log</span>
          </header>
          <div class="card-body">
            <div class="form-grid">
              <label class="form-field"><span class="field-label">配置名称<small>同步更新配置列表</small></span><input class="field-input" aria-label="配置名称" :value="profileName" @change="changeProfileName" /></label>
              <label class="form-field"><span class="field-label">日志等级<small>log.level</small></span><select class="field-select" aria-label="日志等级" :value="logLevel" :disabled="Boolean(errors?.log)" @change="changeLogLevel"><option value="">未设置</option><option v-for="item in ['trace', 'debug', 'info', 'warn', 'error', 'fatal', 'panic']" :key="item" :value="item">{{ item }}</option></select></label>
              <label class="form-field"><span class="field-label">Mixed 入站端口<small>listen_port</small></span><span class="input-suffix-wrap"><input class="field-input" aria-label="Mixed 入站端口" type="number" min="1" max="65535" :value="mixedPort" :disabled="Boolean(errors?.inbounds)" placeholder="未配置" @change="changeMixedPort" /><span class="input-suffix">端口</span></span></label>
              <label class="form-field"><span class="field-label">Clash API 端口<small>external_controller</small></span><span class="input-suffix-wrap"><input class="field-input" aria-label="Clash API 端口" type="number" min="1" max="65535" :value="apiPort" :disabled="Boolean(errors?.experimental)" @change="changeApiPort" /><span class="input-suffix">端口</span></span></label>
              <label class="form-field"><span class="field-label">缓存文件<small>核心工作目录下</small></span><input class="field-input" aria-label="缓存文件" :value="cachePath" :disabled="Boolean(errors?.experimental)" @change="changeCachePath" /></label>
              <label class="form-field"><span class="field-label">网络接口<small>default_interface</small></span><select class="field-select" aria-label="网络接口" :value="defaultInterface" :disabled="Boolean(errors?.route)" @change="changeInterface"><option value="">自动选择</option><option v-for="item in interfaceOptions" :key="item" :value="item">{{ item }}</option></select></label>
            </div>
            <p v-for="message in commonErrors" :key="message" class="form-error" role="alert">{{ message }}</p>
          </div>
        </article>
      </div>

      <aside class="config-aside">
        <article class="card feature-card">
          <header class="card-head"><div><h2 class="card-title">核心功能</h2><p class="card-subtitle">常用兼容与网络选项</p></div></header>
          <div class="card-body setting-list">
            <div class="setting-row"><div class="setting-copy"><strong>启用 IPv6</strong><span>允许 DNS 与出站连接使用 IPv6</span></div><button class="setting-switch" :class="{ on: ipv6Enabled }" type="button" role="switch" aria-label="启用 IPv6" :aria-checked="ipv6Enabled" :disabled="Boolean(errors?.dns)" @click="setIPv6(!ipv6Enabled)"><span /></button></div>
            <div class="setting-row"><div class="setting-copy"><strong>Clash API</strong><span>开放控制接口与流量统计</span></div><button class="setting-switch" :class="{ on: clashApiEnabled }" type="button" role="switch" aria-label="Clash API" :aria-checked="clashApiEnabled" :disabled="Boolean(errors?.experimental)" @click="setClashApi(!clashApiEnabled)"><span /></button></div>
            <div class="setting-row"><div class="setting-copy"><strong>FakeIP 持久化</strong><span>核心重启后保留映射</span></div><button class="setting-switch" :class="{ on: fakeIpPersisted }" type="button" role="switch" aria-label="FakeIP 持久化" :aria-checked="fakeIpPersisted" :disabled="Boolean(errors?.experimental)" @click="setFakeIpPersistence(!fakeIpPersisted)"><span /></button></div>
          </div>
        </article>
        <div class="info-callout"><AppIcon name="shield" :size="15" /><span>低频模块使用完整 JSON 片段编辑，未建模字段会原样保留。应用前仍应使用目标版本的 <code>sing-box check</code> 校验。</span></div>
      </aside>
    </div>
  </section>
</template>

<style scoped>
.overview-panel { display: grid; gap: 10px; }
.card { overflow: hidden; }
.card-head { display: flex; min-height: 50px; align-items: center; justify-content: space-between; gap: 16px; padding: 12px 14px; border-bottom: 1px solid var(--line); }
.card-title, .card-subtitle { margin: 0; }
.card-title { font-size: 14px; font-weight: 710; letter-spacing: -.15px; }
.card-subtitle { margin-top: 2px; color: var(--text-faint); font-size: 10px; }
.card-body { padding: 14px; }
.type-badge { display: inline-flex; min-height: 22px; align-items: center; padding: 3px 8px; border-radius: 999px; color: var(--text-soft); background: var(--surface-soft); font-size: 9px; font-weight: 660; white-space: nowrap; }
.config-health-strip { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 8px; }
.config-health-strip > div { display: grid; min-width: 0; min-height: 54px; grid-template-columns: auto auto minmax(0, 1fr); align-items: center; gap: 7px; padding: 8px 10px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface); box-shadow: var(--shadow-sm); }
.config-health-strip > div > span { display: grid; width: 27px; height: 27px; place-items: center; border-radius: 8px; color: var(--primary); background: var(--primary-soft); }
.config-health-strip > div:nth-child(2) > span { color: var(--cyan); background: var(--cyan-soft); }
.config-health-strip > div:nth-child(3) > span { color: var(--violet); background: color-mix(in srgb, var(--violet) 12%, transparent); }
.config-health-strip > div:nth-child(4) > span { color: var(--orange); background: var(--orange-soft); }
.config-health-strip strong { color: var(--text); font-size: 16px; font-variant-numeric: tabular-nums; }
.config-health-strip strong small { color: var(--text-faint); font-size: 8px; font-weight: 600; }
.config-health-strip em { overflow: hidden; color: var(--text-faint); font-size: 8px; font-style: normal; text-overflow: ellipsis; white-space: nowrap; }
.config-layout { display: grid; grid-template-columns: minmax(0, 1.68fr) minmax(250px, .62fr); gap: 12px; align-items: start; }
.config-main, .config-aside { display: grid; min-width: 0; gap: 12px; }
.config-map { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1px; padding: 8px; background: var(--line); }
.config-map-item { display: grid; min-width: 0; min-height: 53px; grid-template-columns: auto minmax(0, 1fr) auto auto; align-items: center; gap: 8px; padding: 8px 9px; border-radius: 0; color: var(--text-soft); background: var(--surface); text-align: left; transition: color 150ms ease, background 150ms ease; }
.config-map-item:hover { color: var(--primary); background: color-mix(in srgb, var(--primary-soft) 42%, var(--surface)); }
.config-map-icon { display: grid; width: 29px; height: 29px; place-items: center; border-radius: 8px; color: var(--text-faint); background: var(--surface-soft); }
.config-map-item.is-configured .config-map-icon { color: var(--primary); background: var(--primary-soft); }
.config-map-copy { min-width: 0; }
.config-map-copy strong, .config-map-copy code { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.config-map-copy strong { color: var(--text); font-size: 10px; font-weight: 670; }
.config-map-copy code { margin-top: 1px; color: var(--text-faint); font-size: 8px; }
.config-map-state { max-width: 100px; overflow: hidden; color: var(--text-faint); font-size: 8px; text-overflow: ellipsis; white-space: nowrap; }
.config-map-item.is-configured .config-map-state { color: var(--green); }
.config-map-chevron { color: var(--text-faint); }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.form-field { display: grid; min-width: 0; gap: 6px; }
.field-label { display: flex; align-items: center; justify-content: space-between; color: var(--text-soft); font-size: 10px; font-weight: 660; }
.field-label small { color: var(--text-faint); font-size: 9px; font-weight: 500; }
.field-input, .field-select { width: 100%; height: 35px; min-height: 35px; padding: 0 10px; border-color: var(--line); border-radius: 9px; background: var(--surface-soft); font-size: 10px; }
.field-select { appearance: none; background-image: linear-gradient(45deg, transparent 50%, var(--text-faint) 50%), linear-gradient(135deg, var(--text-faint) 50%, transparent 50%); background-position: calc(100% - 15px) 16px, calc(100% - 11px) 16px; background-repeat: no-repeat; background-size: 4px 4px, 4px 4px; }
.input-suffix-wrap { position: relative; }
.input-suffix-wrap .field-input { padding-right: 48px; }
.input-suffix { position: absolute; top: 50%; right: 11px; color: var(--text-faint); font-size: 9px; transform: translateY(-50%); }
.form-error { margin: 10px 0 0; color: var(--red); font-size: 10px; }
.setting-list { display: grid; }
.setting-row { display: flex; min-height: 57px; align-items: center; gap: 12px; padding: 10px 0; border-bottom: 1px solid var(--line); }
.setting-row:first-child { padding-top: 0; }
.setting-row:last-child { padding-bottom: 0; border-bottom: 0; }
.setting-copy { min-width: 0; flex: 1; }
.setting-copy strong, .setting-copy span { display: block; }
.setting-copy strong { font-size: 11px; font-weight: 650; }
.setting-copy span { margin-top: 2px; color: var(--text-faint); font-size: 9px; }
.setting-switch { position: relative; width: 34px; min-height: 20px; flex: 0 0 auto; padding: 0; border-radius: 7px; background: var(--line-strong); }
.setting-switch > span { position: absolute; top: 4px; left: 4px; width: 12px; height: 12px; border-radius: 4px; background: #fff; transition: transform 150ms ease; }
.setting-switch.on { background: var(--primary); }
.setting-switch.on > span { transform: translateX(14px); }
.info-callout { display: flex; gap: 10px; padding: 11px 12px; border: 1px solid color-mix(in srgb, var(--primary) 14%, var(--line)); border-radius: 10px; color: var(--text-soft); background: var(--primary-soft); font-size: 10px; }
.info-callout :deep(.icon) { flex: 0 0 auto; color: var(--primary); }
.info-callout code { color: var(--primary-strong); }

@media (max-width: 1080px) {
  .config-layout { grid-template-columns: minmax(0, 1fr); }
  .config-aside { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 768px) {
  .overview-panel { gap: 9px; }
  .config-health-strip { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px; }
  .config-health-strip > div { min-height: 49px; padding: 7px 8px; }
  .config-health-strip strong { font-size: 14px; }
  .config-aside, .form-grid { grid-template-columns: 1fr; }
  .config-map { padding: 6px; }
  .config-map-item { min-height: 49px; grid-template-columns: auto minmax(0, 1fr) auto; gap: 6px; padding: 7px; }
  .config-map-state { display: none; }
  .config-map-icon { width: 27px; height: 27px; }
  .field-input, .field-select { height: 40px; min-height: 40px; font-size: 11px; }
}
@media (max-width: 420px) {
  .card-head { align-items: flex-start; }
  .config-map-copy strong { font-size: 11px; }
  .config-map-copy code { font-size: 9px; }
  .field-label small { display: none; }
}
@media (min-width: 1280px) {
  .config-map { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .form-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); }
}
</style>
