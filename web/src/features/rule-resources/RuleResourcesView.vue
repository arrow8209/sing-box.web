<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ApiError } from "@/api/client";
import AppToast from "@/components/AppToast.vue";
import AppIcon from "@/layout/AppIcon.vue";
import RuleCatalogDialog from "./RuleCatalogDialog.vue";
import RuleResourceDeleteDialog from "./RuleResourceDeleteDialog.vue";
import RuleResourceDialog from "./RuleResourceDialog.vue";
import RuleResourceTable from "./RuleResourceTable.vue";
import { ruleResourceApi } from "./api";
import { useRuleResourceStore } from "./store";
import type { ProfileReference, RuleResource, RuleResourceDetail, RuleResourceInput, RuleResourcePatch } from "./types";

const store = useRuleResourceStore();
const router = useRouter();
const dialogOpen = ref(false);
const catalogOpen = ref(false);
const editing = ref<RuleResourceDetail | null>(null);
const saving = ref(false);
const batchUpdating = ref(false);
const query = ref("");
const sourceFilter = ref<"all" | "remote" | "manual">("all");
const notice = ref("");
const toastTone = ref<"info" | "error" | "success">("info");
const loadError = ref("");
const deleting = ref<RuleResource | null>(null);
const deleteOpen = ref(false);
const deleteBusy = ref(false);
const referencesLoading = ref(false);
const references = ref<ProfileReference[]>([]);

const catalogRuleCounts = computed(() => {
  const counts = new Map<string, number>();
  const catalog = store.catalog.value;
  if (!catalog) return counts;
  for (const item of catalog.items) {
    const base = item.type === "geoip" ? catalog.geoip : catalog.geosite;
    counts.set(item.key, item.count);
    counts.set(`${base}${item.name}.srs`, item.count);
  }
  return counts;
});
const resources = computed(() => store.resources.value.map((resource) => {
  if (resource.ruleCount != null) return resource;
  const count = catalogRuleCounts.value.get(resource.catalogKey ?? "") ?? catalogRuleCounts.value.get(resource.url);
  return count === undefined ? resource : { ...resource, ruleCount: count };
}));
const filtered = computed(() => {
  const keyword = query.value.trim().toLowerCase();
  return resources.value.filter((item) => (sourceFilter.value === "all" || item.sourceType === sourceFilter.value) && (!keyword || `${item.name} ${item.url} ${item.catalogKey ?? ""} ${item.localPath}`.toLowerCase().includes(keyword)));
});
const remote = computed(() => resources.value.filter((item) => item.sourceType === "remote"));
const scheduled = computed(() => remote.value.filter((item) => !item.disabled && Boolean(item.updateInterval)));
const manualUpdate = computed(() => remote.value.filter((item) => !item.disabled && !item.updateInterval));
const manual = computed(() => resources.value.filter((item) => item.sourceType === "manual"));
const knownRules = computed(() => resources.value.reduce((total, item) => total + (item.ruleCount ?? 0), 0));
const nextScheduled = computed(() => [...scheduled.value].filter((item) => item.nextUpdateAt).sort((a, b) => String(a.nextUpdateAt).localeCompare(String(b.nextUpdateAt)))[0]);
const hasFilter = computed(() => Boolean(query.value) || sourceFilter.value !== "all");

onMounted(() => {
  void store.refresh().catch((cause: unknown) => { loadError.value = cause instanceof Error ? cause.message : "加载规则集失败"; });
  void store.loadCatalog(false).catch(() => { /* 目录不可用时仍可管理已有规则。 */ });
});

