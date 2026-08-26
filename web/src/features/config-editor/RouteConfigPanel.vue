<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import AppIcon from "@/layout/AppIcon.vue";
import type { Draft } from "@/features/profiles/types";
import { useRuleResourceStore } from "@/features/rule-resources/store";
import RouteRuleEditorDialog from "./RouteRuleEditorDialog.vue";
import {
  buildRouteRuleSetOptions,
  isRouteObject,
  mergeSelectedRuleBindings,
  removeRouteRule,
  routeOutboundTags,
  routeRules,
  routeRuleSetTags,
  updateRouteFinal,
  upsertRouteRule,
  type RoutePanelSave,
  type RouteRule,
  type RouteRuleEditorResult
} from "./route";

const props = defineProps<{ draft: Draft; error?: string }>();
const emit = defineEmits<{ editJson: []; save: [payload: RoutePanelSave] }>();
const store = useRuleResourceStore();
const editingIndex = ref<number | null>(null);
const editorOpen = ref(false);
const resourceError = ref("");

const route = computed<RouteRule>(() => isRouteObject(props.draft.config.route) ? props.draft.config.route : {});
const rules = computed(() => routeRules(route.value));
const outbounds = computed(() => routeOutboundTags(props.draft.config));
const defaultOutbound = computed(() => typeof route.value.final === "string" ? route.value.final : "");
const options = computed(() => buildRouteRuleSetOptions(route.value, props.draft.bindings, store.resources.value));
const editingRule = computed(() => {
  if (editingIndex.value === null) return null;
  const item = rules.value[editingIndex.value];
  return isRouteObject(item) ? item : null;
});
const remoteRuleSet = computed(() => Array.isArray(route.value.rule_set) && route.value.rule_set.some((item) => isRouteObject(item) && item.type === "remote"));
const routedCount = computed(() => rules.value.filter((rule) => isRouteObject(rule) && (typeof rule.action !== "string" || rule.action === "route")).length);
const specialCount = computed(() => rules.value.length - routedCount.value);

onMounted(async () => {
  if (store.resources.value.length) return;
  try { await store.refresh(); }
  catch (cause) { resourceError.value = cause instanceof Error ? cause.message : "无法读取规则集"; }
});

function bufferText(): string {
  const source = props.draft.buffers.route;
  return source?.trim() ? source : JSON.stringify(route.value, null, 2);
}

function openEditor(index: number | null): void {
  editingIndex.value = index;
  editorOpen.value = true;
}

function closeEditor(): void {
  editorOpen.value = false;
  editingIndex.value = null;
}

function submit(result: RouteRuleEditorResult): void {
  const index = editingIndex.value ?? rules.value.length;
  emit("save", {
    text: upsertRouteRule(bufferText(), index, result.rule),
    bindings: mergeSelectedRuleBindings(props.draft.bindings, options.value, result.resourceIds)
  });
  closeEditor();
}

function remove(index: number): void {
  emit("save", { text: removeRouteRule(bufferText(), index), bindings: props.draft.bindings.map((binding) => ({ ...binding })) });
  if (editingIndex.value === index) closeEditor();
}

function changeFinal(event: Event): void {
  const tag = (event.currentTarget as HTMLSelectElement).value;
  emit("save", { text: updateRouteFinal(bufferText(), tag), bindings: props.draft.bindings.map((binding) => ({ ...binding })) });
}

function list(value: unknown): string[] {
  if (typeof value === "string") return value.trim() ? [value.trim()] : [];
  if (!Array.isArray(value)) return [];
  return value.flatMap((item) => typeof item === "string" && item.trim() ? [item.trim()] : []);
}

function ruleTitle(rule: unknown, index: number): string {
  if (!isRouteObject(rule)) return `无法识别的规则 ${index + 1}`;
  const ruleSets = routeRuleSetTags(rule);
  if (ruleSets.length === 1) return ruleSets[0]!;
  if (ruleSets.length > 1) return `${ruleSets[0]} 等 ${ruleSets.length} 个规则集`;
  const conditions: Array<[string, unknown]> = [["protocol", rule.protocol], ["network", rule.network], ["clash_mode", rule.clash_mode], ["inbound", rule.inbound]];
  for (const [label, value] of conditions) {
    const values = list(value);
    if (values.length) return `${label} · ${values.join("、")}`;
  }
  return `自定义匹配规则 ${index + 1}`;
}

function actionLabel(rule: unknown): string {
  if (!isRouteObject(rule)) return "需使用 JSON 编辑";
  const action = typeof rule.action === "string" ? rule.action : "route";
  if (action === "route") return typeof rule.outbound === "string" ? `路由到 ${rule.outbound}` : "使用默认出站";
  if (action === "reject") return "拒绝连接";
  if (action === "hijack-dns") return "DNS 劫持";
  return action;
}

