<script setup lang="ts">
import { onMounted } from "vue";
import { useCapabilitiesStore } from "@/features/capabilities/store";

const capabilities = useCapabilitiesStore();
onMounted(() => { void capabilities.refresh(); });
</script>

<template>
  <section class="runtime-unavailable">
    <h1>运行数据</h1>
    <template v-if="capabilities.capabilities.value?.runtime === false">
      <h2>其余运行数据尚未接入</h2>
      <p v-if="capabilities.capabilities.value?.proxyGroups">实时流量等运行数据将在后续阶段提供；活动连接可在独立页面查看，策略组可在仪表盘切换节点。</p>
      <p v-else-if="capabilities.capabilities.value?.connections">实时流量和策略组等运行数据将在后续阶段提供；活动连接现已可在独立页面查看。</p>
      <p v-else>实时流量、活动连接和策略组等运行数据将在后续阶段提供。</p>
      <a v-if="capabilities.capabilities.value?.connections" class="connections-link" href="/connections">查看活动连接</a>
    </template>
    <p v-else>正在确认运行数据能力。</p>
  </section>
</template>

<style scoped>
.runtime-unavailable { max-width: 720px; padding: 24px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); }
h1, h2 { margin: 0 0 12px; } p { color: var(--text-soft); }
.connections-link { display: inline-flex; margin-top: 4px; color: var(--primary); font-weight: 650; text-decoration: none; }.connections-link:hover { text-decoration: underline; }
</style>
