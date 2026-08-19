<script setup lang="ts">
import { computed, ref } from "vue";
import { useDraftStore } from "../../../stores/draft";
import { collectReferences } from "../references";

const draft = useDraftStore();

interface OutboundLike {
  type: string;
  tag: string;
  outbounds?: string[];
  [key: string]: unknown;
}

const defaults: Record<string, OutboundLike> = {
  direct: { type: "direct", tag: "direct" },
  block: { type: "block", tag: "block" },
  selector: { type: "selector", tag: "selector", outbounds: [] },
  urltest: {
    type: "urltest",
    tag: "urltest",
    outbounds: [],
    url: "https://www.gstatic.com/generate_204",
    interval: "3m",
    tolerance: 150,
  },
};

const rows = computed(
  () => (draft.draft?.content.config.outbounds as OutboundLike[]) ?? [],
);

const references = computed(() =>
  collectReferences((draft.draft?.content.config as Record<string, unknown>) ?? {}),
);

const editingIndex = ref<number | null>(null);
const editingText = ref("");

const structuredTypes = new Set(["direct", "block", "selector", "urltest"]);

function commit(next: OutboundLike[]): void {
  const config = { ...(draft.draft?.content.config ?? {}) };
  config.outbounds = next;
  draft.replaceConfig(config);
}

function updateField(index: number, path: string[], value: unknown): void {
  const next = [...rows.value];
  const current = { ...next[index] };
  let container: Record<string, unknown> = current;
  for (let depth = 0; depth < path.length - 1; depth++) {
    const segment = path[depth];
    const existing = container[segment];
    container[segment] =
      existing !== null && typeof existing === "object"
        ? { ...(existing as Record<string, unknown>) }
        : {};
    container = container[segment] as Record<string, unknown>;
  }
  const last = path[path.length - 1];
  if (value === undefined) {
    delete container[last];
  } else {
    container[last] = value;
  }
  next[index] = current;
  commit(next);
}

function onText(index: number, path: string[], event: Event): void {
  const value = (event.target as HTMLInputElement).value;
  updateField(index, path, value === "" ? undefined : value);
}

function onCheckbox(index: number, path: string[], event: Event): void {
  updateField(index, path, (event.target as HTMLInputElement).checked);
}

function onNumber(index: number, path: string[], event: Event): void {
  const raw = (event.target as HTMLInputElement).value;
  updateField(index, path, raw === "" ? undefined : Number(raw));
}

function onTagList(index: number, path: string[], event: Event): void {
  const raw = (event.target as HTMLInputElement).value;
  const list = raw === "" ? [] : raw.split(",").map((entry) => entry.trim()).filter(Boolean);
  updateField(index, path, list);
}

function addOutbound(type: string): void {
  commit([...rows.value, { ...defaults[type] }]);
}

function removeOutbound(index: number): void {
  commit(rows.value.filter((_, rowIndex) => rowIndex !== index));
}

function openJSON(index: number): void {
  editingIndex.value = index;
  editingText.value = JSON.stringify(rows.value[index] ?? null, null, 2);
}

function onJSONInput(): void {
  const index = editingIndex.value;
  if (index === null) {
    return;
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(editingText.value);
  } catch {
    draft.setEditorBuffer(`outbounds/${index}`, editingText.value);
    return;
  }
  const next = [...rows.value];
  next[index] = parsed as OutboundLike;
  commit(next);
  draft.applyEditorBuffer(`outbounds/${index}`, editingText.value);
}

function missingReferences(): string[] {
  return references.value.missing.map((entry) => entry.tag);
}
</script>

