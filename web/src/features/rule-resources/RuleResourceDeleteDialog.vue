<script setup lang="ts">
import AppDialog from "@/components/AppDialog.vue";
import AppIcon from "@/layout/AppIcon.vue";
import type { ProfileReference, RuleResource } from "./types";

defineProps<{ open: boolean; resource?: RuleResource | null; references: ProfileReference[]; busy?: boolean; loading?: boolean }>();
const emit = defineEmits<{ close: []; confirm: []; profile: [reference: ProfileReference] }>();
</script>

<template>
  <AppDialog :open="open" title="删除规则集" description="删除后固定规则文件也会一并移除" :close-disabled="busy" @close="emit('close')">
    <div v-if="resource" class="delete-body"><span class="delete-icon"><AppIcon name="trash" :size="21" /></span><div><strong>确定删除“{{ resource.name }}”吗？</strong><p>此操作不能撤销，但不会修改其他规则资源。</p></div></div>
    <p v-if="loading" class="reference-note">正在检查档案引用…</p>
    <div v-else-if="references.length" class="reference-warning"><strong>该规则集仍被以下档案引用，暂时不能删除：</strong><button v-for="reference in references" :key="reference.id" type="button" @click="emit('profile', reference)">{{ reference.name }}</button></div>
    <template #footer><button type="button" class="secondary" :disabled="busy" @click="emit('close')">取消</button><button type="button" class="danger" :disabled="busy || loading || references.length > 0" @click="emit('confirm')">{{ busy ? "删除中…" : "确认删除" }}</button></template>
  </AppDialog>
</template>

<style scoped>
.delete-body { display: flex; align-items: flex-start; gap: 12px; padding: 12px; border-radius: 11px; background: var(--red-soft); }.delete-icon { display: grid; width: 42px; height: 42px; flex: 0 0 auto; place-items: center; border-radius: 11px; color: var(--red); background: var(--surface); }.delete-body strong { font-size: var(--font-size-base); }.delete-body p { margin: 4px 0 0; color: var(--text-soft); font-size: var(--font-size-sm); }.reference-note { color: var(--text-faint); font-size: var(--font-size-sm); }.reference-warning { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 12px; padding: 10px; border: 1px solid color-mix(in srgb, var(--red) 20%, var(--line)); border-radius: 9px; }.reference-warning strong { width: 100%; color: var(--red); font-size: var(--font-size-base); }.reference-warning button { min-height: 28px; padding: 4px 7px; color: var(--primary); background: var(--primary-soft); font-size: var(--font-size-sm); }.secondary, .danger { min-height: 36px; padding: 7px 11px; border: 1px solid var(--line); font-size: var(--font-size-sm); }.secondary { color: var(--text-soft); background: var(--surface); }.danger { color: #fff; border-color: var(--red); background: var(--red); }
</style>
