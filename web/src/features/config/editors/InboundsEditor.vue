<script setup lang="ts">
import { computed, ref } from "vue";
import { useDraftStore } from "../../../stores/draft";

const draft = useDraftStore();

interface InboundLike {
  type: string;
  tag: string;
  [key: string]: unknown;
}

const defaults = {
  mixed: { type: "mixed", tag: "mixed-in", listen: "127.0.0.1", listen_port: 7890 },
  tun: {
    type: "tun",
    tag: "tun-in",
    interface_name: "singtun0",
    address: ["172.19.0.1/30"],
    auto_route: true,
    strict_route: true,
  },
} as const;

const rows = computed(
  () => (draft.draft?.content.config.inbounds as InboundLike[]) ?? [],
);

const editingIndex = ref<number | null>(null);
const editingText = ref("");

const structuredTypes = new Set(["mixed", "tun"]);

function commit(next: InboundLike[]): void {
  const config = { ...(draft.draft?.content.config ?? {}) };
  config.inbounds = next;
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

function addInbound(type: "mixed" | "tun"): void {
  commit([...rows.value, { ...defaults[type] }]);
}

function removeInbound(index: number): void {
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
    draft.setEditorBuffer(`inbounds/${index}`, editingText.value);
    return;
  }
  const next = [...rows.value];
  next[index] = parsed as InboundLike;
  commit(next);
  draft.applyEditorBuffer(`inbounds/${index}`, editingText.value);
}

function titleOf(row: InboundLike): string {
  return typeof row.tag === "string" ? row.tag : row.type;
}
</script>

<template>
  <div class="inbounds-editor">
    <div v-for="(row, index) in rows" :key="index" class="inbound-card">
      <div class="inbound-head">
        <span class="inbound-title">{{ titleOf(row) }}</span>
        <div class="inbound-actions">
          <button class="btn btn-sm" type="button" name="编辑 JSON" @click="openJSON(index)">
            编辑 JSON
          </button>
          <button class="btn btn-sm" type="button" :name="`删除入站 ${index}`" @click="removeInbound(index)">
            删除
          </button>
        </div>
      </div>

      <textarea
        v-if="editingIndex === index"
        class="inbound-json"
        v-model="editingText"
        spellcheck="false"
        @input="onJSONInput"
      ></textarea>

      <template v-else-if="structuredTypes.has(row.type)">
        <label class="form-row">
          <span class="form-label">标签</span>
          <input
            aria-label="入站标签"
            type="text"
            :value="row.tag ?? ''"
            @input="onText(index, ['tag'], $event)"
          />
        </label>

        <template v-if="row.type === 'mixed'">
          <label class="form-row">
            <span class="form-label">监听地址</span>
            <input
              aria-label="监听地址"
              type="text"
              :value="(row.listen as string) ?? ''"
              @input="onText(index, ['listen'], $event)"
            />
          </label>
          <label class="form-row">
            <span class="form-label">监听端口</span>
            <input
              aria-label="监听端口"
              type="number"
              :value="(row.listen_port as number) ?? ''"
              @input="onNumber(index, ['listen_port'], $event)"
            />
          </label>
          <label class="form-row">
            <span class="form-label">TCP Fast Open</span>
            <input
              aria-label="TCP Fast Open"
              type="checkbox"
              :checked="row.tcp_fast_open === true"
              @change="onCheckbox(index, ['tcp_fast_open'], $event)"
            />
          </label>
          <label class="form-row">
            <span class="form-label">TCP Multi Path</span>
            <input
              aria-label="TCP Multi Path"
              type="checkbox"
              :checked="row.tcp_multi_path === true"
              @change="onCheckbox(index, ['tcp_multi_path'], $event)"
            />
          </label>
          <label class="form-row">
            <span class="form-label">UDP Fragment</span>
            <input
              aria-label="UDP Fragment"
              type="checkbox"
              :checked="row.udp_fragment === true"
              @change="onCheckbox(index, ['udp_fragment'], $event)"
            />
          </label>
        </template>

        <template v-else-if="row.type === 'tun'">
          <label class="form-row">
            <span class="form-label">接口名称</span>
            <input
              aria-label="接口名称"
              type="text"
              :value="(row.interface_name as string) ?? ''"
              @input="onText(index, ['interface_name'], $event)"
            />
          </label>
          <label class="form-row">
            <span class="form-label">MTU</span>
            <input
              aria-label="MTU"
              type="number"
              :value="(row.mtu as number) ?? ''"
              @input="onNumber(index, ['mtu'], $event)"
            />
          </label>
          <label class="form-row">
            <span class="form-label">协议栈</span>
            <select
              aria-label="协议栈"
              :value="(row.stack as string) ?? 'system'"
              @change="onText(index, ['stack'], $event)"
            >
              <option value="system">system</option>
              <option value="gvisor">gvisor</option>
              <option value="mixed">mixed</option>
            </select>
          </label>
          <label class="form-row">
            <span class="form-label">自动路由</span>
            <input
              aria-label="自动路由"
              type="checkbox"
              :checked="row.auto_route === true"
              @change="onCheckbox(index, ['auto_route'], $event)"
            />
          </label>
          <label class="form-row">
            <span class="form-label">严格路由</span>
            <input
              aria-label="严格路由"
              type="checkbox"
              :checked="row.strict_route === true"
              @change="onCheckbox(index, ['strict_route'], $event)"
            />
          </label>
          <label class="form-row">
            <span class="form-label">端点独立 NAT</span>
            <input
              aria-label="端点独立 NAT"
              type="checkbox"
              :checked="row.endpoint_independent_nat === true"
              @change="onCheckbox(index, ['endpoint_independent_nat'], $event)"
            />
          </label>
        </template>
      </template>
      <pre v-else class="inbound-preview">{{ JSON.stringify(row, null, 2) }}</pre>
    </div>

    <div class="inbound-add">
      <button class="btn" type="button" name="添加入站" @click="addInbound('mixed')">添加入站（mixed）</button>
      <button class="btn" type="button" @click="addInbound('tun')">添加入站（tun）</button>
    </div>
  </div>
</template>

<style scoped>
.inbounds-editor {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.inbound-card {
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.inbound-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.inbound-title {
  font-weight: 600;
  color: var(--text);
  font-size: 13px;
}

.inbound-actions,
.inbound-add {
  display: flex;
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
.form-row input[type="number"],
.form-row select {
  flex: 1;
  min-width: 0;
  padding: 8px 10px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  font: inherit;
}

.inbound-json {
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

.inbound-preview {
  margin: 0;
  padding: 10px;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
  color: var(--text-soft);
  font-size: 12px;
  overflow: auto;
}
</style>
