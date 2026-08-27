<script setup lang="ts">
import { computed, ref } from "vue";
import AppIcon from "@/layout/AppIcon.vue";
import { useProfileStore } from "@/features/profiles/store";
import type { Draft } from "@/features/profiles/types";
import ApplicationBar from "@/features/application/ApplicationBar.vue";
import PreviewDialog from "@/features/application/PreviewDialog.vue";
import ValidationErrors from "@/features/application/ValidationErrors.vue";
import { useApplicationStore } from "@/features/application/store";
import ConfigTabs from "./ConfigTabs.vue";
import DnsConfigPanel from "./DnsConfigPanel.vue";
import InboundConfigPanel from "./InboundConfigPanel.vue";
import ModuleJsonDialog from "./ModuleJsonDialog.vue";
import OutboundConfigPanel from "./OutboundConfigPanel.vue";
import {
  ensureDefaultFakeIpDnsRuleText,
  parseDnsObjectText,
  reconcileDnsEditorState,
  renameDnsServerInRuleText,
  renameDnsServerTagInBuffer,
  type DnsPanelSave
} from "./dns";
import { renameOutboundTagInBuffer, type OutboundPanelSave } from "./outbound";
import OverviewPanel from "./OverviewPanel.vue";
import ReferenceErrors from "./ReferenceErrors.vue";
import RouteConfigPanel from "./RouteConfigPanel.vue";
import type { RoutePanelSave } from "./route";
import StructuredModulePanel from "./StructuredModulePanel.vue";
import { configTabs, moduleLabels, type ConfigModule, type ConfigTabId } from "./module-map";
import { saveModuleBuffer } from "./paths";