function conditionLabels(rule: unknown): string[] {
  if (!isRouteObject(rule)) return ["unknown"];
  const labels: string[] = [];
  if (routeRuleSetTags(rule).length) labels.push(`rule_set · ${routeRuleSetTags(rule).length}`);
  for (const key of ["protocol", "network", "clash_mode", "inbound", "domain", "domain_suffix", "ip_cidr"]) {
    if (rule[key] !== undefined) labels.push(key);
  }
  return labels.length ? labels.slice(0, 4) : ["catch-all"];
}
</script>

<template>
  <section class="route-panel">
    <div class="route-layout">
      <article class="route-list-card">
        <header><div><h2>路由规则</h2><p>规则按从上到下的顺序匹配流量</p></div><div><button class="json-button" type="button" @click="emit('editJson')"><AppIcon name="code" :size="14" />编辑全部 JSON</button><button class="add-button" type="button" :disabled="Boolean(error)" @click="openEditor(null)"><AppIcon name="plus" :size="14" />新增规则</button></div></header>
        <div v-if="rules.length" class="route-rule-list">
          <article v-for="(rule, index) in rules" :key="index" class="route-rule-row" :data-route-rule-index="index">
            <span class="rule-order">{{ index + 1 }}</span><span class="rule-icon"><AppIcon :name="isRouteObject(rule) && rule.action === 'reject' ? 'shield' : isRouteObject(rule) && rule.action === 'hijack-dns' ? 'dns' : 'route'" :size="17" /></span>
            <div class="rule-copy"><strong>{{ ruleTitle(rule, index) }}</strong><span>{{ actionLabel(rule) }}</span><div><b v-for="label in conditionLabels(rule)" :key="label">{{ label }}</b></div></div>
            <div class="rule-actions"><button type="button" :aria-label="`编辑规则 ${index + 1}`" title="编辑规则" :disabled="Boolean(error) || !isRouteObject(rule)" @click="openEditor(index)"><AppIcon name="edit" :size="14" /></button><button class="danger" type="button" :aria-label="`删除规则 ${index + 1}`" title="删除规则" :disabled="Boolean(error)" @click="remove(index)"><AppIcon name="trash" :size="14" /></button></div>
          </article>
        </div>
        <button v-else class="empty-state" type="button" :disabled="Boolean(error)" @click="openEditor(null)"><AppIcon name="route" :size="31" /><strong>尚未添加路由规则</strong><span>从现有规则集中选择条件，创建第一条规则</span></button>
      </article>

      <aside>
        <article class="side-card"><header><div><h2>默认出站</h2><p>所有规则均未命中时使用</p></div></header><div class="default-outbound"><label><span>最终出站<small>final</small></span><select aria-label="默认出站" :value="defaultOutbound" :disabled="Boolean(error)" @change="changeFinal"><option value="">未设置</option><option v-if="defaultOutbound && !outbounds.includes(defaultOutbound)" :value="defaultOutbound">{{ defaultOutbound }} · 已失效</option><option v-for="tag in outbounds" :key="tag" :value="tag">{{ tag }}</option></select></label></div></article>
        <article class="side-card"><header><div><h2>规则概览</h2><p>当前配置中的执行动作</p></div></header><div class="route-stats"><div><strong>{{ rules.length }}</strong><span>全部规则</span></div><div><strong>{{ routedCount }}</strong><span>路由动作</span></div><div><strong>{{ specialCount }}</strong><span>特殊动作</span></div></div><p class="order-note"><AppIcon name="info" :size="13" />越靠前的规则优先级越高；完整字段可继续通过 JSON 编辑。</p></article>
      </aside>
    </div>
    <p v-if="remoteRuleSet" class="panel-warning" role="alert">原生 remote 规则集会阻止应用，请改用规则集中心中的本地受管资源。</p>
    <p v-if="resourceError" class="panel-warning" role="status">{{ resourceError }}；仍可编辑当前 JSON 中已有的规则集引用。</p>
    <p v-if="error" class="panel-error" role="alert">{{ error }}</p>
    <RouteRuleEditorDialog :open="editorOpen" :rule="editingRule" :options="options" :outbounds="outbounds" :default-outbound="defaultOutbound" @close="closeEditor" @submit="submit" />
  </section>
</template>

