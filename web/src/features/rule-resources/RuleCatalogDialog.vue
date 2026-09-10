<script setup lang="ts">
import { computed, ref, watch } from "vue";
import AppDialog from "@/components/AppDialog.vue";
import AppIcon from "@/layout/AppIcon.vue";
import { useRuleResourceStore } from "./store";
import type { RuleCatalogItem, RuleCatalogPreview } from "./types";

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ close: []; notice: [message: string]; error: [message: string] }>();
const store = useRuleResourceStore();
const query = ref("");
const category = ref<"all" | "geosite" | "geoip">("all");
const page = ref(1);
const pageSize = 27;
const preview = ref<RuleCatalogPreview | null>(null);
const previewItem = ref<RuleCatalogItem | null>(null);
const previewLoading = ref(false);
const importing = ref("");
const localError = ref("");

const filtered = computed(() => {
  const tokens = query.value.trim().toLowerCase().split(/\s+/).filter(Boolean);
  return (store.catalog.value?.items ?? []).filter((item) => (category.value === "all" || item.type === category.value) && tokens.every((token) => `${item.name} ${item.type}`.includes(token)));
});
const pageCount = computed(() => Math.max(1, Math.ceil(filtered.value.length / pageSize)));
const pageItems = computed(() => filtered.value.slice((page.value - 1) * pageSize, page.value * pageSize));
const importedCount = computed(() => new Set(store.resources.value.filter((item) => item.sourceType === "remote" && item.format === "binary").map((item) => item.catalogKey).filter(Boolean)).size);
const filterOptions = computed(() => [
  { value: "all" as const, label: "全部", count: store.catalog.value?.total ?? 0 },
  { value: "geosite" as const, label: "GeoSite", count: store.catalog.value?.geositeCount ?? 0 },
  { value: "geoip" as const, label: "GeoIP", count: store.catalog.value?.geoipCount ?? 0 }
]);
watch([query, category], () => { page.value = 1; });
watch(() => props.open, (open) => { if (open) void load(false); else { preview.value = null; previewItem.value = null; } });

async function load(force: boolean): Promise<void> {
  localError.value = "";
  try { await store.loadCatalog(force); }
  catch (cause) { localError.value = cause instanceof Error ? cause.message : "规则集目录加载失败"; }
}
function installed(item: RuleCatalogItem): boolean {
  const catalog = store.catalog.value;
  const base = item.type === "geoip" ? catalog?.geoip : catalog?.geosite;
  const endpoint = base ? `${base}${item.name}.srs` : "";
  return store.resources.value.some((resource) => resource.sourceType === "remote" && resource.format === "binary" && (resource.catalogKey === item.key || (!resource.catalogKey && Boolean(endpoint) && resource.url === endpoint)));
}
async function showPreview(item: RuleCatalogItem): Promise<void> {
  previewLoading.value = true; localError.value = ""; previewItem.value = item;
  try { preview.value = await store.previewCatalog(item.key); }
  catch (cause) { localError.value = cause instanceof Error ? cause.message : "预览失败"; previewItem.value = null; }
  finally { previewLoading.value = false; }
}
async function importItem(item: RuleCatalogItem): Promise<void> {
  importing.value = `${item.key}:binary`; localError.value = "";
  try {
    const result = await store.importCatalog(item.key);
    emit("notice", result.warning || `${item.name} 已以 Binary 格式导入并完成首次更新。`);
  } catch (cause) { emit("error", cause instanceof Error ? cause.message : "导入规则集失败"); }
  finally { importing.value = ""; }
}
function back(): void { preview.value = null; previewItem.value = null; }
function setCategory(value: "all" | "geosite" | "geoip"): void { category.value = value; }
</script>

