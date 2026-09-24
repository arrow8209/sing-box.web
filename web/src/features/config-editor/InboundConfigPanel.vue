<script setup lang="ts">
import { computed, ref, toRaw } from "vue";
import AppToast from "@/components/AppToast.vue";
import AppIcon from "@/layout/AppIcon.vue";
import type { Draft } from "@/features/profiles/types";
import type { FreezeOperation } from "@/features/profiles/types";
import type { CandidateItem, CandidateView } from "@/features/application/types";
import InboundEditorDialog from "@/features/inbounds/InboundEditorDialog.vue";
import InboundImportDialog from "@/features/inbounds/InboundImportDialog.vue";
import { editorType, inboundEndpoint, inboundTypes, isInboundConfig, resourceMode, uniqueInboundTag, withoutLegacyInboundFields } from "@/features/inbounds/model";
import { useInboundStore } from "@/features/inbounds/store";
import type { InboundConfig, InboundDetail, InboundInput } from "@/features/inbounds/types";
import { applyInboundRouteIntents, managedInboundSniff, restoreInboundEditorState, type InboundPanelSave, type InboundResourceBinding, type InboundRouteIntent } from "./inbound";

const props = defineProps<{ draft: Draft; profileName: string; error?: string; candidate?: CandidateView | null; serverResolution?: boolean }>();
const emit = defineEmits<{ editJson: []; save: [payload: InboundPanelSave]; freeze: [operation: FreezeOperation] }>();
const resources = useInboundStore();
const importOpen = ref(false);
const association = ref<{ index: number; tag: string } | null>(null);
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
const sourceBindings = computed(() => restoreInboundEditorState(props.draft.editorState?.inbounds).resourceBindings ?? []);

function bindingFor(tag: string): InboundResourceBinding | undefined { return sourceBindings.value.find((item) => item.tag === tag); }
function candidateFor(tag: string): CandidateItem | undefined { return props.candidate?.items.find((item) => item.module === "inbounds" && item.tag === tag); }
function candidateEndpoint(tag: string, fallback: InboundConfig): string {
  const item = candidateFor(tag);
  return item?.listenPort !== undefined ? `${item.listen || "0.0.0.0"}:${item.listenPort}` : inboundEndpoint(fallback);
}
function freezeOperation(action: "detach" | "copy", tag: string, newTag?: string): FreezeOperation | null {
  const item = candidateFor(tag);
  if (!item?.token || !item.text || !item.inboundStateText || !props.candidate || props.candidate.profileId !== props.draft.profileId || props.candidate.revision !== props.draft.revision) {
    show("候选尚不可用或已过期，请刷新后重试；源已删除时可明确恢复此前副本。", "error");
    return null;
  }
  return { action, module: "inbounds", tag, ...(newTag ? { newTag } : {}), text: item.text,
    routeText: item.routeText, inboundStateText: item.inboundStateText, digest: props.candidate.digest,
    resourceDigests: props.candidate.resourceDigests, token: item.token };
}
function openImport(index?: number, tag?: string): void {
  association.value = index !== undefined && tag ? { index, tag } : null;
  importOpen.value = true;
}
function closeImport(): void { importOpen.value = false; association.value = null; }

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
function saveItems(items: unknown[], intents: InboundRouteIntent[] = [], bindings = sourceBindings.value): boolean {
  try {
    const route = applyInboundRouteIntents(props.draft, intents);
    const nextTags = new Set(items.filter(isInboundConfig).map((item) => item.tag));
    const linked = bindings.filter((item) => nextTags.has(item.tag)).map((item) => ({ ...item }));
    if (linked.length) route.editorState.resourceBindings = linked;
    else delete route.editorState.resourceBindings;
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
  if (props.serverResolution && bindingFor(config.tag)) { show("关联入站的生成结果只读；请先解除关联，才能编辑独立配置。", "error"); return; }
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
  if (!saveItems(next, intents, sourceBindings.value.filter((item) => item.tag !== previousTag))) return;
  const tag = input.config.tag;
  closeEditor();
  show(`${tag} 已更新为当前自定义配置的独立副本。`, "success");
}
function duplicate(index: number, config: InboundConfig): void {
  if (props.serverResolution && bindingFor(config.tag)) {
    const operation = freezeOperation("copy", config.tag, uniqueInboundTag(`${config.tag}-copy`, tags.value));
    if (operation) emit("freeze", operation);
    return;
  }
  const next = cloneItems();
  const copy = cloneConfig(config);
  copy.tag = uniqueInboundTag(`${copy.tag}-copy`, tags.value);
  next.splice(index + 1, 0, copy);
  const managed = managedInboundSniff(props.draft, config.tag);
  const intents = managed.enabled ? [{ tag: copy.tag, sniff: true, sniffTimeout: managed.timeout }] : [];
  if (!saveItems(next, intents)) return;
  show(`${copy.tag} 已复制为独立入站。`, "success");
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
    const profileId = props.draft.profileId;
    const target = association.value ? { ...association.value } : null;
    if (target && ids.length !== 1) throw new Error("请选择一个源资源。");
    const details = await Promise.all(ids.map((id) => resources.get(id)));
    if (props.draft.profileId !== profileId) throw new Error("档案已切换，请重新选择入站资源。");
    const next = cloneItems();
    const currentTarget = target ? next[target.index] : null;
    if (target && (!isInboundConfig(currentTarget) || currentTarget.tag !== target.tag)) throw new Error("入站已变更，请重新选择关联。");
    const used = new Set(tags.value);
    const intents: InboundRouteIntent[] = [];
    const bindings = sourceBindings.value.map((item) => ({ ...item }));
    let renamed = 0;
    for (const detail of details) {
      const source = structuredClone(detail.config);
      const legacySniff = detail.mode === "structured" && source.sniff === true;
      const legacyTimeout = typeof source.sniff_timeout === "string" && source.sniff_timeout.trim() ? source.sniff_timeout : "300ms";
      const config = detail.mode === "structured" ? withoutLegacyInboundFields(source) : source;
      const tag = target?.tag ?? uniqueInboundTag(config.tag, used);
      if (tag !== config.tag) renamed++;
      config.tag = tag;
      used.add(tag);
      if (target) next[target.index] = config;
      else next.push(config);
      const previousBinding = bindings.findIndex((item) => item.tag === tag);
      const binding = { tag, resourceId: detail.id, sourceTag: detail.config.tag };
      if (previousBinding >= 0) bindings[previousBinding] = binding;
      else bindings.push(binding);
      intents.push({ tag, sniff: legacySniff, sniffTimeout: legacyTimeout });
    }
    if (!saveItems(next, intents, bindings)) return;
    closeImport();
    show(target ? `${target.tag} 已关联源资源，检测前会自动同步。` : `已导入并关联 ${details.length} 个入站${renamed ? `，其中 ${renamed} 个 tag 已自动改名` : ""}。`, "success");
  } catch (cause) { show(cause instanceof Error ? cause.message : "导入入站配置失败", "error"); }
  finally { importing.value = false; }
}

