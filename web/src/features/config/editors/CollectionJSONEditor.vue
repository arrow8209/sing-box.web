<script setup lang="ts">
import { ref } from "vue";
import { useDraftStore } from "../../../stores/draft";

const props = defineProps<{
  module: string;
  modelValue: unknown[];
}>();

const emit = defineEmits<{ "update:modelValue": [value: unknown[]] }>();

const draft = useDraftStore();
const editingIndex = ref<number | null>(null);
const editingText = ref("");

const rows = () => props.modelValue;

function bufferKey(index: number): string {
  return `${props.module}/${index}`;
}

function rowBufferText(index: number): string {
  return draft.draft?.content.editorBuffers[bufferKey(index)]?.text ?? "";
}

function openJSON(index: number): void {
  editingIndex.value = index;
  const buffer = draft.draft?.content.editorBuffers[bufferKey(index)];
  editingText.value =
    buffer !== undefined ? buffer.text : JSON.stringify(rows()[index] ?? null, null, 2);
}

function commitRows(next: unknown[], remap?: Record<string, string>): void {
  emit("update:modelValue", next);
  const config = { ...(draft.draft?.content.config ?? {}) };
  config[props.module] = next;
  draft.replaceConfig(config, remap);
}

function onJSONInput(event: Event): void {
  const index = editingIndex.value;
  if (index === null) {
    return;
  }
  const text = (event.target as HTMLTextAreaElement).value;
  editingText.value = text;
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    draft.setEditorBuffer(bufferKey(index), text);
    return;
  }
  const next = [...rows()];
  next[index] = parsed;
  commitRows(next);
  draft.applyEditorBuffer(bufferKey(index), text);
}

function addRow(): void {
  commitRows([...rows(), {}]);
}

function removeRow(index: number): void {
  const next = rows().filter((_, rowIndex) => rowIndex !== index);
  const remap: Record<string, string> = {};
  for (let rowIndex = index + 1; rowIndex < rows().length; rowIndex++) {
    remap[bufferKey(rowIndex)] = bufferKey(rowIndex - 1);
  }
  delete draft.draft?.content.editorBuffers[bufferKey(index)];
  commitRows(next, remap);
  if (editingIndex.value === index) {
    editingIndex.value = null;
  }
}

function moveRow(index: number, direction: -1 | 1): void {
  const target = index + direction;
  if (target < 0 || target >= rows().length) {
    return;
  }
  const next = [...rows()];
  [next[index], next[target]] = [next[target], next[index]];
  const remap: Record<string, string> = {
    [bufferKey(index)]: bufferKey(target),
    [bufferKey(target)]: bufferKey(index),
  };
  commitRows(next, remap);
  if (editingIndex.value === index) {
    editingIndex.value = target;
  } else if (editingIndex.value === target) {
    editingIndex.value = index;
  }
}

function titleOf(index: number): string {
  const row = rows()[index] as Record<string, unknown> | undefined;
  if (row !== null && typeof row === "object" && row !== undefined) {
    if (typeof row.tag === "string") {
      return row.tag;
    }
    if (typeof row.type === "string") {
      return row.type;
    }
  }
  return `行 ${index + 1}`;
}
</script>

<template>
  <div class="collection-editor">
    <div v-for="(row, index) in rows()" :key="index" class="collection-row">
      <div class="collection-row-head">
        <span class="collection-title">{{ titleOf(index) }}</span>
        <div class="collection-actions">
          <button class="btn btn-sm" type="button" :name="`上移行 ${index}`" @click="moveRow(index, -1)">
            ↑
          </button>
          <button class="btn btn-sm" type="button" :name="`编辑 JSON ${index}`" @click="openJSON(index)">
            编辑 JSON
          </button>
          <button class="btn btn-sm" type="button" :name="`删除行 ${index}`" @click="removeRow(index)">
            删除
          </button>
        </div>
      </div>
      <textarea
        v-if="editingIndex === index"
        class="collection-textarea"
        :value="editingText"
        spellcheck="false"
        @input="onJSONInput"
      ></textarea>
      <pre v-else class="collection-preview">{{ rowBufferText(index) || JSON.stringify(row, null, 2) }}</pre>
    </div>
    <button class="btn" type="button" name="添加行" @click="addRow">添加行</button>
  </div>
</template>

<style scoped>
.collection-editor {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.collection-row {
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.collection-row-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.collection-title {
  font-weight: 600;
  color: var(--text);
  font-size: 13px;
}

.collection-actions {
  display: flex;
  gap: 6px;
}

.collection-textarea {
  min-height: 160px;
  resize: vertical;
  padding: 10px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  font-family: ui-monospace, "SF Mono", monospace;
  font-size: 12px;
  line-height: 1.5;
}

.collection-preview {
  margin: 0;
  padding: 10px;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
  color: var(--text-soft);
  font-size: 12px;
  overflow: auto;
  max-height: 180px;
}
</style>
