<script setup lang="ts">
import { computed } from "vue";
import { useDraftStore } from "../../../stores/draft";
import { setAtPath } from "../model";

const draft = useDraftStore();

interface ServerEntry {
  tag?: string;
  address?: string;
}

interface RuleEntry {
  query?: string[];
  server?: string;
}

const dnsConfig = computed(() => (draft.draft?.content.config.dns as Record<string, unknown>) ?? {});

const servers = computed(() => (dnsConfig.value.servers as ServerEntry[]) ?? []);
const rules = computed(() => (dnsConfig.value.rules as RuleEntry[]) ?? []);

const disableCache = computed(() => dnsConfig.value.disable_cache === true);
const disableExpire = computed(() => dnsConfig.value.disable_expire === true);
const independentCache = computed(() => dnsConfig.value.independent_cache === true);
const finalServer = computed(() => (dnsConfig.value.final as string) ?? "");

function update(path: string[], value: unknown): void {
  const config = draft.draft?.content.config ?? {};
  draft.replaceConfig(setAtPath(config, ["dns", ...path], value));
}

function onServerAddress(index: number, event: Event): void {
  const next = servers.value.map((server, serverIndex) =>
    serverIndex === index
      ? { ...server, address: (event.target as HTMLInputElement).value }
      : server,
  );
  update(["servers"], next);
}

function addServer(): void {
  update(["servers"], [...servers.value, { address: "" }]);
}

function removeServer(index: number): void {
  update(["servers"], servers.value.filter((_, serverIndex) => serverIndex !== index));
}

function moveServer(index: number, direction: -1 | 1): void {
  const target = index + direction;
  if (target < 0 || target >= servers.value.length) {
    return;
  }
  const next = [...servers.value];
  [next[index], next[target]] = [next[target], next[index]];
  update(["servers"], next);
}

function onRuleQuery(index: number, event: Event): void {
  const query = (event.target as HTMLInputElement).value;
  const next = rules.value.map((rule, ruleIndex) =>
    ruleIndex === index
      ? { ...rule, query: query === "" ? [] : query.split(/\s*,\s*/).filter(Boolean) }
      : rule,
  );
  update(["rules"], next);
}

function onRuleServer(index: number, event: Event): void {
  const next = rules.value.map((rule, ruleIndex) =>
    ruleIndex === index ? { ...rule, server: (event.target as HTMLInputElement).value } : rule,
  );
  update(["rules"], next);
}

function moveRule(index: number, direction: -1 | 1): void {
  const target = index + direction;
  if (target < 0 || target >= rules.value.length) {
    return;
  }
  const next = [...rules.value];
  [next[index], next[target]] = [next[target], next[index]];
  update(["rules"], next);
}

function onCheckbox(path: string[], event: Event): void {
  update(path, (event.target as HTMLInputElement).checked);
}
</script>

<template>
  <form class="dns-editor" @submit.prevent>
    <fieldset class="editor-section">
      <legend class="section-title">服务器</legend>
      <div v-for="(server, index) in servers" :key="index" class="server-row">
        <input
          aria-label="服务器地址"
          type="text"
          :value="server.address ?? ''"
          @input="onServerAddress(index, $event)"
        />
        <button class="btn btn-sm" type="button" :name="`上移服务器 ${index}`" @click="moveServer(index, -1)">
          ↑
        </button>
        <button class="btn btn-sm" type="button" :name="`删除服务器 ${index}`" @click="removeServer(index)">
          删除
        </button>
      </div>
      <button class="btn" type="button" name="添加服务器" @click="addServer">添加服务器</button>
    </fieldset>

    <fieldset class="editor-section">
      <legend class="section-title">规则</legend>
      <div v-for="(rule, index) in rules" :key="index" class="server-row">
        <input
          aria-label="规则查询"
          type="text"
          :value="(rule.query ?? []).join(',')"
          @input="onRuleQuery(index, $event)"
        />
        <input
          aria-label="规则服务器"
          type="text"
          :value="rule.server ?? ''"
          @input="onRuleServer(index, $event)"
        />
        <button class="btn btn-sm" type="button" :name="`上移规则 ${index}`" @click="moveRule(index, -1)">
          ↑
        </button>
      </div>
    </fieldset>

    <fieldset class="editor-section">
      <legend class="section-title">行为</legend>
      <label class="form-row">
        <span class="form-label">禁用缓存</span>
        <input aria-label="禁用缓存" type="checkbox" :checked="disableCache" @change="onCheckbox(['disable_cache'], $event)" />
      </label>
      <label class="form-row">
        <span class="form-label">禁用过期</span>
        <input aria-label="禁用过期" type="checkbox" :checked="disableExpire" @change="onCheckbox(['disable_expire'], $event)" />
      </label>
      <label class="form-row">
        <span class="form-label">独立缓存</span>
        <input aria-label="独立缓存" type="checkbox" :checked="independentCache" @change="onCheckbox(['independent_cache'], $event)" />
      </label>
      <label class="form-row">
        <span class="form-label">最终服务器</span>
        <input
          aria-label="最终服务器"
          type="text"
          :value="finalServer"
          @input="update(['final'], ($event.target as HTMLInputElement).value)"
        />
      </label>
    </fieldset>
  </form>
</template>

<style scoped>
.dns-editor {
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-width: 640px;
}

.editor-section {
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.section-title {
  padding: 0 6px;
  color: var(--text-soft);
  font-size: 13px;
}

.server-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.server-row input[type="text"] {
  flex: 1;
  min-width: 0;
  padding: 8px 10px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  font: inherit;
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

.form-row input[type="text"] {
  flex: 1;
  padding: 8px 10px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  font: inherit;
}
</style>
