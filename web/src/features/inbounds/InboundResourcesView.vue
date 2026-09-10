<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import AppToast from "@/components/AppToast.vue";
import AppIcon from "@/layout/AppIcon.vue";
import { inboundTypes, uniqueInboundTag, updatedLabel } from "./model";
import { useInboundStore } from "./store";
import InboundDeleteDialog from "./InboundDeleteDialog.vue";
import InboundEditorDialog from "./InboundEditorDialog.vue";
import type { InboundDetail, InboundInput, InboundSummary } from "./types";

const store = useInboundStore();
const loadError = ref("");
const openingId = ref("");
const saving = ref(false);
const deleting = ref(false);
const editorOpen = ref(false);
const deleteOpen = ref(false);
const editing = ref<InboundDetail | null>(null);
const deleteTarget = ref<InboundSummary | null>(null);
const notice = ref("");
const noticeTone = ref<"info" | "error" | "success">("info");
const typeCount = computed(() => new Set(store.resources.value.map((item) => item.type)).size);
const customCount = computed(() => store.resources.value.filter((item) => item.mode === "custom").length);
const tags = computed(() => store.resources.value.map((item) => item.tag));

onMounted(refresh);

async function refresh(): Promise<void> {
  try { await store.refresh(); loadError.value = ""; }
  catch (cause) { loadError.value = cause instanceof Error ? cause.message : "无法加载入站配置"; }
}
function show(message: string, tone: "info" | "error" | "success" = "info"): void {
  notice.value = "";
  noticeTone.value = tone;
  queueMicrotask(() => { notice.value = message; });
}
function iconFor(item: InboundSummary): string {
  if (item.mode === "custom") return "code";
  return inboundTypes.find((candidate) => candidate.type === item.type)?.icon ?? "inbound";
}
function openCreate(): void { editing.value = null; editorOpen.value = true; }
function closeEditor(): void { editorOpen.value = false; editing.value = null; }
async function openEdit(item: InboundSummary): Promise<void> {
  openingId.value = item.id;
  try { editing.value = await store.get(item.id); editorOpen.value = true; }
  catch (cause) { show(cause instanceof Error ? cause.message : "无法读取入站配置", "error"); }
  finally { openingId.value = ""; }
}
async function submit(input: InboundInput): Promise<void> {
  saving.value = true;
  try {
    const summary = editing.value ? await store.replace(editing.value.id, input) : await store.create(input);
    closeEditor();
    show(`${summary.tag} 已保存到入站配置资源库。`, "success");
  } catch (cause) { show(cause instanceof Error ? cause.message : "保存入站配置失败", "error"); }
  finally { saving.value = false; }
}
async function duplicate(item: InboundSummary): Promise<void> {
  openingId.value = item.id;
  try {
    const detail = await store.get(item.id);
    const config = structuredClone(detail.config);
    config.tag = uniqueInboundTag(`${config.tag}-copy`, tags.value);
    const summary = await store.create({ mode: detail.mode, config });
    show(`${summary.tag} 已复制到资源库。`, "success");
  } catch (cause) { show(cause instanceof Error ? cause.message : "复制入站配置失败", "error"); }
  finally { openingId.value = ""; }
}
function requestDelete(item?: InboundSummary): void {
  const target = item ?? store.resources.value.find((candidate) => candidate.id === editing.value?.id);
  if (!target) return;
  deleteTarget.value = target;
  editorOpen.value = false;
  editing.value = null;
  deleteOpen.value = true;
}
function closeDelete(): void { deleteOpen.value = false; deleteTarget.value = null; }
async function confirmDelete(): Promise<void> {
  if (!deleteTarget.value) return;
  deleting.value = true;
  const target = deleteTarget.value;
  try {
    await store.remove(target.id);
    closeDelete();
    show(`${target.tag} 已删除；自定义配置中的已有副本不受影响。`, "success");
  } catch (cause) { show(cause instanceof Error ? cause.message : "删除入站配置失败", "error"); }
  finally { deleting.value = false; }
}
</script>

