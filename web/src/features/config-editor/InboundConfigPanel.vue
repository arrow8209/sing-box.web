<script setup lang="ts">
import { computed, ref, toRaw } from "vue";
import AppToast from "@/components/AppToast.vue";
import AppIcon from "@/layout/AppIcon.vue";
import type { Draft } from "@/features/profiles/types";
import InboundEditorDialog from "@/features/inbounds/InboundEditorDialog.vue";
import InboundImportDialog from "@/features/inbounds/InboundImportDialog.vue";
import { editorType, inboundEndpoint, inboundTypes, isInboundConfig, resourceMode, uniqueInboundTag, withoutLegacyInboundFields } from "@/features/inbounds/model";
import { useInboundStore } from "@/features/inbounds/store";
import type { InboundConfig, InboundDetail, InboundInput } from "@/features/inbounds/types";
import { applyInboundRouteIntents, managedInboundSniff, type InboundPanelSave, type InboundRouteIntent } from "./inbound";

const props = defineProps<{ draft: Draft; profileName: string; error?: string }>();
const emit = defineEmits<{ editJson: []; save: [payload: InboundPanelSave] }>();
const resources = useInboundStore();
const importOpen = ref(false);
const editorOpen = ref(false);
const importing = ref(false);
const editingIndex = ref<number | null>(null);
const editing = ref<InboundDetail | null>(null);
const notice = ref("");
const noticeTone = ref<"info" | "error" | "success">("info");

const rawItems = computed<unknown[]>(() => Array.isArray(props.draft.config.inbounds) ? props.draft.config.inbounds : []);
const entries = computed(() => rawItems.value.flatMap((value, index) => isInboundConfig(value) ? [{ index, config: value }] : []));
const invalidCount = computed(() => rawItems.value.length - entries.value.length);
const tags = computed(() => entries.value.map((item) => item.config.tag));

