<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import AppToast from "@/components/AppToast.vue";
import AppIcon from "@/layout/AppIcon.vue";
import type { Draft } from "@/features/profiles/types";
import { useRuleResourceStore } from "@/features/rule-resources/store";
import DnsRuleEditorDialog from "./DnsRuleEditorDialog.vue";
import DnsServerEditorDialog from "./DnsServerEditorDialog.vue";
import FakeIpDnsRuleDialog from "./FakeIpDnsRuleDialog.vue";
import {
  createDnsStateId,
  dnsArrayItemText,
  dnsRuleAction,
  dnsRuleActionOptions,
  dnsRulePayloadText,
  dnsRuleServer,
  dnsRuleType,
  dnsRules,
  dnsServerAddress,
  dnsServerDetour,
  dnsServers,
  dnsServerTags,
  dnsServerType,
  ensureDefaultFakeIpDnsRuleText,
  fakeIpDnsRuleParts,
  insertDnsRuleText,
  isDefaultFakeIpDnsRule,
  isDnsObject,
  parseDnsObjectText,
  reconcileDnsEditorState,
  removeDnsArrayItem,
  renameDnsServerInRuleText,
  renameDnsServerTagInBuffer,
  updateDnsBoolean,
  updateDnsString,
  upsertDnsArrayItemText,
  type DnsEditorState,
  type DnsObject,
  type DnsPanelSave,
  type DnsRule,
  type DnsRuleEditorResult,
  type DnsServer,
  type DnsServerEditorResult
} from "./dns";
import {
  buildRouteRuleSetOptions,
  mergeSelectedRuleBindings,
  routeOutboundTags,
  type RouteRuleSetOption
} from "./route";

const props = defineProps<{ draft: Draft; error?: string }>();
const emit = defineEmits<{ editJson: []; save: [payload: DnsPanelSave] }>();
const resourceStore = useRuleResourceStore();
const serverEditorOpen = ref(false);
const editingServerIndex = ref<number | null>(null);
const ruleEditorOpen = ref(false);
const fakeIpRuleEditorOpen = ref(false);
const editingRuleKey = ref<string | null>(null);
const resourceError = ref("");
const notice = ref("");
const noticeTone = ref<"info" | "error" | "success">("info");

interface DnsRuleViewItem {
  key: string;
  enabled: boolean;
  text: string;
  rule: DnsRule | null;
  sourceIndex?: number;
  disabledId?: string;
  builtIn?: "fakeip";
}

interface DnsRuleMarker {
  key: "marker";
  marker: true;
}

const sourceDns = computed<DnsObject>(() => isDnsObject(props.draft.config.dns) ? props.draft.config.dns : {});
const sourceBuffer = computed(() => {
  const source = props.draft.buffers.dns;
  return source?.trim() ? source : JSON.stringify(sourceDns.value, null, 2);
});
const normalizedBuffer = computed(() => ensureDefaultFakeIpDnsRuleText(sourceBuffer.value, props.draft.editorState?.dns));
const dns = computed<DnsObject>(() => parseDnsObjectText(normalizedBuffer.value) ?? sourceDns.value);
const invalidDnsObject = computed(() => props.draft.config.dns !== undefined && !isDnsObject(props.draft.config.dns));
const rawServers = computed(() => dnsServers(dns.value));
const activeRules = computed(() => dnsRules(dns.value));
const serverTags = computed(() => dnsServerTags(dns.value));
const outbounds = computed(() => routeOutboundTags(props.draft.config));
const invalidServers = computed(() => dns.value.servers !== undefined && !Array.isArray(dns.value.servers));
const invalidRules = computed(() => dns.value.rules !== undefined && !Array.isArray(dns.value.rules));
const generalInteractionDisabled = computed(() => Boolean(props.error) || invalidDnsObject.value);
const serverInteractionDisabled = computed(() => generalInteractionDisabled.value || invalidServers.value);
const ruleInteractionDisabled = computed(() => generalInteractionDisabled.value || invalidRules.value);
const disableCache = computed(() => dns.value.disable_cache === true);
const disableExpire = computed(() => dns.value.disable_expire === true);
const independentCache = computed(() => dns.value.independent_cache === true);
const finalServer = computed(() => typeof dns.value.final === "string" ? dns.value.final : "");
const strategy = computed(() => typeof dns.value.strategy === "string" ? dns.value.strategy : "");
const clientSubnet = computed(() => typeof dns.value.client_subnet === "string" ? dns.value.client_subnet : "");
const finalOptions = computed(() => [...new Set([
  ...(finalServer.value && !serverTags.value.includes(finalServer.value) ? [finalServer.value] : []),
  ...serverTags.value
])]);
const editorState = computed(() => reconcileDnsEditorState(props.draft.editorState?.dns, dns.value));

const serverItems = computed(() => rawServers.value.map((value, index) => ({
  index,
  server: isDnsObject(value) ? value : null,
  text: dnsArrayItemText(bufferText(), "servers", index) ?? JSON.stringify(value, null, 2)
})));
const editingServer = computed<DnsServer | null>(() => {
  if (editingServerIndex.value === null) return null;
  return serverItems.value.find((item) => item.index === editingServerIndex.value)?.server ?? null;
});
const editingServerText = computed(() => editingServerIndex.value === null ? undefined : serverItems.value.find((item) => item.index === editingServerIndex.value)?.text);
const reservedServerTags = computed(() => serverItems.value.flatMap((item) => item.server && typeof item.server.tag === "string" && item.index !== editingServerIndex.value ? [item.server.tag] : []));