function show(message: string, tone: "info" | "error" | "success" = "info"): void { notice.value = ""; toastTone.value = tone; queueMicrotask(() => { notice.value = message; }); }
function openCreate(): void { editing.value = null; dialogOpen.value = true; }
async function openEdit(resource: RuleResource): Promise<void> {
  loadError.value = "";
  try { editing.value = await store.get(resource.id); dialogOpen.value = true; }
  catch (cause) { show(cause instanceof Error ? cause.message : "读取规则集失败", "error"); }
}
async function submit(input: RuleResourceInput): Promise<void> {
  saving.value = true;
  try {
    if (editing.value) await store.patch(editing.value.id, input as RuleResourcePatch);
    else await store.create(input);
    dialogOpen.value = false; editing.value = null;
    show("规则集已保存。", "success");
  } catch (cause) { show(cause instanceof Error ? cause.message : "保存规则集失败", "error"); }
  finally { saving.value = false; }
}
async function update(resource: RuleResource): Promise<void> {
  try {
    const result = await store.update(resource.id);
    show(result.message || (result.changed ? "规则集已更新，请重新应用相关档案。" : "规则集内容未变化。"), result.changed ? "success" : "info");
  } catch (cause) { show(cause instanceof Error ? cause.message : "更新规则集失败", "error"); }
}
async function updateAll(): Promise<void> {
  batchUpdating.value = true;
  try {
    const result = await store.updateAll(resources.value);
    show(`网络规则更新完成：${result.changed} 个有变化，${result.unchanged} 个未变化，${result.failed} 个失败。`, result.failed ? "error" : "success");
  } catch (cause) { show(cause instanceof Error ? cause.message : "批量更新规则集失败", "error"); }
  finally { batchUpdating.value = false; }
}
async function requestDelete(resource: RuleResource): Promise<void> {
  dialogOpen.value = false; deleting.value = resource; deleteOpen.value = true; references.value = []; referencesLoading.value = true;
  try { references.value = await ruleResourceApi.references(resource.id); }
  catch (cause) { show(cause instanceof Error ? cause.message : "检查规则集引用失败", "error"); }
  finally { referencesLoading.value = false; }
}
async function confirmDelete(): Promise<void> {
  if (!deleting.value) return;
  deleteBusy.value = true;
  try {
    await store.remove(deleting.value.id); deleteOpen.value = false; deleting.value = null; show("规则集已删除。", "success");
  } catch (cause) {
    if (cause instanceof ApiError && cause.status === 409) {
      const profiles = cause.body.details?.profiles;
      references.value = Array.isArray(profiles) ? profiles as ProfileReference[] : [];
    }
    show(cause instanceof Error ? cause.message : "删除规则集失败", "error");
  } finally { deleteBusy.value = false; }
}
function openProfile(reference: ProfileReference): void { deleteOpen.value = false; void router.push({ name: "profiles", query: { open: reference.id } }); }
function clearFilter(): void { query.value = ""; sourceFilter.value = "all"; }
function stamp(value?: string): string { return value ? new Date(value).toLocaleString("zh-CN") : "尚未生成计划"; }
</script>

<template>
  <section class="rules-page">
    <header class="page-intro"><div><h1>规则集中心</h1><p>集中导入、更新和维护 sing-box 域名与 IP 规则资源。</p></div><div class="page-actions"><button type="button" @click="catalogOpen = true"><AppIcon name="database" :size="15" />规则集中心</button><button type="button" :disabled="batchUpdating || !remote.some((item) => !item.disabled)" @click="updateAll"><AppIcon name="rotate" :size="15" />{{ batchUpdating ? `更新中 ${store.updatingIds.value.length} 项` : "更新网络规则" }}</button><button type="button" class="primary" data-create-rule @click="openCreate"><AppIcon name="plus" :size="15" />添加规则集</button></div></header>

    <p v-if="loadError" class="page-error" role="alert">{{ loadError }}</p>

    <div class="summary-grid">
      <article class="summary-card"><span class="summary-icon"><AppIcon name="database" /></span><div><strong>{{ resources.length }}</strong><span>规则集 · 已统计 {{ knownRules.toLocaleString("zh-CN") }} 条规则</span></div></article>
      <article class="summary-card"><span class="summary-icon cyan"><AppIcon name="clock" /></span><div><strong>{{ scheduled.length }}</strong><span>定时更新 · {{ remote.length }} 个网络资源</span></div></article>
      <article class="summary-card"><span class="summary-icon orange"><AppIcon name="code" /></span><div><strong>{{ manual.length }}</strong><span>纯手工规则 · 直接编辑 JSON 源文件</span></div></article>
    </div>

    <article class="schedule-banner"><span class="schedule-icon"><AppIcon name="clock" :size="18" /></span><div class="schedule-copy"><strong>{{ scheduled.length ? "网络规则定时更新已启用" : "尚未配置网络规则定时更新" }}</strong><span>{{ nextScheduled ? `最近计划：${nextScheduled.name} · ${stamp(nextScheduled.nextUpdateAt)}` : "设置更新周期后，服务端会在浏览器关闭后继续调度" }}</span></div><div class="schedule-stats"><span><strong>{{ scheduled.length }}</strong> 个计划</span><span><strong>{{ manualUpdate.length }}</strong> 个仅手动</span></div></article>

    <div class="rules-toolbar"><label><AppIcon name="search" :size="15" /><input v-model="query" type="search" placeholder="搜索规则集名称、来源或 URL..." /></label><select v-model="sourceFilter"><option value="all">全部来源</option><option value="remote">网络规则集</option><option value="manual">纯手工规则</option></select><span>{{ filtered.length }} / {{ resources.length }}</span></div>

    <RuleResourceTable :resources="filtered" :updating-ids="store.updatingIds.value" :filtered="hasFilter" @edit="openEdit" @update="update" @clear-filter="clearFilter" />
    <RuleResourceDialog :open="dialogOpen" :resource="editing" :busy="saving" @close="dialogOpen = false" @submit="submit" @delete="requestDelete" />
    <RuleCatalogDialog :open="catalogOpen" @close="catalogOpen = false" @notice="(message) => show(message, 'success')" @error="(message) => show(message, 'error')" />
    <RuleResourceDeleteDialog :open="deleteOpen" :resource="deleting" :references="references" :busy="deleteBusy" :loading="referencesLoading" @close="deleteOpen = false" @confirm="confirmDelete" @profile="openProfile" />
    <AppToast v-if="notice" :message="notice" :tone="toastTone" />
  </section>