function unlink(config: InboundConfig): void {
  if (props.serverResolution) { const operation = freezeOperation("detach", config.tag); if (operation) emit("freeze", operation); return; }
  if (saveItems(cloneItems(), [], sourceBindings.value.filter((item) => item.tag !== config.tag))) {
    show(`${config.tag} 已解除关联，保留当前内容作为独立入站。`, "success");
  }
}
function restoreSaved(config: InboundConfig): void {
  emit("freeze", { action: "restore_saved", module: "inbounds", tag: config.tag });
}
</script>

<template>
  <section class="inbound-panel">
    <div class="inbound-layout">
      <article class="inbound-card"><header><div><h2>当前自定义配置的入站</h2><p>{{ serverResolution ? '关联入站由服务端生成候选；独立入站自行维护' : '关联入站在预览、检测和应用前同步源资源；独立入站自行维护' }}</p></div><div><button class="json-button" type="button" @click="emit('editJson')"><AppIcon name="code" :size="14" />{{ serverResolution && sourceBindings.length ? '查看已保存输入' : '编辑全部 JSON' }}</button><button class="import-button" type="button" @click="openImport()"><AppIcon name="download" :size="14" />导入入站配置</button></div></header><div class="entry-list">
        <article v-for="entry in entries" :key="entry.index" class="entry-row">
          <span class="entry-icon" :class="{ custom: editorType(entry.config) === 'custom', tun: entry.config.type === 'tun' }"><AppIcon :name="iconFor(entry.config)" :size="18" /></span>
          <div class="entry-copy"><div><strong>{{ entry.config.tag }}</strong><b>{{ candidateFor(entry.config.tag)?.type ?? entry.config.type }}</b><em :title="candidateFor(entry.config.tag)?.sourceName ?? bindingFor(entry.config.tag)?.sourceTag">{{ bindingFor(entry.config.tag) ? '已关联源资源' : '独立配置' }}</em></div><p><span>{{ candidateEndpoint(entry.config.tag, entry.config) }}</span><i />{{ bindingFor(entry.config.tag) ? `来源：${candidateFor(entry.config.tag)?.sourceName ?? bindingFor(entry.config.tag)?.sourceTag}` : editorType(entry.config) === "custom" ? `完整配置 · ${Object.keys(entry.config).length} 个字段` : inboundTypes.find((item) => item.type === entry.config.type)?.description }}</p><small v-if="serverResolution && bindingFor(entry.config.tag)">{{ candidateFor(entry.config.tag) ? `服务端候选${candidateFor(entry.config.tag)?.sniff ? ' · 已生成嗅探规则' : ''}` : '候选未就绪；源失效时可恢复此前已保存副本' }}</small></div>
          <div class="entry-actions">
            <button type="button" :aria-label="`关联源资源 ${entry.config.tag}`" title="选择关联的源资源" @click="openImport(entry.index, entry.config.tag)"><AppIcon name="link" :size="15" /></button>
            <button v-if="bindingFor(entry.config.tag) && (!serverResolution || candidateFor(entry.config.tag))" type="button" :aria-label="`解除关联 ${entry.config.tag}`" title="解除关联并固定当前候选" @click="unlink(entry.config)"><AppIcon name="x" :size="15" /></button>
            <button v-if="serverResolution && bindingFor(entry.config.tag) && !candidateFor(entry.config.tag)" type="button" :aria-label="`恢复此前副本 ${entry.config.tag}`" title="源不可用时明确恢复此前已保存副本，内容可能过期" @click="restoreSaved(entry.config)"><AppIcon name="rotate" :size="15" /></button>
            <button type="button" :aria-label="`编辑 ${entry.config.tag}`" title="编辑为独立入站" @click="openEditor(entry.index, entry.config)"><AppIcon name="edit" :size="15" /></button>
            <button type="button" :aria-label="`复制 ${entry.config.tag}`" title="复制为独立入站" @click="duplicate(entry.index, entry.config)"><AppIcon name="copy" :size="15" /></button>
            <button class="danger" type="button" :aria-label="`删除 ${entry.config.tag}`" title="删除入站" @click="remove(entry.index, entry.config)"><AppIcon name="trash" :size="15" /></button>
          </div>
        </article>
        <button v-if="!rawItems.length" class="empty-state" type="button" @click="openImport()"><AppIcon name="inbound" :size="31" /><strong>尚未导入入站</strong><span>从入站配置资源库选择一个或多个项目</span></button>
        <p v-if="invalidCount" class="invalid-items" role="alert"><AppIcon name="info" :size="14" />还有 {{ invalidCount }} 个无法结构化显示的数组项，请使用“编辑全部 JSON”处理。</p>
      </div></article>
      <aside><article class="options-card"><header><div><h2>来源与同步</h2><p>关联资源或独立维护</p></div></header><div><p><AppIcon name="link" :size="16" /><span><strong>关联源资源</strong><small>新导入的入站自动关联；旧副本可点击链接按钮选择来源</small></span></p><p><AppIcon name="rotate" :size="16" /><span><strong>服务端候选</strong><small>预览、检测和应用由服务端读取最新来源，并保留本地 tag</small></span></p><p><AppIcon name="edit" :size="16" /><span><strong>独立维护</strong><small>解除关联后固定所见候选，再维护本档案</small></span></p></div></article><p class="info-callout"><AppIcon name="info" :size="15" /><span>源资源已删除或读取失败时会停止生成并提示。可重新关联，或明确恢复此前已保存的旧副本；旧内容可能过期。</span></p><p class="info-callout"><AppIcon name="shield" :size="15" /><span>TUN 模式需要系统网络管理权限；应用前仍会由 sing-box 核心执行真实配置检查。</span></p></aside>
    </div>
    <p v-if="error" class="panel-error" role="alert">{{ error }}</p>
    <InboundImportDialog :open="importOpen" :profile-name="profileName" :current-tags="tags" :target-tag="association?.tag" :busy="importing" @close="closeImport" @import="importSelected" />
    <InboundEditorDialog :open="editorOpen" :resource="editing" :existing-tags="tags" :linked-source="bindingFor(editing?.config.tag ?? '')?.sourceTag" kind="profile" :allow-delete="false" @close="closeEditor" @submit="submitEdit" />
    <AppToast v-if="notice" :message="notice" :tone="noticeTone" />
  </section>
