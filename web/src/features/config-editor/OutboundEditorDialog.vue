<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import AppDialog from "@/components/AppDialog.vue";
import AppIcon from "@/layout/AppIcon.vue";
import type { SubscriptionDetail } from "@/features/subscriptions/types";
import {
  cloneValue,
  isOutboundConfig,
  isPolicyOutbound,
  outboundEditorType,
  outboundMemberInitial,
  outboundReferenceTags,
  outboundTypes,
  uniqueOutboundTag,
  validateOutboundJson,
  type OutboundConfig,
  type OutboundEditorResult,
  type OutboundEditorType,
  type OutboundReferenceSelection
} from "./outbound";

interface ReferenceMember {
  key: string;
  id: string;
  tag: string;
  type: string;
  badge: string;
  configs: OutboundConfig[];
}

interface ReferenceGroup {
  id: string;
  name: string;
  total: number;
  members: ReferenceMember[];
  expanded: boolean;
  note?: string;
}

interface EditorForm {
  tag: string;
  hidden: boolean;
  include: string;
  exclude: string;
  icon: string;
  url: string;
  interval: string;
  tolerance: string;
}

const props = withDefaults(defineProps<{
  open: boolean;
  outbound: OutboundConfig | null;
  editingIndex: number | null;
  outbounds: OutboundConfig[];
  reservedTags?: string[];
  subscriptions?: SubscriptionDetail[];
  busy?: boolean;
}>(), { reservedTags: () => [], subscriptions: () => [], busy: false });
const emit = defineEmits<{ close: []; submit: [result: OutboundEditorResult] }>();

const customExample = `{
  "type": "hysteria2",
  "tag": "proxy-hy2",
  "server": "example.com",
  "server_port": 443,
  "password": "change-me",
  "tls": {
    "enabled": true,
    "server_name": "example.com"
  }
}`;

const selected = ref<OutboundEditorType>("selector");
const selectedKeys = ref<string[]>([]);
const expanded = reactive<Record<string, boolean>>({});
const customSource = ref(customExample);
const customReferenceSnapshot = ref<string[]>([]);
const baseConfig = ref<OutboundConfig | null>(null);
const originalType = ref("");
const form = reactive<EditorForm>({
  tag: "🚀 节点选择",
  hidden: false,
  include: "",
  exclude: "",
  icon: "",
  url: "https://www.gstatic.com/generate_204",
  interval: "3m",
  tolerance: "150"
});

const originalTag = computed(() => props.outbound?.tag ?? "");
const existingTags = computed(() => [...new Set([
  ...props.outbounds.filter((_, index) => index !== props.editingIndex).map((item) => item.tag),
  ...props.reservedTags
])]);
const customValidation = computed(() => validateOutboundJson(customSource.value, existingTags.value, originalTag.value));
const selectedSet = computed(() => new Set(selectedKeys.value));

function makeMember(key: string, id: string, tag: string, type: string, configs: OutboundConfig[] = [], badge = ""): ReferenceMember {
  return { key, id, tag, type, configs, badge: badge || outboundMemberInitial(tag) };
}

