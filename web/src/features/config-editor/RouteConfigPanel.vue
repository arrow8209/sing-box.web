<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import AppToast from "@/components/AppToast.vue";
import AppIcon from "@/layout/AppIcon.vue";
import type { Draft } from "@/features/profiles/types";
import { useRuleResourceStore } from "@/features/rule-resources/store";
import RouteRuleEditorDialog from "./RouteRuleEditorDialog.vue";
import {
  buildRouteRuleSetOptions,
  createRouteStateId,
  insertRouteRuleText,
  isRouteObject,
  mergeSelectedRuleBindings,
  parseRouteRuleText,
  removeRouteRule,
  reorderRouteRules,
  restoreRouteEditorState,
  routeDefaultDomainResolverTag,
  routeDnsServerTags,
  routeInboundTags,
  routeOutboundTags,
  routeRuleAction,
  routeRuleActionOptions,
  routeRulePayloadText,
  routeRuleSetTags,
  routeRuleText,
  routeRules,
  routeRuleType,
  updateRouteBoolean,
  updateRouteDefaultDomainResolver,
  updateRouteFinal,
  updateRouteString,
  upsertRouteRuleText,
  type RouteEditorState,
  type RoutePanelSave,
  type RouteRule,
  type RouteRuleEditorResult
} from "./route";

const props = defineProps<{ draft: Draft; error?: string }>();
const emit = defineEmits<{ editJson: []; save: [payload: RoutePanelSave] }>();
const store = useRuleResourceStore();
const editingKey = ref<string | null>(null);
const editorOpen = ref(false);
const resourceError = ref("");
const notice = ref("");
const noticeTone = ref<"info" | "error" | "success">("info");
const renderVersion = ref(0);

interface RouteRuleViewItem {
  key: string;
  enabled: boolean;
  text: string;
  rule: RouteRule | null;
  sourceIndex?: number;
  disabledId?: string;
}

interface RouteMarkerItem {
  key: "marker";
  marker: true;
}

type RouteListItem = RouteRuleViewItem | RouteMarkerItem;

interface RouteRuleDrag {
  pointerId: number;
  handle: HTMLElement;
  row: HTMLElement;
  list: HTMLElement;
  key: string;
  startX: number;
  startY: number;
  lastClientY: number;
  offsetY: number;
  started: boolean;
  ghost: HTMLElement | null;
  scrollFrame: number | null;
}

let drag: RouteRuleDrag | null = null;

const route = computed<RouteRule>(() => isRouteObject(props.draft.config.route) ? props.draft.config.route : {});
const activeRules = computed(() => routeRules(route.value));
const editorState = computed(() => restoreRouteEditorState(props.draft.editorState?.route, activeRules.value.length));
const routeWithDisabledReferences = computed(() => ({
  ...route.value,
  rules: [...activeRules.value, ...editorState.value.disabledRules.flatMap((item) => {
    const rule = parseRouteRuleText(item.text);
    return rule ? [rule] : [];
  })]
}));
const outbounds = computed(() => routeOutboundTags(props.draft.config));
const inbounds = computed(() => routeInboundTags(props.draft.config));
const dnsServers = computed(() => routeDnsServerTags(props.draft.config));
const defaultOutbound = computed(() => typeof route.value.final === "string" ? route.value.final : "");
const defaultDnsServer = computed(() => routeDefaultDomainResolverTag(route.value));
const defaultInterface = computed(() => typeof route.value.default_interface === "string" ? route.value.default_interface : "");
const autoDetectInterface = computed(() => route.value.auto_detect_interface !== false);
const findProcess = computed(() => route.value.find_process === true);
const options = computed(() => buildRouteRuleSetOptions(routeWithDisabledReferences.value, props.draft.bindings, store.resources.value));
const remoteRuleSet = computed(() => Array.isArray(route.value.rule_set) && route.value.rule_set.some((item) => isRouteObject(item) && item.type === "remote"));
const invalidRules = computed(() => route.value.rules !== undefined && !Array.isArray(route.value.rules));
const ruleInteractionDisabled = computed(() => Boolean(props.error) || invalidRules.value);
const outboundOptions = computed(() => withCurrent(outbounds.value, defaultOutbound.value));
const dnsOptions = computed(() => withCurrent(dnsServers.value, defaultDnsServer.value));
const interfaceOptions = computed(() => withCurrent(["eth0", "wlan0", "en0"], defaultInterface.value));

