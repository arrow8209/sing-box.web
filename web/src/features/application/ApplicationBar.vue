<script setup lang="ts">
import { useProfileStore } from "@/features/profiles/store";

withDefaults(defineProps<{ busy?: boolean; applyDisabled?: boolean; applyTitle?: string }>(), { busy: false, applyDisabled: false, applyTitle: "检测通过后复制配置文件" });
const emit = defineEmits<{ preview: []; validate: []; apply: [] }>();
const profiles = useProfileStore();
</script>
<template>
  <div class="application-actions" role="group" aria-label="配置操作">
    <button data-preview type="button" class="secondary" :disabled="busy || profiles.saveState.value !== 'saved'" @click="emit('preview')">预览</button>
    <button data-validate type="button" class="secondary" :disabled="busy || profiles.saveState.value !== 'saved'" @click="emit('validate')">检测</button>
    <button data-apply type="button" :disabled="busy || applyDisabled" :title="applyTitle" @click="emit('apply')">{{ busy ? "处理中" : "应用" }}</button>
  </div>
</template>
<style scoped>
.application-actions { display: flex; flex: 0 0 auto; align-items: center; gap: 8px; }
button { min-height: 34px; padding: 6px 10px; font-size: var(--font-size-sm); }
.secondary { color: var(--text); background: var(--surface-soft); }
</style>
