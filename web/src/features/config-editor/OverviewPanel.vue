<script setup lang="ts">
import { computed } from "vue";
import type { Draft } from "@/features/profiles/types";
import { updateStructuredPath } from "./paths";
const props = defineProps<{ draft: Draft; error?: string }>();
const emit = defineEmits<{ update: [draft: Draft, error?: string]; editJson: [module: "log"] }>();
const level = computed(() => typeof props.draft.config.log === "object" && props.draft.config.log !== null ? String((props.draft.config.log as Record<string, unknown>).level ?? "") : "");
function change(value: string): void { const result = updateStructuredPath(props.draft, "log", ["level"], value); emit("update", result.draft, result.error?.message); }
</script>
<template>
  <section class="panel"><header><div><h2>概览</h2><p>编辑常用日志级别；其他字段会保留在模块 JSON 中。</p></div><button type="button" class="secondary" @click="emit('editJson', 'log')">编辑 JSON</button></header><label>日志级别<select :value="level" @change="change(($event.target as HTMLSelectElement).value)"><option value="">未设置</option><option value="trace">trace</option><option value="debug">debug</option><option value="info">info</option><option value="warn">warn</option><option value="error">error</option><option value="fatal">fatal</option></select></label><p v-if="error" role="alert" class="error">{{ error }}</p></section>
</template>
<style scoped>
.panel { padding: 18px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); }.panel header { display: flex; justify-content: space-between; gap: 12px; }.panel h2 { margin: 0; }.panel p { color: var(--text-soft); }.secondary { color: var(--text); background: var(--surface-soft); } label { display: grid; gap: 6px; max-width: 340px; font-weight: 650; } select { min-height: 42px; padding: 9px 11px; border: 1px solid var(--line-strong); border-radius: var(--radius-sm); color: var(--text); background: var(--surface); }.error { color: var(--red); }
</style>