</template>

<style scoped>
.rules-page { display: grid; gap: var(--section-gap); }.page-intro { display: flex; align-items: end; justify-content: space-between; gap: var(--section-gap); }.page-intro h1 { margin: 0; font-size: 22px; letter-spacing: -.45px; }.page-intro p { margin: 5px 0 0; color: var(--text-soft); font-size: 12px; }.page-actions { display: flex; gap: 7px; }.page-actions button { display: inline-flex; min-height: 35px; align-items: center; gap: 6px; padding: 7px 10px; border: 1px solid var(--line); color: var(--text-soft); background: var(--surface); font-size: 10px; }.page-actions button.primary { color: #fff; border-color: var(--primary); background: var(--primary); }.page-error { margin: 0; padding: 10px 12px; border-radius: 9px; color: var(--red); background: var(--red-soft); font-size: 10px; }.summary-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: var(--section-gap); }.summary-card { display: flex; min-height: 66px; align-items: center; gap: 10px; padding: 10px var(--card-padding); border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.summary-icon { display: grid; width: 32px; height: 32px; flex: 0 0 auto; place-items: center; border-radius: 11px; color: var(--primary); background: var(--primary-soft); }.summary-icon.cyan { color: var(--cyan); background: var(--cyan-soft); }.summary-icon.orange { color: var(--orange); background: var(--orange-soft); }.summary-card strong, .summary-card span { display: block; }.summary-card strong { color: var(--text); font-size: 19px; }.summary-card div span { margin-top: 2px; color: var(--text-soft); font-size: 9px; }.schedule-banner { display: flex; min-height: 56px; align-items: center; gap: 13px; padding: 8px var(--card-padding); border: 1px solid color-mix(in srgb, var(--primary) 15%, var(--line)); border-radius: var(--radius); background: linear-gradient(115deg, color-mix(in srgb, var(--primary-soft) 66%, var(--surface)), var(--surface)); box-shadow: var(--shadow-sm); }.schedule-icon { display: grid; width: 38px; height: 38px; flex: 0 0 auto; place-items: center; border-radius: 11px; color: var(--primary); background: var(--surface); }.schedule-copy { min-width: 0; flex: 1; }.schedule-copy strong, .schedule-copy span { display: block; }.schedule-copy strong { font-size: 11px; }.schedule-copy span { overflow: hidden; margin-top: 3px; color: var(--text-faint); font-size: 9px; text-overflow: ellipsis; white-space: nowrap; }.schedule-stats { display: flex; gap: 7px; }.schedule-stats span { padding: 7px 9px; border: 1px solid var(--line); border-radius: 8px; color: var(--text-faint); background: var(--surface); font-size: 8px; }.schedule-stats strong { margin-right: 3px; color: var(--text); font-size: 10px; }.rules-toolbar { display: flex; align-items: center; gap: 9px; padding: 10px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.rules-toolbar label { position: relative; min-width: 220px; flex: 1; }.rules-toolbar label > :first-child { position: absolute; z-index: 1; top: 50%; left: 11px; color: var(--text-faint); transform: translateY(-50%); }.rules-toolbar input { width: 100%; min-height: 36px; padding-left: 34px; background: var(--surface-soft); font-size: 10px; }.rules-toolbar select { width: 145px; min-height: 36px; background: var(--surface-soft); font-size: 9px; }.rules-toolbar > span { min-width: 48px; color: var(--text-faint); font-size: 8px; text-align: center; }
@media (max-width: 800px) { .page-intro { align-items: flex-start; flex-direction: column; }.page-actions { width: 100%; flex-wrap: wrap; }.page-actions button { flex: 1; justify-content: center; }.rules-toolbar { flex-wrap: wrap; }.rules-toolbar label { min-width: 100%; }.rules-toolbar select { flex: 1; }.schedule-banner { align-items: flex-start; flex-wrap: wrap; }.schedule-copy { min-width: calc(100% - 54px); }.schedule-stats { width: 100%; }.schedule-stats span { flex: 1; text-align: center; } }
@media (max-width: 430px) { .page-actions button { min-width: calc(50% - 4px); }.page-actions button.primary { flex-basis: 100%; }.schedule-copy span { white-space: normal; } }
@media (max-width: 680px) {
  .summary-grid { gap: 8px; }
  .summary-card { min-width: 0; min-height: 64px; justify-content: center; padding: 8px 4px; text-align: center; }
  .summary-icon { display: none; }
  .summary-card div span { font-size: 10px; }
}
</style>