function show(message: string, tone: "info" | "error" | "success" = "info"): void {
  notice.value = "";
  noticeTone.value = tone;
  queueMicrotask(() => { notice.value = message; });
}
function iconFor(config: InboundConfig): string {
  return editorType(config) === "custom" ? "code" : inboundTypes.find((item) => item.type === config.type)?.icon ?? "inbound";
}
function cloneItems(): unknown[] { return structuredClone(toRaw(rawItems.value)); }
function cloneConfig(config: InboundConfig): InboundConfig { return structuredClone(toRaw(config)); }
function saveItems(items: unknown[], intents: InboundRouteIntent[] = []): boolean {
  try {
    const route = applyInboundRouteIntents(props.draft, intents);
    emit("save", { text: JSON.stringify(items, null, 2), ...route });
    return true;
  } catch (cause) {
    show(cause instanceof Error ? cause.message : "无法更新协议嗅探路由规则", "error");
    return false;
  }
}
function closeEditor(): void {
  editorOpen.value = false;
  editing.value = null;
  editingIndex.value = null;
}
function openEditor(index: number, config: InboundConfig): void {
  editingIndex.value = index;
  editing.value = { id: `profile-inbound-${index}`, mode: resourceMode(config), config: cloneConfig(config), updatedAt: props.draft.savedAt };
  editorOpen.value = true;
}
function submitEdit(input: InboundInput): void {
  if (editingIndex.value === null) return;
  const next = cloneItems();
  next[editingIndex.value] = structuredClone(toRaw(input.config));
  const previousTag = editing.value?.config.tag ?? input.config.tag;
  const managed = managedInboundSniff(props.draft, previousTag);
  const legacySniff = input.mode === "structured" && editing.value?.mode === "structured" && editing.value.config.sniff === true;
  const legacyTimeout = typeof editing.value?.config.sniff_timeout === "string" && editing.value.config.sniff_timeout.trim()
    ? editing.value.config.sniff_timeout : "300ms";
  const intents = managed.enabled
    ? [{ previousTag, tag: input.config.tag, sniff: true, sniffTimeout: managed.timeout }]
    : legacySniff ? [{ previousTag, tag: input.config.tag, sniff: true, sniffTimeout: legacyTimeout }] : [];
  if (!saveItems(next, intents)) return;
  const tag = input.config.tag;
  closeEditor();
  show(`${tag} 已更新为当前自定义配置的独立副本。`, "success");
}
function duplicate(index: number, config: InboundConfig): void {
  const next = cloneItems();
  const copy = cloneConfig(config);
  copy.tag = uniqueInboundTag(`${copy.tag}-copy`, tags.value);
  next.splice(index + 1, 0, copy);
  const managed = managedInboundSniff(props.draft, config.tag);
  const intents = managed.enabled ? [{ tag: copy.tag, sniff: true, sniffTimeout: managed.timeout }] : [];
  if (!saveItems(next, intents)) return;
  show(`${copy.tag} 已复制到当前配置。`, "success");
}
function remove(index: number, config: InboundConfig): void {
  const next = cloneItems();
  next.splice(index, 1);
  if (!saveItems(next, [{ previousTag: config.tag, tag: config.tag, sniff: false }])) return;
  show(`${config.tag} 已从当前配置移除，一级资源库不受影响。`, "success");
}
async function importSelected(ids: string[]): Promise<void> {
  importing.value = true;
  try {
    const details = await Promise.all(ids.map((id) => resources.get(id)));
    const next = cloneItems();
    const used = new Set(tags.value);
    const intents: InboundRouteIntent[] = [];
    let renamed = 0;
    for (const detail of details) {
      const source = structuredClone(detail.config);
      const legacySniff = detail.mode === "structured" && source.sniff === true;
      const legacyTimeout = typeof source.sniff_timeout === "string" && source.sniff_timeout.trim() ? source.sniff_timeout : "300ms";
      const config = detail.mode === "structured" ? withoutLegacyInboundFields(source) : source;
      const tag = uniqueInboundTag(config.tag, used);
      if (tag !== config.tag) renamed++;
      config.tag = tag;
      used.add(tag);
      // Only the complete config object enters the draft. No library ID or
      // source metadata is retained, so future edits and deletes are isolated.
      next.push(config);
      if (legacySniff) intents.push({ tag, sniff: true, sniffTimeout: legacyTimeout });
    }
    if (!saveItems(next, intents)) return;
    importOpen.value = false;
    show(`已创建 ${details.length} 个独立副本${renamed ? `，其中 ${renamed} 个 tag 已自动改名` : ""}。`, "success");
  } catch (cause) { show(cause instanceof Error ? cause.message : "导入入站配置失败", "error"); }
  finally { importing.value = false; }
}
</script>