const ruleItems = computed<DnsRuleViewItem[]>(() => {
  const active = activeRules.value.map((rule, index): DnsRuleViewItem => ({
    key: `active:${index}`,
    enabled: true,
    sourceIndex: index,
    text: dnsArrayItemText(bufferText(), "rules", index) ?? JSON.stringify(rule, null, 2),
    rule: isDnsObject(rule) ? rule : null
  }));
  const result = [...active];
  for (const item of editorState.value.disabledRules) {
    result.splice(Math.max(0, Math.min(item.position, result.length)), 0, {
      key: `disabled:${item.id}`,
      enabled: false,
      disabledId: item.id,
      text: item.text,
      rule: parseDnsObjectText(item.text)
    });
  }
  let builtInAssigned = false;
  return result.map((item) => {
    if (!builtInAssigned && isDefaultFakeIpDnsRule(item.rule)) {
      builtInAssigned = true;
      return { ...item, builtIn: "fakeip" };
    }
    return item;
  });
});
const listItems = computed<Array<DnsRuleViewItem | DnsRuleMarker>>(() => {
  const result: Array<DnsRuleViewItem | DnsRuleMarker> = [...ruleItems.value];
  result.splice(editorState.value.insertionPosition, 0, { key: "marker", marker: true });
  return result;
});
const editingRule = computed(() => ruleItems.value.find((item) => item.key === editingRuleKey.value) ?? null);
const fakeIpServers = computed<DnsServer[]>(() => serverItems.value.flatMap((item) => item.server && dnsServerType(item.server) === "fakeip" ? [item.server] : []));
const ruleSetOptions = computed<RouteRuleSetOption[]>(() => {
  const options = buildRouteRuleSetOptions(props.draft.config.route, props.draft.bindings, resourceStore.resources.value);
  const known = new Set(options.map((option) => option.tag));
  for (const item of ruleItems.value) {
    for (const tag of ruleSetTags(item.rule)) {
      if (known.has(tag)) continue;
      known.add(tag);
      options.push({ id: `dns-current:${tag}`, tag, name: tag, kind: "missing", detail: "当前 DNS JSON 引用", disabled: false });
    }
  }
  return options;
});

onMounted(async () => {
  if (resourceStore.resources.value.length) return;
  try { await resourceStore.refresh(); }
  catch (cause) { resourceError.value = cause instanceof Error ? cause.message : "无法读取规则集"; }
});

function show(message: string, tone: "info" | "error" | "success" = "info"): void {
  notice.value = "";
  noticeTone.value = tone;
  queueMicrotask(() => { notice.value = message; });
}

function bufferText(): string {
  return normalizedBuffer.value;
}

function bindings() {
  return (Array.isArray(props.draft.bindings) ? props.draft.bindings : []).map((binding) => ({ ...binding }));
}

function stateFrom(items = ruleItems.value, insertionPosition = editorState.value.insertionPosition): DnsEditorState {
  return {
    version: 1,
    insertionPosition: Math.max(0, Math.min(insertionPosition, items.length)),
    disabledRules: items.flatMap((item, position) => item.enabled ? [] : [{
      id: item.disabledId ?? createDnsStateId(),
      position,
      text: item.text
    }])
  };
}

function save(text: string, state = stateFrom(), nextBindings = bindings(), renamed?: { from: string; to: string }): void {
  emit("save", { text, editorState: state, bindings: nextBindings, renamed });
}

function editJson(): void {
  if (normalizedBuffer.value !== sourceBuffer.value) save(normalizedBuffer.value, editorState.value);
  emit("editJson");
}

function toggleGeneral(key: "disable_cache" | "disable_expire" | "independent_cache", value: boolean, label: string): void {
  save(updateDnsBoolean(bufferText(), key, value));
  show(`${label}已${value ? "开启" : "关闭"}。`, "success");
}

function changeGeneral(key: "final" | "strategy" | "client_subnet", event: Event, message: string): void {
  save(updateDnsString(bufferText(), key, (event.currentTarget as HTMLInputElement | HTMLSelectElement).value));
  show(message, "success");
}

function openServerEditor(index: number | null): void {
  if (serverInteractionDisabled.value) return;
  if (index !== null && !serverItems.value[index]?.server) return;
  editingServerIndex.value = index;
  serverEditorOpen.value = true;
}

function closeServerEditor(): void {
  serverEditorOpen.value = false;
  editingServerIndex.value = null;
}

function submitServer(result: DnsServerEditorResult): void {
  const wasEditing = editingServerIndex.value !== null;
  const index = editingServerIndex.value ?? rawServers.value.length;
  const originalTag = editingServer.value && typeof editingServer.value.tag === "string" ? editingServer.value.tag : "";
  const nextTag = typeof result.server.tag === "string" ? result.server.tag : "";
  const renamed = originalTag && nextTag && originalTag !== nextTag ? { from: originalTag, to: nextTag } : undefined;
  let text = upsertDnsArrayItemText(bufferText(), "servers", index, result.text);
  let nextItems = ruleItems.value;
  if (renamed) {
    text = renameDnsServerTagInBuffer(text, "dns", renamed.from, renamed.to);
    nextItems = ruleItems.value.map((item) => {
      if (item.enabled) return item;
      const updated = renameDnsServerInRuleText(item.text, renamed.from, renamed.to);
      return updated === item.text ? item : { ...item, text: updated, rule: parseDnsObjectText(updated) };
    });
  }
  save(text, stateFrom(nextItems), bindings(), renamed);
  closeServerEditor();
  show(`${nextTag || "DNS 服务器"} 已${wasEditing ? "更新" : "添加"}。`, "success");
}