<template>
  <section class="inbound-resources-page">
    <header class="page-intro"><div><h1>入站配置</h1><p>集中维护可复用的入站配置；自定义配置导入时只复制当前内容。</p></div><button class="primary-action" type="button" @click="openCreate"><AppIcon name="plus" :size="15" />新建入站配置</button></header>
    <p v-if="loadError" class="page-error" role="alert">{{ loadError }} <button type="button" @click="refresh">重试</button></p>
    <div class="summary-grid">
      <article class="summary-card"><span class="summary-icon"><AppIcon name="inbound" /></span><div><strong>{{ store.resources.value.length }}</strong><span>入站配置 · 独立资源库</span></div></article>
      <article class="summary-card"><span class="summary-icon cyan"><AppIcon name="server" /></span><div><strong>{{ typeCount }}</strong><span>配置类型 · 常用类型与完整 JSON</span></div></article>
      <article class="summary-card"><span class="summary-icon orange"><AppIcon name="copy" /></span><div><strong>{{ customCount }}</strong><span>JSON 配置 · 导入后生成独立副本</span></div></article>
    </div>
    <div class="resource-layout">
      <article class="resource-card"><header><div><h2>可复用入站</h2><p>编辑这里的配置不会改变已导入的副本</p></div><span>{{ store.resources.value.length }} 项</span></header><div class="resource-list" :aria-busy="store.loading.value">
        <article v-for="item in store.resources.value" :key="item.id" class="resource-row" :class="{ opening: openingId === item.id }"><span class="entry-icon" :class="{ custom: item.mode === 'custom', tun: item.type === 'tun' }"><AppIcon :name="iconFor(item)" :size="18" /></span><div class="entry-copy"><div><strong>{{ item.tag }}</strong><b>{{ item.type }}</b><em :class="{ custom: item.mode === 'custom' }">{{ item.mode === "custom" ? "完整 JSON" : "可导入" }}</em></div><p><span>{{ item.endpoint }}</span><i />最后编辑 {{ updatedLabel(item.updatedAt) }}</p></div><div class="entry-actions"><button type="button" :disabled="openingId === item.id" :aria-label="`编辑 ${item.tag}`" title="编辑入站配置" @click="openEdit(item)"><AppIcon name="edit" :size="15" /></button><button type="button" :disabled="openingId === item.id" :aria-label="`复制 ${item.tag}`" title="复制入站配置" @click="duplicate(item)"><AppIcon name="copy" :size="15" /></button><button class="danger" type="button" :disabled="openingId === item.id" :aria-label="`删除 ${item.tag}`" title="删除入站配置" @click="requestDelete(item)"><AppIcon name="trash" :size="15" /></button></div></article>
        <button v-if="!store.loading.value && !store.resources.value.length" class="empty-state" type="button" @click="openCreate"><AppIcon name="inbound" :size="31" /><strong>尚未新建入站配置</strong><span>创建后即可在自定义配置中导入</span></button>
      </div></article>
      <aside><article class="guide-card"><header><div><h2>导入规则</h2><p>复制，而不是引用</p></div></header><div class="guide-list"><div><AppIcon name="plus" :size="17" /><span><strong>先新建</strong><small>在资源库维护常用入站参数</small></span></div><div><AppIcon name="copy" :size="17" /><span><strong>再导入</strong><small>自定义配置获得完整内容副本</small></span></div><div><AppIcon name="shield" :size="17" /><span><strong>互不关联</strong><small>导入后任一侧修改或删除均不联动</small></span></div></div></article><p class="info-callout"><AppIcon name="info" :size="15" /><span>导入时若 <code>tag</code> 与当前自定义配置重复，会自动生成不冲突的新名称。</span></p></aside>
    </div>
    <InboundEditorDialog :open="editorOpen" :resource="editing" :existing-tags="tags" :busy="saving" @close="closeEditor" @submit="submit" @delete="requestDelete()" />
    <InboundDeleteDialog :open="deleteOpen" :tag="deleteTarget?.tag ?? ''" :busy="deleting" @close="closeDelete" @confirm="confirmDelete" />
    <AppToast v-if="notice" :message="notice" :tone="noticeTone" />
  </section>
</template>