const ruleItems = computed<RouteRuleViewItem[]>(() => {
  const active = activeRules.value.map((rule, index): RouteRuleViewItem => ({
    key: `active:${index}`,
    enabled: true,
    sourceIndex: index,
    text: routeRuleText(bufferText(), index) ?? JSON.stringify(rule, null, 2),
    rule: isRouteObject(rule) ? rule : null
  }));
  const result = [...active];
  for (const item of editorState.value.disabledRules) {
    result.splice(Math.max(0, Math.min(item.position, result.length)), 0, {
      key: `disabled:${item.id}`,
      enabled: false,
      disabledId: item.id,
      text: item.text,
      rule: parseRouteRuleText(item.text)
    });
  }
  return result;
});

const listItems = computed<RouteListItem[]>(() => {
  const result: RouteListItem[] = [...ruleItems.value];
  result.splice(editorState.value.insertionPosition, 0, { key: "marker", marker: true });
  return result;
});
const editingItem = computed(() => ruleItems.value.find((item) => item.key === editingKey.value) ?? null);

onMounted(async () => {
  if (store.resources.value.length) return;
  try { await store.refresh(); }
  catch (cause) { resourceError.value = cause instanceof Error ? cause.message : "无法读取规则集"; }
});
onBeforeUnmount(() => cleanupDrag());

function withCurrent(values: string[], current: string): string[] {
  return [...new Set(current && !values.includes(current) ? [current, ...values] : values)];
}

function show(message: string, tone: "info" | "error" | "success" = "info"): void {
  notice.value = "";
  noticeTone.value = tone;
  queueMicrotask(() => { notice.value = message; });
}

function bufferText(): string {
  const source = props.draft.buffers.route;
  return source?.trim() ? source : JSON.stringify(route.value, null, 2);
}

function bindings() {
  return (Array.isArray(props.draft.bindings) ? props.draft.bindings : []).map((binding) => ({ ...binding }));
}

function stateFrom(items = ruleItems.value, insertionPosition = editorState.value.insertionPosition): RouteEditorState {
  return {
    version: 1,
    insertionPosition: Math.max(0, Math.min(insertionPosition, items.length)),
    disabledRules: items.flatMap((item, position) => item.enabled ? [] : [{
      id: item.disabledId ?? createRouteStateId(),
      position,
      text: item.text
    }])
  };
}

function save(text: string, state = stateFrom(), nextBindings = bindings()): void {
  emit("save", { text, bindings: nextBindings, editorState: state });
}

function openEditor(key: string | null): void {
  if (ruleInteractionDisabled.value) return;
  const item = key ? ruleItems.value.find((candidate) => candidate.key === key) : null;
  if (key && !item?.rule) return;
  editingKey.value = key;
  editorOpen.value = true;
}

function closeEditor(): void {
  editorOpen.value = false;
  editingKey.value = null;
}

function submit(result: RouteRuleEditorResult): void {
  const item = editingItem.value;
  let text = bufferText();
  let state = stateFrom();
  if (item?.enabled && item.sourceIndex !== undefined) {
    text = upsertRouteRuleText(text, item.sourceIndex, result.text);
  } else if (item) {
    const nextItems = ruleItems.value.map((candidate) => candidate.key === item.key ? { ...candidate, text: result.text, rule: result.rule } : candidate);
    state = stateFrom(nextItems);
  } else {
    const insertion = editorState.value.insertionPosition;
    const sourceIndex = ruleItems.value.slice(0, insertion).filter((candidate) => candidate.enabled).length;
    text = insertRouteRuleText(text, sourceIndex, result.text);
    const nextItems = [...ruleItems.value];
    nextItems.splice(insertion, 0, { key: "new", enabled: true, text: result.text, rule: result.rule });
    state = stateFrom(nextItems, insertion);
  }
  save(text, state, mergeSelectedRuleBindings(props.draft.bindings, options.value, result.resourceIds));
  show(item ? "路由规则已更新。" : "路由规则已添加到插入点之后。", "success");
  closeEditor();
}

function remove(item: RouteRuleViewItem): void {
  const position = ruleItems.value.findIndex((candidate) => candidate.key === item.key);
  if (position < 0) return;
  const nextItems = [...ruleItems.value];
  nextItems.splice(position, 1);
  const insertion = editorState.value.insertionPosition - (position < editorState.value.insertionPosition ? 1 : 0);
  const text = item.enabled && item.sourceIndex !== undefined ? removeRouteRule(bufferText(), item.sourceIndex) : bufferText();
  save(text, stateFrom(nextItems, insertion));
  show(`${ruleTypeLabel(item)} 已从当前配置移除。`, "success");
  if (editingKey.value === item.key) closeEditor();
}

