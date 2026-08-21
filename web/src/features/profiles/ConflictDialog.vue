<script setup lang="ts">
defineProps<{ open: boolean; latestRevision?: number; busy?: boolean }>();
const emit = defineEmits<{ load: []; overwrite: [] }>();
</script>

<template>
  <section v-if="open" class="conflict" role="alert">
    <h2>草稿版本冲突</h2><p>另一个页面已保存服务器草稿（修订 {{ latestRevision ?? "未知" }}）。本地修改仍保留，请明确选择处理方式。</p>
    <div><button class="secondary" :disabled="busy" @click="emit('load')">加载服务器版本</button><button :disabled="busy" @click="emit('overwrite')">用本地内容覆盖</button></div>
  </section>
</template>

<style scoped>
.conflict { padding: 16px; border: 1px solid color-mix(in srgb, var(--red) 45%, var(--line)); border-radius: var(--radius); background: var(--red-soft); }.conflict h2 { margin: 0; font-size: 16px; }.conflict p { color: var(--text-soft); }.conflict div { display: flex; flex-wrap: wrap; gap: 8px; }.secondary { color: var(--text); background: var(--surface); }
</style>
