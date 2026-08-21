<script setup lang="ts">
import { computed } from "vue";
import type { RuleBinding } from "@/features/profiles/types";
import type { RuleResource } from "./types";
const props = defineProps<{ bindings: RuleBinding[]; resources: RuleResource[] }>();
const emit = defineEmits<{ change: [bindings: RuleBinding[]] }>();
const available = computed(() => props.resources.filter((item) => !item.disabled));
function change(index: number, patch: Partial<RuleBinding>): void { emit("change", props.bindings.map((item, current) => current === index ? { ...item, ...patch } : { ...item })); }
function add(): void { const resource = available.value[0]; if (resource) emit("change", [...props.bindings.map((item) => ({ ...item })), { resourceId: resource.id, tag: resource.name, enabled: true }]); }
function remove(index: number): void { emit("change", props.bindings.filter((_, current) => current !== index).map((item) => ({ ...item }))); }
function resourceFor(id: string): RuleResource | undefined { return props.resources.find((item) => item.id === id); }
</script>
<template>
  <section class="bindings"><header><div><h3>Web 规则资源绑定</h3><p>绑定将自动保存到当前档案草稿；路径由服务端固定管理。</p></div><button type="button" :disabled="!available.length" @click="add">添加绑定</button></header><p v-if="!available.length" class="hint">没有可绑定的已启用规则资源。请先在“规则资源”页面创建或启用资源。</p><div v-for="(binding, index) in bindings" :key="`${binding.resourceId}-${index}`" class="binding"><label>资源<select :value="binding.resourceId" @change="change(index, { resourceId: ($event.target as HTMLSelectElement).value })"><option v-for="resource in available" :key="resource.id" :value="resource.id">{{ resource.name }}</option></select></label><label>sing-box tag<input :value="binding.tag" required @input="change(index, { tag: ($event.target as HTMLInputElement).value })" /></label><label class="enabled"><input :checked="binding.enabled" type="checkbox" @change="change(index, { enabled: ($event.target as HTMLInputElement).checked })" /> 启用</label><label>本地路径<input :value="resourceFor(binding.resourceId)?.localPath ?? '资源不存在'" readonly /></label><button type="button" @click="remove(index)">移除</button></div></section>
</template>
<style scoped>
.bindings { display: grid; gap: 12px; margin-top: 14px; padding-top: 14px; border-top: 1px solid var(--line); }.bindings header { display: flex; justify-content: space-between; gap: 10px; }.bindings h3, .bindings p { margin: 0; }.bindings p, .hint { color: var(--text-soft); }.binding { display: grid; grid-template-columns: 1fr 1fr auto 1.3fr auto; align-items: end; gap: 8px; padding: 10px; border-radius: var(--radius-sm); background: var(--surface-soft); }.binding label { display: grid; gap: 4px; font-size: 13px; font-weight: 600; }.binding input, .binding select { width: 100%; }.binding .enabled { display: flex; align-items: center; min-height: 34px; }.binding .enabled input { width: auto; } @media (max-width: 850px) { .binding { grid-template-columns: 1fr; } }
</style>
