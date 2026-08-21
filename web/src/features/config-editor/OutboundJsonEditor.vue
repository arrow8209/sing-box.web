<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { format, applyEdits } from "jsonc-parser";
import { validateModuleShape } from "./json";
const props = defineProps<{ text: string; error?: string }>();
const emit = defineEmits<{ save: [text: string]; buffer: [text: string] }>();
const text = ref(props.text);
watch(() => props.text, (value) => { text.value = value; });
const validation = computed(() => validateModuleShape(text.value, "outbounds", "array"));
function pretty(): void { if (!validation.value) text.value = applyEdits(text.value, format(text.value, undefined, { insertSpaces: true, tabSize: 2, eol: "\n" })); }
function save(): void { if (!validation.value) emit("save", text.value); }
</script>
<template>
  <section class="outbound-editor"><h2>手工出站节点</h2><p>手工粘贴或编辑 sing-box <code>outbounds</code> JSON。仅接受数组；内容按原值显示和保存。</p><textarea v-model="text" data-outbound-json aria-label="出站 JSON" spellcheck="false" @input="emit('buffer', text)" /><p v-if="validation || error" class="error" role="alert">{{ validation?.message ?? error }}</p><div><button type="button" class="secondary" @click="pretty">格式化</button><button type="button" data-save-module :disabled="Boolean(validation)" @click="save">保存出站 JSON</button></div></section>
</template>
<style scoped>
.outbound-editor { padding: 18px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); }.outbound-editor h2 { margin: 0; }.outbound-editor p { color: var(--text-soft); }.outbound-editor textarea { width: 100%; min-height: 330px; padding: 12px; border: 1px solid var(--line); border-radius: var(--radius-sm); background: var(--surface-soft); color: var(--text); font: 13px/1.55 ui-monospace, SFMono-Regular, Consolas, monospace; }.outbound-editor div { display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; }.outbound-editor .secondary { color: var(--text); background: var(--surface-soft); }.outbound-editor .error { color: var(--red); }
</style>