const referenceGroups = computed<ReferenceGroup[]>(() => {
  const current = props.outbounds.flatMap((item, index) => index === props.editingIndex
    ? []
    : [makeMember(`profile:${index}`, `profile-${index}`, item.tag, item.type)]);
  if (!current.some((item) => item.tag === "direct")) {
    current.unshift(makeMember("builtin:direct", "direct", "direct", "Built-In", [{ type: "direct", tag: "direct" }], "🎯"));
  }
  if (!current.some((item) => item.tag === "block")) {
    current.splice(1, 0, makeMember("builtin:block", "block", "block", "Built-In", [{ type: "block", tag: "block" }], "🛑"));
  }

  const subscriptionMembers = props.subscriptions.flatMap((subscription) => {
    const configs = subscription.outbounds.filter(isOutboundConfig).map((item) => cloneValue(item));
    return configs.length
      ? [makeMember(`subscription:${subscription.id}`, subscription.id, subscription.name, "Subscription", configs)]
      : [];
  });
  const groups: ReferenceGroup[] = [
    { id: "built-in", name: "内置", total: current.length, members: current, expanded: true },
    { id: "subscriptions", name: "订阅", total: subscriptionMembers.length, members: subscriptionMembers, expanded: true }
  ];

  for (const subscription of props.subscriptions) {
    const members = subscription.outbounds.flatMap((node, index) => isOutboundConfig(node)
      ? [makeMember(
          `subscription-node:${subscription.id}:${index}`,
          `${subscription.id}-node-${index + 1}`,
          node.tag,
          node.type,
          [cloneValue(node)]
        )]
      : []);
    groups.push({
      id: `subscription-nodes:${subscription.id}`,
      name: subscription.name,
      total: subscription.outbounds.length,
      members,
      expanded: false,
      note: subscription.outbounds.length > members.length ? `有 ${subscription.outbounds.length - members.length} 个节点缺少 type 或 tag，无法引用。` : undefined
    });
  }

  const knownTags = new Set(groups
    .filter((group) => group.id !== "subscriptions")
    .flatMap((group) => group.members.map((member) => member.tag)));
  const unresolved = props.outbound
    ? outboundReferenceTags(props.outbound)
        .filter((tag) => !knownTags.has(tag))
        .map((tag, index) => makeMember(`current:${index}:${tag}`, `current-${index}`, tag, "Current"))
    : [];
  if (unresolved.length) groups.push({ id: "current", name: "当前引用", total: unresolved.length, members: unresolved, expanded: true });
  return groups;
});

const allMembers = computed(() => referenceGroups.value.flatMap((group) => group.members));
const selectedMembers = computed(() => selectedKeys.value.flatMap((key) => {
  const member = allMembers.value.find((candidate) => candidate.key === key);
  return member ? [member] : [];
}));
const resolvedReferenceCount = computed(() => new Set(selectedMembers.value.flatMap((member) => member.configs.length
  ? member.configs.map((config) => config.tag)
  : [member.tag])).size);
const structuredError = computed(() => {
  const tag = form.tag.trim();
  if (!tag) return "出站名称不能为空。";
  if (Array.from(tag).length > 128) return "出站名称不能超过 128 个字符。";
  if (tag !== originalTag.value && existingTags.value.includes(tag)) return `出站名称已存在：${tag}`;
  if ((selected.value === "selector" || selected.value === "urltest") && resolvedReferenceCount.value === 0) {
    return "手动选择和自动选择至少需要引用一个出站、订阅或订阅节点。";
  }
  if (selected.value === "urltest") {
    if (!form.url.trim()) return "测延迟链接不能为空。";
    if (!form.interval.trim()) return "测试间隔不能为空。";
    const tolerance = Number(form.tolerance);
    if (!Number.isFinite(tolerance) || tolerance < 0) return "测试容差必须是非负数。";
  }
  return "";
});
const valid = computed(() => selected.value === "custom" ? customValidation.value.valid : !structuredError.value);

watch(() => [props.open, props.outbound, props.subscriptions] as const, ([open]) => {
  if (open) initialize();
}, { immediate: true });