function toggleRule(item: RouteRuleViewItem): void {
  const position = ruleItems.value.findIndex((candidate) => candidate.key === item.key);
  if (position < 0) return;
  const nextItems = [...ruleItems.value];
  let text = bufferText();
  if (item.enabled && item.sourceIndex !== undefined) {
    const id = createRouteStateId();
    text = removeRouteRule(text, item.sourceIndex);
    nextItems[position] = { ...item, key: `disabled:${id}`, enabled: false, sourceIndex: undefined, disabledId: id };
  } else {
    const sourceIndex = ruleItems.value.slice(0, position).filter((candidate) => candidate.enabled).length;
    text = insertRouteRuleText(text, sourceIndex, item.text);
    nextItems[position] = { ...item, key: `active:new:${createRouteStateId()}`, enabled: true, sourceIndex, disabledId: undefined };
  }
  save(text, stateFrom(nextItems));
  show(item.enabled ? "规则已停用，不会写入 sing-box 配置。" : "规则已启用并恢复到 sing-box 配置。", "success");
}

function updateCommon(text: string, message: string): void {
  save(text);
  show(message, "success");
}

function toggleCommon(key: "find_process" | "auto_detect_interface", value: boolean): void {
  updateCommon(updateRouteBoolean(bufferText(), key, value), value ? "路由设置已启用。" : "路由设置已关闭。");
}

function changeDefaultInterface(event: Event): void {
  updateCommon(updateRouteString(bufferText(), "default_interface", (event.currentTarget as HTMLSelectElement).value), "默认出站接口已更新。");
}

function changeDefaultDns(event: Event): void {
  updateCommon(updateRouteDefaultDomainResolver(bufferText(), route.value, (event.currentTarget as HTMLSelectElement).value), "节点域名解析服务器已更新。");
}

function changeFinal(event: Event): void {
  updateCommon(updateRouteFinal(bufferText(), (event.currentTarget as HTMLSelectElement).value), "默认出站标签已更新。");
}

function ruleTypeLabel(item: RouteRuleViewItem): string {
  return item.rule ? routeRuleType(item.rule) : "unknown";
}

function rulePayloadLabel(item: RouteRuleViewItem): string {
  if (!item.rule) return "需使用 JSON 编辑";
  const type = routeRuleType(item.rule);
  if (type === "rule_set") return routeRuleSetTags(item.rule).join(", ") || "未设置规则集";
  if (type === "inline") return "完整 JSON 规则";
  return routeRulePayloadText(item.rule, type) || "未设置载荷";
}

function actionLabel(item: RouteRuleViewItem): string {
  const action = routeRuleAction(item.rule);
  return routeRuleActionOptions.find((option) => option.value === action)?.label ?? action;
}

function actionDetail(item: RouteRuleViewItem): string {
  if (!item.rule) return "需使用 JSON 编辑";
  const action = routeRuleAction(item.rule);
  if (action === "route") return typeof item.rule.outbound === "string" && item.rule.outbound ? item.rule.outbound : defaultOutbound.value || "使用默认出站";
  if (action === "route-options") return "JSON 路由选项";
  if (action === "reject") return ({ default: "默认拒绝", drop: "静默丢弃", reply: "返回响应" } as Record<string, string>)[String(item.rule.method ?? "default")] ?? "拒绝连接";
  if (action === "hijack-dns") return "劫持至 DNS";
  if (action === "sniff") return Array.isArray(item.rule.sniffer) && item.rule.sniffer.length ? item.rule.sniffer.join(", ") : "全部探测器";
  if (action === "resolve") return typeof item.rule.server === "string" && item.rule.server ? item.rule.server : "由 DNS 规则决定";
  return action;
}

function actionIcon(item: RouteRuleViewItem): string {
  const action = routeRuleAction(item.rule);
  if (action === "reject") return "shield";
  if (action === "hijack-dns" || action === "resolve") return "dns";
  if (action === "sniff") return "search";
  return "route";
}

function displayIndex(item: RouteRuleViewItem): string {
  return String(ruleItems.value.findIndex((candidate) => candidate.key === item.key) + 1).padStart(2, "0");
}

function persistOrder(items: RouteRuleViewItem[], insertion: number, movedKey: string): void {
  const orderedIndexes = items.flatMap((item) => item.enabled && item.sourceIndex !== undefined ? [item.sourceIndex] : []);
  const text = reorderRouteRules(bufferText(), orderedIndexes);
  const changed = items.some((item, index) => item.key !== ruleItems.value[index]?.key) || insertion !== editorState.value.insertionPosition;
  if (!changed) return;
  const before = ruleItems.value.findIndex((item) => item.key === movedKey) + 1;
  const after = items.findIndex((item) => item.key === movedKey) + 1;
  save(text, stateFrom(items, insertion));
  show(before === after ? "已调整规则与新增插入点的相对位置。" : `规则已从第 ${before} 位移动到第 ${after} 位。`, "success");
  void nextTick(() => {
    const handles = [...document.querySelectorAll<HTMLElement>("[data-route-rule-drag-handle]")];
    handles[Math.max(0, after - 1)]?.focus({ preventScroll: true });
  });
}

