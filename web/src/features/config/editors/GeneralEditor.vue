<script setup lang="ts">
import { computed } from "vue";
import { useDraftStore } from "../../../stores/draft";
import { getAtPath, setAtPath } from "../model";

const draft = useDraftStore();

const logConfig = computed(
  () => (draft.draft?.content.config.log as Record<string, unknown>) ?? {},
);

const disabled = computed(() => logConfig.value.disabled === true);
const level = computed(() => (logConfig.value.level as string) ?? "info");
const output = computed(() => (logConfig.value.output as string) ?? "");
const timestamp = computed(() => logConfig.value.timestamp === true);

function update(path: string[], value: unknown): void {
  const config = draft.draft?.content.config ?? {};
  draft.replaceConfig(setAtPath(config, ["log", ...path], value));
}

function onDisabled(event: Event): void {
  update(["disabled"], (event.target as HTMLInputElement).checked);
}

function onLevel(event: Event): void {
  update(["level"], (event.target as HTMLSelectElement).value);
}

function onOutput(event: Event): void {
  const value = (event.target as HTMLInputElement).value;
  update(["output"], value === "" ? undefined : value);
}

function onTimestamp(event: Event): void {
  update(["timestamp"], (event.target as HTMLInputElement).checked);
}

function ensureBase(): Record<string, unknown> {
  return getAtPath(draft.draft?.content.config, ["log"]) as Record<string, unknown> | undefined ?? {};
}
void ensureBase;
</script>

<template>
  <form class="general-editor" @submit.prevent>
    <label class="form-row">
      <span class="form-label">禁用日志</span>
      <input
        aria-label="禁用日志"
        type="checkbox"
        :checked="disabled"
        @change="onDisabled"
      />
    </label>
    <label class="form-row">
      <span class="form-label">日志级别</span>
      <select aria-label="日志级别" :value="level" @change="onLevel">
        <option value="trace">trace</option>
        <option value="debug">debug</option>
        <option value="info">info</option>
        <option value="warn">warn</option>
        <option value="error">error</option>
        <option value="fatal">fatal</option>
        <option value="panic">panic</option>
      </select>
    </label>
    <label class="form-row">
      <span class="form-label">日志输出路径</span>
      <input
        aria-label="日志输出路径"
        type="text"
        :value="output"
        placeholder="留空使用标准输出"
        @input="onOutput"
      />
    </label>
    <label class="form-row">
      <span class="form-label">日志时间戳</span>
      <input
        aria-label="日志时间戳"
        type="checkbox"
        :checked="timestamp"
        @change="onTimestamp"
      />
    </label>
  </form>
</template>

<style scoped>
.general-editor {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-width: 520px;
}

.form-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.form-label {
  flex: 0 0 120px;
  color: var(--text-soft);
  font-size: 13px;
}

.form-row input[type="text"],
.form-row select {
  flex: 1;
  padding: 8px 10px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  font: inherit;
}
</style>
