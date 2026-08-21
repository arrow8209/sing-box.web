<script setup lang="ts">
import type { RuleResource } from "./types";
defineProps<{ resources: RuleResource[]; updatingId?: string | null }>();
const emit = defineEmits<{ edit: [resource: RuleResource]; update: [resource: RuleResource]; remove: [resource: RuleResource] }>();
function stamp(value?: string): string { return value ? new Date(value).toLocaleString() : "—"; }
function digest(value?: string): string { return value ? `${value.slice(0, 12)}…` : "—"; }
</script>
<template>
  <div class="table-wrap"><table><thead><tr><th>资源</th><th>状态与周期</th><th>固定相对路径</th><th>下载记录</th><th>操作</th></tr></thead><tbody><tr v-for="resource in resources" :key="resource.id"><td><strong>{{ resource.name }}</strong><small>{{ resource.format === 'binary' ? 'binary' : 'source' }} · {{ resource.url }}</small></td><td><span :class="{ disabled: resource.disabled }">{{ resource.disabled ? '已禁用' : '已启用' }}</span><small>{{ resource.updateInterval || '手动更新' }}<template v-if="resource.nextUpdateAt"> · 下次 {{ stamp(resource.nextUpdateAt) }}</template></small></td><td><code>{{ resource.localPath }}</code></td><td><small>最近尝试：{{ stamp(resource.lastAttemptAt) }}<br />最近成功：{{ stamp(resource.lastSuccessAt) }}<br />摘要：{{ digest(resource.lastDigest) }}</small><p v-if="resource.lastError" class="error">{{ resource.lastError }}</p></td><td class="actions"><button type="button" @click="emit('edit', resource)">编辑</button><button type="button" :data-update-rule="resource.id" :disabled="updatingId === resource.id" @click="emit('update', resource)">{{ updatingId === resource.id ? '更新中…' : '立即更新' }}</button><button type="button" :data-delete-rule="resource.id" :disabled="updatingId === resource.id" @click="emit('remove', resource)">删除</button></td></tr><tr v-if="!resources.length"><td colspan="5">尚无规则资源。</td></tr></tbody></table></div>
</template>
<style scoped>
.table-wrap { overflow-x: auto; border: 1px solid var(--line); border-radius: var(--radius); } table { width: 100%; min-width: 900px; border-collapse: collapse; } th, td { padding: 12px; text-align: left; vertical-align: top; border-bottom: 1px solid var(--line); } th { color: var(--text-soft); font-size: 13px; } td small, td strong { display: block; } td small { margin-top: 4px; color: var(--text-soft); overflow-wrap: anywhere; } code { font-size: 12px; }.disabled, .error { color: var(--red); }.error { margin: 6px 0 0; }.actions { display: flex; flex-wrap: wrap; gap: 6px; }
</style>