function moveWithKeyboard(item: RouteRuleViewItem, direction: -1 | 1): void {
  if (ruleInteractionDisabled.value) return;
  const current = ruleItems.value.findIndex((candidate) => candidate.key === item.key);
  const target = current + direction;
  if (current < 0 || target < 0 || target >= ruleItems.value.length) return;
  const next = [...ruleItems.value];
  [next[current], next[target]] = [next[target]!, next[current]!];
  persistOrder(next, editorState.value.insertionPosition, item.key);
}

function onHandleKeydown(event: KeyboardEvent, item: RouteRuleViewItem): void {
  if (event.key !== "ArrowUp" && event.key !== "ArrowDown") return;
  event.preventDefault();
  moveWithKeyboard(item, event.key === "ArrowUp" ? -1 : 1);
}

function onPointerDown(event: PointerEvent, item: RouteRuleViewItem): void {
  if (ruleInteractionDisabled.value || drag || (event.pointerType === "mouse" && event.button !== 0)) return;
  const handle = event.currentTarget as HTMLElement;
  const row = handle.closest<HTMLElement>(".route-rule-row");
  const list = row?.closest<HTMLElement>(".route-rule-list");
  if (!row || !list) return;
  event.preventDefault();
  handle.focus({ preventScroll: true });
  drag = { pointerId: event.pointerId, handle, row, list, key: item.key, startX: event.clientX, startY: event.clientY, lastClientY: event.clientY, offsetY: 0, started: false, ghost: null, scrollFrame: null };
  handle.setPointerCapture?.(event.pointerId);
  document.addEventListener("pointermove", onPointerMove, { passive: false });
  document.addEventListener("pointerup", onPointerUp);
  document.addEventListener("pointercancel", onPointerCancel);
  document.addEventListener("keydown", onDragKeydown);
}

function beginDrag(): void {
  if (!drag || drag.started) return;
  const rect = drag.row.getBoundingClientRect();
  const ghost = drag.row.cloneNode(true) as HTMLElement;
  ghost.classList.add("route-rule-drag-ghost");
  ghost.setAttribute("aria-hidden", "true");
  ghost.style.width = `${rect.width}px`;
  ghost.style.height = `${rect.height}px`;
  ghost.style.left = `${rect.left}px`;
  ghost.style.top = `${rect.top}px`;
  drag.offsetY = drag.startY - rect.top;
  drag.ghost = ghost;
  drag.started = true;
  drag.row.classList.add("is-drag-placeholder");
  document.body.classList.add("is-route-rule-dragging");
  document.body.append(ghost);
}

function placeDraggedRule(clientY: number): void {
  if (!drag?.started || !drag.ghost) return;
  drag.ghost.style.top = `${clientY - drag.offsetY}px`;
  const nextItem = [...drag.list.children]
    .filter((element) => element !== drag?.row)
    .find((element) => {
      const rect = element.getBoundingClientRect();
      return clientY < rect.top + rect.height / 2;
    });
  drag.list.insertBefore(drag.row, nextItem ?? null);
}

function updateAutoScroll(clientY: number): void {
  if (!drag?.started) return;
  if (drag.scrollFrame !== null) cancelAnimationFrame(drag.scrollFrame);
  drag.scrollFrame = null;
  const edge = Math.min(80, window.innerHeight / 4);
  const distance = clientY < edge ? clientY - edge : clientY > window.innerHeight - edge ? clientY - (window.innerHeight - edge) : 0;
  if (!distance) return;
  const step = () => {
    if (!drag?.started) return;
    window.scrollBy(0, Math.sign(distance) * Math.min(18, Math.max(4, Math.abs(distance) / 4)));
    placeDraggedRule(drag.lastClientY);
    drag.scrollFrame = requestAnimationFrame(step);
  };
  drag.scrollFrame = requestAnimationFrame(step);
}

function onPointerMove(event: PointerEvent): void {
  if (!drag || event.pointerId !== drag.pointerId) return;
  event.preventDefault();
  drag.lastClientY = event.clientY;
  if (!drag.started && Math.hypot(event.clientX - drag.startX, event.clientY - drag.startY) < 4) return;
  beginDrag();
  placeDraggedRule(event.clientY);
  updateAutoScroll(event.clientY);
}