<template>
  <AppDialog :open="open" title="规则集中心" description="发现、预览并导入 GUI-for-Cores Ruleset-Hub 的完整规则目录" class="catalog-dialog" :close-disabled="Boolean(importing)" @close="emit('close')">
    <template v-if="!preview">
      <div class="catalog-overview"><span class="overview-icon"><AppIcon name="database" :size="18" /></span><div><strong>MetaCubeX 完整规则目录</strong><span>目录由 GUI-for-Cores Ruleset-Hub 提供，导入后由本服务独立下载和调度。</span></div><div class="overview-stats"><span><strong>{{ store.catalog.value?.total ?? 0 }}</strong><small>全部规则</small></span><span><strong>{{ importedCount }}</strong><small>已导入</small></span></div></div>
      <div class="catalog-toolbar"><label class="catalog-search"><AppIcon name="search" :size="15" /><input v-model="query" placeholder="搜索规则名称或类型..." /></label><div class="catalog-filters" role="group" aria-label="规则类型"><button v-for="item in filterOptions" :key="item.value" type="button" :class="{ active: category === item.value }" @click="setCategory(item.value)"><span>{{ item.label }}</span><small>{{ item.count }}</small></button></div><button class="refresh-button" type="button" :disabled="store.catalogLoading.value" @click="load(true)"><AppIcon name="rotate" :size="14" />刷新</button></div>
      <p v-if="localError" class="catalog-error" role="alert">{{ localError }}</p>
      <div class="results-head"><span><strong>{{ filtered.length }}</strong> 个匹配项目</span><small v-if="store.catalog.value?.fetchedAt"><i />目录刷新于 {{ new Date(store.catalog.value.fetchedAt).toLocaleString("zh-CN") }}</small></div>
      <div v-if="pageItems.length" class="catalog-grid">
        <article v-for="item in pageItems" :key="item.key" class="catalog-card">
          <header class="catalog-card__header">
            <strong :title="item.name">{{ item.name }}</strong>
            <b class="catalog-card__type" :class="item.type">{{ item.type }}</b>
          </header>
          <div class="catalog-card__meta">
            <span>规则数量：{{ item.count }}</span>
            <button class="preview-button" type="button" :aria-label="`预览 ${item.name} 规则`" title="预览规则" :disabled="previewLoading" @click="showPreview(item)"><AppIcon name="eye" :size="16" /></button>
          </div>
          <div class="catalog-card__actions">
            <button class="more-actions" type="button" :disabled="previewLoading" @click="showPreview(item)">更多</button>
            <button class="add-config-button" type="button" :class="{ added: installed(item) }" :disabled="installed(item) || Boolean(importing)" @click="importItem(item)">{{ installed(item) ? "已添加到配置" : "添加到配置" }}</button>
          </div>
        </article>
      </div>
      <div v-else-if="!store.catalogLoading.value" class="catalog-empty"><AppIcon name="search" :size="28" /><strong>没有匹配的规则</strong><span>换一个关键词或清除类型筛选。</span></div>
      <div v-if="store.catalogLoading.value" class="catalog-empty"><AppIcon name="rotate" :size="28" /><strong>正在加载完整目录</strong><span>首次下载约 100 KB，请稍候。</span></div>
      <nav v-if="filtered.length > pageSize" class="pagination" aria-label="规则目录分页"><button type="button" :disabled="page <= 1" @click="page--">上一页</button><span>第 {{ page }} / {{ pageCount }} 页</span><button type="button" :disabled="page >= pageCount" @click="page++">下一页</button></nav>
    </template>
    <template v-else>
      <div class="preview-summary"><span class="item-icon" :class="previewItem?.type"><AppIcon :name="previewItem?.type === 'geoip' ? 'globe' : 'rules'" :size="17" /></span><div><strong>{{ previewItem?.name }}</strong><span>{{ preview.ruleBlocks }} 个规则块 · {{ preview.ruleCount.toLocaleString("zh-CN") }} 条规则信息</span></div><b>{{ previewItem?.type }}</b></div>
      <div class="preview-editor"><header><b>JSON</b><span>Source 内容结构预览（仅预览）</span></header><pre>{{ JSON.stringify(preview.content, null, 2) }}</pre></div>
      <p v-if="preview.truncated" class="preview-note"><AppIcon name="info" :size="14" />规则内容较大，页面仅展示部分结构；导入时仍会下载完整 Binary 文件。</p>
    </template>
    <template #footer><template v-if="preview"><button type="button" class="back-button" @click="back">返回中心</button><span class="footer-spacer" /><button v-if="previewItem" type="button" class="primary-footer" :disabled="installed(previewItem) || Boolean(importing)" @click="importItem(previewItem)">{{ installed(previewItem) ? "已添加到配置" : "添加到配置" }}</button></template><template v-else><span class="footer-note"><AppIcon name="lightning" :size="13" />网络规则统一以 Binary 格式导入</span><button type="button" class="primary-footer" @click="emit('close')">完成</button></template></template>
  </AppDialog>
