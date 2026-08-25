<script setup lang="ts">
import AppDialog from "@/components/AppDialog.vue";

defineProps<{ open: boolean; tag: string; busy?: boolean }>();
const emit = defineEmits<{ close: []; confirm: [] }>();
</script>

<template>
  <AppDialog class="inbound-delete-dialog" :open="open" title="删除入站配置" :description="`将从资源库删除 ${tag}`" :close-disabled="busy" @close="emit('close')">
    <p class="warning">删除后无法恢复，但已经导入各自定义配置的独立副本不会被修改或删除。</p>
    <template #footer><button class="secondary-button" type="button" :disabled="busy" @click="emit('close')">取消</button><button class="danger-button" type="button" :disabled="busy" @click="emit('confirm')">{{ busy ? "正在删除" : "确认删除" }}</button></template>
  </AppDialog>
</template>

<style scoped>
:global(.inbound-delete-dialog) { width: min(460px, calc(100vw - 32px)); }.warning { margin: 0; padding: 12px; border: 1px solid color-mix(in srgb, var(--red) 18%, var(--line)); border-radius: 10px; color: var(--text-soft); background: var(--red-soft); font-size: 10px; }.secondary-button, .danger-button { min-height: 36px; padding: 7px 11px; border: 1px solid var(--line); font-size: 10px; }.secondary-button { color: var(--text-soft); background: var(--surface); }.danger-button { color: #fff; border-color: var(--red); background: var(--red); }
@media (max-width: 680px) { :global(.inbound-delete-dialog) { width: 100%; } }
</style>