function removeServer(index: number): void {
  const item = serverItems.value[index];
  if (!item) return;
  const tag = item.server && typeof item.server.tag === "string" ? item.server.tag : `服务器 ${index + 1}`;
  save(removeDnsArrayItem(bufferText(), "servers", index));
  show(`${tag} 已移除；原有引用会保留，便于手工迁移。`, "success");
}

function openRuleEditor(key: string | null): void {
  if (ruleInteractionDisabled.value) return;
  const item = key ? ruleItems.value.find((candidate) => candidate.key === key) : null;
  if (key && !item?.rule) return;
  editingRuleKey.value = key;
  if (item?.builtIn === "fakeip") fakeIpRuleEditorOpen.value = true;
  else ruleEditorOpen.value = true;
}

function closeRuleEditor(): void {
  ruleEditorOpen.value = false;
  fakeIpRuleEditorOpen.value = false;
  editingRuleKey.value = null;
}

function submitRule(result: DnsRuleEditorResult): void {
  const item = editingRule.value;
  let text = bufferText();
  let state = stateFrom();
  if (item?.enabled && item.sourceIndex !== undefined) {
    text = upsertDnsArrayItemText(text, "rules", item.sourceIndex, result.text);
  } else if (item) {
    const nextItems = ruleItems.value.map((candidate) => candidate.key === item.key ? { ...candidate, text: result.text, rule: result.rule } : candidate);
    state = stateFrom(nextItems);
  } else {
    const insertion = editorState.value.insertionPosition;
    const sourceIndex = ruleItems.value.slice(0, insertion).filter((candidate) => candidate.enabled).length;
    text = insertDnsRuleText(text, sourceIndex, result.text);
    const nextItems = [...ruleItems.value];
    nextItems.splice(insertion, 0, { key: "new", enabled: true, text: result.text, rule: result.rule });
    state = stateFrom(nextItems, insertion);
  }
  save(text, state, mergeSelectedRuleBindings(props.draft.bindings, ruleSetOptions.value, result.resourceIds));
  show(item?.builtIn === "fakeip" ? "内置 Fake-IP 规则已更新。" : item ? "DNS 规则已更新。" : "DNS 规则已添加到插入点之后。", "success");
  closeRuleEditor();
}

function removeRule(item: DnsRuleViewItem): void {
  if (item.builtIn === "fakeip") {
    show("内置 Fake-IP 规则可以停用，但不能删除。", "info");
    return;
  }
  const position = ruleItems.value.findIndex((candidate) => candidate.key === item.key);
  if (position < 0) return;
  const nextItems = [...ruleItems.value];
  nextItems.splice(position, 1);
  const insertion = editorState.value.insertionPosition - (position < editorState.value.insertionPosition ? 1 : 0);
  const text = item.enabled && item.sourceIndex !== undefined ? removeDnsArrayItem(bufferText(), "rules", item.sourceIndex) : bufferText();
  save(text, stateFrom(nextItems, insertion));
  show("DNS 规则已从当前配置移除。", "success");
}

function toggleRule(item: DnsRuleViewItem): void {
  const position = ruleItems.value.findIndex((candidate) => candidate.key === item.key);
  if (position < 0) return;
  const nextItems = [...ruleItems.value];
  let text = bufferText();
  if (item.enabled && item.sourceIndex !== undefined) {
    const id = createDnsStateId();
    text = removeDnsArrayItem(text, "rules", item.sourceIndex);
    nextItems[position] = { ...item, key: `disabled:${id}`, enabled: false, sourceIndex: undefined, disabledId: id };
  } else {
    const sourceIndex = ruleItems.value.slice(0, position).filter((candidate) => candidate.enabled).length;
    text = insertDnsRuleText(text, sourceIndex, item.text);
    nextItems[position] = { ...item, key: `active:new:${createDnsStateId()}`, enabled: true, sourceIndex, disabledId: undefined };
  }
  save(text, stateFrom(nextItems));
  if (item.builtIn === "fakeip") {
    show(item.enabled ? "内置 Fake-IP 规则已停用，可随时重新启用。" : "内置 Fake-IP 规则已启用并恢复到 DNS 配置。", "success");
  } else {
    show(item.enabled ? "规则已停用，不会写入 sing-box 配置。" : "规则已启用并恢复到 DNS 配置。", "success");
  }
}

function ruleSetTags(rule: unknown): string[] {
  if (!isDnsObject(rule)) return [];
  if (typeof rule.rule_set === "string") return rule.rule_set.trim() ? [rule.rule_set.trim()] : [];
  return Array.isArray(rule.rule_set) ? rule.rule_set.flatMap((tag) => typeof tag === "string" && tag.trim() ? [tag.trim()] : []) : [];
}

