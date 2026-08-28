<script setup lang="ts">
withDefaults(defineProps<{ busy?: boolean; applyDisabled?: boolean; applyTitle?: string }>(), { busy: false, applyDisabled: false, applyTitle: "检测通过后停止核心、复制配置并重新启动" });
const emit = defineEmits<{ apply: []; duplicate: []; rename: []; remove: [] }>();
</script>

<template>
  <div class="profile-actions" role="group" aria-label="档案操作">
    <button data-apply type="button" :disabled="busy || applyDisabled" :title="applyTitle" @click="emit('apply')">{{ busy ? "处理中" : "应用" }}</button>
    <button class="secondary" type="button" :disabled="busy" @click="emit('duplicate')">复制</button>
    <button class="secondary" type="button" :disabled="busy" @click="emit('rename')">重命名</button>
    <button class="danger" type="button" :disabled="busy" @click="emit('remove')">删除</button>
  </div>
</template>

<style scoped>
.profile-actions { display: flex; flex: 0 0 auto; align-items: center; gap: 6px; }
button { min-height: 34px; padding: 6px 9px; border-radius: 8px; font-size: 10px; }
.secondary { color: var(--text); background: var(--surface-soft); }
.danger { background: var(--red); }
</style>