<style scoped>
.route-panel { display: grid; gap: 10px; }.route-layout { display: grid; grid-template-columns: minmax(0, 1.65fr) minmax(230px, .65fr); gap: 12px; }.route-list-card, .side-card { border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.route-list-card > header, .side-card > header { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; padding: 14px 15px; border-bottom: 1px solid var(--line); }.route-list-card h2, .side-card h2, .route-list-card header p, .side-card header p { margin: 0; }.route-list-card h2, .side-card h2 { font-size: 12px; }.route-list-card header p, .side-card header p { margin-top: 2px; color: var(--text-faint); font-size: 9px; }.route-list-card header > div:last-child { display: flex; gap: 6px; }.route-list-card header button { display: inline-flex; min-height: 31px; align-items: center; gap: 5px; padding: 5px 8px; font-size: 9px; }.json-button { color: var(--text-soft); border: 1px solid var(--line); background: var(--surface-soft); }.add-button { color: #fff; background: var(--primary); }.route-rule-list { display: grid; gap: 7px; padding: 10px; }.route-rule-row { display: grid; min-width: 0; grid-template-columns: 25px 34px minmax(0, 1fr) auto; align-items: center; gap: 9px; padding: 9px 10px; border: 1px solid var(--line); border-radius: 11px; background: var(--surface); transition: 150ms ease; }.route-rule-row:hover { border-color: color-mix(in srgb, var(--primary) 25%, var(--line)); box-shadow: 0 7px 20px rgb(34 50 89 / 6%); transform: translateY(-1px); }.rule-order { display: grid; width: 23px; height: 23px; place-items: center; border-radius: 7px; color: var(--text-faint); background: var(--surface-soft); font-size: 8px; font-weight: 700; font-variant-numeric: tabular-nums; }.rule-icon { display: grid; width: 34px; height: 34px; place-items: center; border-radius: 10px; color: var(--primary); background: var(--primary-soft); }.rule-copy { min-width: 0; }.rule-copy > strong, .rule-copy > span { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.rule-copy > strong { font-size: 10px; }.rule-copy > span { margin-top: 2px; color: var(--text-soft); font-size: 8px; }.rule-copy > div { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 5px; }.rule-copy b { padding: 2px 5px; border-radius: 5px; color: var(--primary); background: var(--primary-soft); font: 7px ui-monospace, monospace; }.rule-actions { display: flex; gap: 2px; }.rule-actions button { display: grid; width: 30px; min-height: 30px; place-items: center; padding: 0; color: var(--text-faint); background: transparent; }.rule-actions button:hover { color: var(--primary); background: var(--primary-soft); }.rule-actions button.danger:hover { color: var(--red); background: var(--red-soft); }.empty-state { display: grid; width: calc(100% - 20px); min-height: 260px; place-items: center; align-content: center; gap: 7px; margin: 10px; color: var(--primary); border: 1px dashed var(--line-strong); background: var(--surface-soft); }.empty-state strong { color: var(--text); font-size: 12px; }.empty-state span { color: var(--text-faint); font-size: 9px; font-weight: 400; }.route-layout aside { display: grid; align-content: start; gap: 12px; }.default-outbound { padding: 12px; }.default-outbound label { display: grid; gap: 6px; }.default-outbound label > span { display: flex; justify-content: space-between; color: var(--text-soft); font-size: 9px; font-weight: 650; }.default-outbound small { color: var(--text-faint); font-size: 8px; font-weight: 520; }.default-outbound select { width: 100%; min-width: 0; font-size: 9px; }.route-stats { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 6px; padding: 11px 11px 6px; }.route-stats > div { display: grid; min-width: 0; place-items: center; gap: 2px; padding: 9px 4px; border-radius: 9px; background: var(--surface-soft); }.route-stats strong { color: var(--primary); font-size: 15px; }.route-stats span { color: var(--text-faint); font-size: 7px; white-space: nowrap; }.order-note { display: flex; align-items: flex-start; gap: 6px; margin: 0; padding: 6px 12px 12px; color: var(--text-faint); font-size: 8px; }.order-note .icon { color: var(--primary); }.panel-warning, .panel-error { margin: 0; padding: 9px 10px; border-radius: 9px; font-size: 9px; }.panel-warning { color: var(--orange); background: var(--orange-soft); }.panel-error { color: var(--red); background: var(--red-soft); }
@media (max-width: 900px) { .route-layout { grid-template-columns: 1fr; }.route-layout aside { grid-template-columns: 1fr 1fr; } }
@media (max-width: 620px) { .route-list-card > header { flex-direction: column; }.route-list-card header > div:last-child { width: 100%; }.route-list-card header button { flex: 1; justify-content: center; }.route-layout aside { grid-template-columns: 1fr; }.route-rule-row { grid-template-columns: 23px minmax(0, 1fr) auto; }.rule-icon { display: none; }.rule-actions button { width: 28px; } }
</style>
