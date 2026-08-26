<script setup lang="ts">
import { computed, ref, watch } from "vue";
import AppDialog from "@/components/AppDialog.vue";
import AppIcon from "@/layout/AppIcon.vue";
import { setJsonPath } from "./json";
import {
  parseRouteRuleText,
  routeRuleAction,
  routeRuleActionOptions,
  routeRulePayloadText,
  routeRulePayloadValue,
  routeRuleSetTags,
  routeRuleType,
  routeRuleTypeOptions,
  type RouteRule,
  type RouteRuleEditorResult,
  type RouteRuleSetOption,
  type RouteRuleType
} from "./route";

const props = withDefaults(defineProps<{
  open: boolean;
  rule: RouteRule | null;
  ruleText?: string;
  options: RouteRuleSetOption[];
  inbounds?: string[];
  outbounds: string[];
  dnsServers?: string[];
  defaultOutbound?: string;
}>(), { inbounds: () => [], dnsServers: () => [] });
const emit = defineEmits<{ close: []; submit: [result: RouteRuleEditorResult] }>();

const routeOptionKeys = [
  "override_address", "override_port", "network_strategy", "fallback_delay",
  "udp_disable_domain_unmapping", "udp_connect", "tls_fragment", "tls_fragment_fallback_delay"
] as const;
const knownActions = new Set<string>(routeRuleActionOptions.map((option) => option.value));
const query = ref("");
const type = ref<RouteRuleType>("rule_set");
const action = ref("route");
const invert = ref(false);
const payload = ref("");
const privateMatch = ref(true);
const inlineText = ref("{}");
const selectedIds = ref<string[]>([]);
const outbound = ref("");
const routeOptionsText = ref("{}");
const rejectMethod = ref("default");
const sniffers = ref<string[]>([]);
const strategy = ref("");
const dnsServer = ref("");
const selectedSet = computed(() => new Set(selectedIds.value));
const filteredOptions = computed(() => {
  const keyword = query.value.trim().toLocaleLowerCase("zh-CN");
  if (!keyword) return props.options;
  return props.options.filter((option) => `${option.name} ${option.tag} ${option.detail}`.toLocaleLowerCase("zh-CN").includes(keyword));
});
const actionOptions = computed<Array<{ value: string; label: string; shortLabel: string; icon: string }>>(() => {
  const values: Array<{ value: string; label: string; shortLabel: string; icon: string }> = routeRuleActionOptions.map((option) => ({ ...option }));
  if (action.value && !values.some((item) => item.value === action.value)) {
    values.push({ value: action.value, label: action.value, shortLabel: action.value, icon: "code" });
  }
  return values;
});
const allOutbounds = computed(() => withCurrentOption(props.outbounds, outbound.value));
const allDnsServers = computed(() => withCurrentOption(props.dnsServers, dnsServer.value));
const allInbounds = computed(() => withCurrentOption(props.inbounds, payload.value));
const snifferOptions = ["http", "tls", "quic", "stun", "dns", "bittorrent", "dtls", "ssh", "rdp", "ntp"];
const placeholders: Partial<Record<RouteRuleType, string>> = {
  network: "tcp 或 udp",
  protocol: "例如 dns、quic、http",
  domain: "example.com",
  domain_suffix: "example.com",
  domain_keyword: "keyword",
  domain_regex: "^.+\\.example\\.com$",
  source_ip_cidr: "192.168.1.0/24",
  ip_cidr: "172.16.0.0/12",
  source_port: "443",
  source_port_range: "1000:2000",
  port: "443",
  port_range: "80:443",
  process_name: "chrome",
  process_path: "/usr/bin/chrome",
  process_path_regex: ".*chrome$"
};

function withCurrentOption(values: string[], current: string): string[] {
  return [...new Set(current && !values.includes(current) ? [current, ...values] : values)];
}

function parseObject(text: string): Record<string, unknown> | null {
  try {
    const value: unknown = JSON.parse(text);
    return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : null;
  } catch {
    return null;
  }
}

function routeOptionsFromRule(rule: RouteRule | null): string {
  if (!rule) return JSON.stringify({ override_address: "1.1.1.1" }, null, 2);
  const fields = Object.fromEntries(routeOptionKeys.flatMap((key) => Object.prototype.hasOwnProperty.call(rule, key) ? [[key, rule[key]]] : []));
  return JSON.stringify(Object.keys(fields).length ? fields : { override_address: "1.1.1.1" }, null, 2);
}

