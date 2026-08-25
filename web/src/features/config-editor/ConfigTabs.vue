<script setup lang="ts">
import { configTabs, type ConfigTabId } from "./module-map";
import AppIcon from "@/layout/AppIcon.vue";
defineProps<{ active: ConfigTabId }>();
const emit = defineEmits<{ select: [tab: ConfigTabId] }>();
const icons: Record<ConfigTabId, string> = { overview: "config", dns: "dns", inbounds: "inbound", outbounds: "outbound", route: "rules", services: "server", advanced: "settings" };
</script>
<template>
  <nav class="config-tabs tabs" aria-label="配置模块" role="tablist">
    <button v-for="tab in configTabs" :key="tab.id" class="config-tab" type="button" role="tab" :aria-selected="active === tab.id" :class="{ active: active === tab.id }" @click="emit('select', tab.id)"><AppIcon :name="icons[tab.id]" :size="15" /><span>{{ tab.label }}</span></button>
  </nav>
</template>
<style scoped>
.tabs { display: flex; gap: 3px; overflow-x: auto; margin-bottom: 12px; padding: 4px; border: 1px solid var(--line); border-radius: 12px; background: var(--surface); scrollbar-width: thin; }.config-tab { display: inline-flex; min-height: 34px; flex: 0 0 auto; align-items: center; gap: 6px; padding: 7px 10px; border-radius: 8px; color: var(--text-soft); background: transparent; font-size: 11px; font-weight: 650; }.config-tab:hover { color: var(--text); background: var(--surface-soft); }.config-tab.active { color: var(--primary-strong); background: var(--primary-soft); }
</style>
