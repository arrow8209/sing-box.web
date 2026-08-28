<script setup lang="ts">
import { computed, ref } from "vue";
import AppToast from "@/components/AppToast.vue";
import AppIcon from "@/layout/AppIcon.vue";
import { isApiError } from "@/features/profiles/api";
import ProfileActions from "@/features/profiles/ProfileActions.vue";
import ProfileDeleteDialog from "@/features/profiles/ProfileDeleteDialog.vue";
import ProfileDialog from "@/features/profiles/ProfileDialog.vue";
import { useProfileStore } from "@/features/profiles/store";
import type { Draft } from "@/features/profiles/types";
import ApplicationBar from "@/features/application/ApplicationBar.vue";
import PreviewDialog from "@/features/application/PreviewDialog.vue";
import ValidationErrors from "@/features/application/ValidationErrors.vue";
import { useApplicationStore } from "@/features/application/store";
import AdvancedPanel from "./AdvancedPanel.vue";
import ConfigTabs from "./ConfigTabs.vue";
import DnsConfigPanel from "./DnsConfigPanel.vue";
import GeneralSettingsPanel from "./GeneralSettingsPanel.vue";
import type { GeneralSettingsSave } from "./general-settings";
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
import ReferenceErrors from "./ReferenceErrors.vue";
import RouteConfigPanel from "./RouteConfigPanel.vue";
import type { RoutePanelSave } from "./route";
import StructuredModulePanel from "./StructuredModulePanel.vue";
import type { InboundPanelSave } from "./inbound";
import { validateModuleShape } from "./json";
import { configSectionFor, configTabs, type ConfigModule, type ConfigTabId } from "./module-map";
import { saveModuleBuffer } from "./paths";