const validation = computed(() => {
  if (type.value === "rule_set" && !selectedIds.value.length) return "请选择至少一个规则集。";
  if (type.value === "inline" && !parseRouteRuleText(inlineText.value)) return "内联规则必须是有效的 JSON 对象。";
  if (!["rule_set", "inline", "ip_is_private"].includes(type.value) && !payload.value.trim()) return "请填写规则的匹配内容。";
  if ((type.value === "source_port" || type.value === "port") && payload.value.split(",").some((item) => !Number.isInteger(Number(item.trim())) || Number(item.trim()) < 0 || Number(item.trim()) > 65535)) return "端口必须是 0 到 65535 之间的整数，多个值用逗号分隔。";
  if (action.value === "route" && !outbound.value) return "请选择规则命中后的出站。";
  if (action.value === "route-options") {
    const value = parseObject(routeOptionsText.value);
    if (!value) return "路由选项必须是有效的 JSON 对象。";
    const reserved = new Set(["action", "invert", "outbound", "method", "sniffer", "strategy", "server", ...routeRuleTypeOptions.map((option) => option.value)]);
    if (Object.keys(value).some((key) => reserved.has(key))) return "路由选项 JSON 不能覆盖匹配条件或 action 字段。";
  }
  return "";
});

function initialize(): void {
  query.value = "";
  type.value = props.rule ? routeRuleType(props.rule) : "rule_set";
  payload.value = routeRulePayloadText(props.rule, type.value);
  privateMatch.value = payload.value !== "false";
  inlineText.value = type.value === "inline"
    ? props.ruleText?.trim() || JSON.stringify(props.rule ?? {}, null, 2)
    : "{}";
  const tags = routeRuleSetTags(props.rule);
  selectedIds.value = tags.flatMap((tag) => {
    const option = props.options.find((candidate) => candidate.tag === tag);
    return option ? [option.id] : [];
  });
  invert.value = props.rule?.invert === true;
  action.value = routeRuleAction(props.rule);
  const currentOutbound = typeof props.rule?.outbound === "string" ? props.rule.outbound : "";
  outbound.value = currentOutbound || props.defaultOutbound || props.outbounds[0] || "";
  routeOptionsText.value = routeOptionsFromRule(props.rule);
  rejectMethod.value = typeof props.rule?.method === "string" ? props.rule.method : "default";
  sniffers.value = Array.isArray(props.rule?.sniffer) ? props.rule.sniffer.filter((item): item is string => typeof item === "string") : [];
  strategy.value = typeof props.rule?.strategy === "string" ? props.rule.strategy : "";
  dnsServer.value = typeof props.rule?.server === "string" ? props.rule.server : "";
}

watch(() => props.open, (open, wasOpen) => { if (open && !wasOpen) initialize(); }, { immediate: true });
watch(() => props.options, () => {
  if (!props.open || !props.rule) return;
  const optionIds = new Set(props.options.map((option) => option.id));
  const selected = new Set(selectedIds.value.filter((id) => optionIds.has(id)));
  for (const tag of routeRuleSetTags(props.rule)) {
    const option = props.options.find((candidate) => candidate.tag === tag);
    if (option) selected.add(option.id);
  }
  selectedIds.value = [...selected];
});

function changeType(event: Event): void {
  const next = (event.currentTarget as HTMLSelectElement).value as RouteRuleType;
  type.value = next;
  selectedIds.value = [];
  payload.value = next === "clash_mode" ? "global" : "";
  privateMatch.value = true;
  if (next === "inline") inlineText.value = JSON.stringify({ domain_suffix: ["example.com"] }, null, 2);
}

function clearActionFields(text: string): string {
  let next = text;
  for (const key of ["outbound", "method", "sniffer", "strategy", "server", ...routeOptionKeys]) {
    next = setJsonPath(next, [key], undefined);
  }
  return next;
}