</template>

<style scoped>
:global(.app-dialog.catalog-dialog) { display: flex; width: min(1440px, calc(100vw - 32px)); max-width: min(1440px, calc(100vw - 32px)); max-height: min(94dvh, 900px); flex-direction: column; overflow: hidden; }:global(.catalog-dialog .app-dialog__content) { display: flex; min-height: 0; flex: 1; flex-direction: column; overflow: auto; }.catalog-overview { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 13px; margin-bottom: 14px; padding: 13px 14px; border: 1px solid color-mix(in srgb, var(--primary) 15%, var(--line)); border-radius: 13px; background: linear-gradient(115deg, color-mix(in srgb, var(--primary-soft) 74%, var(--surface)), var(--surface) 70%); }.overview-icon { display: grid; width: 40px; height: 40px; place-items: center; border-radius: 11px; color: var(--primary); background: var(--surface); }.catalog-overview > div:nth-child(2) strong, .catalog-overview > div:nth-child(2) span { display: block; }.catalog-overview > div:nth-child(2) strong { font-size: 12px; }.catalog-overview > div:nth-child(2) span { margin-top: 2px; color: var(--text-soft); font-size: 10px; }.overview-stats { display: grid; grid-template-columns: repeat(2, minmax(76px, 1fr)); gap: 7px; }.overview-stats > span { padding: 7px 10px; border: 1px solid var(--line); border-radius: 9px; background: var(--surface); text-align: center; }.overview-stats strong, .overview-stats small { display: block; }.overview-stats strong { font-size: 15px; }.overview-stats small { color: var(--text-faint); font-size: var(--font-size-sm); }.catalog-toolbar { display: grid; grid-template-columns: minmax(240px, 1fr) auto auto; gap: 9px; margin-bottom: 13px; }.catalog-search { position: relative; }.catalog-search > :first-child { position: absolute; z-index: 1; top: 50%; left: 11px; color: var(--text-faint); transform: translateY(-50%); }.catalog-search input { width: 100%; min-height: 39px; padding-left: 35px; background: var(--surface-soft); font-size: 10px; }.catalog-filters { display: flex; gap: 3px; padding: 3px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-soft); }.catalog-filters button { display: inline-flex; min-height: 31px; align-items: center; gap: 5px; padding: 5px 8px; color: var(--text-soft); background: transparent; font-size: 9px; }.catalog-filters button.active { color: var(--primary); background: var(--surface); }.catalog-filters small { padding: 1px 4px; border-radius: 999px; background: var(--line); font-size: var(--font-size-xs); }.refresh-button { display: inline-flex; min-height: 39px; align-items: center; gap: 6px; padding: 6px 10px; color: var(--text-soft); border: 1px solid var(--line); background: var(--surface); font-size: 9px; }.catalog-error { margin: 0 0 10px; padding: 8px 10px; border-radius: 8px; color: var(--red); background: var(--red-soft); font-size: 9px; }.results-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 9px; color: var(--text-soft); font-size: 9px; }.results-head strong { color: var(--text); font-size: 11px; }.results-head small { display: inline-flex; align-items: center; gap: 6px; color: var(--text-faint); }.results-head i { width: 6px; height: 6px; border-radius: 50%; background: var(--green); }.catalog-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; }.catalog-card { position: relative; display: flex; min-height: 187px; flex-direction: column; padding: 13px; border: 1px solid var(--line); border-radius: 12px; background: var(--surface); }.catalog-card.installed::before { position: absolute; inset: 0 0 auto; height: 2px; content: ""; background: var(--green); }.catalog-card header { display: flex; min-width: 0; align-items: center; gap: 9px; }.item-icon { display: grid; width: 36px; height: 36px; flex: 0 0 auto; place-items: center; border-radius: 10px; color: var(--primary); background: var(--primary-soft); }.item-icon.geoip { color: var(--cyan); background: var(--cyan-soft); }.catalog-card header div { min-width: 0; flex: 1; }.catalog-card header strong, .catalog-card header span { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.catalog-card header strong { font-size: 10px; }.catalog-card header span { color: var(--text-faint); font-size: var(--font-size-sm); }.catalog-card header b, .preview-summary > b { padding: 3px 6px; border-radius: 6px; color: var(--primary); background: var(--primary-soft); font: var(--font-size-xs) ui-monospace, monospace; }.catalog-card header b.geoip { color: var(--cyan); background: var(--cyan-soft); }.catalog-card > p { margin: 11px 0 8px; color: var(--text-soft); font-size: 9px; }.item-meta { display: flex; align-items: center; justify-content: space-between; margin-top: auto; color: var(--text-faint); }.item-meta > span, .item-meta button { display: inline-flex; min-height: 26px; align-items: center; gap: 4px; }.item-meta strong { color: var(--text-soft); font-size: 9px; }.item-meta small { font-size: var(--font-size-xs); }.item-meta button { padding: 3px 5px; color: var(--primary); background: transparent; font-size: var(--font-size-sm); }.item-actions { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px; margin-top: 8px; padding-top: 9px; border-top: 1px solid var(--line); }.item-actions button { display: flex; min-height: 32px; align-items: center; justify-content: center; gap: 5px; padding: 5px; border: 1px solid var(--line); color: var(--text-soft); background: var(--surface); font-size: var(--font-size-sm); }.item-actions button.binary { color: #fff; border-color: var(--primary); background: var(--primary); }.item-actions button.added:disabled { color: var(--green); border-color: color-mix(in srgb, var(--green) 20%, var(--line)); background: var(--green-soft); opacity: 1; }.catalog-empty { display: grid; min-height: 260px; place-items: center; align-content: center; gap: 7px; color: var(--text-faint); }.catalog-empty strong { color: var(--text); font-size: 11px; }.catalog-empty span { font-size: 9px; }.pagination { display: flex; align-items: center; justify-content: center; gap: 10px; margin-top: 13px; }.pagination button { min-height: 29px; padding: 4px 8px; color: var(--text-soft); border: 1px solid var(--line); background: var(--surface); font-size: var(--font-size-sm); }.pagination span { color: var(--text-faint); font-size: var(--font-size-sm); }.preview-summary { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; padding: 11px 12px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-soft); }.preview-summary div { min-width: 0; flex: 1; }.preview-summary strong, .preview-summary span { display: block; }.preview-summary strong { font-size: 11px; }.preview-summary span { color: var(--text-faint); font-size: var(--font-size-sm); }.preview-editor { overflow: hidden; border-radius: 10px; background: #10182a; }.preview-editor header { display: flex; gap: 8px; padding: 9px 12px; color: #91a0bb; border-bottom: 1px solid rgb(255 255 255 / 8%); font-size: 9px; }.preview-editor header b { color: #8aa2ff; }.preview-editor pre { min-height: 340px; max-height: 55dvh; margin: 0; padding: 15px; overflow: auto; color: #d4def0; font: 10px/1.65 ui-monospace, monospace; }.preview-note { display: flex; align-items: center; gap: 7px; margin: 10px 0 0; padding: 9px 10px; color: var(--text-soft); background: var(--primary-soft); font-size: 9px; }.footer-spacer { flex: 1; }.footer-note { display: inline-flex; align-items: center; gap: 5px; margin-right: auto; color: var(--text-faint); font-size: var(--font-size-sm); }.primary-footer, .back-button, :global(.catalog-dialog .app-dialog__footer > button) { min-height: 35px; padding: 6px 10px; font-size: 9px; }.back-button { color: var(--text-soft); border: 1px solid var(--line); background: var(--surface); }.primary-footer { color: #fff; background: var(--primary); }
@media (max-width: 900px) { .catalog-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }.catalog-toolbar { grid-template-columns: minmax(0, 1fr) auto; }.catalog-search { grid-column: 1 / -1; } }
@media (max-width: 680px) { :global(.app-dialog.catalog-dialog) { width: 100%; max-width: 100%; }.catalog-overview { grid-template-columns: auto minmax(0, 1fr); }.overview-stats { grid-column: 1 / -1; }.catalog-grid { grid-template-columns: 1fr; }.catalog-toolbar { grid-template-columns: minmax(0, 1fr) auto; }.catalog-filters { overflow-x: auto; }.catalog-filters button { padding-inline: 6px; }.catalog-card { min-height: 180px; }.footer-note { display: none; } }

.catalog-grid { grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
.catalog-card { min-height: 92px; padding: 9px 10px 8px; border: 0; border-radius: 9px; box-shadow: 0 4px 13px rgb(23 32 51 / 10%); }
.catalog-card .catalog-card__header { justify-content: space-between; gap: 10px; }
.catalog-card .catalog-card__header > strong { min-width: 0; overflow: hidden; color: var(--text); font-size: 15px; font-weight: 720; line-height: 1.35; text-overflow: ellipsis; white-space: nowrap; }
.catalog-card header .catalog-card__type { flex: 0 0 auto; padding: 1px 4px; border: 1px solid currentColor; border-radius: 5px; color: var(--cyan); background: color-mix(in srgb, var(--cyan-soft) 45%, transparent); font: 9px/1.25 ui-monospace, monospace; font-weight: 500; }
.catalog-card header .catalog-card__type.geoip { color: var(--primary); background: color-mix(in srgb, var(--primary-soft) 45%, transparent); }
.catalog-card__meta { display: flex; min-width: 0; align-items: center; justify-content: space-between; gap: 8px; margin-top: 3px; color: var(--text-soft); font-size: 12px; line-height: 1.4; }
.catalog-card__meta > span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.catalog-card__meta .preview-button { display: inline-grid; width: 23px; min-height: 23px; flex: 0 0 auto; place-items: center; padding: 2px; border-radius: 5px; color: var(--text); background: transparent; }
.catalog-card__meta .preview-button:hover { color: var(--primary); background: var(--primary-soft); }
.catalog-card__actions { display: flex; align-items: end; justify-content: space-between; gap: 10px; margin-top: auto; }
.catalog-card__actions .more-actions { min-height: 22px; padding: 2px 0; color: var(--text); background: transparent; font-size: 12px; line-height: 18px; }
.catalog-card__actions .more-actions:hover:not(:disabled) { color: var(--primary); }
.catalog-card__actions .add-config-button { min-height: 22px; padding: 2px 0; color: var(--primary-strong); background: transparent; font-size: 12px; line-height: 18px; }
.catalog-card__actions .add-config-button:hover:not(:disabled) { color: var(--primary); text-decoration: underline; text-underline-offset: 2px; }
.catalog-card__actions .add-config-button.added:disabled { color: var(--green); cursor: default; opacity: 1; }

@media (max-width: 1180px) { .catalog-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
@media (max-width: 900px) { .catalog-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 680px) { .catalog-grid { grid-template-columns: 1fr; }.catalog-card { min-height: 92px; } }
:global(.catalog-dialog:not([open])) { display: none; }
</style>