<template>
  <div class="outbounds-editor">
    <p v-if="missingReferences().length > 0" class="reference-warning" role="alert">
      缺失引用：{{ missingReferences().join("、") }}
    </p>
    <p v-if="references.duplicates.length > 0" class="reference-warning">
      标签重复：{{ references.duplicates.join("、") }}
    </p>

    <div v-for="(row, index) in rows" :key="index" class="outbound-card">
      <div class="outbound-head">
        <span class="outbound-title">{{ row.tag || row.type }}</span>
        <div class="outbound-actions">
          <button class="btn btn-sm" type="button" name="编辑 JSON" @click="openJSON(index)">
            编辑 JSON
          </button>
          <button class="btn btn-sm" type="button" :name="`删除出站 ${index}`" @click="removeOutbound(index)">
            删除
          </button>
        </div>
      </div>

      <textarea
        v-if="editingIndex === index"
        class="outbound-json"
        v-model="editingText"
        spellcheck="false"
        @input="onJSONInput"
      ></textarea>

      <template v-else-if="structuredTypes.has(row.type)">
        <label class="form-row">
          <span class="form-label">标签</span>
          <input
            aria-label="出站标签"
            type="text"
            :value="row.tag ?? ''"
            @input="onText(index, ['tag'], $event)"
          />
        </label>

        <template v-if="row.type === 'selector' || row.type === 'urltest'">
          <label class="form-row">
            <span class="form-label">出站引用</span>
            <input
              aria-label="选择器出站引用"
              type="text"
              :value="(row.outbounds ?? []).join(',')"
              @input="onTagList(index, ['outbounds'], $event)"
            />
          </label>
          <label class="form-row">
            <span class="form-label">中断已有连接</span>
            <input
              aria-label="中断已有连接"
              type="checkbox"
              :checked="row.interrupt_exist_connections === true"
              @change="onCheckbox(index, ['interrupt_exist_connections'], $event)"
            />
          </label>
        </template>

        <template v-if="row.type === 'urltest'">
          <label class="form-row">
            <span class="form-label">测试 URL</span>
            <input
              aria-label="测试 URL"
              type="text"
              :value="(row.url as string) ?? ''"
              @input="onText(index, ['url'], $event)"
            />
          </label>
          <label class="form-row">
            <span class="form-label">测试间隔</span>
            <input
              aria-label="测试间隔"
              type="text"
              :value="(row.interval as string) ?? ''"
              @input="onText(index, ['interval'], $event)"
            />
          </label>
          <label class="form-row">
            <span class="form-label">容忍度</span>
            <input
              aria-label="容忍度"
              type="number"
              :value="(row.tolerance as number) ?? ''"
              @input="onNumber(index, ['tolerance'], $event)"
            />
          </label>
        </template>
      </template>
      <pre v-else class="outbound-preview">{{ JSON.stringify(row, null, 2) }}</pre>
    </div>

    <div class="outbound-add">
      <button class="btn" type="button" name="添加出站" @click="addOutbound('direct')">
        添加 direct 出站
      </button>
      <button class="btn" type="button" @click="addOutbound('block')">添加 block 出站</button>
      <button class="btn" type="button" @click="addOutbound('selector')">添加 selector 出站</button>
      <button class="btn" type="button" @click="addOutbound('urltest')">添加 urltest 出站</button>
    </div>
  </div>
</template>

<style scoped>
.outbounds-editor {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.reference-warning {
  margin: 0;
  padding: 8px 12px;
  border: 1px solid var(--orange);
  border-radius: var(--radius-sm);
  background: var(--orange-soft);
  color: var(--orange);
  font-size: 13px;
}

.outbound-card {
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.outbound-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.outbound-title {
  font-weight: 600;
  color: var(--text);
  font-size: 13px;
}

.outbound-actions,
.outbound-add {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
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
.form-row input[type="number"] {
  flex: 1;
  min-width: 0;
  padding: 8px 10px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  font: inherit;
}

.outbound-json {
  min-height: 160px;
  resize: vertical;
  padding: 10px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  font-family: ui-monospace, "SF Mono", monospace;
  font-size: 12px;
}

.outbound-preview {
  margin: 0;
  padding: 10px;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
  color: var(--text-soft);
  font-size: 12px;
  overflow: auto;
}
</style>
