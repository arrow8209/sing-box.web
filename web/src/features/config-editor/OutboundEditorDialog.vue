<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import AppDialog from "@/components/AppDialog.vue";
import AppIcon from "@/layout/AppIcon.vue";
import type { SubscriptionDetail } from "@/features/subscriptions/types";
import {
  cloneValue,
  createOutboundGroupId,
  isOutboundConfig,
  outboundMemberInitial,
  outboundTypes,
  uniqueOutboundTag,
  type OutboundConfig,
  type OutboundEditorGroup,
  type OutboundEditorResult,
  type OutboundEditorType,
  type OutboundReference
} from "./outbound";

interface ReferenceMember {
  key: string;
  tag: string;
  type: string;
  badge: string;
  reference: OutboundReference;
  count: number;
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
  group: OutboundEditorGroup | null;
  groups: OutboundEditorGroup[];
  reservedTags?: string[];
  subscriptions?: SubscriptionDetail[];
  busy?: boolean;
}>(), { reservedTags: () => [], subscriptions: () => [], busy: false });
const emit = defineEmits<{ close: []; submit: [result: OutboundEditorResult] }>();

const selected = ref<OutboundEditorType>("selector");
const selectedKeys = ref<string[]>([]);
const expanded = reactive<Record<string, boolean>>({});
const baseConfig = ref<OutboundConfig | null>(null);
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

const originalTag = computed(() => props.group?.tag ?? "");
const existingTags = computed(() => [...new Set([
  ...props.groups.filter((group) => group.id !== props.group?.id).map((group) => group.tag),
  ...props.reservedTags
])]);
const selectedSet = computed(() => new Set(selectedKeys.value));

function referenceKey(reference: OutboundReference): string {
  if (reference.kind === "builtin") return `builtin:${reference.tag}`;
  if (reference.kind === "outbound") return `outbound:${reference.outboundId}`;
  if (reference.kind === "subscription") return `subscription:${reference.subscriptionId}`;
  if (reference.kind === "subscription-node") return `subscription-node:${reference.subscriptionId}:${reference.nodeTag}`;
  return `current:${reference.kind}:${reference.tag}`;
}

function makeMember(tag: string, type: string, reference: OutboundReference, count = 1, badge = ""): ReferenceMember {
  return { key: referenceKey(reference), tag, type, reference, count, badge: badge || outboundMemberInitial(tag) };
}