<template>
  <section class="inbound-panel">
    <div class="inbound-layout">
      <article class="inbound-card"><header><div><h2>当前自定义配置的入站</h2><p>从一级“入站配置”导入后，作为独立副本维护</p></div><div><button class="json-button" type="button" @click="emit('editJson')"><AppIcon name="code" :size="14" />编辑全部 JSON</button><button class="import-button" type="button" @click="importOpen = true"><AppIcon name="download" :size="14" />导入入站配置</button></div></header><div class="entry-list">
        <article v-for="entry in entries" :key="entry.index" class="entry-row"><span class="entry-icon" :class="{ custom: editorType(entry.config) === 'custom', tun: entry.config.type === 'tun' }"><AppIcon :name="iconFor(entry.config)" :size="18" /></span><div class="entry-copy"><div><strong>{{ entry.config.tag }}</strong><b>{{ entry.config.type }}</b><em>独立配置</em></div><p><span>{{ inboundEndpoint(entry.config) }}</span><i />{{ editorType(entry.config) === "custom" ? `完整配置 · ${Object.keys(entry.config).length} 个字段` : inboundTypes.find((item) => item.type === entry.config.type)?.description }}</p></div><div class="entry-actions"><button type="button" :aria-label="`编辑 ${entry.config.tag}`" title="编辑独立入站副本" @click="openEditor(entry.index, entry.config)"><AppIcon name="edit" :size="15" /></button><button type="button" :aria-label="`复制 ${entry.config.tag}`" title="复制入站" @click="duplicate(entry.index, entry.config)"><AppIcon name="copy" :size="15" /></button><button class="danger" type="button" :aria-label="`删除 ${entry.config.tag}`" title="删除入站" @click="remove(entry.index, entry.config)"><AppIcon name="trash" :size="15" /></button></div></article>
        <button v-if="!rawItems.length" class="empty-state" type="button" @click="importOpen = true"><AppIcon name="inbound" :size="31" /><strong>尚未导入入站</strong><span>从入站配置资源库选择一个或多个项目</span></button>
        <p v-if="invalidCount" class="invalid-items" role="alert"><AppIcon name="info" :size="14" />还有 {{ invalidCount }} 个无法结构化显示的数组项，请使用“编辑全部 JSON”处理。</p>
      </div></article>
      <aside><article class="options-card"><header><div><h2>副本边界</h2><p>当前档案独立维护</p></div></header><div><p><AppIcon name="copy" :size="16" /><span><strong>一次性复制</strong><small>导入时读取源配置的完整当前内容</small></span></p><p><AppIcon name="edit" :size="16" /><span><strong>独立编辑</strong><small>保存后只改变当前自定义配置草稿</small></span></p><p><AppIcon name="trash" :size="16" /><span><strong>独立删除</strong><small>删除任一侧都不会联动另一侧</small></span></p></div></article><p class="info-callout"><AppIcon name="info" :size="15" /><span>导入不记录资源库 ID。源配置后续修改或删除，不会影响这里已有的副本。</span></p><p class="info-callout"><AppIcon name="shield" :size="15" /><span>TUN 模式需要系统网络管理权限；应用前仍会由 sing-box 核心执行真实配置检查。</span></p></aside>
    </div>
    <p v-if="error" class="panel-error" role="alert">{{ error }}</p>
    <InboundImportDialog :open="importOpen" :profile-name="profileName" :current-tags="tags" :busy="importing" @close="importOpen = false" @import="importSelected" />
    <InboundEditorDialog :open="editorOpen" :resource="editing" :existing-tags="tags" kind="profile" :allow-delete="false" @close="closeEditor" @submit="submitEdit" />
    <AppToast v-if="notice" :message="notice" :tone="noticeTone" />
  </section>
</template>