const store = useProfileStore();
const application = useApplicationStore();
const active = ref<ConfigTabId>("general");
const dialogModule = ref<ConfigModule | null>(null);
const errors = ref<Record<string, string>>({});
const previewOpen = ref(false);
const renameOpen = ref(false);
const deletingId = ref<string | null>(null);
const requiresCoreDecision = ref(false);
const profileBusy = ref(false);
const profileError = ref("");
const notice = ref("");
const draft = computed(() => store.draft.value);
const profile = computed(() => store.profiles.value.find((item) => item.id === store.activeProfileId.value));
const deleting = computed(() => store.profiles.value.find((item) => item.id === deletingId.value) ?? null);
const activeModules = computed(() => configTabs.find((tab) => tab.id === active.value)?.modules ?? []);
const applyDisabledReason = computed(() => {
  if (profileBusy.value || application.applying.value) return "正在处理档案操作";
  if (store.saveState.value !== "saved") return "请先等待草稿保存完成";
  if (Object.keys(errors.value).length > 0) return "请先修正当前编辑错误";
  return "";
});
async function duplicateProfile(): Promise<void> {
  if (!profile.value) return;
  profileBusy.value = true; profileError.value = "";
  try { await store.duplicate(profile.value.id); application.clear(); } catch (cause) { profileError.value = cause instanceof Error ? cause.message : "复制失败"; } finally { profileBusy.value = false; }
}
function openRename(): void { profileError.value = ""; renameOpen.value = true; }
async function renameProfile(name: string): Promise<void> {
  if (!profile.value) return;
  profileBusy.value = true; profileError.value = "";
  try { await store.rename(profile.value.id, name); renameOpen.value = false; } catch (cause) { profileError.value = cause instanceof Error ? cause.message : "重命名失败"; } finally { profileBusy.value = false; }
}
function openDelete(): void { profileError.value = ""; deletingId.value = store.activeProfileId.value; }
async function deleteProfile(stopCurrent: boolean): Promise<void> {
  if (!deleting.value) return;
  profileBusy.value = true; profileError.value = "";
  try {
    await store.remove(deleting.value.id, deleting.value.name, stopCurrent);
    deletingId.value = null;
    requiresCoreDecision.value = false;
    application.clear();
  } catch (cause) {
    if (isApiError(cause) && cause.status === 409 && cause.body.code === "current_profile_requires_core_decision") requiresCoreDecision.value = true;
    else profileError.value = cause instanceof Error ? cause.message : "删除失败";
  } finally { profileBusy.value = false; }
}
function applyResult(next: Draft, module: string, error?: string): void { store.mutate((current) => Object.assign(current, next)); if (error) errors.value = { ...errors.value, [module]: error }; else { const copy = { ...errors.value }; delete copy[module]; errors.value = copy; } }
function persist(module: ConfigModule, text: string): boolean {
  if (!draft.value) return false;
  const prepared = module === "dns" ? ensureDefaultFakeIpDnsRuleText(text, draft.value.editorState?.dns) : text;
  const result = saveModuleBuffer(draft.value, module, prepared);
  const shapeError = validateModuleShape(prepared, module, configSectionFor(module).type);
  const error = result.error ?? shapeError;
  if (module === "dns" && !error) {
    const dns = parseDnsObjectText(prepared);
    if (dns) result.draft.editorState = { ...(result.draft.editorState ?? {}), dns: reconcileDnsEditorState(result.draft.editorState?.dns, dns) };
  }
  applyResult(result.draft, module, error?.message);
  return !error;
}
function save(module: ConfigModule, text: string): void { if (persist(module, text)) dialogModule.value = null; }
function dialogText(module: ConfigModule): string {
  if (!draft.value) return "";
  const buffer = draft.value.buffers[module];
  if (buffer?.trim()) return buffer;
  const configured = draft.value.config[module];
  if (configured !== undefined) return JSON.stringify(configured, null, 2);
  if (module === "ntp") return JSON.stringify({ enabled: true, server: "time.apple.com", server_port: 123, interval: "30m" }, null, 2);
  return configSectionFor(module).type === "array" ? "[]" : "{}";
}
function moduleConfigured(module: ConfigModule): boolean {
  const value = draft.value?.config[module];
  if (Array.isArray(value)) return value.length > 0;
  return typeof value === "object" && value !== null && Object.keys(value).length > 0;
}
function clearDialogModule(): void {
  if (dialogModule.value) save(dialogModule.value, "");
}
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
function saveInbounds(payload: InboundPanelSave): void {
  if (!draft.value) return;
  let result = saveModuleBuffer(draft.value, "inbounds", payload.text);
  let errorModule = "inbounds";
  if (!result.error && payload.routeText !== undefined) {
    result = saveModuleBuffer(result.draft, "route", payload.routeText);
    errorModule = "route";
  }
  const editorState: Record<string, unknown> = { ...(result.draft.editorState ?? {}), inbounds: payload.editorState };
  if (payload.routeEditorState) editorState.route = payload.routeEditorState;
  result.draft.editorState = editorState;
  applyResult(result.draft, errorModule, result.error?.message);
  if (!result.error) {
    const copy = { ...errors.value };
    delete copy.inbounds;
    if (payload.routeText !== undefined) delete copy.route;
    errors.value = copy;
  }
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
function fromGeneralSettings(payload: GeneralSettingsSave): void { applyResult(payload.draft, payload.module, payload.error); }
async function preview(): Promise<void> { await application.preview(); previewOpen.value = Boolean(application.previewResult.value); }
async function validate(): Promise<void> { await application.validate(); }
async function applyProfile(): Promise<void> {
  profileBusy.value = true; profileError.value = ""; notice.value = "";
  try {
    const result = await application.apply();
    if (result?.applied) {
      queueMicrotask(() => { notice.value = "配置检测通过，已复制配置文件。"; });
    } else if (result) {
      profileError.value = result.validation.message || "配置检查失败，未复制配置文件。";
    } else if (application.error.value) {
      profileError.value = application.error.value.message;
    }
  } finally {
    profileBusy.value = false;
  }
}
function jumpTo(path: string): void {
  const module = path.split(/[.\[/]/)[0] as ConfigModule;
  const tab = configTabs.find((candidate) => (candidate.modules as readonly string[]).includes(module));
  if (tab) active.value = tab.id;
}
</script>
<template>
  <section class="workspace">
    <template v-if="draft">
      <header class="config-context-bar">
        <span class="config-context-icon"><AppIcon name="config" /></span>
        <div class="config-context-copy"><h1>{{ profile?.name || "配置工作台" }}</h1><p>草稿会自动保存，未识别字段会按原始 JSON 文本保留。</p></div>
        <div class="config-context-controls">
          <span class="save-status" :class="store.saveState.value"><AppIcon :name="store.saveState.value === 'saved' ? 'check' : 'rotate'" :size="14" />{{ store.saveState.value === "saving" ? "正在保存" : store.saveState.value === "saved" ? "草稿已保存" : "存在未保存更改" }}</span>
          <ProfileActions :busy="profileBusy" @duplicate="duplicateProfile" @rename="openRename" @remove="openDelete" />
          <ApplicationBar :busy="profileBusy || application.applying.value" :apply-disabled="Boolean(applyDisabledReason)" :apply-title="applyDisabledReason || '检测通过后复制配置文件'" @preview="preview" @validate="validate" @apply="applyProfile" />
        </div>
      </header>
      <p v-if="profileError" class="profile-error" role="alert">{{ profileError }}</p>
      <ConfigTabs :active="active" @select="active = $event" />
      <GeneralSettingsPanel v-if="active === 'general'" :draft="draft" :error="errors.log" :experimental-error="errors.experimental" @save="fromGeneralSettings" @edit-json="dialogModule = 'log'" />
      <InboundConfigPanel v-else-if="active === 'inbounds'" :draft="draft" :profile-name="profile?.name ?? '当前配置'" :error="errors.inbounds" @edit-json="dialogModule = 'inbounds'" @save="saveInbounds" />
      <OutboundConfigPanel v-else-if="active === 'outbounds'" :draft="draft" :error="errors.outbounds" @edit-json="dialogModule = 'outbounds'" @save="saveOutbounds" />
      <RouteConfigPanel v-else-if="active === 'route'" :draft="draft" :error="errors.route" @edit-json="dialogModule = 'route'" @save="saveRoute" />
      <DnsConfigPanel v-else-if="active === 'dns'" :draft="draft" :error="errors.dns" @edit-json="dialogModule = 'dns'" @save="saveDns" />
      <AdvancedPanel v-else-if="active === 'advanced'" :draft="draft" :errors="errors" @edit-json="dialogModule = $event" />
      <div v-else class="modules"><StructuredModulePanel v-for="module in activeModules" :key="module" :draft="draft" :module="module" :error="errors[module]" @edit-json="dialogModule = $event" /></div>
      <ReferenceErrors :errors="errors" />
      <ValidationErrors :validation="application.validation.value" :error="application.error.value" @jump="jumpTo" />
      <ModuleJsonDialog v-if="dialogModule" :open="true" :module="dialogModule" :text="dialogText(dialogModule)" :configured="moduleConfigured(dialogModule)" :error="errors[dialogModule]" @close="dialogModule = null" @buffer="persist(dialogModule!, $event)" @save="save(dialogModule!, $event)" @clear="clearDialogModule" />
      <PreviewDialog :open="previewOpen" :preview="application.previewResult.value" @close="previewOpen = false" />
      <ProfileDialog :open="renameOpen" title="重命名档案" :initial-name="profile?.name" :busy="profileBusy" @close="renameOpen = false" @submit="renameProfile" />
      <ProfileDeleteDialog :open="deleting !== null" :name="deleting?.name ?? ''" :requires-core-decision="requiresCoreDecision" :busy="profileBusy" @close="deletingId = null; requiresCoreDecision = false" @submit="deleteProfile" />
      <AppToast v-if="notice" :message="notice" tone="success" />
    </template>
    <p v-else class="empty">请先在“配置档案”中选择一个可用档案。</p>
  </section>
</template>
<style scoped>
.workspace { max-width: 1240px; }
.config-context-bar { display: flex; min-height: 68px; align-items: center; gap: 11px; margin-bottom: 12px; padding: 11px 13px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }
.config-context-icon { display: grid; width: 38px; height: 38px; flex: 0 0 auto; place-items: center; border-radius: 11px; color: var(--primary); background: var(--primary-soft); }
.config-context-copy { min-width: 0; flex: 1; }
.config-context-controls { display: flex; flex: 0 0 auto; align-items: center; gap: 8px; }
.workspace h1 { overflow: hidden; margin: 0; font-size: 17px; letter-spacing: -.25px; text-overflow: ellipsis; white-space: nowrap; }
.workspace header p, .empty { overflow: hidden; margin: 3px 0 0; color: var(--text-faint); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.save-status { display: inline-flex; align-items: center; gap: 5px; padding: 5px 8px; border-radius: 999px; color: var(--orange); background: var(--orange-soft); font-size: 10px; font-weight: 650; white-space: nowrap; }
.save-status.saved { color: var(--green); background: var(--green-soft); }
.save-status.saving { color: var(--primary); background: var(--primary-soft); }
.profile-error { margin: 0 0 12px; padding: 10px 12px; border-radius: var(--radius-sm); color: var(--red); background: var(--red-soft); font-size: 11px; }
.modules { display: grid; gap: 14px; }
.tabs + .panel, .tabs + .outbound-editor { margin-top: 0; }
.empty { padding: 24px; border: 1px dashed var(--line-strong); border-radius: var(--radius); }

@media (max-width: 760px) {
  .config-context-bar { flex-wrap: wrap; }
  .config-context-copy { min-width: calc(100% - 54px); }
  .config-context-controls { width: 100%; flex-wrap: wrap; justify-content: flex-end; }
}
</style>