<style scoped>
.inbound-resources-page { display: grid; gap: var(--section-gap); }.page-intro { display: flex; align-items: end; justify-content: space-between; gap: var(--section-gap); }.page-intro h1 { margin: 0; font-size: 22px; letter-spacing: -.45px; }.page-intro p { margin: 5px 0 0; color: var(--text-soft); font-size: 12px; }.primary-action { display: inline-flex; min-height: 35px; align-items: center; gap: 6px; padding: 7px 11px; font-size: 10px; }.page-error { display: flex; align-items: center; justify-content: space-between; gap: var(--section-gap); margin: 0; padding: 10px 12px; border-radius: 9px; color: var(--red); background: var(--red-soft); font-size: 10px; }.page-error button { min-height: 28px; padding: 4px 8px; color: var(--red); background: var(--surface); font-size: 9px; }.summary-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: var(--section-gap); }.summary-card { display: flex; min-height: 66px; align-items: center; gap: 10px; padding: 10px var(--card-padding); border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.summary-icon { display: grid; width: 32px; height: 32px; flex: 0 0 auto; place-items: center; border-radius: 11px; color: var(--primary); background: var(--primary-soft); }.summary-icon.cyan { color: var(--cyan); background: var(--cyan-soft); }.summary-icon.orange { color: var(--orange); background: var(--orange-soft); }.summary-card strong, .summary-card div span { display: block; }.summary-card strong { font-size: 19px; }.summary-card div span { margin-top: 2px; color: var(--text-soft); font-size: 9px; }.resource-layout { display: grid; grid-template-columns: minmax(0, 1.65fr) minmax(230px, .65fr); gap: var(--section-gap); }.resource-card, .guide-card { border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.resource-card > header, .guide-card > header { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--section-gap); padding: 8px var(--card-padding); border-bottom: 1px solid var(--line); }.resource-card h2, .guide-card h2, .resource-card header p, .guide-card header p { margin: 0; }.resource-card h2, .guide-card h2 { font-size: 12px; }.resource-card header p, .guide-card header p { margin-top: 2px; color: var(--text-faint); font-size: 9px; }.resource-card header > span { padding: 3px 7px; border-radius: 6px; color: var(--primary); background: var(--primary-soft); font-size: 9px; font-weight: 700; }.resource-list { display: grid; gap: 7px; padding: 10px; }.resource-row { display: flex; min-width: 0; align-items: center; gap: 8px; padding: 8px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-soft); transition: 150ms ease; }.resource-row:hover { border-color: color-mix(in srgb, var(--primary) 26%, var(--line)); background: color-mix(in srgb, var(--primary-soft) 42%, var(--surface)); }.resource-row.opening { opacity: .65; }.entry-icon { display: grid; width: 35px; height: 35px; flex: 0 0 auto; place-items: center; border-radius: 9px; color: var(--primary); background: var(--primary-soft); }.entry-icon.custom { color: var(--violet); background: color-mix(in srgb, var(--violet) 12%, transparent); }.entry-icon.tun { color: var(--cyan); background: var(--cyan-soft); }.entry-copy { min-width: 0; flex: 1; }.entry-copy > div { display: flex; min-width: 0; align-items: center; gap: 6px; }.entry-copy strong { overflow: hidden; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }.entry-copy b, .entry-copy em { padding: 2px 6px; border-radius: 5px; color: var(--primary); background: var(--primary-soft); font: 700 var(--font-size-sm) ui-monospace, monospace; font-style: normal; }.entry-copy em { color: var(--green); background: var(--green-soft); }.entry-copy em.custom { color: var(--violet); background: color-mix(in srgb, var(--violet) 12%, transparent); }.entry-copy p { display: flex; align-items: center; gap: 6px; overflow: hidden; margin: 4px 0 0; color: var(--text-faint); font-size: var(--font-size-sm); text-overflow: ellipsis; white-space: nowrap; }.entry-copy p span { overflow: hidden; text-overflow: ellipsis; }.entry-copy p i { width: 3px; height: 3px; flex: 0 0 auto; border-radius: 50%; background: var(--line-strong); }.entry-actions { display: flex; gap: 3px; }.entry-actions button { display: inline-grid; width: 29px; min-height: 29px; place-items: center; padding: 0; color: var(--text-faint); background: transparent; }.entry-actions button:hover { color: var(--primary); background: var(--primary-soft); }.entry-actions button.danger { color: var(--text-faint); }.entry-actions button.danger:hover { color: var(--red); background: var(--red-soft); }.empty-state { display: grid; min-height: var(--empty-state-height); place-items: center; align-content: center; gap: 7px; color: var(--primary); border: 1px dashed var(--line-strong); background: var(--surface-soft); }.empty-state strong { color: var(--text); font-size: 12px; }.empty-state span { color: var(--text-faint); font-size: 9px; font-weight: 400; }.resource-layout aside { display: grid; align-content: start; gap: var(--section-gap); }.guide-list { display: grid; gap: 8px; padding: 12px; }.guide-list > div { display: flex; align-items: center; gap: 9px; padding: 6px 8px; border-radius: 9px; background: var(--surface-soft); }.guide-list > div > .icon { color: var(--primary); }.guide-list strong, .guide-list small { display: block; }.guide-list strong { font-size: 9px; }.guide-list small { margin-top: 2px; color: var(--text-faint); font-size: var(--font-size-sm); }.info-callout { display: flex; align-items: flex-start; gap: 8px; margin: 0; padding: 10px; border: 1px solid var(--line); border-radius: 10px; color: var(--primary); background: var(--surface); font-size: 9px; }.info-callout span { color: var(--text-soft); }.info-callout code { color: var(--primary-strong); font: inherit; font-weight: 700; }
@media (max-width: 900px) { .resource-layout { grid-template-columns: 1fr; }.resource-layout aside { display: flex; align-items: flex-start; }.resource-layout aside > * { min-width: 0; flex: 1; } }
@media (max-width: 680px) { .page-intro { align-items: flex-start; flex-direction: column; }.primary-action { width: 100%; justify-content: center; }.summary-grid { gap: 8px; }.resource-layout aside { display: grid; }.resource-row { align-items: flex-start; flex-wrap: wrap; }.entry-copy { padding-top: 2px; }.entry-actions { width: 100%; justify-content: flex-end; border-top: 1px solid var(--line); padding-top: 6px; }.entry-actions button { flex: 1; }.entry-copy > div { flex-wrap: wrap; } }
@media (max-width: 680px) {
  .summary-grid { gap: 8px; }
  .summary-card { min-width: 0; min-height: 64px; justify-content: center; padding: 8px 4px; text-align: center; }
  .summary-icon { display: none; }
  .summary-card div span { font-size: 10px; }
}
</style>
