<script setup lang="ts">
defineProps<{ errors: Record<string, string> }>();
</script>
<template>
  <section v-if="Object.keys(errors).length" class="errors" aria-live="polite"><h2>待修复的配置引用</h2><ul><li v-for="(message, module) in errors" :key="module"><strong>{{ module }}</strong>：{{ message }}</li></ul><p>无效文本已保留并会自动保存，但在修复前不能应用配置。</p></section>
</template>
<style scoped>
.errors { margin-top: 16px; padding: 14px; border: 1px solid color-mix(in srgb, var(--red) 40%, var(--line)); border-radius: var(--radius); color: var(--red); background: var(--red-soft); }.errors h2 { margin: 0; font-size: 16px; }.errors ul { margin: 8px 0; padding-left: 20px; }.errors p { margin-bottom: 0; }
</style>