function text(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

function initialize(): void {
  const outbound = props.outbound;
  baseConfig.value = outbound ? cloneValue(outbound) : null;
  originalType.value = outbound?.type ?? "";
  selected.value = outbound ? outboundEditorType(outbound) : "selector";
  form.tag = outbound?.tag ?? uniqueOutboundTag("🚀 节点选择", existingTags.value);
  form.hidden = outbound?.hidden === true;
  form.include = text(outbound?.include);
  form.exclude = text(outbound?.exclude);
  form.icon = text(outbound?.icon);
  form.url = text(outbound?.url, "https://www.gstatic.com/generate_204");
  form.interval = text(outbound?.interval, "3m");
  form.tolerance = typeof outbound?.tolerance === "number" ? String(outbound.tolerance) : "150";
  customSource.value = selected.value === "custom" && outbound ? JSON.stringify(outbound, null, 2) : customExample;
  customReferenceSnapshot.value = [];
  for (const key of Object.keys(expanded)) delete expanded[key];
  for (const group of referenceGroups.value) expanded[group.id] = group.expanded;

  const keys: string[] = [];
  const claimed = new Set<string>();
  const references = outbound ? outboundReferenceTags(outbound) : [];
  const priorityGroups = [
    referenceGroups.value.find((group) => group.id === "built-in"),
    ...referenceGroups.value.filter((group) => group.id.startsWith("subscription-nodes:")),
    referenceGroups.value.find((group) => group.id === "current")
  ].filter((group): group is ReferenceGroup => Boolean(group));
  for (const tag of references) {
    const member = priorityGroups.flatMap((group) => group.members).find((candidate) => candidate.tag === tag && !claimed.has(candidate.key));
    if (!member) continue;
    keys.push(member.key);
    claimed.add(member.key);
    const group = referenceGroups.value.find((candidate) => candidate.members.some((item) => item.key === member.key));
    if (group) expanded[group.id] = true;
  }
  selectedKeys.value = keys;
}

function chooseType(type: OutboundEditorType): void {
  if (type === selected.value) return;
  if (type === "custom") {
    const config = buildStructuredConfig();
    customReferenceSnapshot.value = outboundReferenceTags(config);
    customSource.value = JSON.stringify(config, null, 2);
  }
  selected.value = type;
}

function removeKeys(config: Record<string, unknown>, keys: string[]): void {
  for (const key of keys) delete config[key];
}

function buildStructuredConfig(): OutboundConfig {
  const type = selected.value === "custom" ? "selector" : selected.value;
  const sameType = originalType.value === type && baseConfig.value && outboundEditorType(baseConfig.value) === type;
  const config: Record<string, unknown> = sameType && baseConfig.value ? cloneValue(baseConfig.value) : {};
  config.type = type;
  config.tag = form.tag.trim();
  const policyKeys = ["hidden", "include", "exclude", "icon", "interrupt_exist_connections", "url", "interval", "tolerance", "outbounds"];
  if (type === "direct" || type === "block") {
    removeKeys(config, policyKeys);
    return config as OutboundConfig;
  }
  config.hidden = form.hidden;
  config.include = form.include.trim();
  config.exclude = form.exclude.trim();
  config.icon = form.icon.trim();
  config.interrupt_exist_connections = true;
  config.outbounds = selectedReferenceTags();
  if (type === "urltest") {
    config.url = form.url.trim();
    config.interval = form.interval.trim();
    config.tolerance = Number(form.tolerance);
  } else {
    removeKeys(config, ["url", "interval", "tolerance"]);
  }
  return config as OutboundConfig;
}

function submit(): void {
  if (!valid.value) return;
  if (selected.value === "custom") {
    if (customValidation.value.valid) {
      const tags = outboundReferenceTags(customValidation.value.config);
      const references = tags.length === customReferenceSnapshot.value.length && tags.every((tag, index) => tag === customReferenceSnapshot.value[index])
        ? buildReferenceSelections()
        : [];
      emit("submit", { config: cloneValue(customValidation.value.config), references, editorType: "custom" });
    }
    return;
  }
  emit("submit", { config: buildStructuredConfig(), references: buildReferenceSelections(), editorType: selected.value });
}

function selectedReferenceTags(): string[] {
  return [...new Set(selectedMembers.value.flatMap((member) => member.configs.length
    ? member.configs.map((config) => config.tag)
    : [member.tag]))];
}

function buildReferenceSelections(): OutboundReferenceSelection[] {
  return selectedMembers.value.flatMap((member) => member.configs.length
    ? member.configs.map((config, index) => ({ key: `${member.key}:${index}`, tag: config.tag, config: cloneValue(config) }))
    : [{ key: member.key, tag: member.tag }]);
}

function formatCustom(): void {
  if (customValidation.value.valid) customSource.value = JSON.stringify(customValidation.value.config, null, 2);
}

function insertIndent(event: KeyboardEvent): void {
  if (event.key === "Tab") {
    event.preventDefault();
    const target = event.currentTarget as HTMLTextAreaElement;
    const start = target.selectionStart;
    const end = target.selectionEnd;
    customSource.value = `${customSource.value.slice(0, start)}  ${customSource.value.slice(end)}`;
    requestAnimationFrame(() => { target.selectionStart = target.selectionEnd = start + 2; });
  }
  if ((event.metaKey || event.ctrlKey) && event.key === "Enter" && valid.value && !props.busy) {
    event.preventDefault();
    submit();
  }
}

function groupSelectedCount(group: ReferenceGroup): number {
  return group.members.filter((member) => selectedSet.value.has(member.key)).length;
}
</script>

<template>
  <AppDialog class="outbound-editor-dialog" :open="open" title="出站" :description="outbound ? '编辑出站类型、筛选条件及引用关系' : '创建内置出站或节点选择策略组'" :close-disabled="busy" :initial-focus="selected === 'custom' ? 'textarea' : 'input[name=tag]'" @close="emit('close')">
    <form id="outbound-editor-form" class="outbound-editor-form" @submit.prevent="submit">
      <label v-if="selected !== 'custom'" class="editor-row"><span class="editor-label">名称<small>tag</small></span><input v-model="form.tag" name="tag" placeholder="例如 🚀 节点选择" required /></label>
      <div class="editor-row"><span class="editor-label">类型<small>type</small></span><div class="type-control">
        <label v-for="item in outboundTypes" :key="item.type"><input type="radio" name="outbound-type" :value="item.type" :checked="selected === item.type" @change="chooseType(item.type)" /><span><AppIcon :name="item.icon" :size="13" />{{ item.label }}</span></label>
      </div></div>

      <template v-if="selected === 'selector' || selected === 'urltest'">
        <div class="editor-row"><span class="editor-label">隐藏<small>仅控制面板</small></span><label class="toggle-control"><span>不在代理组快捷选择中显示</span><input v-model="form.hidden" name="hidden" type="checkbox" aria-label="隐藏出站" /></label></div>
        <label class="editor-row"><span class="editor-label">包含<small>include</small></span><input v-model="form.include" name="include" placeholder="keywords1|keywords2" /></label>
        <label class="editor-row"><span class="editor-label">排除<small>exclude</small></span><input v-model="form.exclude" name="exclude" placeholder="keywords1|keywords2" /></label>
        <label class="editor-row"><span class="editor-label">图标<small>icon</small></span><span class="icon-control"><span class="icon-preview" :class="{ empty: !form.icon }"><img v-if="form.icon" :src="form.icon" alt="" /><AppIcon v-else name="globe" :size="15" /></span><input v-model="form.icon" name="icon" placeholder="https://" /></span></label>
        <template v-if="selected === 'urltest'">
          <label class="editor-row"><span class="editor-label">测延迟链接<small>url</small></span><input v-model="form.url" name="url" placeholder="https://www.gstatic.com/generate_204" /></label>
          <label class="editor-row"><span class="editor-label">测试间隔<small>interval</small></span><input v-model="form.interval" name="interval" placeholder="3m" /></label>
          <label class="editor-row"><span class="editor-label">测试容差<small>tolerance</small></span><span class="suffix-input"><input v-model="form.tolerance" name="tolerance" type="number" min="0" /><span>ms</span></span></label>
        </template>

        <div class="reference-divider"><span>引用出站 &amp; 引用订阅</span><strong>已选 {{ selectedKeys.length }} 项 · {{ resolvedReferenceCount }} 个出站</strong></div>
        <div class="reference-list">
          <section v-for="group in referenceGroups" :key="group.id" class="reference-group" :class="{ expanded: expanded[group.id] }">
            <button class="reference-head" type="button" :aria-expanded="expanded[group.id]" @click="expanded[group.id] = !expanded[group.id]"><span>{{ group.name }}</span><b v-if="groupSelectedCount(group)">已选 {{ groupSelectedCount(group) }}</b><strong>{{ group.total }}</strong><AppIcon name="chevron" :size="13" /></button>
            <div v-if="expanded[group.id]" class="reference-body">
              <div v-if="group.members.length" class="reference-grid">
                <label v-for="member in group.members" :key="member.key" class="reference-tile"><input v-model="selectedKeys" type="checkbox" :value="member.key" /><span class="reference-content"><span class="reference-badge">{{ member.badge }}</span><span><strong>{{ member.tag }}</strong><small>{{ member.type }}</small></span><AppIcon name="check" :size="12" /></span></label>
              </div>
              <p v-else class="reference-empty">该分组暂无可引用项目</p>
              <p v-if="group.note" class="reference-note">{{ group.note }}</p>
            </div>
          </section>
        </div>
        <p class="copy-note"><AppIcon name="copy" :size="14" />选择订阅或订阅节点时，会把完整节点复制进当前档案；之后两侧修改互不联动。</p>
      </template>

      <div v-else-if="selected !== 'custom'" class="no-options"><AppIcon :name="selected === 'block' ? 'shield' : 'lightning'" :size="28" /><strong>{{ selected === "block" ? "阻断出站无需额外参数" : "直连出站无需额外参数" }}</strong><span>保存后会生成只包含 <code>type</code> 与 <code>tag</code> 的 sing-box 出站对象。</span></div>

      <template v-else>
        <div class="custom-divider"><span>完整出站对象</span></div>
        <div class="json-workbench"><div class="json-toolbar"><div><b>JSON</b><span>完整出站对象</span></div><div><button type="button" @click="customSource = customExample">示例</button><button type="button" :disabled="!customValidation.valid" @click="formatCustom"><AppIcon name="sparkles" :size="13" />格式化</button></div></div><textarea v-model="customSource" aria-label="出站 JSON" :aria-invalid="!customValidation.valid" spellcheck="false" @keydown="insertIndent" /></div>
        <p class="validation" :class="customValidation.valid ? 'valid' : 'invalid'" role="status"><AppIcon :name="customValidation.valid ? 'check' : 'info'" :size="14" />{{ customValidation.message }}</p>
        <p class="copy-note"><AppIcon name="info" :size="14" />可使用当前 Schema 支持的任意出站类型；完整字段会原样写入 <code>outbounds</code>。</p>
      </template>
      <p v-if="selected !== 'custom' && structuredError" class="validation invalid" role="alert"><AppIcon name="info" :size="14" />{{ structuredError }}</p>
    </form>
    <template #footer>
      <button class="secondary-button" type="button" :disabled="busy" @click="emit('close')">取消</button>
      <button class="primary-button" type="submit" form="outbound-editor-form" :disabled="!valid || busy"><AppIcon :name="outbound ? 'save' : 'plus'" :size="14" />{{ busy ? "正在保存" : outbound ? "保存" : "确认添加" }}</button>
    </template>
  </AppDialog>
</template>

<style scoped>
:global(.outbound-editor-dialog) { width: min(980px, calc(100vw - 32px)); }.outbound-editor-form { display: grid; }.editor-row { display: grid; min-width: 0; grid-template-columns: 132px minmax(0, 1fr); align-items: center; gap: 18px; padding: 7px 0; }.editor-label { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; color: var(--text); font-size: 11px; font-weight: 680; }.editor-label small { color: var(--text-faint); font-size: 8px; font-weight: 520; }.type-control { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); overflow: hidden; padding: 3px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-soft); }.type-control label { min-width: 0; }.type-control input, .reference-tile > input { position: absolute; width: 1px; height: 1px; overflow: hidden; opacity: 0; }.type-control label > span { display: flex; min-height: 34px; align-items: center; justify-content: center; gap: 6px; border-radius: 7px; color: var(--text-faint); font-size: 9px; font-weight: 650; cursor: pointer; transition: 150ms ease; }.type-control label > span:hover { color: var(--text); }.type-control input:checked + span { color: var(--primary); background: var(--surface); box-shadow: 0 2px 9px rgb(34 49 83 / 9%); }.type-control input:focus-visible + span, .reference-tile input:focus-visible + span { outline: 3px solid color-mix(in srgb, var(--primary) 22%, transparent); }.toggle-control { display: flex; min-height: 42px; align-items: center; justify-content: space-between; gap: 12px; padding: 0 11px; border: 1px solid var(--line-strong); border-radius: var(--radius-sm); color: var(--text-faint); background: var(--surface-soft); font-size: 9px; }.toggle-control input { width: 34px; min-height: 19px; accent-color: var(--primary); }.icon-control { display: flex; min-width: 0; align-items: center; gap: 8px; }.icon-control > input { min-width: 0; flex: 1; }.icon-preview { display: grid; width: 42px; height: 42px; flex: 0 0 auto; overflow: hidden; place-items: center; border: 1px solid var(--line); border-radius: 9px; color: var(--primary); background: var(--primary-soft); }.icon-preview img { width: 100%; height: 100%; object-fit: cover; }.icon-preview.empty { color: var(--text-faint); background: var(--surface-soft); }.suffix-input { position: relative; }.suffix-input input { width: 100%; padding-right: 42px; }.suffix-input > span { position: absolute; top: 50%; right: 11px; color: var(--text-faint); font-size: 9px; transform: translateY(-50%); }.reference-divider, .custom-divider { display: flex; min-height: 41px; align-items: center; gap: 12px; margin-top: 12px; color: var(--text-soft); font-size: 9px; font-weight: 680; }.reference-divider::before, .reference-divider::after, .custom-divider::before, .custom-divider::after { height: 1px; content: ""; background: var(--line); }.reference-divider::before, .custom-divider::before { width: 22px; }.reference-divider::after, .custom-divider::after { flex: 1; }.reference-divider strong { order: 3; color: var(--primary); font-size: 8px; }.reference-list { display: grid; gap: 7px; }.reference-group { overflow: hidden; border: 1px solid var(--line); border-radius: 10px; background: var(--surface); }.reference-head { display: flex; width: 100%; min-height: 39px; align-items: center; gap: 8px; padding: 7px 11px; color: var(--text-soft); border-radius: 0; background: var(--surface-soft); text-align: left; font-size: 10px; transition: 150ms ease; }.reference-head:hover { color: var(--primary); background: var(--primary-soft); }.reference-head > span { min-width: 0; overflow: hidden; flex: 1; text-overflow: ellipsis; white-space: nowrap; }.reference-head b { padding: 2px 6px; border-radius: 999px; color: var(--primary); background: var(--primary-soft); font-size: 8px; }.reference-head strong { color: var(--text-faint); font-size: 9px; }.reference-head .icon { transition: transform 150ms ease; }.reference-group.expanded .reference-head .icon { transform: rotate(90deg); }.reference-body { padding: 9px; border-top: 1px solid var(--line); }.reference-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 7px; }.reference-tile { min-width: 0; }.reference-content { position: relative; display: flex; min-width: 0; min-height: 50px; align-items: center; gap: 8px; padding: 7px 24px 7px 8px; border: 1px solid transparent; border-radius: 9px; background: var(--surface-soft); cursor: pointer; transition: 150ms ease; }.reference-content:hover { border-color: color-mix(in srgb, var(--primary) 24%, var(--line)); transform: translateY(-1px); }.reference-content > span:nth-child(2) { min-width: 0; }.reference-content strong, .reference-content small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.reference-content strong { color: var(--text); font-size: 9px; }.reference-content small { margin-top: 2px; color: var(--text-faint); font-size: 8px; }.reference-badge { display: grid !important; width: 29px; height: 29px; flex: 0 0 auto; place-items: center; border-radius: 8px; color: var(--primary); background: var(--primary-soft); font-size: 8px; font-weight: 720; }.reference-content > .icon { position: absolute; top: 6px; right: 6px; color: transparent; }.reference-tile input:checked + .reference-content { border-color: color-mix(in srgb, var(--primary) 40%, var(--line)); background: var(--primary-soft); }.reference-tile input:checked + .reference-content > .icon { color: var(--primary); }.reference-note, .reference-empty { margin: 8px 2px 0; color: var(--text-faint); font-size: 8px; }.reference-empty { margin: 0; padding: 12px; text-align: center; }.copy-note { display: flex; align-items: flex-start; gap: 7px; margin: 10px 0 0; padding: 9px 10px; border: 1px solid var(--line); border-radius: 9px; color: var(--text-soft); background: var(--surface-soft); font-size: 9px; }.copy-note .icon { color: var(--primary); }.copy-note code, .no-options code { color: var(--primary); font: inherit; font-weight: 700; }.no-options { display: grid; min-height: 210px; place-items: center; align-content: center; gap: 7px; margin-top: 10px; border: 1px dashed var(--line-strong); border-radius: 12px; color: var(--text-faint); background: var(--surface-soft); text-align: center; }.no-options > .icon { margin-bottom: 3px; color: var(--primary); }.no-options strong { color: var(--text); font-size: 12px; }.no-options span { max-width: 410px; font-size: 9px; }.json-workbench { overflow: hidden; border: 1px solid #2a3447; border-radius: 12px; background: #111827; }.json-toolbar { display: flex; min-height: 42px; align-items: center; justify-content: space-between; gap: 14px; padding: 7px 10px 7px 13px; border-bottom: 1px solid #293245; color: #8f9bb0; font-size: 9px; }.json-toolbar b { margin-right: 8px; padding: 3px 7px; border-radius: 6px; color: #aab9ff; background: rgb(113 136 255 / 14%); font: 720 9px ui-monospace, monospace; }.json-toolbar button { display: inline-flex; min-height: 27px; align-items: center; gap: 5px; padding: 5px 8px; color: #aab4c7; background: transparent; font-size: 9px; }.json-toolbar button:hover { color: #fff; background: rgb(255 255 255 / 8%); }.json-workbench textarea { display: block; width: 100%; min-height: 310px; padding: 16px 17px; border: 0; border-radius: 0; outline: 0; color: #c9d5eb; background: #111827; caret-color: #8ea1ff; font: 12px/1.7 ui-monospace, SFMono-Regular, Consolas, monospace; resize: vertical; tab-size: 2; }.json-workbench textarea[aria-invalid="true"] { box-shadow: inset 0 0 0 1px rgb(235 90 101 / 44%); }.validation { display: flex; min-height: 35px; align-items: center; gap: 7px; margin: 10px 0 0; padding: 8px 10px; border-radius: 9px; font-size: 9px; }.validation.valid { color: var(--green); background: var(--green-soft); }.validation.invalid { color: var(--red); background: var(--red-soft); }.secondary-button, .primary-button { display: inline-flex; min-height: 36px; align-items: center; gap: 6px; padding: 7px 11px; border: 1px solid var(--line); font-size: 10px; }.secondary-button { color: var(--text-soft); background: var(--surface); }.primary-button { color: #fff; border-color: var(--primary); background: var(--primary); }
@media (max-width: 760px) { :global(.outbound-editor-dialog) { width: 100%; }.editor-row { grid-template-columns: 1fr; gap: 6px; padding: 8px 0; }.editor-label { justify-content: flex-start; }.type-control, .reference-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 430px) { .type-control { grid-template-columns: repeat(2, minmax(0, 1fr)); }.reference-grid { grid-template-columns: 1fr; }.toggle-control { align-items: flex-start; padding-block: 8px; }.reference-divider strong { max-width: 45%; text-align: right; } }
</style>