function submit(): void {
  if (validation.value) return;
  const selectedOptions = selectedIds.value.flatMap((id) => {
    const option = props.options.find((candidate) => candidate.id === id);
    return option ? [option] : [];
  });
  const tags = [...new Set(selectedOptions.map((option) => option.tag))];
  const originalType = routeRuleType(props.rule);
  let text = type.value === "inline"
    ? inlineText.value
    : props.ruleText?.trim() || JSON.stringify(props.rule ?? {}, null, 2);
  if (type.value !== "inline") {
    if (originalType !== "inline" && originalType !== type.value) text = setJsonPath(text, [originalType], undefined);
    const original = originalType === type.value ? props.rule?.[type.value] : undefined;
    const matchValue = routeRulePayloadValue(type.value, type.value === "ip_is_private" ? String(privateMatch.value) : payload.value, tags, original);
    text = setJsonPath(text, [type.value], matchValue);
  }
  text = setJsonPath(text, ["invert"], invert.value || undefined);
  text = setJsonPath(text, ["action"], action.value);
  if (knownActions.has(action.value)) {
    text = clearActionFields(text);
    if (action.value === "route") text = setJsonPath(text, ["outbound"], outbound.value);
    if (action.value === "route-options") {
      for (const [key, value] of Object.entries(parseObject(routeOptionsText.value) ?? {})) text = setJsonPath(text, [key], value);
    }
    if (action.value === "reject") text = setJsonPath(text, ["method"], rejectMethod.value);
    if (action.value === "sniff") text = setJsonPath(text, ["sniffer"], sniffers.value.length ? [...sniffers.value] : undefined);
    if (action.value === "resolve") {
      text = setJsonPath(text, ["strategy"], strategy.value || undefined);
      text = setJsonPath(text, ["server"], dnsServer.value || undefined);
    }
  }
  const rule = parseRouteRuleText(text);
  if (!rule) return;
  emit("submit", {
    rule,
    text,
    resourceIds: selectedOptions.flatMap((option) => option.resourceId ? [option.resourceId] : [])
  });
}
</script>

