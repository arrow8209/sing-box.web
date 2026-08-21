<script setup lang="ts">
import { ref, watch } from "vue";
const props = defineProps<{ open: boolean; title: string; text: string; error?: string }>();
const emit = defineEmits<{ close: []; save: [text: string]; buffer: [text: string] }>();
const buffer = ref(props.text);
watch(() => [props.open, props.text] as const, ([open, text]) => { if (open) buffer.value = text; });
</script>
<template>
  <div v-if="open" class="backdrop" role="presentation"><section class="dialog" role="dialog" aria-modal="true" :aria-label="title"><header><h2>{{ title }}</h2><button type="button" class="plain" aria-label="关闭" @click="emit('close')">×</button></header><textarea v-model="buffer" aria-label="模块 JSON" spellcheck="false" @input="emit('buffer', buffer)" /><p v-if="error" role="alert">{{ error }}</p><footer><button type="button" class="secondary" @click="emit('close')">取消</button><button type="button" data-save-module @click="emit('save', buffer)">保存 JSON</button></footer></section></div>
</template>
<style scoped>
.backdrop { position: fixed; z-index: 30; inset: 0; display: grid; place-items: center; padding: 18px; background: rgb(0 0 0 / .45); }.dialog { width: min(780px, 100%); padding: 18px; border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-lg); }.dialog header, footer { display: flex; align-items: center; justify-content: space-between; gap: 10px; }.dialog h2 { margin: 0; font-size: 18px; }.plain { min-height: 30px; width: 32px; background: transparent; color: var(--text); font-size: 22px; }.secondary { color: var(--text); background: var(--surface-soft); } textarea { width: 100%; min-height: min(55vh, 460px); margin: 16px 0; padding: 12px; border: 1px solid var(--line); border-radius: var(--radius-sm); background: var(--surface-soft); color: var(--text); font: 13px/1.55 ui-monospace, SFMono-Regular, Consolas, monospace; resize: vertical; }.dialog p { color: var(--red); }
</style>