const store = useProfileStore();
const application = useApplicationStore();
const active = ref<ConfigTabId>("overview");
const dialogModule = ref<ConfigModule | null>(null);
const errors = ref<Record<string, string>>({});
const previewOpen = ref(false);
const topLevelModules = new Set(["$schema", ...Object.keys(moduleLabels)]);
const topLevelModuleCount = topLevelModules.size;
const draft = computed(() => store.draft.value);
const profile = computed(() => store.profiles.value.find((item) => item.id === store.activeProfileId.value));
const activeModules = computed(() => configTabs.find((tab) => tab.id === active.value)?.modules ?? []);
const configurationSummary = computed(() => {
  const config = draft.value?.config ?? {};
  const configured = Object.entries(config).filter(([key, value]) => topLevelModules.has(key) && (Array.isArray(value) ? value.length > 0 : Boolean(value && typeof value === "object" ? Object.keys(value as Record<string, unknown>).length : value))).length;
  const route = config.route as Record<string, unknown> | undefined;
  return { configured, inbounds: Array.isArray(config.inbounds) ? config.inbounds.length : 0, outbounds: Array.isArray(config.outbounds) ? config.outbounds.length : 0, rules: Array.isArray(route?.rules) ? route.rules.length : 0 };
});
function applyResult(next: Draft, module: string, error?: string): void { store.mutate((current) => Object.assign(current, next)); if (error) errors.value = { ...errors.value, [module]: error }; else { const copy = { ...errors.value }; delete copy[module]; errors.value = copy; } }
function persist(module: ConfigModule, text: string): boolean {
  if (!draft.value) return false;
  const prepared = module === "dns" ? ensureDefaultFakeIpDnsRuleText(text, draft.value.editorState?.dns) : text;
  const result = saveModuleBuffer(draft.value, module, prepared);
  if (module === "dns" && !result.error) {
    const dns = parseDnsObjectText(prepared);
    if (dns) result.draft.editorState = { ...(result.draft.editorState ?? {}), dns: reconcileDnsEditorState(result.draft.editorState?.dns, dns) };
  }
  applyResult(result.draft, module, result.error?.message);
  return !result.error;
}
function save(module: ConfigModule, text: string): void { if (persist(module, text)) dialogModule.value = null; }
function saveOutbounds(payload: OutboundPanelSave): void {
  if (!draft.value) return;
  let result = saveModuleBuffer(draft.value, "outbounds", payload.text);
  if (payload.renamed) {
    for (const [module, source] of Object.entries(result.draft.buffers)) {
      if (module === "outbounds") continue;
      const updated = renameOutboundTagInBuffer(source, module, payload.renamed.from, payload.renamed.to);
      if (updated !== source) result = saveModuleBuffer(result.draft, module, updated);
    }
  }
  result.draft.editorState = { ...(result.draft.editorState ?? {}), outbounds: payload.editorState };
  applyResult(result.draft, "outbounds", result.error?.message);
}
function saveRoute(payload: RoutePanelSave): void {
  if (!draft.value) return;
  const result = saveModuleBuffer(draft.value, "route", payload.text);
  result.draft.bindings = payload.bindings.map((binding) => ({ ...binding }));
  result.draft.editorState = { ...(result.draft.editorState ?? {}), route: payload.editorState };
  applyResult(result.draft, "route", result.error?.message);
}
function renameDisabledDnsServerReferences(value: unknown, from: string, to: string): unknown {
  if (!value || typeof value !== "object" || Array.isArray(value)) return value;
  const state = value as Record<string, unknown>;
  if (!Array.isArray(state.disabledRules)) return value;
  return {
    ...state,
    disabledRules: state.disabledRules.map((item) => {
      if (!item || typeof item !== "object" || Array.isArray(item) || typeof (item as Record<string, unknown>).text !== "string") return item;
      const record = item as Record<string, unknown>;
      return { ...record, text: renameDnsServerInRuleText(String(record.text), from, to) };
    })
  };
}
function saveDns(payload: DnsPanelSave): void {
  if (!draft.value) return;
  const text = ensureDefaultFakeIpDnsRuleText(payload.text, payload.editorState);
  let result = saveModuleBuffer(draft.value, "dns", text);
  if (payload.renamed) {
    for (const [module, source] of Object.entries(result.draft.buffers)) {
      if (module === "dns") continue;
      const updated = renameDnsServerTagInBuffer(source, module, payload.renamed.from, payload.renamed.to);
      if (updated !== source) result = saveModuleBuffer(result.draft, module, updated);
    }
  }
  const dns = parseDnsObjectText(text);
  const dnsEditorState = dns ? reconcileDnsEditorState(payload.editorState, dns) : payload.editorState;
  const editorState: Record<string, unknown> = { ...(result.draft.editorState ?? {}), dns: dnsEditorState };
  if (payload.renamed && editorState.route) {
    editorState.route = renameDisabledDnsServerReferences(editorState.route, payload.renamed.from, payload.renamed.to);
  }
  result.draft.editorState = editorState;
  result.draft.bindings = payload.bindings.map((binding) => ({ ...binding }));
  applyResult(result.draft, "dns", result.error?.message);
}
function fromStructured(next: Draft, error?: string): void { applyResult(next, "log", error); }
async function preview(): Promise<void> { await application.preview(); previewOpen.value = Boolean(application.previewResult.value); }
async function validate(): Promise<void> { await application.validate(); }
function jumpTo(path: string): void {
  const module = path.split(/[.\[/]/)[0] as ConfigModule;
  const tab = configTabs.find((candidate) => (candidate.modules as readonly string[]).includes(module));
  if (tab) active.value = tab.id;
}
</script>
<template>
  <section class="workspace"><template v-if="draft"><header class="config-context-bar"><span class="config-context-icon"><AppIcon name="config" /></span><div class="config-context-copy"><h1>{{ profile?.name || "配置工作台" }}</h1><p>草稿会自动保存，未识别字段会按原始 JSON 文本保留。</p></div><div class="config-context-controls"><span class="save-status" :class="store.saveState.value"><AppIcon :name="store.saveState.value === 'saved' ? 'check' : 'rotate'" :size="14" />{{ store.saveState.value === "saving" ? "正在保存" : store.saveState.value === "saved" ? "草稿已保存" : "存在未保存更改" }}</span><a class="manage-profiles" href="/profiles">管理档案</a><ApplicationBar @preview="preview" @validate="validate" /></div></header><div class="config-health-strip" aria-label="配置摘要"><div><span><AppIcon name="check" :size="17" /></span><strong>{{ configurationSummary.configured }}<small>/{{ topLevelModuleCount }}</small></strong><em>已配置模块</em></div><div><span><AppIcon name="inbound" :size="17" /></span><strong>{{ configurationSummary.inbounds }}</strong><em>入站</em></div><div><span><AppIcon name="outbound" :size="17" /></span><strong>{{ configurationSummary.outbounds }}</strong><em>出站</em></div><div><span><AppIcon name="route" :size="17" /></span><strong>{{ configurationSummary.rules }}</strong><em>路由规则</em></div></div><ConfigTabs :active="active" @select="active = $event" /><OverviewPanel v-if="active === 'overview'" :draft="draft" :error="errors.log" @update="fromStructured" @edit-json="dialogModule = $event" /><InboundConfigPanel v-else-if="active === 'inbounds'" :draft="draft" :profile-name="profile?.name ?? '当前配置'" :error="errors.inbounds" @edit-json="dialogModule = 'inbounds'" @save="persist('inbounds', $event)" /><OutboundConfigPanel v-else-if="active === 'outbounds'" :draft="draft" :error="errors.outbounds" @edit-json="dialogModule = 'outbounds'" @save="saveOutbounds" /><RouteConfigPanel v-else-if="active === 'route'" :draft="draft" :error="errors.route" @edit-json="dialogModule = 'route'" @save="saveRoute" /><DnsConfigPanel v-else-if="active === 'dns'" :draft="draft" :error="errors.dns" @edit-json="dialogModule = 'dns'" @save="saveDns" /><div v-else class="modules"><StructuredModulePanel v-for="module in activeModules" :key="module" :draft="draft" :module="module" :error="errors[module]" @edit-json="dialogModule = $event" /></div><ReferenceErrors :errors="errors" /><ValidationErrors :validation="application.validation.value" :error="application.error.value" @jump="jumpTo" /><ModuleJsonDialog v-if="dialogModule" :open="true" :title="`编辑 ${moduleLabels[dialogModule]} JSON`" :text="draft.buffers[dialogModule] ?? ''" :error="errors[dialogModule]" @close="dialogModule = null" @buffer="persist(dialogModule!, $event)" @save="save(dialogModule!, $event)" /><PreviewDialog :open="previewOpen" :preview="application.previewResult.value" @close="previewOpen = false" /></template><p v-else class="empty">请先在“配置档案”中选择一个可用档案。</p></section>
</template>
<style scoped>
.workspace { max-width: 1240px; }.config-context-bar { display: flex; min-height: 68px; align-items: center; gap: 11px; margin-bottom: 12px; padding: 11px 13px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.config-context-icon { display: grid; width: 38px; height: 38px; flex: 0 0 auto; place-items: center; border-radius: 11px; color: var(--primary); background: var(--primary-soft); }.config-context-copy { min-width: 0; flex: 1; }.config-context-controls { display: flex; flex: 0 0 auto; align-items: center; gap: 8px; }.workspace h1 { overflow: hidden; margin: 0; font-size: 17px; letter-spacing: -.25px; text-overflow: ellipsis; white-space: nowrap; }.workspace header p, .empty { overflow: hidden; margin: 3px 0 0; color: var(--text-faint); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }.save-status { display: inline-flex; align-items: center; gap: 5px; padding: 5px 8px; border-radius: 999px; color: var(--orange); background: var(--orange-soft); font-size: 10px; font-weight: 650; white-space: nowrap; }.save-status.saved { color: var(--green); background: var(--green-soft); }.save-status.saving { color: var(--primary); background: var(--primary-soft); }.manage-profiles { padding: 7px 9px; border-radius: 8px; color: var(--text-soft); background: var(--surface-soft); font-size: 10px; font-weight: 650; text-decoration: none; white-space: nowrap; }.manage-profiles:hover { color: var(--primary); background: var(--primary-soft); }.config-health-strip { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; margin-bottom: 12px; }.config-health-strip > div { display: grid; grid-template-columns: 34px minmax(0, 1fr); grid-template-rows: auto auto; align-items: center; gap: 0 9px; min-height: 64px; padding: 9px 11px; border: 1px solid var(--line); border-radius: 11px; background: var(--surface); box-shadow: var(--shadow-sm); }.config-health-strip > div > span { grid-row: span 2; display: grid; width: 34px; height: 34px; place-items: center; border-radius: 9px; color: var(--primary); background: var(--primary-soft); }.config-health-strip > div:nth-child(2) > span { color: var(--cyan); background: var(--cyan-soft); }.config-health-strip > div:nth-child(3) > span { color: var(--violet); background: color-mix(in srgb, var(--violet) 12%, transparent); }.config-health-strip > div:nth-child(4) > span { color: var(--orange); background: var(--orange-soft); }.config-health-strip strong { overflow: hidden; font-size: 18px; line-height: 1.1; text-overflow: ellipsis; white-space: nowrap; }.config-health-strip strong small { margin-left: 1px; color: var(--text-faint); font-size: 10px; }.config-health-strip em { color: var(--text-faint); font-size: 9px; font-style: normal; }.modules { display: grid; gap: 14px; margin-top: 18px; }.tabs + .panel, .tabs + .outbound-editor { margin-top: 0; }.empty { padding: 24px; border: 1px dashed var(--line-strong); border-radius: var(--radius); }
@media (max-width: 760px) { .config-context-bar { flex-wrap: wrap; }.config-context-copy { min-width: calc(100% - 54px); }.config-context-controls { width: 100%; flex-wrap: wrap; justify-content: flex-end; }.config-health-strip { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }.config-health-strip > div { min-height: 56px; padding: 8px; }.config-health-strip > div > span { width: 30px; height: 30px; }.config-health-strip strong { font-size: 16px; } }
</style>
