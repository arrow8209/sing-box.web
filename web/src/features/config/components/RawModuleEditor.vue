<script setup lang="ts">
import { computed, ref } from "vue";
import { useDraftStore } from "../../../stores/draft";
import { isSensitiveModule, topLevelKey } from "../modules";
import type { Draft } from "../../../api/types";

const props = defineProps<{ module: string; label?: string }>();

const editorName = computed(() => `${props.label ?? props.module} JSON`);

const draft = useDraftStore();
const revealed = ref(false);
let revealTimer: ReturnType<typeof setTimeout> | null = null;

// 描述符键 → 顶层配置键（general→log、routing→route），缓冲区按顶层键存储。
const topKey = topLevelKey(props.module);
const sensitive = isSensitiveModule(props.module);

function bufferOf(content: Draft["content"] | undefined) {
  return content?.editorBuffers[topKey];
}

const bufferText = computed(() => {
  if (draft.draft === null) {
    return "";
  }
  const buffer = bufferOf(draft.draft.content);
  if (buffer !== undefined) {
    return buffer.text;
  }
  // 与后端模块文件一致：以顶层键包装（如 {"log": {...}}）。
  const canonical = draft.draft.content.config[topKey];
  return JSON.stringify({ [topKey]: canonical ?? null }, null, 2);
});

const validJSON = computed(() => {
  try {
    JSON.parse(bufferText.value);
    return true;
  } catch {
    return false;
  }
});

const isReadOnly = computed(() => draft.status === "error" || draft.draft === null);

function onInput(event: Event): void {
  const text = (event.target as HTMLTextAreaElement).value;
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    draft.setEditorBuffer(topKey, text);
    return;
  }
  // 原始 JSON 以顶层键呈现（如 {"log": ...}），解析结果与现有规范配置顶层合并。
  const nextConfig = {
    ...(draft.draft?.content.config ?? {}),
    ...(parsed as Record<string, unknown>),
  };
  // 先更新规范配置，再同步有效已应用的缓冲区。
  draft.replaceConfig(nextConfig);
  draft.applyEditorBuffer(topKey, text);
}

function scheduleMask(): void {
  if (revealTimer !== null) {
    clearTimeout(revealTimer);
  }
  revealTimer = setTimeout(() => {
    revealed.value = false;
  }, 5 * 60 * 1000);
}

function reveal(): void {
  if (window.confirm("该模块包含敏感值，确认显示完整 JSON 并编辑？")) {
    revealed.value = true;
    scheduleMask();
  }
}

function onRevealBlur(): void {
  scheduleMask();
}

function showText(): string {
  if (sensitive && !revealed.value) {
    return "***REDACTED***";
  }
  return bufferText.value;
}
</script>

<template>
  <section class="raw-editor" :aria-label="`${props.module} JSON`">
    <div v-if="sensitive && !revealed" class="redacted-preview">
      <code class="redacted-text">{{ showText() }}</code>
      <button class="btn" name="显示完整 JSON 并编辑" type="button" @click="reveal">
        显示完整 JSON 并编辑
      </button>
    </div>
    <div v-else class="editor-pane">
      <textarea
        class="raw-textarea"
        :aria-label="editorName"
        :value="showText()"
        :disabled="isReadOnly"
        spellcheck="false"
        @input="onInput"
        @blur="onRevealBlur"
      ></textarea>
      <p class="buffer-state" :class="{ invalid: !validJSON }">
        {{ validJSON ? "JSON 有效" : "JSON 无效，尚未应用" }}
      </p>
      <button
        v-if="!validJSON && !isReadOnly"
        class="btn"
        name="放弃未应用 JSON"
        type="button"
        @click="draft.discardUnappliedBuffer(topKey)"
      >
        放弃未应用 JSON
      </button>
    </div>
  </section>
</template>

<style scoped>
.raw-editor {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.redacted-preview {
  display: flex;
  flex-direction: column;
  gap: 10px;
  align-items: flex-start;
}

.redacted-text {
  display: block;
  width: 100%;
  padding: 12px;
  border: 1px dashed var(--line-strong);
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
  color: var(--text-faint);
  font-family: ui-monospace, "SF Mono", monospace;
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-all;
}

.editor-pane {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.raw-textarea {
  min-height: 240px;
  resize: vertical;
  padding: 12px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  font-family: ui-monospace, "SF Mono", monospace;
  font-size: 13px;
  line-height: 1.5;
  white-space: pre;
  overflow: auto;
}

.buffer-state {
  margin: 0;
  font-size: 12px;
  color: var(--green);
}

.buffer-state.invalid {
  color: var(--orange);
}
</style>