<template>
  <AppDialog class="route-rule-editor-dialog" :open="open" :title="rule ? '编辑规则' : '新增规则'" description="设置匹配条件、执行动作以及对应的出站或 DNS 参数" :initial-focus="rule ? 'select[name=route-rule-type]' : 'input[type=search]'" @close="emit('close')">
    <form id="route-rule-editor-form" class="route-rule-editor-form" @submit.prevent="submit">
      <label class="editor-row"><span class="editor-label">规则类型<small>type</small></span><select :value="type" name="route-rule-type" aria-label="规则类型" @change="changeType"><option v-for="item in routeRuleTypeOptions" :key="item.value" :value="item.value">{{ item.label }}</option></select></label>
      <div class="editor-row"><span class="editor-label">规则动作<small>action</small></span><div class="action-control" :class="{ custom: actionOptions.length > 6 }">
        <label v-for="item in actionOptions" :key="item.value"><input v-model="action" type="radio" name="route-action" :value="item.value" /><span><AppIcon :name="item.icon" :size="13" />{{ item.shortLabel }}</span></label>
      </div></div>
      <div class="editor-row"><span class="editor-label">反向匹配<small>invert</small></span><label class="toggle-control"><span>匹配不符合当前条件的流量</span><input v-model="invert" type="checkbox" aria-label="反向匹配" /></label></div>

      <template v-if="type === 'rule_set'">
        <div class="ruleset-divider"><span>全部规则集</span><strong>{{ options.length }} 项 · 已选 {{ selectedIds.length }} 项</strong></div>
        <label class="ruleset-search"><AppIcon name="search" :size="14" /><input v-model="query" type="search" aria-label="搜索规则集" placeholder="搜索规则集名称或 tag" /><span>{{ filteredOptions.length }} / {{ options.length }}</span></label>
        <div class="ruleset-options" aria-label="规则集选择">
          <label v-for="option in filteredOptions" :key="option.id" class="ruleset-option" :class="{ disabled: option.disabled && !selectedSet.has(option.id), missing: option.kind === 'missing' }">
            <input v-model="selectedIds" type="checkbox" :value="option.id" :disabled="option.disabled && !selectedSet.has(option.id)" :aria-label="`选择规则集 ${option.name}`" />
            <span class="ruleset-option-content"><span class="ruleset-copy"><strong>{{ option.name }}</strong><small v-if="option.tag !== option.name">{{ option.tag }}</small><em>{{ option.disabled ? "已停用 · " : "" }}{{ option.detail }}</em></span><AppIcon name="check" :size="13" /></span>
          </label>
          <p v-if="!filteredOptions.length" class="ruleset-empty">{{ options.length ? "没有匹配的规则集" : "尚无规则集，请先到规则集中心导入资源" }}</p>
        </div>
      </template>
      <label v-else-if="type === 'inbound'" class="editor-row payload-row"><span class="editor-label">入站标签<small>payload</small></span><select v-model="payload" aria-label="入站标签"><option value="" disabled>请选择入站</option><option v-for="tag in allInbounds" :key="tag" :value="tag">{{ tag }}</option></select></label>
      <div v-else-if="type === 'clash_mode'" class="editor-row payload-row"><span class="editor-label">Clash 模式<small>payload</small></span><div class="segmented two"><label><input v-model="payload" type="radio" value="global" /><span>全局</span></label><label><input v-model="payload" type="radio" value="direct" /><span>直连</span></label></div></div>
      <div v-else-if="type === 'ip_is_private'" class="editor-row payload-row"><span class="editor-label">是否为私有 IP<small>payload</small></span><label class="toggle-control"><span>匹配私有网络地址</span><input v-model="privateMatch" type="checkbox" aria-label="匹配私有 IP" /></label></div>
      <label v-else-if="type === 'inline'" class="editor-row payload-row align-start"><span class="editor-label">内联规则<small>payload</small></span><textarea v-model="inlineText" class="code-input" aria-label="内联规则 JSON" spellcheck="false" /></label>
      <label v-else class="editor-row payload-row"><span class="editor-label">匹配内容<small>payload</small></span><input v-model="payload" type="text" :inputmode="type === 'source_port' || type === 'port' ? 'numeric' : undefined" :placeholder="placeholders[type] || '输入匹配内容，多个值用逗号分隔'" aria-label="匹配内容" /></label>

      <div class="action-panel">
        <label v-if="action === 'route'" class="editor-row"><span class="editor-label">出站标签<small>outbound</small></span><select v-model="outbound" aria-label="目标出站"><option value="" disabled>请选择目标出站</option><option v-for="tag in allOutbounds" :key="tag" :value="tag">{{ tag }}</option></select></label>
        <label v-else-if="action === 'route-options'" class="editor-row align-start"><span class="editor-label">路由选项<small>JSON</small></span><textarea v-model="routeOptionsText" class="code-input" aria-label="路由选项 JSON" spellcheck="false" /></label>
        <div v-else-if="action === 'reject'" class="editor-row"><span class="editor-label">拒绝方式<small>method</small></span><div class="segmented three"><label><input v-model="rejectMethod" type="radio" value="default" /><span>默认</span></label><label><input v-model="rejectMethod" type="radio" value="drop" /><span>静默丢弃</span></label><label><input v-model="rejectMethod" type="radio" value="reply" /><span>返回响应</span></label></div></div>
        <div v-else-if="action === 'hijack-dns'" class="action-empty"><AppIcon name="dns" :size="23" /><strong>DNS 请求将被劫持到 sing-box DNS 模块</strong><span>此动作不需要额外参数。</span></div>
        <div v-else-if="action === 'sniff'" class="editor-row align-start"><span class="editor-label">探测器<small>留空表示全部</small></span><div class="sniffer-grid"><label v-for="item in snifferOptions" :key="item"><input v-model="sniffers" type="checkbox" :value="item" /><span>{{ item }}<AppIcon name="check" :size="10" /></span></label></div></div>
        <template v-else-if="action === 'resolve'">
          <label class="editor-row"><span class="editor-label">解析策略<small>strategy</small></span><select v-model="strategy" aria-label="解析策略"><option value="">默认</option><option value="prefer_ipv4">IPv4 优先</option><option value="prefer_ipv6">IPv6 优先</option><option value="ipv4_only">仅 IPv4</option><option value="ipv6_only">仅 IPv6</option></select></label>
          <label class="editor-row"><span class="editor-label">DNS 服务器<small>server</small></span><select v-model="dnsServer" aria-label="DNS 服务器"><option value="">由 DNS 路由规则决定</option><option v-for="tag in allDnsServers" :key="tag" :value="tag">{{ tag }}</option></select></label>
        </template>
        <p v-else class="unknown-action">该动作的专有字段会按原始 JSON 保留。</p>
      </div>
      <p v-if="rule" class="preserve-note"><AppIcon name="info" :size="14" />未在此表单展示的原有匹配字段会继续保留。</p>
      <p v-if="validation" class="validation" role="alert"><AppIcon name="info" :size="14" />{{ validation }}</p>
    </form>
    <template #footer>
      <button class="secondary-button" type="button" @click="emit('close')">取消</button>
      <button class="primary-button" type="submit" form="route-rule-editor-form" :disabled="Boolean(validation)"><AppIcon :name="rule ? 'save' : 'check'" :size="14" />{{ rule ? "保存规则" : "确认添加" }}</button>
    </template>
  </AppDialog>
