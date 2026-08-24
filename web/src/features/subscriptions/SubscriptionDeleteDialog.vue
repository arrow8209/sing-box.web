<script setup lang="ts">
import AppDialog from "@/components/AppDialog.vue";

defineProps<{ open: boolean; name: string; busy?: boolean }>();
const emit = defineEmits<{ close: []; confirm: [] }>();
</script>

<template>
  <AppDialog class="subscription-delete-dialog" :open="open" title="删除 JSON 订阅" :description="`将永久删除 ${name} 的全部节点`" :close-disabled="busy" @close="emit('close')">
    <p class="warning">删除后无法恢复，已有配置草稿不会自动修改。请确认不再需要这组节点。</p>
    <template #footer>
      <button class="secondary-button" type="button" :disabled="busy" @click="emit('close')">取消</button>
      <button class="danger-button" type="button" :disabled="busy" @click="emit('confirm')">{{ busy ? "正在删除" : "确认删除" }}</button>
    </template>
  </AppDialog>
</template>

<style scoped>
:global(.subscription-delete-dialog) { width: min(460px, calc(100vw - 32px)); }.warning { margin: 0; padding: 12px; border: 1px solid color-mix(in srgb, var(--red) 18%, var(--line)); border-radius: 10px; color: var(--text-soft); background: var(--red-soft); font-size: 10px; }.secondary-button, .danger-button { min-height: 36px; padding: 7px 11px; border: 1px solid var(--line); font-size: 10px; }.secondary-button { color: var(--text-soft); background: var(--surface); }.danger-button { color: #fff; border-color: var(--red); background: var(--red); }
@media (max-width: 680px) { :global(.subscription-delete-dialog) { width: 100%; } }
</style>
