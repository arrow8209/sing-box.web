<script setup lang="ts">
import { useProfileStore } from "@/features/profiles/store";

const emit = defineEmits<{ preview: []; validate: [] }>();
const profiles = useProfileStore();
</script>
<template>
  <div class="application-actions" role="group" aria-label="配置操作">
    <button data-preview type="button" class="secondary" :disabled="profiles.saveState.value !== 'saved'" @click="emit('preview')">预览</button>
    <button data-validate type="button" class="secondary" :disabled="profiles.saveState.value !== 'saved'" @click="emit('validate')">检查</button>
    <button data-apply type="button" disabled title="应用逻辑尚未实现">应用</button>
  </div>
</template>
<style scoped>
.application-actions { display: flex; flex: 0 0 auto; align-items: center; gap: 8px; }
button { min-height: 34px; padding: 6px 10px; font-size: 10px; }
.secondary { color: var(--text); background: var(--surface-soft); }
</style>
