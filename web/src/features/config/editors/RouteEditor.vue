<script setup lang="ts">
import { computed, ref } from "vue";
import { useDraftStore } from "../../../stores/draft";

const draft = useDraftStore();

interface RuleLike {
  action?: string;
  outbound?: string;
  invert?: boolean;
  domain_suffix?: string[];
  [key: string]: unknown;
}

interface RuleSetLike {
  tag?: string;
  type?: string;
  format?: string;
  url?: string;
  download_detour?: string;
  update_interval?: string;
  [key: string]: unknown;
}

const routeConfig = computed(
  () => (draft.draft?.content.config.route as Record<string, unknown>) ?? {},
);

const rules = computed(() => (routeConfig.value.rules as RuleLike[]) ?? []);
const ruleSets = computed(() => (routeConfig.value.rule_set as RuleSetLike[]) ?? []);

const editingRuleIndex = ref<number | null>(null);
const editingRuleText = ref("");

function commitRoute(next: Record<string, unknown>): void {
  const config = { ...(draft.draft?.content.config ?? {}) };
  config.route = next;
  draft.replaceConfig(config);
}

function updateRouteField(path: string[], value: unknown): void {
  const next = { ...routeConfig.value };
  let container: Record<string, unknown> = next;
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
  commitRoute(next);
}

function onText(path: string[], event: Event): void {
  const value = (event.target as HTMLInputElement).value;
  updateRouteField(path, value === "" ? undefined : value);
}

function onCheckbox(path: string[], event: Event): void {
  updateRouteField(path, (event.target as HTMLInputElement).checked);
}

function updateRule(index: number, path: string[], value: unknown): void {
  const nextRules = [...rules.value];
  const current = { ...nextRules[index] };
  const last = path[path.length - 1];
  if (value === undefined) {
    delete current[last];
  } else {
    current[last] = value;
  }
  nextRules[index] = current;
  updateRouteField(["rules"], nextRules);
}

function onRuleText(index: number, path: string[], event: Event): void {
  const value = (event.target as HTMLInputElement).value;
  updateRule(index, path, value === "" ? undefined : value);
}

function onRuleList(index: number, path: string[], event: Event): void {
  const raw = (event.target as HTMLInputElement).value;
  updateRule(index, path, raw === "" ? undefined : raw.split(",").map((entry) => entry.trim()));
}

function onRuleSelect(index: number, path: string[], event: Event): void {
  updateRule(index, path, (event.target as HTMLSelectElement).value);
}

function onRuleCheckbox(index: number, path: string[], event: Event): void {
  updateRule(index, path, (event.target as HTMLInputElement).checked);
}

function addRule(): void {
  updateRouteField(["rules"], [...rules.value, { action: "route" }]);
}

function removeRule(index: number): void {
  updateRouteField(["rules"], rules.value.filter((_, ruleIndex) => ruleIndex !== index));
}

function moveRule(index: number, direction: -1 | 1): void {
  const target = index + direction;
  if (target < 0 || target >= rules.value.length) {
    return;
  }
  const next = [...rules.value];
  [next[index], next[target]] = [next[target], next[index]];
  updateRouteField(["rules"], next);
}

function openRuleJSON(index: number): void {
  editingRuleIndex.value = index;
  editingRuleText.value = JSON.stringify(rules.value[index] ?? null, null, 2);
}

function onRuleJSONInput(): void {
  const index = editingRuleIndex.value;
  if (index === null) {
    return;
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(editingRuleText.value);
  } catch {
    draft.setEditorBuffer(`route/rules/${index}`, editingRuleText.value);
    return;
  }
  const nextRules = [...rules.value];
  nextRules[index] = parsed as RuleLike;
  updateRouteField(["rules"], nextRules);
  draft.applyEditorBuffer(`route/rules/${index}`, editingRuleText.value);
}

function updateRuleSet(index: number, path: string[], value: unknown): void {
  const nextSets = [...ruleSets.value];
  const current = { ...nextSets[index] };
  const last = path[path.length - 1];
  if (value === undefined) {
    delete current[last];
  } else {
    current[last] = value;
  }
  nextSets[index] = current;
  updateRouteField(["rule_set"], nextSets);
}

function onRuleSetText(index: number, path: string[], event: Event): void {
  const value = (event.target as HTMLInputElement).value;
  updateRuleSet(index, path, value === "" ? undefined : value);
}
</script>

