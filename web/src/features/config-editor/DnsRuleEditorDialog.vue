<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import AppDialog from "@/components/AppDialog.vue";
import AppIcon from "@/layout/AppIcon.vue";
import { setJsonPath } from "./json";
import type { RouteRuleSetOption } from "./route";
import {
  dnsRuleAction,
  dnsRuleActionOptions,
  dnsRulePayloadText,
  dnsRulePayloadValue,
  dnsRuleServer,
  dnsRuleType,
  dnsRuleTypeOptions,
  parseDnsObjectText,
  type DnsRule,
  type DnsRuleEditorResult,
  type DnsRuleType
} from "./dns";

const props = defineProps<{
  open: boolean;
  rule: DnsRule | null;
  ruleText?: string;
  servers: string[];
  options: RouteRuleSetOption[];
}>();
const emit = defineEmits<{ close: []; submit: [result: DnsRuleEditorResult] }>();

const baseText = ref("{}");
const inlineText = ref("{}");
const selectedRuleSetTags = ref<string[]>([]);
const form = reactive({
  type: "domain_suffix" as DnsRuleType,
  payload: "",
  action: "route",
  server: "",
  invert: false,
  strategy: "",
  disableCache: false,
  rejectMethod: "default",
  rcode: "NOERROR"
});

const serverOptions = computed(() => [...new Set([
  ...(form.server && !props.servers.includes(form.server) ? [form.server] : []),
  ...props.servers
])]);
const selectedSet = computed(() => new Set(selectedRuleSetTags.value));
const validation = computed(() => {
  if (form.type === "inline") return parseDnsObjectText(inlineText.value) ? "" : "完整 JSON 必须是一个有效对象。";
  if (form.type === "rule_set" && !selectedRuleSetTags.value.length) return "请至少选择一个规则集。";
  if (form.type !== "rule_set" && !form.payload.trim()) return "匹配内容不能为空。";
  if ((form.type === "source_port" || form.type === "port") && form.payload.split(",").some((item) => !Number.isInteger(Number(item.trim())))) return "端口必须是整数，多项请使用逗号分隔。";
  if (form.action === "route" && !form.server) return "请选择目标 DNS 服务器。";
  return "";
});

watch(() => props.open, (open, wasOpen) => {
  if (open && !wasOpen) initialize();
}, { immediate: true });

