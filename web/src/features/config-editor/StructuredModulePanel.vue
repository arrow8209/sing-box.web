<script setup lang="ts">
import { computed } from "vue";
import type { Draft } from "@/features/profiles/types";
import { moduleLabels, type ConfigModule } from "./module-map";
const props = defineProps<{ draft: Draft; module: ConfigModule; error?: string }>();
const emit = defineEmits<{ editJson: [module: ConfigModule] }>();
const value = computed(() => props.draft.config[props.module]);
const present = computed(() => value.value !== undefined || props.draft.buffers[props.module] !== undefined);
const summary = computed(() => Array.isArray(value.value) ? `${value.value.length} 个项目` : present.value ? "已配置" : "未配置");
</script>
<template>
  <section class="panel"><header><div><h2>{{ moduleLabels[module] }}</h2><p>{{ summary }}</p></div><button type="button" @click="emit('editJson', module)">编辑 JSON</button></header><template v-if="Array.isArray(value)"><ul><li v-for="(item, index) in value" :key="index"><strong>{{ typeof item === 'object' && item ? (item as Record<string, unknown>).tag ?? `项目 ${index + 1}` : `项目 ${index + 1}` }}</strong><span v-if="typeof item === 'object' && item">{{ (item as Record<string, unknown>).type }}</span></li></ul></template><p v-else class="hint">此模块提供完整 JSON 编辑；未知字段会原样保留。</p><p v-if="error" class="error" role="alert">{{ error }}</p></section>
</template>
<style scoped>
.panel { padding: 18px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); }.panel header { display: flex; align-items: start; justify-content: space-between; gap: 12px; }.panel h2, .panel p { margin: 0; }.panel p, li span { color: var(--text-soft); }.panel ul { display: grid; gap: 7px; padding: 0; list-style: none; }.panel li { display: flex; justify-content: space-between; gap: 8px; padding: 9px; border-radius: var(--radius-sm); background: var(--surface-soft); }.panel .error { margin-top: 12px; color: var(--red); }
</style>