<style scoped>
.inbound-panel { display: grid; gap: 10px; }.inbound-layout { display: grid; grid-template-columns: minmax(0, 1.7fr) minmax(230px, .65fr); gap: var(--section-gap); }.inbound-card, .options-card { border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.inbound-card > header, .options-card > header { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--section-gap); padding: 8px var(--card-padding); border-bottom: 1px solid var(--line); }.inbound-card h2, .options-card h2, .inbound-card header p, .options-card header p { margin: 0; }.inbound-card h2, .options-card h2 { font-size: 12px; }.inbound-card header p, .options-card header p { margin-top: 2px; color: var(--text-faint); font-size: 9px; }.inbound-card header > div:last-child { display: flex; gap: 6px; }.inbound-card header button { display: inline-flex; min-height: 31px; align-items: center; gap: 5px; padding: 5px 8px; font-size: 9px; }.json-button { color: var(--text-soft); border: 1px solid var(--line); background: var(--surface-soft); }.import-button { color: #fff; background: var(--primary); }.entry-list { display: grid; gap: 7px; padding: 10px; }.entry-row { display: flex; min-width: 0; align-items: center; gap: 8px; padding: 8px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-soft); transition: 150ms ease; }.entry-row:hover { border-color: color-mix(in srgb, var(--primary) 26%, var(--line)); background: color-mix(in srgb, var(--primary-soft) 42%, var(--surface)); }.entry-icon { display: grid; width: 35px; height: 35px; flex: 0 0 auto; place-items: center; border-radius: 9px; color: var(--primary); background: var(--primary-soft); }.entry-icon.custom { color: var(--violet); background: color-mix(in srgb, var(--violet) 12%, transparent); }.entry-icon.tun { color: var(--cyan); background: var(--cyan-soft); }.entry-copy { min-width: 0; flex: 1; }.entry-copy > div { display: flex; min-width: 0; align-items: center; gap: 6px; }.entry-copy strong { overflow: hidden; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }.entry-copy b, .entry-copy em { padding: 2px 6px; border-radius: 5px; color: var(--primary); background: var(--primary-soft); font: 700 var(--font-size-sm) ui-monospace, monospace; font-style: normal; }.entry-copy em { color: var(--green); background: var(--green-soft); }.entry-copy p { display: flex; align-items: center; gap: 6px; overflow: hidden; margin: 4px 0 0; color: var(--text-faint); font-size: var(--font-size-sm); text-overflow: ellipsis; white-space: nowrap; }.entry-copy p span { overflow: hidden; text-overflow: ellipsis; }.entry-copy p i { width: 3px; height: 3px; flex: 0 0 auto; border-radius: 50%; background: var(--line-strong); }.entry-actions { display: flex; gap: 3px; }.entry-actions button { display: inline-grid; width: 29px; min-height: 29px; place-items: center; padding: 0; color: var(--text-faint); background: transparent; }.entry-actions button:hover { color: var(--primary); background: var(--primary-soft); }.entry-actions button.danger:hover { color: var(--red); background: var(--red-soft); }.empty-state { display: grid; min-height: var(--empty-state-height); place-items: center; align-content: center; gap: 7px; color: var(--primary); border: 1px dashed var(--line-strong); background: var(--surface-soft); }.empty-state strong { color: var(--text); font-size: 12px; }.empty-state span { color: var(--text-faint); font-size: 9px; font-weight: 400; }.invalid-items { display: flex; align-items: center; gap: 7px; margin: 0; padding: 9px; border-radius: 8px; color: var(--orange); background: var(--orange-soft); font-size: 9px; }.inbound-layout aside { display: grid; align-content: start; gap: var(--section-gap); }.options-card > div { display: grid; gap: 8px; padding: 12px; }.options-card > div > p { display: flex; align-items: center; gap: 9px; margin: 0; padding: 9px; border-radius: 9px; color: var(--primary); background: var(--surface-soft); }.options-card strong, .options-card small { display: block; }.options-card strong { color: var(--text); font-size: 9px; }.options-card small { margin-top: 2px; color: var(--text-faint); font-size: var(--font-size-sm); }.info-callout { display: flex; align-items: flex-start; gap: 8px; margin: 0; padding: 10px; border: 1px solid var(--line); border-radius: 10px; color: var(--primary); background: var(--surface); font-size: 9px; }.info-callout span { color: var(--text-soft); }.panel-error { margin: 0; color: var(--red); font-size: 10px; }
@media (max-width: 900px) { .inbound-layout { grid-template-columns: 1fr; }.inbound-layout aside { grid-template-columns: 1fr 1fr; }.options-card { grid-row: span 2; } }
@media (max-width: 680px) { .inbound-card > header { flex-direction: column; }.inbound-card header > div:last-child { width: 100%; }.inbound-card header button { flex: 1; justify-content: center; }.inbound-layout aside { grid-template-columns: 1fr; }.options-card { grid-row: auto; }.entry-row { align-items: flex-start; flex-wrap: wrap; }.entry-copy { padding-top: 2px; }.entry-copy > div { flex-wrap: wrap; }.entry-actions { width: 100%; justify-content: flex-end; border-top: 1px solid var(--line); padding-top: 6px; }.entry-actions button { flex: 1; } }
</style>