function matcher(pattern: string): RegExp | null {
  if (!pattern) return null;
  try {
    return new RegExp(pattern);
  } catch {
    return new RegExp(pattern.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"));
  }
}

function matches(tag: string): boolean {
  const include = matcher(form.include.trim());
  const exclude = matcher(form.exclude.trim());
  return (include ? include.test(tag) : true) && !(exclude?.test(tag) ?? false);
}

const standardMembers = computed(() => [
  makeMember("direct", "Built-In", { kind: "builtin", tag: "direct" }, 1, "🎯"),
  makeMember("block", "Built-In", { kind: "builtin", tag: "block" }, 1, "🛑"),
  ...props.groups.flatMap((group) => group.id === props.group?.id
    ? []
    : [makeMember(group.tag, group.type, { kind: "outbound", outboundId: group.id, tag: group.tag })])
]);

const subscriptionMembers = computed(() => props.subscriptions.flatMap((subscription) => {
  const count = subscription.outbounds.filter(isOutboundConfig).filter((node) => matches(node.tag)).length;
  return subscription.outbounds.some(isOutboundConfig)
    ? [makeMember(subscription.name, "Subscription", { kind: "subscription", subscriptionId: subscription.id, tag: subscription.name }, count)]
    : [];
}));

const knownKeys = computed(() => new Set([
  ...standardMembers.value.map((member) => member.key),
  ...subscriptionMembers.value.map((member) => member.key),
  ...props.subscriptions.flatMap((subscription) => subscription.outbounds.filter(isOutboundConfig).map((node) => referenceKey({
    kind: "subscription-node",
    subscriptionId: subscription.id,
    nodeTag: node.tag
  })))
]));

const currentMembers = computed(() => (props.group?.references ?? []).flatMap((reference) => knownKeys.value.has(referenceKey(reference))
  ? []
  : [makeMember(
      reference.kind === "subscription-node" ? reference.nodeTag : "tag" in reference ? reference.tag : "失效引用",
      "Not Found",
      cloneValue(reference),
      reference.resolvedTags?.length ?? 1,
      "!"
    )]));

const referenceGroups = computed<ReferenceGroup[]>(() => {
  const result: ReferenceGroup[] = [
    { id: "built-in", name: "内置", total: standardMembers.value.length, members: standardMembers.value, expanded: true },
    { id: "subscriptions", name: "订阅", total: subscriptionMembers.value.length, members: subscriptionMembers.value, expanded: true }
  ];
  for (const subscription of props.subscriptions) {
    const members = subscription.outbounds.flatMap((node) => isOutboundConfig(node)
      ? [makeMember(node.tag, node.type, { kind: "subscription-node", subscriptionId: subscription.id, nodeTag: node.tag }, matches(node.tag) ? 1 : 0)]
      : []);
    result.push({
      id: `subscription-nodes:${subscription.id}`,
      name: subscription.name,
      total: members.length,
      members,
      expanded: false,
      note: subscription.outbounds.length > members.length ? `有 ${subscription.outbounds.length - members.length} 个节点缺少 type 或 tag，无法引用。` : undefined
    });
  }
  if (currentMembers.value.length) result.push({ id: "current", name: "当前失效引用", total: currentMembers.value.length, members: currentMembers.value, expanded: true });
  return result;
});

const allMembers = computed(() => referenceGroups.value.flatMap((group) => group.members));
const selectedMembers = computed(() => selectedKeys.value.flatMap((key) => {
  const member = allMembers.value.find((candidate) => candidate.key === key);
  return member ? [member] : [];
}));
const resolvedReferenceCount = computed(() => selectedMembers.value.reduce((count, member) => count + member.count, 0));
const structuredError = computed(() => {
  const tag = form.tag.trim();
  if (!tag) return "出站名称不能为空。";
  if (Array.from(tag).length > 128) return "出站名称不能超过 128 个字符。";
  if (tag !== originalTag.value && existingTags.value.includes(tag)) return `出站名称已存在：${tag}`;
  if (selected.value === "urltest") {
    if (!form.url.trim()) return "测延迟链接不能为空。";
    if (!form.interval.trim()) return "测试间隔不能为空。";
    const tolerance = Number(form.tolerance);
    if (!Number.isFinite(tolerance) || tolerance < 0) return "测试容差必须是非负数。";
  }
  return "";
});

watch(() => props.open, (open, wasOpen) => {
  if (open && !wasOpen) initialize();
}, { immediate: true });

function text(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

function initialize(): void {
  const group = props.group;
  baseConfig.value = group ? cloneValue(group.config) : null;
  selected.value = group?.type ?? "selector";
  form.tag = group?.tag ?? uniqueOutboundTag("🚀 节点选择", existingTags.value);
  form.hidden = group?.hidden ?? false;
  form.include = group?.include ?? "";
  form.exclude = group?.exclude ?? "";
  form.icon = group?.icon ?? "";
  form.url = text(group?.config.url, "https://www.gstatic.com/generate_204");
  form.interval = text(group?.config.interval, "3m");
  form.tolerance = typeof group?.config.tolerance === "number" ? String(group.config.tolerance) : "150";
  for (const key of Object.keys(expanded)) delete expanded[key];
  referenceGroups.value.forEach((referenceGroup) => { expanded[referenceGroup.id] = referenceGroup.expanded; });
  selectedKeys.value = (group?.references ?? []).map(referenceKey);
  for (const key of selectedKeys.value) {
    const referenceGroup = referenceGroups.value.find((candidate) => candidate.members.some((member) => member.key === key));
    if (referenceGroup) expanded[referenceGroup.id] = true;
  }
}

function chooseType(type: OutboundEditorType): void {
  selected.value = type;
}

function buildConfig(): OutboundConfig {
  const config = baseConfig.value ? cloneValue(baseConfig.value) : { type: selected.value, tag: form.tag.trim() };
  config.type = selected.value;
  config.tag = form.tag.trim();
  if (selected.value === "urltest") {
    config.url = form.url.trim();
    config.interval = form.interval.trim();
    config.tolerance = Number(form.tolerance);
  }
  return config;
}

function submit(): void {
  if (structuredError.value) return;
  const id = props.group?.id ?? createOutboundGroupId(props.groups.map((group) => group.id));
  emit("submit", {
    group: {
      id,
      tag: form.tag.trim(),
      type: selected.value,
      hidden: form.hidden,
      include: form.include.trim(),
      exclude: form.exclude.trim(),
      icon: form.icon.trim(),
      references: selectedMembers.value.map((member) => cloneValue(member.reference)),
      runtimeTags: props.group?.runtimeTags ?? [],
      config: buildConfig()
    }
  });
}

function groupSelectedCount(group: ReferenceGroup): number {
  return group.members.filter((member) => selectedSet.value.has(member.key)).length;
}
</script>

<template>
  <AppDialog class="outbound-editor-dialog" :open="open" title="出站" :description="group ? '编辑出站类型、筛选条件及引用关系' : '创建出站或节点选择策略组'" :close-disabled="busy" initial-focus="input[name=tag]" @close="emit('close')">
    <form id="outbound-editor-form" class="outbound-editor-form" @submit.prevent="submit">
      <label class="editor-row"><span class="editor-label">名称<small>tag</small></span><input v-model="form.tag" name="tag" placeholder="例如 🚀 节点选择" required /></label>
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
          <section v-for="referenceGroup in referenceGroups" :key="referenceGroup.id" class="reference-group" :class="{ expanded: expanded[referenceGroup.id] }">
            <button class="reference-head" type="button" :aria-expanded="expanded[referenceGroup.id]" @click="expanded[referenceGroup.id] = !expanded[referenceGroup.id]"><span>{{ referenceGroup.name }}</span><b v-if="groupSelectedCount(referenceGroup)">已选 {{ groupSelectedCount(referenceGroup) }}</b><strong>{{ referenceGroup.total }}</strong><AppIcon name="chevron" :size="13" /></button>
            <div v-if="expanded[referenceGroup.id]" class="reference-body">
              <div v-if="referenceGroup.members.length" class="reference-grid">
                <label v-for="member in referenceGroup.members" :key="member.key" class="reference-tile"><input v-model="selectedKeys" type="checkbox" :value="member.key" /><span class="reference-content"><span class="reference-badge">{{ member.badge }}</span><span><strong>{{ member.tag }}</strong><small>{{ member.type }}</small></span><AppIcon name="check" :size="12" /></span></label>
              </div>
              <p v-else class="reference-empty">该分组暂无可引用项目</p>
              <p v-if="referenceGroup.note" class="reference-note">{{ referenceGroup.note }}</p>
            </div>
          </section>
        </div>
        <p class="copy-note"><AppIcon name="info" :size="14" />选择整个订阅时，会按包含/排除条件展开节点；再次编辑仍保留“整个订阅”的选择关系。</p>
      </template>

      <div v-else class="no-options"><AppIcon :name="selected === 'block' ? 'shield' : 'lightning'" :size="28" /><strong>{{ selected === "block" ? "阻断出站无需额外参数" : "直连出站无需额外参数" }}</strong><span>保存后会生成只包含 <code>type</code> 与 <code>tag</code> 的 sing-box 出站对象。</span></div>
      <p v-if="structuredError" class="validation invalid" role="alert"><AppIcon name="info" :size="14" />{{ structuredError }}</p>
    </form>
    <template #footer>
      <button class="secondary-button" type="button" :disabled="busy" @click="emit('close')">取消</button>
      <button class="primary-button" type="submit" form="outbound-editor-form" :disabled="Boolean(structuredError) || busy"><AppIcon :name="group ? 'save' : 'plus'" :size="14" />{{ busy ? "正在保存" : group ? "保存" : "确认添加" }}</button>
    </template>
  </AppDialog>
</template>

<style scoped>
:global(.outbound-editor-dialog) { width: min(980px, calc(100vw - 32px)); }.outbound-editor-form { display: grid; }.editor-row { display: grid; min-width: 0; grid-template-columns: 132px minmax(0, 1fr); align-items: center; gap: 18px; padding: 7px 0; }.editor-label { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; color: var(--text); font-size: 11px; font-weight: 680; }.editor-label small { color: var(--text-faint); font-size: 8px; font-weight: 520; }.type-control { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); overflow: hidden; padding: 3px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-soft); }.type-control label { min-width: 0; }.type-control input, .reference-tile > input { position: absolute; width: 1px; height: 1px; overflow: hidden; opacity: 0; }.type-control label > span { display: flex; min-height: 34px; align-items: center; justify-content: center; gap: 6px; border-radius: 7px; color: var(--text-faint); font-size: 9px; font-weight: 650; cursor: pointer; transition: 150ms ease; }.type-control label > span:hover { color: var(--text); }.type-control input:checked + span { color: var(--primary); background: var(--surface); box-shadow: 0 2px 9px rgb(34 49 83 / 9%); }.type-control input:focus-visible + span, .reference-tile input:focus-visible + span { outline: 3px solid color-mix(in srgb, var(--primary) 22%, transparent); }.toggle-control { display: flex; min-height: 42px; align-items: center; justify-content: space-between; gap: 12px; padding: 0 11px; border: 1px solid var(--line-strong); border-radius: var(--radius-sm); color: var(--text-faint); background: var(--surface-soft); font-size: 9px; }.toggle-control input { width: 34px; min-height: 19px; accent-color: var(--primary); }.icon-control { display: flex; min-width: 0; align-items: center; gap: 8px; }.icon-control > input { min-width: 0; flex: 1; }.icon-preview { display: grid; width: 42px; height: 42px; flex: 0 0 auto; overflow: hidden; place-items: center; border: 1px solid var(--line); border-radius: 9px; color: var(--primary); background: var(--primary-soft); }.icon-preview img { width: 100%; height: 100%; object-fit: cover; }.icon-preview.empty { color: var(--text-faint); background: var(--surface-soft); }.suffix-input { position: relative; }.suffix-input input { width: 100%; padding-right: 42px; }.suffix-input > span { position: absolute; top: 50%; right: 11px; color: var(--text-faint); font-size: 9px; transform: translateY(-50%); }.reference-divider { display: flex; min-height: 41px; align-items: center; gap: 12px; margin-top: 12px; color: var(--text-soft); font-size: 9px; font-weight: 680; }.reference-divider::before, .reference-divider::after { height: 1px; content: ""; background: var(--line); }.reference-divider::before { width: 22px; }.reference-divider::after { flex: 1; }.reference-divider strong { order: 3; color: var(--primary); font-size: 8px; }.reference-list { display: grid; gap: 7px; }.reference-group { overflow: hidden; border: 1px solid var(--line); border-radius: 10px; background: var(--surface); }.reference-head { display: flex; width: 100%; min-height: 39px; align-items: center; gap: 8px; padding: 7px 11px; color: var(--text-soft); border-radius: 0; background: var(--surface-soft); text-align: left; font-size: 10px; transition: 150ms ease; }.reference-head:hover { color: var(--primary); background: var(--primary-soft); }.reference-head > span { min-width: 0; overflow: hidden; flex: 1; text-overflow: ellipsis; white-space: nowrap; }.reference-head b { padding: 2px 6px; border-radius: 999px; color: var(--primary); background: var(--primary-soft); font-size: 8px; }.reference-head strong { color: var(--text-faint); font-size: 9px; }.reference-head .icon { transition: transform 150ms ease; }.reference-group.expanded .reference-head .icon { transform: rotate(90deg); }.reference-body { padding: 9px; border-top: 1px solid var(--line); }.reference-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 7px; }.reference-tile { min-width: 0; }.reference-content { position: relative; display: flex; min-width: 0; min-height: 50px; align-items: center; gap: 8px; padding: 7px 24px 7px 8px; border: 1px solid transparent; border-radius: 9px; background: var(--surface-soft); cursor: pointer; transition: 150ms ease; }.reference-content:hover { border-color: color-mix(in srgb, var(--primary) 24%, var(--line)); transform: translateY(-1px); }.reference-content > span:nth-child(2) { min-width: 0; }.reference-content strong, .reference-content small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.reference-content strong { color: var(--text); font-size: 9px; }.reference-content small { margin-top: 2px; color: var(--text-faint); font-size: 8px; }.reference-badge { display: grid !important; width: 29px; height: 29px; flex: 0 0 auto; place-items: center; border-radius: 8px; color: var(--primary); background: var(--primary-soft); font-size: 8px; font-weight: 720; }.reference-content > .icon { position: absolute; top: 6px; right: 6px; color: transparent; }.reference-tile input:checked + .reference-content { border-color: color-mix(in srgb, var(--primary) 40%, var(--line)); background: var(--primary-soft); }.reference-tile input:checked + .reference-content > .icon { color: var(--primary); }.reference-note, .reference-empty { margin: 8px 2px 0; color: var(--text-faint); font-size: 8px; }.reference-empty { margin: 0; padding: 12px; text-align: center; }.copy-note { display: flex; align-items: flex-start; gap: 7px; margin: 10px 0 0; padding: 9px 10px; border: 1px solid var(--line); border-radius: 9px; color: var(--text-soft); background: var(--surface-soft); font-size: 9px; }.copy-note .icon { color: var(--primary); }.copy-note code, .no-options code { color: var(--primary); font: inherit; font-weight: 700; }.no-options { display: grid; min-height: 210px; place-items: center; align-content: center; gap: 7px; margin-top: 10px; border: 1px dashed var(--line-strong); border-radius: 12px; color: var(--text-faint); background: var(--surface-soft); text-align: center; }.no-options > .icon { margin-bottom: 3px; color: var(--primary); }.no-options strong { color: var(--text); font-size: 12px; }.no-options span { max-width: 410px; font-size: 9px; }.validation { display: flex; min-height: 35px; align-items: center; gap: 7px; margin: 10px 0 0; padding: 8px 10px; border-radius: 9px; font-size: 9px; }.validation.invalid { color: var(--red); background: var(--red-soft); }.secondary-button, .primary-button { display: inline-flex; min-height: 36px; align-items: center; gap: 6px; padding: 7px 11px; border: 1px solid var(--line); font-size: 10px; }.secondary-button { color: var(--text-soft); background: var(--surface); }.primary-button { color: #fff; border-color: var(--primary); background: var(--primary); }
@media (max-width: 760px) { :global(.outbound-editor-dialog) { width: 100%; }.editor-row { grid-template-columns: 1fr; gap: 6px; padding: 8px 0; }.editor-label { justify-content: flex-start; }.type-control, .reference-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 430px) { .reference-grid { grid-template-columns: 1fr; }.toggle-control { align-items: flex-start; padding-block: 8px; }.reference-divider strong { max-width: 45%; text-align: right; } }
</style>
