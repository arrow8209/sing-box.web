<script setup lang="ts">
import AppIcon from "@/layout/AppIcon.vue";
import type { RuleResource } from "./types";

defineProps<{ resources: RuleResource[]; updatingIds?: string[]; filtered?: boolean }>();
const emit = defineEmits<{ edit: [resource: RuleResource]; update: [resource: RuleResource]; clearFilter: [] }>();

const intervalLabels: Record<string, string> = { "15m": "每 15 分钟", "1h": "每 1 小时", "6h": "每 6 小时", "12h": "每 12 小时", "24h": "每天", "168h": "每 7 天" };
function stamp(value?: string): string { return value ? new Date(value).toLocaleString("zh-CN", { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" }) : "尚未更新"; }
function size(value?: number): string {
  if (value === undefined) return "--";
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}
function count(value?: number): string { return value === undefined ? "--" : value.toLocaleString("zh-CN"); }
function source(resource: RuleResource): string {
  if (resource.sourceType === "manual") return "Web 手工维护 · JSON";
  if (resource.catalogKey) return `Ruleset-Hub · ${resource.catalogKey.replace(":", "/")}`;
  try { return new URL(resource.url).hostname; } catch { return resource.url; }
}
function schedule(resource: RuleResource): string {
  if (resource.disabled) return "已禁用";
  if (resource.sourceType === "manual") return "手工维护";
  return intervalLabels[resource.updateInterval] ?? (resource.updateInterval || "仅手动更新");
}
</script>

<template>
  <article class="ruleset-table-card">
    <div v-if="resources.length" class="table-wrap">
      <table class="ruleset-data-table">
        <colgroup>
          <col class="column-ruleset" />
          <col class="column-source" />
          <col class="column-format" />
          <col class="column-count" />
          <col class="column-schedule" />
          <col class="column-updated" />
          <col class="column-actions" />
        </colgroup>
        <thead><tr><th>规则集</th><th>来源</th><th>格式</th><th>规则数</th><th>更新策略</th><th>最近更新</th><th class="right">操作</th></tr></thead>
        <tbody>
          <tr v-for="resource in resources" :key="resource.id" :class="{ disabled: resource.disabled }">
            <td><div class="ruleset-name"><span class="ruleset-icon" :class="resource.sourceType"><AppIcon :name="resource.sourceType === 'manual' ? 'code' : 'globe'" :size="15" /></span><div><strong>{{ resource.name }}</strong><small :title="resource.url || resource.localPath">{{ source(resource) }}</small></div></div></td>
            <td><span class="source-badge" :class="resource.sourceType">{{ resource.sourceType === "manual" ? "纯手工" : "网络" }}</span></td>
            <td><span class="format-badge">{{ resource.format }}</span><small class="path" :title="resource.localPath">{{ resource.localPath }}</small></td>
            <td><strong class="rule-count">{{ count(resource.ruleCount) }}</strong><small>{{ size(resource.sizeBytes) }}</small></td>
            <td><div class="update-plan"><span :class="{ scheduled: resource.sourceType === 'remote' && Boolean(resource.updateInterval) && !resource.disabled }"><AppIcon :name="resource.sourceType === 'manual' ? 'edit' : 'clock'" :size="11" />{{ schedule(resource) }}</span><small v-if="resource.nextUpdateAt">下次 {{ stamp(resource.nextUpdateAt) }}</small><small v-else>{{ resource.sourceType === "manual" ? "直接编辑规则内容" : "等待手动更新" }}</small></div></td>
            <td><div class="updated" :class="{ failed: resource.lastError }"><strong>{{ stamp(resource.lastSuccessAt) }}</strong><span :title="resource.lastError">{{ resource.lastError || (resource.sourceType === "manual" ? "手工内容" : "网络内容") }}</span></div></td>
            <td><div class="table-actions"><button v-if="resource.sourceType === 'remote'" type="button" :aria-label="`立即更新 ${resource.name}`" :title="resource.disabled ? '已禁用' : '立即更新'" :disabled="resource.disabled || updatingIds?.includes(resource.id)" @click="emit('update', resource)"><AppIcon name="rotate" :size="14" /></button><button type="button" :aria-label="`编辑 ${resource.name}`" title="编辑规则集" :disabled="updatingIds?.includes(resource.id)" @click="emit('edit', resource)"><AppIcon :name="resource.sourceType === 'manual' ? 'code' : 'edit'" :size="14" /></button></div></td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else class="empty-state"><AppIcon :name="filtered ? 'search' : 'database'" :size="28" /><strong>{{ filtered ? "没有匹配的规则集" : "尚无规则集" }}</strong><span>{{ filtered ? "调整关键词或来源筛选后重试" : "可从规则集中心导入，或手动添加规则集" }}</span><button v-if="filtered" type="button" @click="emit('clearFilter')">清除筛选</button></div>
  </article>
</template>

<style scoped>
.ruleset-table-card { overflow: hidden; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.table-wrap { overflow-x: auto; }.ruleset-data-table { width: 100%; min-width: 980px; table-layout: fixed; border-collapse: collapse; }.column-ruleset { width: 24%; }.column-source { width: 10%; }.column-format { width: 29%; }.column-count { width: 7%; }.column-schedule { width: 12%; }.column-updated { width: 12%; }.column-actions { width: 6%; }.ruleset-data-table th { padding: 8px var(--card-padding); color: var(--text-faint); background: var(--surface-soft); font-size: 9px; font-weight: 700; text-align: left; }.ruleset-data-table th.right { text-align: right; }.ruleset-data-table td { overflow: hidden; padding: 8px var(--card-padding); border-top: 1px solid var(--line); vertical-align: middle; }.ruleset-data-table tr.disabled { opacity: .58; }.ruleset-data-table tr:hover td { background: color-mix(in srgb, var(--surface-soft) 72%, transparent); }.ruleset-name { display: flex; min-width: 0; align-items: center; gap: 10px; }.ruleset-name > div { min-width: 0; }.ruleset-icon { display: grid; width: 31px; height: 31px; flex: 0 0 auto; place-items: center; border-radius: 9px; color: var(--primary); background: var(--primary-soft); }.ruleset-icon.manual { color: var(--green); background: var(--green-soft); }.ruleset-name strong, .ruleset-name small { display: block; max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.ruleset-name strong { color: var(--text); font-size: 11px; }.ruleset-name small { margin-top: 1px; color: var(--text-faint); font-size: var(--font-size-sm); }.source-badge { display: inline-flex; min-height: 22px; align-items: center; gap: 5px; padding: 3px 8px; border-radius: 999px; color: var(--primary); background: var(--primary-soft); font-size: 9px; font-weight: 670; }.source-badge::before { width: 4px; height: 4px; border-radius: 50%; content: ""; background: currentColor; }.source-badge.manual { color: var(--green); background: var(--green-soft); }.format-badge { display: inline-flex; padding: 3px 7px; border-radius: 6px; color: var(--primary); background: var(--primary-soft); font: 9px ui-monospace, monospace; }.path { display: block; width: 100%; overflow: hidden; margin-top: 3px; color: var(--text-faint); font-size: var(--font-size-xs); text-overflow: ellipsis; white-space: nowrap; }.rule-count { display: block; color: var(--text); font-size: 10px; font-variant-numeric: tabular-nums; }.rule-count + small { display: block; margin-top: 2px; color: var(--text-faint); font-size: var(--font-size-sm); }.update-plan span, .update-plan small, .updated strong, .updated span { display: block; }.update-plan span { display: inline-flex; align-items: center; gap: 4px; color: var(--text-soft); font-size: 9px; font-weight: 650; }.update-plan span.scheduled { color: var(--primary); }.update-plan small, .updated span { margin-top: 2px; color: var(--text-faint); font-size: var(--font-size-sm); }.updated { max-width: 130px; }.updated strong { color: var(--text-soft); font-size: 9px; }.updated span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.updated.failed strong, .updated.failed span { color: var(--red); }.table-actions { display: flex; justify-content: flex-end; gap: 4px; }.table-actions button { display: inline-grid; width: 30px; min-height: 30px; place-items: center; padding: 0; border: 0; border-radius: 7px; color: var(--text-soft); background: transparent; }.table-actions button:hover { color: var(--primary); background: var(--primary-soft); }.empty-state { display: grid; min-height: var(--empty-state-height); place-items: center; align-content: center; gap: 7px; color: var(--text-faint); text-align: center; }.empty-state strong { color: var(--text); font-size: 12px; }.empty-state span { font-size: 9px; }.empty-state button { min-height: 31px; margin-top: 4px; padding: 5px 9px; color: var(--primary); background: var(--primary-soft); font-size: 9px; }
</style>