<template>
  <div class="route-editor">
    <fieldset class="editor-section">
      <legend class="section-title">顶层设置</legend>
      <label class="form-row">
        <span class="form-label">自动检测接口</span>
        <input
          aria-label="自动检测接口"
          type="checkbox"
          :checked="routeConfig.auto_detect_interface === true"
          @change="onCheckbox(['auto_detect_interface'], $event)"
        />
      </label>
      <label class="form-row">
        <span class="form-label">查找进程</span>
        <input
          aria-label="查找进程"
          type="checkbox"
          :checked="routeConfig.find_process === true"
          @change="onCheckbox(['find_process'], $event)"
        />
      </label>
      <label class="form-row">
        <span class="form-label">默认接口</span>
        <input
          aria-label="默认接口"
          type="text"
          :value="(routeConfig.default_interface as string) ?? ''"
          @input="onText(['default_interface'], $event)"
        />
      </label>
      <label class="form-row">
        <span class="form-label">默认域名解析器</span>
        <input
          aria-label="默认域名解析器"
          type="text"
          :value="(routeConfig.default_domain_resolver as string) ?? ''"
          @input="onText(['default_domain_resolver'], $event)"
        />
      </label>
      <label class="form-row">
        <span class="form-label">最终出站</span>
        <input
          aria-label="最终出站"
          type="text"
          :value="(routeConfig.final as string) ?? ''"
          @input="onText(['final'], $event)"
        />
      </label>
    </fieldset>

    <fieldset class="editor-section">
      <legend class="section-title">规则</legend>
      <div v-for="(rule, index) in rules" :key="index" class="rule-card">
        <div class="rule-head">
          <span class="rule-title">规则 {{ index + 1 }}</span>
          <div class="rule-actions">
            <button class="btn btn-sm" type="button" :name="`上移规则 ${index}`" @click="moveRule(index, -1)">
              ↑
            </button>
            <button class="btn btn-sm" type="button" :name="`编辑规则 JSON ${index}`" @click="openRuleJSON(index)">
              编辑 JSON
            </button>
            <button class="btn btn-sm" type="button" :name="`删除规则 ${index}`" @click="removeRule(index)">
              删除
            </button>
          </div>
        </div>

        <textarea
          v-if="editingRuleIndex === index"
          class="rule-json"
          v-model="editingRuleText"
          spellcheck="false"
          @input="onRuleJSONInput"
        ></textarea>

        <template v-else>
          <label class="form-row">
            <span class="form-label">动作</span>
            <select aria-label="规则动作" :value="rule.action ?? 'route'" @change="onRuleSelect(index, ['action'], $event)">
              <option value="route">route</option>
              <option value="reject">reject</option>
              <option value="hijack-dns">hijack-dns</option>
              <option value="sniff">sniff</option>
              <option value="resolve">resolve</option>
            </select>
          </label>
          <label class="form-row">
            <span class="form-label">出站</span>
            <input
              aria-label="规则出站"
              type="text"
              :value="(rule.outbound as string) ?? ''"
              @input="onRuleText(index, ['outbound'], $event)"
            />
          </label>
          <label class="form-row">
            <span class="form-label">反向匹配</span>
            <input
              aria-label="反向匹配"
              type="checkbox"
              :checked="rule.invert === true"
              @change="onRuleCheckbox(index, ['invert'], $event)"
            />
          </label>
          <label class="form-row">
            <span class="form-label">域名后缀条件</span>
            <input
              aria-label="域名后缀条件"
              type="text"
              :value="(rule.domain_suffix ?? []).join(',')"
              @input="onRuleList(index, ['domain_suffix'], $event)"
            />
          </label>
        </template>
      </div>
      <button class="btn" type="button" name="添加规则" @click="addRule">添加规则</button>
    </fieldset>

    <fieldset class="editor-section">
      <legend class="section-title">规则集</legend>
      <div v-for="(ruleSet, index) in ruleSets" :key="index" class="rule-card">
        <span class="rule-title">规则集 {{ index + 1 }}</span>
        <label class="form-row">
          <span class="form-label">标签</span>
          <input
            aria-label="规则集标签"
            type="text"
            :value="(ruleSet.tag as string) ?? ''"
            @input="onRuleSetText(index, ['tag'], $event)"
          />
        </label>
        <template v-if="ruleSet.type === 'remote'">
          <label class="form-row">
            <span class="form-label">URL</span>
            <input
              aria-label="规则集 URL"
              type="text"
              :value="(ruleSet.url as string) ?? ''"
              @input="onRuleSetText(index, ['url'], $event)"
            />
          </label>
          <label class="form-row">
            <span class="form-label">下载出站</span>
            <input
              aria-label="规则集下载出站"
              type="text"
              :value="(ruleSet.download_detour as string) ?? ''"
              @input="onRuleSetText(index, ['download_detour'], $event)"
            />
          </label>
          <label class="form-row">
            <span class="form-label">更新间隔</span>
            <input
              aria-label="规则集更新间隔"
              type="text"
              :value="(ruleSet.update_interval as string) ?? ''"
              @input="onRuleSetText(index, ['update_interval'], $event)"
            />
          </label>
        </template>
        <pre v-else class="rule-preview">{{ JSON.stringify(ruleSet, null, 2) }}</pre>
      </div>
    </fieldset>
  </div>
</template>

<style scoped>
.route-editor {
  display: flex;
  flex-direction: column;
  gap: 14px;
  max-width: 720px;
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

.rule-card {
  border-top: 1px dashed var(--line);
  padding-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.rule-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.rule-title {
  font-weight: 600;
  color: var(--text);
  font-size: 13px;
}

.rule-actions {
  display: flex;
  gap: 6px;
}

.form-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.form-label {
  flex: 0 0 140px;
  color: var(--text-soft);
  font-size: 13px;
}

.form-row input[type="text"],
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

.rule-json {
  min-height: 140px;
  resize: vertical;
  padding: 10px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  font-family: ui-monospace, "SF Mono", monospace;
  font-size: 12px;
}

.rule-preview {
  margin: 0;
  padding: 10px;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
  color: var(--text-soft);
  font-size: 12px;
  overflow: auto;
}
</style>