function onPointerUp(event: PointerEvent): void {
  if (drag && event.pointerId === drag.pointerId) finishDrag(true);
}

function onPointerCancel(event: PointerEvent): void {
  if (drag && event.pointerId === drag.pointerId) finishDrag(false);
}

function onDragKeydown(event: KeyboardEvent): void {
  if (event.key !== "Escape" || !drag) return;
  event.preventDefault();
  finishDrag(false);
}

function cleanupDrag(): void {
  if (!drag) return;
  if (drag.scrollFrame !== null) cancelAnimationFrame(drag.scrollFrame);
  try { if (drag.handle.hasPointerCapture?.(drag.pointerId)) drag.handle.releasePointerCapture?.(drag.pointerId); } catch { /* DOM may already be redrawn. */ }
  drag.ghost?.remove();
  drag.row.classList.remove("is-drag-placeholder");
  document.body.classList.remove("is-route-rule-dragging");
  document.removeEventListener("pointermove", onPointerMove);
  document.removeEventListener("pointerup", onPointerUp);
  document.removeEventListener("pointercancel", onPointerCancel);
  document.removeEventListener("keydown", onDragKeydown);
  drag = null;
}

function finishDrag(commit: boolean): void {
  if (!drag) return;
  const current = drag;
  const orderedKeys = current.started
    ? [...current.list.children].flatMap((element) => element instanceof HTMLElement && element.dataset.routeKey ? [element.dataset.routeKey] : [])
    : [];
  const movedKey = current.key;
  const started = current.started;
  cleanupDrag();
  renderVersion.value++;
  if (!commit || !started) return;
  const byKey = new Map(ruleItems.value.map((item) => [item.key, item]));
  const items = orderedKeys.flatMap((key) => key === "marker" ? [] : byKey.get(key) ? [byKey.get(key)!] : []);
  const insertion = orderedKeys.indexOf("marker");
  if (items.length === ruleItems.value.length && insertion >= 0) persistOrder(items, insertion, movedKey);
}
</script>

