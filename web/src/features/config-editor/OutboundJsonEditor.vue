<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { format, applyEdits } from "jsonc-parser";
import { validateModuleShape } from "./json";
import JsonEditor from "@/components/JsonEditor.vue";
const props = defineProps<{ text: string; error?: string }>();
const emit = defineEmits<{ save: [text: string]; buffer: [text: string] }>();
const text = ref(props.text);
watch(() => props.text, (value) => { text.value = value; });
const validation = computed(() => validateModuleShape(text.value, "outbounds", "array"));
function pretty(): void { if (!validation.value) text.value = applyEdits(text.value, format(text.value, undefined, { insertSpaces: true, tabSize: 2, eol: "\n" })); }
function save(): void { if (!validation.value) emit("save", text.value); }
</script>
<template>
  <section class="outbound-editor"><h2>手工出站节点</h2><p>手工粘贴或编辑 sing-box <code>outbounds</code> JSON。仅接受数组；内容按原值显示和保存。</p><JsonEditor v-model="text" label="出站 JSON" data-outbound-json :error="validation?.message ?? error" @update:model-value="emit('buffer', text)" /><div><button type="button" class="secondary" @click="pretty">格式化</button><button type="button" data-save-module :disabled="Boolean(validation)" @click="save">保存出站 JSON</button></div></section>
</template>
<style scoped>
.outbound-editor { padding: 18px; }.outbound-editor h2 { margin: 0; }.outbound-editor p { color: var(--text-soft); }.outbound-editor div { display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; }
</style>