function stringValue(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

function ruleSetTags(rule: DnsRule | null): string[] {
  if (!rule) return [];
  if (typeof rule.rule_set === "string") return rule.rule_set.trim() ? [rule.rule_set.trim()] : [];
  return Array.isArray(rule.rule_set) ? rule.rule_set.flatMap((tag) => typeof tag === "string" && tag.trim() ? [tag.trim()] : []) : [];
}

function initialize(): void {
  const rule = props.rule;
  baseText.value = props.ruleText?.trim() || JSON.stringify(rule ?? {}, null, 2);
  inlineText.value = baseText.value;
  form.type = rule ? dnsRuleType(rule) : "domain_suffix";
  form.payload = rule ? dnsRulePayloadText(rule, form.type) : "";
  form.action = rule ? dnsRuleAction(rule) : "route";
  form.server = rule ? dnsRuleServer(rule) : props.servers[0] ?? "";
  form.invert = rule?.invert === true;
  form.strategy = stringValue(rule?.strategy);
  form.disableCache = rule?.disable_cache === true;
  form.rejectMethod = stringValue(rule?.method, "default");
  form.rcode = stringValue(rule?.rcode, "NOERROR");
  selectedRuleSetTags.value = ruleSetTags(rule);
}

function chooseType(type: DnsRuleType): void {
  form.type = type;
}

function chooseAction(action: string): void {
  form.action = action;
}

function toggleRuleSet(option: RouteRuleSetOption): void {
  if (option.disabled && !selectedSet.value.has(option.tag)) return;
  selectedRuleSetTags.value = selectedSet.value.has(option.tag)
    ? selectedRuleSetTags.value.filter((tag) => tag !== option.tag)
    : [...selectedRuleSetTags.value, option.tag];
}

function buildText(): string {
  if (form.type === "inline") return inlineText.value.trim();
  let text = baseText.value.trim() || "{}";
  const previousType = props.rule ? dnsRuleType(props.rule) : null;
  if (previousType && previousType !== "inline" && previousType !== form.type) text = setJsonPath(text, [previousType], undefined);
  const original = props.rule?.[form.type];
  text = setJsonPath(text, [form.type], dnsRulePayloadValue(form.type, form.payload, selectedRuleSetTags.value, original));
  text = setJsonPath(text, ["invert"], form.invert ? true : undefined);
  text = setJsonPath(text, ["action"], form.action);
  text = setJsonPath(text, ["server"], form.action === "route" ? form.server : undefined);
  text = setJsonPath(text, ["strategy"], (form.action === "route" || form.action === "route-options") ? form.strategy || undefined : undefined);
  text = setJsonPath(text, ["disable_cache"], (form.action === "route" || form.action === "route-options") && form.disableCache ? true : undefined);
  text = setJsonPath(text, ["method"], form.action === "reject" && form.rejectMethod !== "default" ? form.rejectMethod : undefined);
  text = setJsonPath(text, ["rcode"], form.action === "predefined" ? form.rcode : undefined);
  return text;
}

function submit(): void {
  if (validation.value) return;
  const text = buildText();
  const rule = parseDnsObjectText(text);
  if (!rule) return;
  const resourceIds = form.type === "rule_set" ? props.options.flatMap((option) => option.resourceId && selectedSet.value.has(option.tag) ? [option.resourceId] : []) : [];
  emit("submit", { rule, text, resourceIds });
}
</script>

<template>
  <AppDialog class="dns-rule-dialog" :open="open" :title="rule ? '编辑 DNS 规则' : '新增 DNS 规则'" :description="rule ? '更新匹配条件与解析动作，未展示字段会原样保留' : '新规则会添加在规则列表的插入点之后'" initial-focus="select[name=rule-type]" @close="emit('close')">
    <form id="dns-rule-form" class="dns-rule-form" @submit.prevent="submit">
      <label class="editor-row"><span class="editor-label">规则类型<small>match</small></span><select :value="form.type" name="rule-type" aria-label="DNS 规则类型" @change="chooseType(($event.target as HTMLSelectElement).value as DnsRuleType)"><option v-for="option in dnsRuleTypeOptions" :key="option.value" :value="option.value">{{ option.label }}</option></select></label>

      <template v-if="form.type === 'inline'">
        <label class="json-field"><span>完整规则 JSON</span><textarea v-model="inlineText" name="rule-json" spellcheck="false" /></label>
      </template>
      <template v-else-if="form.type === 'rule_set'">
        <div class="rule-set-divider"><span>选择规则集</span><strong>已选 {{ selectedRuleSetTags.length }} 项</strong></div>
        <div class="rule-set-grid">
          <label v-for="option in options" :key="option.id" class="rule-set-option" :class="{ selected: selectedSet.has(option.tag), disabled: option.disabled && !selectedSet.has(option.tag) }">
            <input type="checkbox" :value="option.tag" :checked="selectedSet.has(option.tag)" :disabled="option.disabled && !selectedSet.has(option.tag)" :aria-label="`选择 DNS 规则集 ${option.tag}`" @change="toggleRuleSet(option)" />
            <span><strong>{{ option.name }}</strong><small>{{ option.detail }}</small><AppIcon name="check" :size="12" /></span>
          </label>
        </div>
      </template>
      <label v-else class="editor-row"><span class="editor-label">匹配内容<small>{{ form.type }}</small></span><input v-model="form.payload" name="payload" aria-label="DNS 匹配内容" placeholder="多项使用逗号分隔" /></label>

      <label class="editor-row"><span class="editor-label">反向匹配<small>invert</small></span><span class="toggle-control"><span>对当前匹配条件取反</span><input v-model="form.invert" name="invert" type="checkbox" aria-label="反向匹配" /></span></label>
      <div class="action-divider"><span>匹配后的动作</span></div>
      <div class="action-control">
        <label v-for="option in dnsRuleActionOptions" :key="option.value"><input type="radio" name="dns-rule-action" :value="option.value" :checked="form.action === option.value" @change="chooseAction(option.value)" /><span><AppIcon :name="option.icon" :size="14" /><strong>{{ option.shortLabel }}</strong><small>{{ option.label }}</small></span></label>
      </div>

      <label v-if="form.action === 'route'" class="editor-row"><span class="editor-label">目标服务器<small>server</small></span><select v-model="form.server" name="target-server" aria-label="目标 DNS 服务器"><option value="">请选择 DNS 服务器</option><option v-for="tag in serverOptions" :key="tag" :value="tag">{{ tag }}</option></select></label>
      <label v-if="form.action === 'route' || form.action === 'route-options'" class="editor-row"><span class="editor-label">解析策略<small>strategy</small></span><select v-model="form.strategy" name="strategy"><option value="">继承 DNS 通用设置</option><option value="prefer_ipv4">优先 IPv4</option><option value="prefer_ipv6">优先 IPv6</option><option value="ipv4_only">仅 IPv4</option><option value="ipv6_only">仅 IPv6</option></select></label>
      <label v-if="form.action === 'route' || form.action === 'route-options'" class="editor-row"><span class="editor-label">跳过缓存<small>disable_cache</small></span><span class="toggle-control"><span>该规则的解析结果不写入缓存</span><input v-model="form.disableCache" name="disable-cache" type="checkbox" aria-label="跳过 DNS 缓存" /></span></label>
      <label v-if="form.action === 'reject'" class="editor-row"><span class="editor-label">拒绝方式<small>method</small></span><select v-model="form.rejectMethod" name="reject-method"><option value="default">默认响应</option><option value="drop">静默丢弃</option></select></label>
      <label v-if="form.action === 'predefined'" class="editor-row"><span class="editor-label">响应码<small>rcode</small></span><select v-model="form.rcode" name="rcode"><option value="NOERROR">NOERROR</option><option value="NXDOMAIN">NXDOMAIN</option><option value="REFUSED">REFUSED</option><option value="SERVFAIL">SERVFAIL</option></select></label>

      <p class="preserve-note"><AppIcon name="info" :size="14" />停用规则只保存在草稿编辑状态中，不会写入实际 DNS JSON。</p>
      <p v-if="validation" class="validation" role="alert"><AppIcon name="info" :size="14" />{{ validation }}</p>
    </form>
    <template #footer>
      <button class="secondary-button" type="button" @click="emit('close')">取消</button>
      <button class="primary-button" type="submit" form="dns-rule-form" :disabled="Boolean(validation)"><AppIcon :name="rule ? 'save' : 'plus'" :size="14" />{{ rule ? "保存" : "确认添加" }}</button>
    </template>
  </AppDialog>
</template>

<style scoped>
:global(.dns-rule-dialog) { width: min(920px, calc(100vw - 32px)); }.dns-rule-form { display: grid; }.editor-row { display: grid; min-width: 0; grid-template-columns: 140px minmax(0, 1fr); align-items: center; gap: 18px; padding: 7px 0; }.editor-label { display: flex; min-width: 0; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 2px 8px; color: var(--text); font-size: var(--font-size-base); font-weight: 680; }.editor-label small { max-width: 100%; overflow-wrap: anywhere; color: var(--text-faint); font-size: var(--font-size-sm); font-weight: 520; }.editor-row input, .editor-row select { width: 100%; min-width: 0; }.toggle-control { display: flex; min-height: 42px; align-items: center; justify-content: space-between; gap: 12px; padding: 0 11px; border: 1px solid var(--line-strong); border-radius: var(--radius-sm); color: var(--text-faint); background: var(--surface-soft); font-size: var(--font-size-base); }.toggle-control input { width: 34px; min-height: 19px; accent-color: var(--primary); }.action-divider, .rule-set-divider { display: flex; min-height: 38px; align-items: center; gap: 10px; margin-top: 7px; color: var(--text-soft); font-size: var(--font-size-xs); font-weight: 680; }.action-divider::before, .action-divider::after, .rule-set-divider::before, .rule-set-divider::after { height: 1px; content: ""; background: var(--line); }.action-divider::before, .rule-set-divider::before { width: 22px; }.action-divider::after, .rule-set-divider::after { flex: 1; }.rule-set-divider strong { order: 3; color: var(--primary); font-size: var(--font-size-base); }.action-control { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 7px; margin-bottom: 8px; }.action-control input, .rule-set-option > input { position: absolute; width: 1px; height: 1px; overflow: hidden; opacity: 0; }.action-control label > span { display: grid; min-height: 67px; grid-template-columns: auto minmax(0, 1fr); grid-template-rows: auto auto; align-content: center; gap: 1px 7px; padding: 8px 10px; border: 1px solid var(--line); border-radius: 10px; color: var(--text-faint); background: var(--surface-soft); cursor: pointer; }.action-control label > span > .icon { grid-row: span 2; align-self: center; }.action-control strong { color: var(--text); font-size: var(--font-size-base); }.action-control small { overflow: hidden; font-size: var(--font-size-xs); text-overflow: ellipsis; white-space: nowrap; }.action-control input:checked + span { color: var(--primary); border-color: color-mix(in srgb, var(--primary) 40%, var(--line)); background: var(--primary-soft); }.action-control input:focus-visible + span, .rule-set-option input:focus-visible + span { outline: 3px solid color-mix(in srgb, var(--primary) 22%, transparent); }.rule-set-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 7px; margin-bottom: 8px; }.rule-set-option { min-width: 0; }.rule-set-option > span { position: relative; display: block; min-height: 51px; padding: 8px 26px 8px 10px; border: 1px solid var(--line); border-radius: 9px; background: var(--surface-soft); cursor: pointer; }.rule-set-option strong, .rule-set-option small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.rule-set-option strong { color: var(--text); font-size: var(--font-size-base); }.rule-set-option small { margin-top: 2px; color: var(--text-faint); font-size: var(--font-size-xs); }.rule-set-option .icon { position: absolute; top: 8px; right: 8px; color: transparent; }.rule-set-option.selected > span { border-color: color-mix(in srgb, var(--primary) 40%, var(--line)); background: var(--primary-soft); }.rule-set-option.selected .icon { color: var(--primary); }.rule-set-option.disabled { opacity: .5; }.json-field { display: grid; gap: 6px; padding: 7px 0; color: var(--text); font-size: var(--font-size-base); font-weight: 680; }.json-field textarea { min-height: 210px; resize: vertical; font: var(--font-size-base)/1.6 ui-monospace, SFMono-Regular, Consolas, monospace; }.preserve-note, .validation { display: flex; align-items: flex-start; gap: 7px; margin: 10px 0 0; padding: 9px 10px; border-radius: 9px; font-size: var(--font-size-xs); }.preserve-note { color: var(--text-soft); border: 1px solid var(--line); background: var(--surface-soft); }.preserve-note .icon { color: var(--primary); }.validation { color: var(--red); background: var(--red-soft); }.secondary-button, .primary-button { display: inline-flex; min-height: 36px; align-items: center; gap: 6px; padding: 7px 11px; border: 1px solid var(--line); font-size: var(--font-size-sm); }.secondary-button { color: var(--text-soft); background: var(--surface); }.primary-button { color: #fff; border-color: var(--primary); background: var(--primary); }
@media (max-width: 760px) { :global(.dns-rule-dialog) { width: 100%; }.editor-row { grid-template-columns: 1fr; gap: 6px; padding: 8px 0; }.editor-label { justify-content: flex-start; }.action-control, .rule-set-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 430px) { .action-control, .rule-set-grid { grid-template-columns: 1fr; }.toggle-control { align-items: flex-start; padding-block: 8px; } }
</style>