<template>
  <section class="route-panel">
    <article class="route-section common-section">
      <header class="card-head"><div><h2>通用设置</h2><p>控制接口探测、域名解析与未命中规则时的默认出口</p></div><div class="header-actions"><button class="json-button" type="button" @click="emit('editJson')"><AppIcon name="code" :size="14" />编辑全部 JSON</button><span class="section-badge">全局行为</span></div></header>
      <div class="route-common-body">
        <div class="route-common-toggles">
          <div class="route-common-toggle"><span class="route-common-icon"><AppIcon name="search" :size="15" /></span><div><strong>查找进程信息</strong><small>为路由规则补充进程名称与路径</small></div><button class="setting-switch" type="button" role="switch" :aria-checked="findProcess" aria-label="切换查找进程信息" :disabled="Boolean(error)" :class="{ on: findProcess }" @click="toggleCommon('find_process', !findProcess)"><span /></button></div>
          <div class="route-common-toggle"><span class="route-common-icon"><AppIcon name="globe" :size="15" /></span><div><strong>自动检测出站接口</strong><small>自动选择可用的系统网络接口</small></div><button class="setting-switch" type="button" role="switch" :aria-checked="autoDetectInterface" aria-label="切换自动检测出站接口" :disabled="Boolean(error)" :class="{ on: autoDetectInterface }" @click="toggleCommon('auto_detect_interface', !autoDetectInterface)"><span /></button></div>
        </div>
        <div class="route-common-fields">
          <label v-if="!autoDetectInterface" class="route-common-field"><span><strong>出站接口名称</strong><small>关闭自动检测后使用的默认接口</small></span><select aria-label="出站接口名称" :value="defaultInterface" :disabled="Boolean(error)" @change="changeDefaultInterface"><option value="">请选择网络接口</option><option v-for="name in interfaceOptions" :key="name" :value="name">{{ name }}</option></select></label>
          <label class="route-common-field"><span><strong>解析节点域名的 DNS 服务器</strong><small>解析代理节点服务器域名时使用</small></span><select aria-label="解析节点域名的 DNS 服务器" :value="defaultDnsServer" :disabled="Boolean(error)" @change="changeDefaultDns"><option value="">由 DNS 路由规则决定</option><option v-for="tag in dnsOptions" :key="tag" :value="tag">{{ tag }}</option></select></label>
          <label class="route-common-field"><span><strong>默认出站标签</strong><small>所有规则均未命中时使用</small></span><select aria-label="默认出站标签" :value="defaultOutbound" :disabled="Boolean(error)" @change="changeFinal"><option value="">未设置</option><option v-for="tag in outboundOptions" :key="tag" :value="tag">{{ tag }}</option></select></label>
        </div>
      </div>
    </article>

    <article class="route-section route-list-card">
      <header class="card-head"><div><h2>路由规则</h2><p>按从上到下的顺序匹配；拖动左侧手柄调整优先级，新增规则会放在插入点之后</p></div><div class="header-actions"><span class="section-badge">已启用 {{ activeRules.length }}/{{ ruleItems.length }}</span><button class="add-button" type="button" :disabled="ruleInteractionDisabled" @click="openEditor(null)"><AppIcon name="plus" :size="14" />新增规则</button></div></header>
      <div class="route-rule-list">
        <template v-for="entry in listItems" :key="`${entry.key}:${renderVersion}`">
          <div v-if="'marker' in entry" class="route-insertion-point" data-route-key="marker"><span /><button type="button" :disabled="ruleInteractionDisabled" @click="openEditor(null)"><AppIcon name="plus" :size="12" />新规则将插入到这里</button><span /></div>
          <article v-else class="route-rule-row" :class="{ 'is-disabled': !entry.enabled }" :data-route-key="entry.key">
            <button class="entry-drag" type="button" data-route-rule-drag-handle title="拖动调整顺序" :aria-label="`拖动调整第 ${Number(displayIndex(entry))} 条规则顺序，按上下方向键也可调整`" :disabled="ruleInteractionDisabled" @pointerdown="onPointerDown($event, entry)" @keydown="onHandleKeydown($event, entry)"><span class="grip-dots" /></button>
            <label class="rule-switch" :title="entry.rule ? entry.enabled ? '停用规则' : '启用规则' : '无法停用非对象规则'"><input type="checkbox" :checked="entry.enabled" :disabled="ruleInteractionDisabled || !entry.rule" :aria-label="`${entry.enabled ? '停用' : '启用'}第 ${Number(displayIndex(entry))} 条规则`" @change="toggleRule(entry)" /><span /></label>
            <span class="rule-index">{{ displayIndex(entry) }}</span>
            <div class="rule-copy"><strong>{{ ruleTypeLabel(entry) }}<span>,</span>{{ rulePayloadLabel(entry) }}<em v-if="entry.rule?.invert === true">反向</em></strong><small>{{ actionLabel(entry) }} · {{ actionDetail(entry) }}</small></div>
            <span class="action-badge" :class="`action-${routeRuleAction(entry.rule)}`"><AppIcon :name="actionIcon(entry)" :size="12" />{{ actionLabel(entry) }}</span>
            <div class="rule-actions"><button type="button" :aria-label="`编辑规则 ${Number(displayIndex(entry))}`" title="编辑规则" :disabled="ruleInteractionDisabled || !entry.rule" @click="openEditor(entry.key)"><AppIcon name="edit" :size="14" /></button><button class="danger" type="button" :aria-label="`删除规则 ${Number(displayIndex(entry))}`" title="删除规则" :disabled="ruleInteractionDisabled" @click="remove(entry)"><AppIcon name="trash" :size="14" /></button></div>
          </article>
        </template>
      </div>
      <p v-if="editorState.disabledRules.length" class="disabled-note"><AppIcon name="info" :size="13" />停用规则保存在草稿编辑状态中，不会进入实际 route JSON 或应用结果。</p>
    </article>
    <p v-if="remoteRuleSet" class="panel-warning" role="alert">原生 remote 规则集会阻止应用，请改用规则集中心中的本地受管资源。</p>
    <p v-if="invalidRules" class="panel-warning" role="alert">route.rules 必须是数组；请使用“编辑全部 JSON”修复后再进行结构化编辑。</p>
    <p v-if="resourceError" class="panel-warning" role="status">{{ resourceError }}；仍可编辑当前 JSON 中已有的规则集引用。</p>
    <p v-if="error" class="panel-error" role="alert">{{ error }}</p>
    <RouteRuleEditorDialog :open="editorOpen" :rule="editingItem?.rule ?? null" :rule-text="editingItem?.text" :options="options" :inbounds="inbounds" :outbounds="outbounds" :dns-servers="dnsServers" :default-outbound="defaultOutbound" @close="closeEditor" @submit="submit" />
    <AppToast v-if="notice" :message="notice" :tone="noticeTone" />
  </section>
</template>