function ruleSummary(item: DnsRuleViewItem): string {
  if (!item.rule) return "无效规则，请使用 JSON 编辑";
  if (item.builtIn === "fakeip") return "Fake-IP 地址映射";
  const type = dnsRuleType(item.rule);
  const payload = dnsRulePayloadText(item.rule, type) || "全部查询";
  const action = dnsRuleAction(item.rule);
  const target = action === "route" ? dnsRuleServer(item.rule) || finalServer.value || "默认服务器" : action;
  return `${type}, ${payload}, ${action}, ${target}`;
}

function ruleDetail(item: DnsRuleViewItem): string {
  if (item.builtIn === "fakeip") {
    const parts = fakeIpDnsRuleParts(item.rule);
    return `AND · 排除 ${parts?.suffixes.length ?? 0} 个后缀 · ${parts?.queryTypes.join(" / ") || "未选择查询类型"} → ${dnsRuleServer(item.rule) || "Fake-IP"}`;
  }
  return `${actionLabel(item.rule)}${item.rule?.invert === true ? " · 反向匹配" : ""}`;
}

function actionLabel(rule: unknown): string {
  const action = dnsRuleAction(rule);
  return dnsRuleActionOptions.find((option) => option.value === action)?.label ?? action;
}

function displayIndex(item: DnsRuleViewItem): string {
  return String(ruleItems.value.findIndex((candidate) => candidate.key === item.key) + 1).padStart(2, "0");
}

function serverIcon(server: DnsServer | null): string {
  return server && dnsServerType(server) === "fakeip" ? "sparkles" : "dns";
}

function serverTone(server: DnsServer | null): string {
  const type = server ? dnsServerType(server) : "";
  if (["https", "h3", "quic", "tls"].includes(type)) return "cyan";
  if (type === "fakeip") return "orange";
  return "";
}
</script>

