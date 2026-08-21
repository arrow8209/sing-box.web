<script setup lang="ts">
import { ref, watch } from "vue";
import AppDialog from "@/components/AppDialog.vue";
import JsonEditor from "@/components/JsonEditor.vue";
const props = defineProps<{ open: boolean; title: string; text: string; error?: string }>();
const emit = defineEmits<{ close: []; save: [text: string]; buffer: [text: string] }>();
const buffer = ref(props.text);
watch(() => [props.open, props.text] as const, ([open, text]) => { if (open) buffer.value = text; });
</script>
<template>
  <AppDialog :open="open" :title="title" initial-focus="textarea" @close="emit('close')">
    <JsonEditor v-model="buffer" label="模块 JSON" :error="error" @update:model-value="emit('buffer', buffer)" />
    <template #footer><button type="button" class="secondary" @click="emit('close')">取消</button><button type="button" data-save-module @click="emit('save', buffer)">保存 JSON</button></template>
  </AppDialog>
</template>
<style scoped>
.json-editor textarea { min-height: min(55vh, 460px); }
</style>