<style scoped>
.route-panel { display: grid; gap: 10px; }.route-section { min-width: 0; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.card-head { display: flex; min-height: 50px; align-items: center; justify-content: space-between; gap: 16px; padding: 12px 14px; border-bottom: 1px solid var(--line); }.card-head h2, .card-head p { margin: 0; }.card-head h2 { font-size: 12px; }.card-head p { margin-top: 2px; color: var(--text-faint); font-size: 9px; }.header-actions { display: flex; flex: 0 0 auto; align-items: center; gap: 8px; }.section-badge { display: inline-flex; min-height: 25px; align-items: center; padding: 4px 8px; border: 1px solid color-mix(in srgb, var(--primary) 13%, var(--line)); border-radius: 999px; color: var(--primary); background: var(--primary-soft); font-size: 8px; font-weight: 680; white-space: nowrap; }.json-button, .add-button { display: inline-flex; min-height: 31px; align-items: center; gap: 5px; padding: 5px 8px; font-size: 9px; }.json-button { color: var(--text-soft); border: 1px solid var(--line); background: var(--surface-soft); }.add-button { color: #fff; background: var(--primary); }.route-common-body { display: grid; gap: 12px; padding: 10px; }.route-common-toggles, .route-common-fields { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }.route-common-toggle { display: flex; min-width: 0; min-height: 67px; align-items: center; gap: 11px; padding: 11px 12px; border: 1px solid var(--line); border-radius: 11px; background: var(--surface-soft); }.route-common-toggle > div { min-width: 0; flex: 1; }.route-common-toggle strong, .route-common-toggle small, .route-common-field strong, .route-common-field small { display: block; }.route-common-toggle strong, .route-common-field strong { color: var(--text); font-size: 10px; font-weight: 660; }.route-common-toggle small, .route-common-field small { margin-top: 2px; color: var(--text-faint); font-size: 8px; font-weight: 500; }.route-common-icon { display: grid; width: 34px; height: 34px; flex: 0 0 auto; place-items: center; border-radius: 9px; color: var(--primary); background: var(--primary-soft); }.setting-switch { position: relative; width: 34px; min-height: 20px; flex: 0 0 auto; padding: 0; border-radius: 7px; background: var(--line-strong); }.setting-switch span { position: absolute; top: 4px; left: 4px; width: 12px; height: 12px; border-radius: 4px; background: #fff; transition: transform 150ms ease; }.setting-switch.on { background: var(--primary); }.setting-switch.on span { transform: translateX(14px); }.route-common-field { display: grid; min-width: 0; grid-template-columns: minmax(150px, .85fr) minmax(180px, 1.15fr); align-items: center; gap: 12px; padding: 10px 12px; border: 1px solid var(--line); border-radius: 11px; background: var(--surface); }.route-common-field:last-child:nth-child(odd) { grid-column: 1 / -1; }.route-common-field select { width: 100%; min-width: 0; font-size: 9px; }.route-rule-list { display: grid; padding: 8px 14px 14px; }.route-rule-row { display: grid; min-width: 0; min-height: 55px; grid-template-columns: auto auto auto minmax(0, 1fr) auto auto; align-items: center; gap: 9px; padding: 8px 5px; border-bottom: 1px solid var(--line); transition: opacity 150ms ease, background 150ms ease; }.route-rule-row:hover { background: var(--surface-soft); }.route-rule-row.is-disabled { opacity: .48; }.entry-drag { display: grid; width: 27px; min-height: 34px; place-items: center; padding: 0; color: var(--text-faint); border-radius: 7px; background: transparent; cursor: grab; touch-action: none; }.entry-drag:hover, .entry-drag:focus-visible { color: var(--primary); background: var(--primary-soft); }.entry-drag:active, :global(body.is-route-rule-dragging) .entry-drag { cursor: grabbing; }.grip-dots { width: 12px; height: 18px; background: radial-gradient(circle, currentColor 1.1px, transparent 1.3px) 0 0 / 6px 6px; }.rule-switch { position: relative; display: block; width: 31px; height: 20px; }.rule-switch input { position: absolute; width: 1px; height: 1px; opacity: 0; }.rule-switch span { position: relative; display: block; width: 31px; height: 20px; border-radius: 6px; background: var(--line-strong); cursor: pointer; transition: background 150ms ease; }.rule-switch span::after { position: absolute; top: 4px; left: 4px; width: 4px; height: 12px; border-radius: 3px; content: ""; background: #fff; transition: left 150ms ease; }.rule-switch input:checked + span { background: var(--primary); }.rule-switch input:checked + span::after { left: 23px; }.rule-switch input:focus-visible + span { outline: 3px solid color-mix(in srgb, var(--primary) 22%, transparent); outline-offset: 2px; }.rule-index { color: var(--text-faint); font: 8px ui-monospace, Consolas, monospace; }.rule-copy { min-width: 0; }.rule-copy strong, .rule-copy small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.rule-copy strong { color: var(--text); font: 650 10px ui-monospace, Consolas, monospace; }.rule-copy strong > span { color: var(--text-faint); }.rule-copy em { display: inline-flex; margin-left: 7px; padding: 2px 5px; border-radius: 5px; color: var(--orange); background: var(--orange-soft); font: 7px var(--font); font-style: normal; }.rule-copy small { margin-top: 3px; color: var(--text-faint); font-size: 8px; }.action-badge { display: inline-flex; min-height: 25px; align-items: center; gap: 5px; padding: 4px 7px; border-radius: 7px; color: var(--primary); background: var(--primary-soft); font-size: 8px; font-weight: 650; white-space: nowrap; }.action-badge.action-reject { color: var(--red); background: var(--red-soft); }.action-badge.action-hijack-dns, .action-badge.action-resolve { color: var(--cyan); background: var(--cyan-soft); }.action-badge.action-sniff { color: var(--violet); background: color-mix(in srgb, var(--violet) 11%, var(--surface)); }.rule-actions { display: flex; gap: 2px; }.rule-actions button { display: grid; width: 30px; min-height: 30px; place-items: center; padding: 0; color: var(--text-faint); background: transparent; }.rule-actions button:hover { color: var(--primary); background: var(--primary-soft); }.rule-actions button.danger:hover { color: var(--red); background: var(--red-soft); }.route-insertion-point { display: flex; min-height: 46px; align-items: center; gap: 10px; }.route-insertion-point > span { height: 1px; flex: 1; background: var(--line); }.route-insertion-point button { display: inline-flex; min-height: 29px; align-items: center; gap: 5px; padding: 5px 8px; border-radius: 7px; color: var(--text-soft); background: var(--surface-soft); font-size: 8px; }.route-insertion-point button:hover { color: var(--primary); background: var(--primary-soft); }.route-rule-row.is-drag-placeholder { border: 1px dashed color-mix(in srgb, var(--primary) 45%, var(--line)); border-radius: 10px; opacity: 1; background: var(--primary-soft); }.route-rule-row.is-drag-placeholder > * { visibility: hidden; }:global(.route-rule-drag-ghost) { position: fixed; z-index: 120; overflow: hidden; margin: 0; border: 1px solid color-mix(in srgb, var(--primary) 38%, var(--line)); border-radius: 10px; opacity: .96; background: var(--surface); box-shadow: var(--shadow-lg); pointer-events: none; transition: none; }:global(.route-rule-drag-ghost.is-disabled) { opacity: .82; }:global(body.is-route-rule-dragging) { cursor: grabbing; user-select: none; }.disabled-note { display: flex; align-items: flex-start; gap: 6px; margin: -5px 14px 13px; padding: 8px 9px; border-radius: 8px; color: var(--text-faint); background: var(--surface-soft); font-size: 8px; }.disabled-note .icon { color: var(--primary); }.panel-warning, .panel-error { margin: 0; padding: 9px 10px; border-radius: 9px; font-size: 9px; }.panel-warning { color: var(--orange); background: var(--orange-soft); }.panel-error { color: var(--red); background: var(--red-soft); }
@media (max-width: 920px) { .route-common-toggles, .route-common-fields { grid-template-columns: 1fr; }.route-common-field:last-child:nth-child(odd) { grid-column: auto; } }
@media (max-width: 760px) { .card-head { align-items: flex-start; }.route-rule-row { grid-template-columns: auto auto minmax(0, 1fr) auto; gap: 8px; }.rule-index, .action-badge { display: none; } }
@media (max-width: 560px) { .card-head { flex-direction: column; gap: 9px; }.header-actions { width: 100%; justify-content: flex-end; }.route-list-card .header-actions .section-badge { margin-right: auto; }.route-common-field { grid-template-columns: 1fr; gap: 7px; }.route-rule-list { padding-inline: 8px; }.route-rule-row { grid-template-columns: auto auto minmax(0, 1fr); }.rule-actions { grid-column: 2 / -1; justify-content: flex-end; padding-top: 4px; border-top: 1px solid var(--line); }.rule-actions button { flex: 1; }.route-insertion-point { gap: 6px; }.route-insertion-point button { white-space: nowrap; } }
@media (max-width: 560px) { .route-rule-row { grid-template-columns: auto auto minmax(0, 1fr) auto; }.rule-actions { grid-column: auto; justify-content: initial; padding-top: 0; border-top: 0; }.rule-actions button { flex: 0 0 auto; } }
</style>