<template>
  <section class="dns-panel">
    <article class="dns-summary-card dns-workbench">
      <header class="dns-head"><div><h2>DNS 设置</h2><p>在同一页面管理缓存行为、上游解析器与按顺序匹配的 DNS 规则</p></div><div class="head-actions"><span class="summary-badge">{{ rawServers.length }} 个服务器 · {{ activeRules.length }}/{{ ruleItems.length }} 条规则启用</span><button class="json-button" type="button" @click="editJson"><AppIcon name="code" :size="14" />编辑全部 JSON</button></div></header>
    </article>

    <article id="dns-general-section" class="dns-section-card">
      <header class="section-toolbar"><div class="section-title"><span class="section-title-icon"><AppIcon name="config" :size="15" /></span><div class="section-copy"><strong>通用设置</strong><span>缓存、回退服务器与默认解析策略</span></div></div><div class="section-actions"><span class="summary-badge">全局行为</span></div></header>
      <div class="general-section">
        <div class="general-toggles">
          <article><span class="setting-icon"><AppIcon name="database" :size="16" /></span><div><strong>禁用 DNS 缓存</strong><small>每次请求都重新查询上游服务器</small></div><button type="button" class="setting-switch" role="switch" aria-label="切换禁用 DNS 缓存" :aria-checked="disableCache" :disabled="generalInteractionDisabled" :class="{ on: disableCache }" @click="toggleGeneral('disable_cache', !disableCache, '禁用 DNS 缓存')"><span /></button></article>
          <article><span class="setting-icon orange"><AppIcon name="clock" :size="16" /></span><div><strong>禁用 DNS 缓存过期</strong><small>缓存记录不会因 TTL 到期而失效</small></div><button type="button" class="setting-switch" role="switch" aria-label="切换禁用 DNS 缓存过期" :aria-checked="disableExpire" :disabled="generalInteractionDisabled" :class="{ on: disableExpire }" @click="toggleGeneral('disable_expire', !disableExpire, '禁用 DNS 缓存过期')"><span /></button></article>
          <article><span class="setting-icon cyan"><AppIcon name="dns" :size="16" /></span><div><strong>独立缓存</strong><small>为每个 DNS 服务器使用独立缓存</small></div><button type="button" class="setting-switch" role="switch" aria-label="切换独立 DNS 缓存" :aria-checked="independentCache" :disabled="generalInteractionDisabled" :class="{ on: independentCache }" @click="toggleGeneral('independent_cache', !independentCache, '独立缓存')"><span /></button></article>
        </div>
        <div class="general-fields">
          <label><span><strong>回退 DNS</strong><small>所有规则未命中时使用的服务器</small></span><select aria-label="回退 DNS" :value="finalServer" :disabled="generalInteractionDisabled" @change="changeGeneral('final', $event, '回退 DNS 已更新。')"><option value="">由 sing-box 自动选择</option><option v-for="tag in finalOptions" :key="tag" :value="tag">{{ tag }}</option></select></label>
          <label><span><strong>解析策略</strong><small>控制 A 与 AAAA 查询的优先级</small></span><select aria-label="DNS 解析策略" :value="strategy" :disabled="generalInteractionDisabled" @change="changeGeneral('strategy', $event, 'DNS 解析策略已更新。')"><option value="">默认</option><option value="prefer_ipv4">优先 IPv4</option><option value="prefer_ipv6">优先 IPv6</option><option value="ipv4_only">仅 IPv4</option><option value="ipv6_only">仅 IPv6</option></select></label>
          <label><span><strong>客户端子网</strong><small>发送 EDNS Client Subnet；留空表示无</small></span><input aria-label="DNS 客户端子网" :value="clientSubnet" :disabled="generalInteractionDisabled" placeholder="无，例如 1.2.3.4/24" @change="changeGeneral('client_subnet', $event, 'DNS 客户端子网已更新。')" /></label>
        </div>
      </div>
    </article>

    <article id="dns-servers-section" class="dns-section-card servers-section">
      <header class="section-toolbar"><div class="section-title"><span class="section-title-icon cyan"><AppIcon name="server" :size="15" /></span><div class="section-copy"><strong>上游 DNS 服务器</strong><span>标签可被规则、回退 DNS 与域名解析器引用</span></div></div><div class="section-actions"><span class="summary-badge">{{ rawServers.length }} 个解析器</span><button class="add-button" type="button" :disabled="serverInteractionDisabled" @click="openServerEditor(null)"><AppIcon name="plus" :size="14" />新增服务器</button></div></header>
        <div class="server-list">
          <article v-for="item in serverItems" :key="item.index" class="server-row" :class="{ invalid: !item.server }">
            <span class="server-icon" :class="serverTone(item.server)"><AppIcon :name="serverIcon(item.server)" :size="17" /></span>
            <div class="server-copy"><strong>{{ item.server && typeof item.server.tag === 'string' ? item.server.tag : `服务器 ${item.index + 1}` }}</strong><span>{{ dnsServerAddress(item.server) }}</span></div>
            <div class="server-meta"><b>{{ item.server ? dnsServerType(item.server) : "invalid" }}</b><em v-if="dnsServerDetour(item.server)"><AppIcon name="route" :size="11" />{{ dnsServerDetour(item.server) }}</em></div>
            <div class="row-actions"><button type="button" :aria-label="`编辑 DNS 服务器 ${item.index + 1}`" title="编辑服务器" :disabled="serverInteractionDisabled || !item.server" @click="openServerEditor(item.index)"><AppIcon name="edit" :size="14" /></button><button class="danger" type="button" :aria-label="`删除 DNS 服务器 ${item.index + 1}`" title="删除服务器" :disabled="serverInteractionDisabled" @click="removeServer(item.index)"><AppIcon name="trash" :size="14" /></button></div>
          </article>
          <button v-if="!serverItems.length" class="empty-state" type="button" :disabled="serverInteractionDisabled" @click="openServerEditor(null)"><AppIcon name="dns" :size="30" /><strong>尚未配置 DNS 服务器</strong><span>点击添加第一个解析器</span></button>
        </div>
    </article>

    <article id="dns-rules-section" class="dns-section-card rules-section">
      <header class="section-toolbar"><div class="section-title"><span class="section-title-icon violet"><AppIcon name="rules" :size="15" /></span><div class="section-copy"><strong>DNS 规则</strong><span>从上到下依次匹配；新增规则会放在插入点之后</span></div></div><div class="section-actions"><span class="summary-badge">已启用 {{ activeRules.length }}/{{ ruleItems.length }}</span><button class="add-button" type="button" :disabled="ruleInteractionDisabled" @click="openRuleEditor(null)"><AppIcon name="plus" :size="14" />新增规则</button></div></header>
        <div class="dns-rule-list">
          <template v-for="entry in listItems" :key="entry.key">
            <div v-if="'marker' in entry" class="dns-insertion-point"><span /><button type="button" :disabled="ruleInteractionDisabled" @click="openRuleEditor(null)"><AppIcon name="plus" :size="12" />新规则将插入到这里</button><span /></div>
            <article v-else class="dns-rule-row" :class="{ 'is-disabled': !entry.enabled, 'is-protected': entry.builtIn === 'fakeip' }" :data-dns-rule-kind="entry.builtIn">
              <label class="rule-switch" :title="entry.rule ? entry.enabled ? '停用规则' : '启用规则' : '无法停用非对象规则'"><input type="checkbox" :checked="entry.enabled" :disabled="ruleInteractionDisabled || !entry.rule" :aria-label="entry.builtIn === 'fakeip' ? `${entry.enabled ? '停用' : '启用'}内置 Fake-IP 规则` : `${entry.enabled ? '停用' : '启用'}第 ${Number(displayIndex(entry))} 条 DNS 规则`" @change="toggleRule(entry)" /><span /></label>
              <span class="rule-index">{{ displayIndex(entry) }}</span>
              <div class="rule-copy"><div class="rule-title-line"><strong>{{ ruleSummary(entry) }}</strong><span v-if="entry.builtIn === 'fakeip'" class="system-badge"><AppIcon name="lock" :size="9" />内置</span></div><small>{{ ruleDetail(entry) }}</small></div>
              <div class="row-actions"><button type="button" :aria-label="entry.builtIn === 'fakeip' ? '编辑内置 Fake-IP 规则' : `编辑 DNS 规则 ${Number(displayIndex(entry))}`" :title="entry.builtIn === 'fakeip' ? '编辑 Fake-IP 规则' : '编辑规则'" :disabled="ruleInteractionDisabled || !entry.rule" @click="openRuleEditor(entry.key)"><AppIcon name="edit" :size="14" /></button><button v-if="entry.builtIn === 'fakeip'" class="protected-action" type="button" aria-label="内置 Fake-IP 规则不能删除" title="内置 Fake-IP 规则不能删除" disabled><AppIcon name="lock" :size="14" /></button><button v-else class="danger" type="button" :aria-label="`删除 DNS 规则 ${Number(displayIndex(entry))}`" title="删除规则" :disabled="ruleInteractionDisabled" @click="removeRule(entry)"><AppIcon name="trash" :size="14" /></button></div>
            </article>
          </template>
          <button v-if="!ruleItems.length" class="empty-state" type="button" :disabled="ruleInteractionDisabled" @click="openRuleEditor(null)"><AppIcon name="rules" :size="30" /><strong>尚未配置 DNS 规则</strong><span>未命中时会使用回退 DNS</span></button>
        </div>
        <p v-if="editorState.disabledRules.length" class="disabled-note"><AppIcon name="info" :size="13" />停用规则保存在草稿编辑状态中，不会进入实际 DNS JSON 或应用结果。</p>
    </article>

    <p v-if="invalidDnsObject" class="panel-warning" role="alert">dns 必须是对象；请使用“编辑全部 JSON”修复后再结构化编辑。</p>
    <p v-if="invalidServers" class="panel-warning" role="alert">dns.servers 必须是数组；请使用“编辑全部 JSON”修复后再结构化编辑。</p>
    <p v-if="invalidRules" class="panel-warning" role="alert">dns.rules 必须是数组；请使用“编辑全部 JSON”修复后再结构化编辑。</p>
    <p v-if="resourceError" class="panel-warning" role="status">{{ resourceError }}；仍可编辑当前 JSON 中已有的规则集引用。</p>
    <p v-if="error" class="panel-error" role="alert">{{ error }}</p>
    <DnsServerEditorDialog :open="serverEditorOpen" :server="editingServer" :server-text="editingServerText" :reserved-tags="reservedServerTags" :outbounds="outbounds" :dns-servers="serverTags" @close="closeServerEditor" @submit="submitServer" />
    <DnsRuleEditorDialog :open="ruleEditorOpen" :rule="editingRule?.rule ?? null" :rule-text="editingRule?.text" :servers="serverTags" :options="ruleSetOptions" @close="closeRuleEditor" @submit="submitRule" />
    <FakeIpDnsRuleDialog :open="fakeIpRuleEditorOpen" :rule="editingRule?.rule ?? null" :enabled="editingRule?.enabled ?? true" :servers="fakeIpServers" @close="closeRuleEditor" @submit="submitRule" />
    <AppToast v-if="notice" :message="notice" :tone="noticeTone" />
  </section>