</template>

<style scoped>
.inbound-panel { display: grid; gap: 10px; }.inbound-layout { display: grid; grid-template-columns: minmax(0, 1.7fr) minmax(230px, .65fr); gap: var(--section-gap); }.inbound-card, .options-card { border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.inbound-card > header, .options-card > header { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--section-gap); padding: 8px var(--card-padding); border-bottom: 1px solid var(--line); }.inbound-card h2, .options-card h2, .inbound-card header p, .options-card header p { margin: 0; }.inbound-card h2, .options-card h2 { font-size: var(--font-size-heading); }.inbound-card header p, .options-card header p { margin-top: 2px; color: var(--text-faint); font-size: var(--font-size-xs); }.inbound-card header > div:last-child { display: flex; gap: 6px; }.inbound-card header button { display: inline-flex; min-height: 31px; align-items: center; gap: 5px; padding: 5px 8px; font-size: var(--font-size-sm); }.json-button { color: var(--text-soft); border: 1px solid var(--line); background: var(--surface-soft); }.import-button { color: #fff; background: var(--primary); }.entry-list { display: grid; gap: 7px; padding: 10px; }.entry-row { display: flex; min-width: 0; align-items: center; gap: 8px; padding: 8px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-soft); transition: 150ms ease; }.entry-row:hover { border-color: color-mix(in srgb, var(--primary) 26%, var(--line)); background: color-mix(in srgb, var(--primary-soft) 42%, var(--surface)); }.entry-icon { display: grid; width: 35px; height: 35px; flex: 0 0 auto; place-items: center; border-radius: 9px; color: var(--primary); background: var(--primary-soft); }.entry-icon.custom { color: var(--violet); background: color-mix(in srgb, var(--violet) 12%, transparent); }.entry-icon.tun { color: var(--cyan); background: var(--cyan-soft); }.entry-copy { min-width: 0; flex: 1; }.entry-copy > div { display: flex; min-width: 0; align-items: center; gap: 6px; }.entry-copy strong { overflow: hidden; font-size: var(--font-size-base); text-overflow: ellipsis; white-space: nowrap; }.entry-copy b, .entry-copy em { padding: 2px 6px; border-radius: 5px; color: var(--primary); background: var(--primary-soft); font: 700 var(--font-size-sm) ui-monospace, monospace; font-style: normal; }.entry-copy em { color: var(--green); background: var(--green-soft); }.entry-copy p { display: flex; align-items: center; gap: 6px; overflow: hidden; margin: 4px 0 0; color: var(--text-faint); font-size: var(--font-size-sm); text-overflow: ellipsis; white-space: nowrap; }.entry-copy p span { overflow: hidden; text-overflow: ellipsis; }.entry-copy p i { width: 3px; height: 3px; flex: 0 0 auto; border-radius: 50%; background: var(--line-strong); }.entry-actions { display: flex; gap: 3px; }.entry-actions button { display: inline-grid; width: 29px; min-height: 29px; place-items: center; padding: 0; color: var(--text-faint); background: transparent; }.entry-actions button:hover { color: var(--primary); background: var(--primary-soft); }.entry-actions button.danger:hover { color: var(--red); background: var(--red-soft); }.empty-state { display: grid; min-height: var(--empty-state-height); place-items: center; align-content: center; gap: 7px; color: var(--primary); border: 1px dashed var(--line-strong); background: var(--surface-soft); }.empty-state strong { color: var(--text); font-size: var(--font-size-base); }.empty-state span { color: var(--text-faint); font-size: var(--font-size-xs); font-weight: 400; }.invalid-items { display: flex; align-items: center; gap: 7px; margin: 0; padding: 9px; border-radius: 8px; color: var(--orange); background: var(--orange-soft); font-size: var(--font-size-xs); }.inbound-layout aside { display: grid; align-content: start; gap: var(--section-gap); }.options-card > div { display: grid; gap: 8px; padding: 12px; }.options-card > div > p { display: flex; align-items: center; gap: 9px; margin: 0; padding: 9px; border-radius: 9px; color: var(--primary); background: var(--surface-soft); }.options-card strong, .options-card small { display: block; }.options-card strong { color: var(--text); font-size: var(--font-size-base); }.options-card small { margin-top: 2px; color: var(--text-faint); font-size: var(--font-size-sm); }.info-callout { display: flex; align-items: flex-start; gap: 8px; margin: 0; padding: 10px; border: 1px solid var(--line); border-radius: 10px; color: var(--primary); background: var(--surface); font-size: var(--font-size-xs); }.info-callout span { color: var(--text-soft); }.panel-error { margin: 0; color: var(--red); font-size: var(--font-size-sm); }
@media (max-width: 900px) { .inbound-layout { grid-template-columns: 1fr; }.inbound-layout aside { grid-template-columns: 1fr 1fr; }.options-card { grid-row: span 2; } }
@media (max-width: 680px) { .inbound-card > header { flex-direction: column; }.inbound-card header > div:last-child { width: 100%; }.inbound-card header button { flex: 1; justify-content: center; }.inbound-layout aside { grid-template-columns: 1fr; }.options-card { grid-row: auto; }.entry-row { align-items: flex-start; flex-wrap: wrap; }.entry-copy { padding-top: 2px; }.entry-copy > div { flex-wrap: wrap; }.entry-actions { width: 100%; justify-content: flex-end; border-top: 1px solid var(--line); padding-top: 6px; }.entry-actions button { flex: 1; } }
</style>