</template>

<style scoped>
:global(.app-dialog.route-rule-editor-dialog) { width: min(900px, calc(100vw - 32px)); }
:global(.route-rule-editor-dialog) { width: min(900px, calc(100vw - 32px)); }.route-rule-editor-form { display: grid; }.editor-row { display: grid; min-width: 0; grid-template-columns: 142px minmax(0, 1fr); align-items: center; gap: 16px; padding: 7px 0; }.editor-row.align-start { align-items: start; }.editor-label { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; color: var(--text); font-size: 10px; font-weight: 680; }.editor-label small { color: var(--text-faint); font-size: 8px; font-weight: 520; }.action-control, .segmented { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); overflow: hidden; padding: 3px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-soft); }.action-control.custom { grid-template-columns: repeat(4, minmax(0, 1fr)); }.segmented.two { grid-template-columns: repeat(2, minmax(0, 1fr)); }.segmented.three { grid-template-columns: repeat(3, minmax(0, 1fr)); }.action-control input, .segmented input, .ruleset-option > input, .sniffer-grid input { position: absolute; width: 1px; height: 1px; overflow: hidden; opacity: 0; }.action-control label > span, .segmented label > span { display: flex; min-height: 34px; align-items: center; justify-content: center; gap: 5px; padding: 4px; border-radius: 7px; color: var(--text-faint); font-size: 8px; font-weight: 650; text-align: center; cursor: pointer; }.action-control input:checked + span, .segmented input:checked + span { color: var(--primary); background: var(--surface); box-shadow: 0 2px 9px rgb(34 49 83 / 9%); }.action-control input:focus-visible + span, .segmented input:focus-visible + span, .ruleset-option input:focus-visible + span, .sniffer-grid input:focus-visible + span { outline: 3px solid color-mix(in srgb, var(--primary) 22%, transparent); }.toggle-control { display: flex; min-height: 40px; align-items: center; justify-content: space-between; gap: 12px; padding: 0 11px; border: 1px solid var(--line); border-radius: 9px; color: var(--text-faint); background: var(--surface-soft); font-size: 8px; }.toggle-control input { width: 34px; min-height: 19px; accent-color: var(--primary); }.payload-row { margin-top: 5px; }.code-input { width: 100%; min-height: 118px; color: #c9d5eb; border-color: #2a3447; background: #111827; font: 10px/1.65 ui-monospace, Consolas, monospace; resize: vertical; }.ruleset-divider { display: flex; min-height: 41px; align-items: center; gap: 12px; margin-top: 8px; color: var(--text-soft); font-size: 9px; font-weight: 680; }.ruleset-divider::before, .ruleset-divider::after { height: 1px; content: ""; background: var(--line); }.ruleset-divider::before { width: 22px; }.ruleset-divider::after { flex: 1; }.ruleset-divider strong { order: 3; color: var(--primary); font-size: 8px; }.ruleset-search { display: flex; min-height: 38px; align-items: center; gap: 7px; margin-bottom: 8px; padding: 0 10px; border: 1px solid var(--line); border-radius: 9px; color: var(--text-faint); background: var(--surface-soft); }.ruleset-search:focus-within { border-color: var(--primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 14%, transparent); }.ruleset-search input { min-width: 0; min-height: 34px; flex: 1; padding: 0; border: 0; background: transparent; font-size: 9px; }.ruleset-search input:focus { outline: 0; }.ruleset-search > span { font-size: 8px; font-variant-numeric: tabular-nums; }.ruleset-options { display: grid; max-height: min(360px, 42vh); grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; overflow-y: auto; padding: 1px 1px 4px; scrollbar-width: thin; }.ruleset-option { min-width: 0; }.ruleset-option-content { position: relative; display: block; min-width: 0; min-height: 61px; padding: 10px 28px 9px 10px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-soft); cursor: pointer; transition: 150ms ease; }.ruleset-option-content:hover { border-color: color-mix(in srgb, var(--primary) 34%, var(--line)); transform: translateY(-1px); }.ruleset-copy { min-width: 0; }.ruleset-copy strong, .ruleset-copy small, .ruleset-copy em { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.ruleset-copy strong { color: var(--text); font-size: 9px; }.ruleset-copy small { margin-top: 2px; color: var(--text-soft); font: 8px ui-monospace, monospace; }.ruleset-copy em { margin-top: 4px; color: var(--text-faint); font-size: 8px; font-style: normal; }.ruleset-option-content > .icon { position: absolute; top: 7px; right: 7px; color: transparent; }.ruleset-option input:checked + .ruleset-option-content { border-color: color-mix(in srgb, var(--primary) 45%, var(--line)); background: var(--primary-soft); }.ruleset-option input:checked + .ruleset-option-content > .icon { color: var(--primary); }.ruleset-option.disabled .ruleset-option-content { opacity: .55; cursor: not-allowed; }.ruleset-option.missing .ruleset-option-content { border-style: dashed; }.ruleset-empty { min-height: 100px; grid-column: 1 / -1; display: grid; place-items: center; margin: 0; color: var(--text-faint); border: 1px dashed var(--line-strong); border-radius: 10px; background: var(--surface-soft); font-size: 9px; }.action-panel { margin-top: 11px; padding: 9px 12px; border: 1px solid var(--line); border-radius: 11px; background: var(--surface-soft); }.action-panel select, .action-panel input, .action-panel textarea { background-color: var(--surface); }.action-empty { display: grid; min-height: 105px; place-items: center; align-content: center; gap: 5px; color: var(--text-faint); text-align: center; }.action-empty .icon { margin-bottom: 3px; color: var(--primary); }.action-empty strong { color: var(--text); font-size: 10px; }.action-empty span, .unknown-action { font-size: 8px; }.sniffer-grid { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 6px; }.sniffer-grid span { position: relative; display: flex; min-height: 32px; align-items: center; justify-content: center; border: 1px solid var(--line); border-radius: 8px; color: var(--text-faint); background: var(--surface); font-size: 8px; cursor: pointer; }.sniffer-grid .icon { position: absolute; top: 4px; right: 4px; color: transparent; }.sniffer-grid input:checked + span { color: var(--primary); border-color: color-mix(in srgb, var(--primary) 38%, var(--line)); background: var(--primary-soft); }.sniffer-grid input:checked + span .icon { color: var(--primary); }.unknown-action { margin: 0; padding: 15px; color: var(--text-faint); text-align: center; }.preserve-note, .validation { display: flex; align-items: flex-start; gap: 7px; margin: 10px 0 0; padding: 9px 10px; border-radius: 9px; font-size: 9px; }.preserve-note { color: var(--text-soft); border: 1px solid var(--line); background: var(--surface-soft); }.preserve-note .icon { color: var(--primary); }.validation { color: var(--red); background: var(--red-soft); }.secondary-button, .primary-button { display: inline-flex; min-height: 36px; align-items: center; gap: 6px; padding: 7px 11px; border: 1px solid var(--line); font-size: 10px; }.secondary-button { color: var(--text-soft); background: var(--surface); }.primary-button { color: #fff; border-color: var(--primary); background: var(--primary); }
@media (max-width: 720px) { :global(.route-rule-editor-dialog) { width: 100%; }.editor-row { grid-template-columns: 1fr; gap: 6px; padding: 8px 0; }.editor-label { justify-content: flex-start; }.action-control { grid-template-columns: repeat(3, minmax(0, 1fr)); }.ruleset-options { grid-template-columns: repeat(2, minmax(0, 1fr)); }.sniffer-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
@media (max-width: 430px) { .action-control, .action-control.custom, .segmented.three { grid-template-columns: repeat(2, minmax(0, 1fr)); }.ruleset-options, .sniffer-grid { grid-template-columns: 1fr; }.ruleset-divider strong { max-width: 48%; text-align: right; }.action-panel { padding: 8px; } }
@media (max-width: 720px) { :global(.app-dialog.route-rule-editor-dialog) { width: 100%; } }
</style>
