<script setup lang="ts">
import { configTabs, type ConfigTabId } from "./module-map";
defineProps<{ active: ConfigTabId }>();
const emit = defineEmits<{ select: [tab: ConfigTabId] }>();
</script>
<template>
  <nav class="tabs" aria-label="配置模块">
    <button v-for="tab in configTabs" :key="tab.id" type="button" :class="{ active: active === tab.id }" @click="emit('select', tab.id)">{{ tab.label }}</button>
  </nav>
</template>
<style scoped>
.tabs { display: flex; gap: 6px; overflow-x: auto; padding-bottom: 4px; border-bottom: 1px solid var(--line); }.tabs button { flex: 0 0 auto; min-height: 34px; padding: 6px 12px; color: var(--text-soft); background: transparent; }.tabs button.active { color: var(--primary-strong); background: var(--primary-soft); }
</style>
