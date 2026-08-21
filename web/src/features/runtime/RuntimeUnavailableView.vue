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
      <h2>二期尚未接入</h2>
      <p>流量、连接和策略组等运行数据将在二期提供；测速将在后续阶段提供。</p>
    </template>
    <p v-else>正在确认运行数据能力。</p>
  </section>
</template>

<style scoped>
.runtime-unavailable { max-width: 720px; padding: 24px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); }
h1, h2 { margin: 0 0 12px; } p { color: var(--text-soft); }
</style>