</template>

<style scoped>
.dns-panel { display: grid; gap: 10px; }.dns-workbench { min-width: 0; overflow: hidden; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.dns-head { display: flex; min-height: 48px; align-items: center; justify-content: space-between; gap: var(--section-gap); padding: 8px var(--card-padding); border-bottom: 1px solid var(--line); }.dns-head h2, .dns-head p { margin: 0; }.dns-head h2 { font-size: var(--font-size-heading); }.dns-head p { margin-top: 2px; color: var(--text-faint); font-size: var(--font-size-xs); }.head-actions { display: flex; flex: 0 0 auto; align-items: center; gap: 7px; }.summary-badge { display: inline-flex; min-height: 26px; align-items: center; padding: 4px 8px; border: 1px solid color-mix(in srgb, var(--primary) 13%, var(--line)); border-radius: 999px; color: var(--primary); background: var(--primary-soft); font-size: var(--font-size-sm); font-weight: 680; white-space: nowrap; }.json-button, .add-button { display: inline-flex; min-height: 31px; align-items: center; justify-content: center; gap: 5px; padding: 5px 8px; font-size: var(--font-size-sm); }.json-button { color: var(--text-soft); border: 1px solid var(--line); background: var(--surface-soft); }.add-button { color: #fff; background: var(--primary); }.dns-section-tabs { display: flex; justify-content: center; gap: 3px; padding: 7px 10px 0; background: var(--surface); }.dns-section-tabs button { display: inline-flex; min-height: 37px; align-items: center; gap: 6px; padding: 7px 13px; border-radius: 9px 9px 0 0; color: var(--text-faint); background: transparent; font-size: var(--font-size-sm); }.dns-section-tabs button:hover { color: var(--text); background: var(--surface-soft); }.dns-section-tabs button.active { color: var(--primary); background: var(--primary-soft); }.dns-section-tabs button > span { display: inline-flex; min-width: 16px; height: 16px; align-items: center; justify-content: center; padding: 0 4px; border-radius: 999px; color: var(--text-faint); background: var(--surface); font-size: var(--font-size-xs); }.general-section, .servers-section, .rules-section { min-height: 0; border-top: 1px solid var(--line); background: var(--surface-soft); }.general-section { display: grid; align-content: start; gap: 10px; padding: 11px; }.general-toggles { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 9px; }.general-toggles article { display: flex; min-width: 0; min-height: 54px; align-items: center; gap: 10px; padding: 11px; border: 1px solid var(--line); border-radius: 11px; background: var(--surface); }.general-toggles article > div { min-width: 0; flex: 1; }.general-toggles strong, .general-toggles small, .general-fields strong, .general-fields small { display: block; }.general-toggles strong, .general-fields strong { color: var(--text); font-size: var(--font-size-base); font-weight: 660; }.general-toggles small, .general-fields small { margin-top: 2px; color: var(--text-faint); font-size: var(--font-size-sm); font-weight: 500; }.setting-icon { display: grid; width: 34px; height: 34px; flex: 0 0 auto; place-items: center; border-radius: 9px; color: var(--primary); background: var(--primary-soft); }.setting-icon.orange { color: var(--orange); background: var(--orange-soft); }.setting-icon.cyan { color: var(--cyan); background: var(--cyan-soft); }.setting-switch { position: relative; width: 34px; min-height: 20px; flex: 0 0 auto; padding: 0; border-radius: 7px; background: var(--line-strong); }.setting-switch span { position: absolute; top: 4px; left: 4px; width: 12px; height: 12px; border-radius: 4px; background: #fff; transition: transform 150ms ease; }.setting-switch.on { background: var(--primary); }.setting-switch.on span { transform: translateX(14px); }.general-fields { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 9px; }.general-fields label { display: grid; min-width: 0; grid-template-columns: minmax(140px, .8fr) minmax(170px, 1.2fr); align-items: center; gap: 11px; padding: 10px 11px; border: 1px solid var(--line); border-radius: 11px; background: var(--surface); }.general-fields label:last-child { grid-column: 1 / -1; }.general-fields select, .general-fields input { width: 100%; min-width: 0; font-size: var(--font-size-base); }.section-toolbar { display: flex; min-height: 44px; align-items: center; justify-content: space-between; gap: var(--section-gap); padding: 6px var(--card-padding); border-bottom: 1px solid var(--line); background: var(--surface); }.section-toolbar > div { min-width: 0; }.section-toolbar strong, .section-toolbar span { display: block; }.section-toolbar strong { font-size: var(--font-size-base); }.section-toolbar span { margin-top: 2px; color: var(--text-faint); font-size: var(--font-size-sm); }.server-list, .dns-rule-list { display: grid; padding: 4px var(--card-padding) 8px; }.server-row { display: grid; min-width: 0; min-height: 48px; grid-template-columns: auto minmax(0, 1fr) auto auto; align-items: center; gap: 10px; padding: 8px 5px; border-bottom: 1px solid var(--line); transition: background 150ms ease; }.server-row:hover, .dns-rule-row:hover { background: color-mix(in srgb, var(--surface) 64%, transparent); }.server-row.invalid { opacity: .58; }.server-icon { display: grid; width: 34px; height: 34px; place-items: center; border-radius: 9px; color: var(--primary); background: var(--primary-soft); }.server-icon.cyan { color: var(--cyan); background: var(--cyan-soft); }.server-icon.orange { color: var(--orange); background: var(--orange-soft); }.server-copy { min-width: 0; }.server-copy strong, .server-copy span { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.server-copy strong { font-size: var(--font-size-base); }.server-copy span { margin-top: 3px; color: var(--text-soft); font: var(--font-size-sm) ui-monospace, SFMono-Regular, Consolas, monospace; }.server-meta { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 5px; }.server-meta b, .server-meta em { display: inline-flex; min-height: 23px; align-items: center; gap: 4px; padding: 3px 7px; border-radius: 7px; font-size: var(--font-size-sm); font-style: normal; white-space: nowrap; }.server-meta b { color: var(--primary); background: var(--primary-soft); font-family: ui-monospace, monospace; }.server-meta em { color: var(--orange); background: var(--orange-soft); }.row-actions { display: flex; gap: 2px; }.row-actions button { display: grid; width: 30px; min-height: 30px; place-items: center; padding: 0; color: var(--text-faint); background: transparent; }.row-actions button:hover { color: var(--primary); background: var(--primary-soft); }.row-actions button.danger:hover { color: var(--red); background: var(--red-soft); }.dns-rule-row { display: grid; min-width: 0; min-height: 46px; grid-template-columns: auto auto minmax(0, 1fr) auto; align-items: center; gap: 9px; padding: 7px 5px; border-bottom: 1px solid var(--line); transition: opacity 150ms ease, background 150ms ease; }.dns-rule-row.is-disabled { opacity: .48; }.rule-switch { position: relative; display: block; width: 31px; height: 20px; }.rule-switch input { position: absolute; width: 1px; height: 1px; opacity: 0; }.rule-switch span { position: relative; display: block; width: 31px; height: 20px; border-radius: 6px; background: var(--line-strong); cursor: pointer; transition: background 150ms ease; }.rule-switch span::after { position: absolute; top: 4px; left: 4px; width: 4px; height: 12px; border-radius: 3px; content: ""; background: #fff; transition: left 150ms ease; }.rule-switch input:checked + span { background: var(--primary); }.rule-switch input:checked + span::after { left: 23px; }.rule-switch input:focus-visible + span { outline: 3px solid color-mix(in srgb, var(--primary) 22%, transparent); outline-offset: 2px; }.rule-index { color: var(--text-faint); font: var(--font-size-sm) ui-monospace, Consolas, monospace; }.rule-copy { min-width: 0; }.rule-copy strong, .rule-copy small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.rule-copy strong { color: var(--text); font: 650 var(--font-size-base) ui-monospace, Consolas, monospace; }.rule-copy small { margin-top: 3px; color: var(--text-faint); font-size: var(--font-size-sm); }.rule-copy em { display: inline-flex; margin-left: 7px; padding: 1px 5px; border-radius: 5px; color: var(--orange); background: var(--orange-soft); font-size: var(--font-size-xs); font-style: normal; }.dns-insertion-point { display: flex; min-height: 44px; align-items: center; gap: 10px; }.dns-insertion-point > span { height: 1px; flex: 1; background: var(--line); }.dns-insertion-point button { display: inline-flex; min-height: 29px; align-items: center; gap: 5px; padding: 5px 8px; color: var(--text-soft); background: var(--surface); font-size: var(--font-size-sm); }.dns-insertion-point button:hover { color: var(--primary); background: var(--primary-soft); }.empty-state { display: grid; min-height: var(--empty-state-height); place-items: center; align-content: center; gap: 7px; color: var(--primary); border: 1px dashed var(--line-strong); background: var(--surface); }.empty-state strong { color: var(--text); font-size: var(--font-size-base); }.empty-state span { color: var(--text-faint); font-size: var(--font-size-sm); font-weight: 500; }.disabled-note { display: flex; align-items: flex-start; gap: 6px; margin: -5px 14px 13px; padding: 8px 9px; border-radius: 8px; color: var(--text-faint); background: var(--surface); font-size: var(--font-size-sm); }.disabled-note .icon { color: var(--primary); }.panel-warning, .panel-error { margin: 0; padding: 9px 10px; border-radius: 9px; font-size: var(--font-size-xs); }.panel-warning { color: var(--orange); background: var(--orange-soft); }.panel-error { color: var(--red); background: var(--red-soft); }
@media (max-width: 920px) { .general-toggles { grid-template-columns: 1fr; }.general-toggles article { min-height: 54px; }.general-fields { grid-template-columns: 1fr; }.general-fields label:last-child { grid-column: auto; } }
@media (max-width: 760px) { .dns-head { align-items: flex-start; flex-direction: column; gap: 9px; }.head-actions { width: 100%; justify-content: flex-end; }.summary-badge { margin-right: auto; }.server-row { grid-template-columns: auto minmax(0, 1fr) auto; }.server-meta { grid-column: 2; justify-content: flex-start; }.server-row > .row-actions { grid-column: 3; grid-row: 1 / span 2; }.general-fields label { grid-template-columns: 1fr; gap: 7px; } }
@media (max-width: 560px) { .dns-section-tabs { justify-content: stretch; }.dns-section-tabs button { min-width: 0; flex: 1; justify-content: center; padding-inline: 7px; }.general-section { padding: 8px; }.general-toggles article { padding: 9px; }.setting-icon { display: none; }.section-toolbar { align-items: flex-start; flex-direction: column; }.section-toolbar .add-button { width: 100%; }.server-list, .dns-rule-list { padding-inline: 8px; }.server-row { grid-template-columns: minmax(0, 1fr) auto; }.server-icon { display: none; }.server-copy { grid-column: 1; }.server-meta { grid-column: 1; }.server-row > .row-actions { grid-column: 2; }.dns-rule-row { grid-template-columns: auto minmax(0, 1fr) auto; }.dns-rule-row > .row-actions { grid-column: 3; grid-row: auto; }.rule-index { display: none; }.summary-badge { display: none; }.dns-insertion-point { gap: 6px; }.dns-insertion-point button { white-space: nowrap; } }
.dns-panel { gap: var(--section-gap); }.dns-summary-card, .dns-section-card { min-width: 0; overflow: hidden; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.dns-head { border-bottom: 0; }.general-section, .servers-section, .rules-section { min-height: 0; }.general-section { border-top: 1px solid var(--line); }.servers-section, .rules-section { border-top: 1px solid var(--line); }.section-title { display: flex; min-width: 0; align-items: center; gap: 10px; }.section-title-icon { display: grid; width: 32px; height: 32px; flex: 0 0 auto; place-items: center; border-radius: 9px; color: var(--primary); background: var(--primary-soft); }.section-title-icon.cyan { color: var(--cyan); background: var(--cyan-soft); }.section-title-icon.violet { color: var(--violet); background: color-mix(in srgb, var(--violet) 12%, var(--surface)); }.section-copy { min-width: 0; }.section-copy strong, .section-copy span { display: block; }.section-copy span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.section-actions { display: flex; flex: 0 0 auto; align-items: center; gap: 7px; }.server-list, .dns-rule-list { background: var(--surface-soft); }.dns-rule-row.is-protected { background: linear-gradient(90deg, color-mix(in srgb, var(--orange-soft) 62%, transparent), transparent 58%); }.dns-rule-row.is-protected.is-disabled { opacity: .64; }.rule-title-line { display: flex; min-width: 0; align-items: center; gap: 7px; }.rule-title-line strong { min-width: 0; }.system-badge { display: inline-flex; min-height: 20px; flex: 0 0 auto; align-items: center; gap: 3px; padding: 2px 6px; border-radius: 999px; color: var(--orange); background: var(--orange-soft); font-size: var(--font-size-xs); font-weight: 700; }.row-actions button.protected-action:disabled { color: var(--orange); background: var(--orange-soft); cursor: not-allowed; opacity: .72; }
@media (max-width: 560px) { .section-toolbar { align-items: stretch; }.section-actions { width: 100%; }.section-actions .summary-badge { display: inline-flex; margin-right: auto; }.section-actions .add-button { width: auto; flex: 1; } }
</style>
