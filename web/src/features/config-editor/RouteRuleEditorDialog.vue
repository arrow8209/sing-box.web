<script setup lang="ts">
import { computed, ref, watch } from "vue";
import AppDialog from "@/components/AppDialog.vue";
import AppIcon from "@/layout/AppIcon.vue";
import {
  cloneRouteValue,
  routeRuleSetTags,
  type RouteRule,
  type RouteRuleEditorResult,
  type RouteRuleSetOption
} from "./route";

const props = defineProps<{
  open: boolean;
  rule: RouteRule | null;
  options: RouteRuleSetOption[];
  outbounds: string[];
  defaultOutbound?: string;
}>();
const emit = defineEmits<{ close: []; submit: [result: RouteRuleEditorResult] }>();

const query = ref("");
const selectedIds = ref<string[]>([]);
const action = ref("route");
const outbound = ref("");
const selectedSet = computed(() => new Set(selectedIds.value));
const filteredOptions = computed(() => {
  const keyword = query.value.trim().toLocaleLowerCase("zh-CN");
  if (!keyword) return props.options;
  return props.options.filter((option) => `${option.name} ${option.tag} ${option.detail}`.toLocaleLowerCase("zh-CN").includes(keyword));
});
const actionOptions = computed(() => {
  const values = [
    { value: "route", label: "路由", icon: "route" },
    { value: "reject", label: "拒绝", icon: "shield" },
    { value: "hijack-dns", label: "DNS 劫持", icon: "dns" }
  ];
  if (action.value && !values.some((item) => item.value === action.value)) values.push({ value: action.value, label: action.value, icon: "code" });
  return values;
});
const validation = computed(() => {
  if (!props.rule && !selectedIds.value.length) return "请选择至少一个规则集。";
  if (action.value === "route" && !outbound.value) return "请选择规则命中后的出站。";
  return "";
});

function initialize(): void {
  query.value = "";
  const tags = routeRuleSetTags(props.rule);
  selectedIds.value = tags.flatMap((tag) => {
    const option = props.options.find((candidate) => candidate.tag === tag);
    return option ? [option.id] : [];
  });
  action.value = typeof props.rule?.action === "string" ? props.rule.action : "route";
  const currentOutbound = typeof props.rule?.outbound === "string" ? props.rule.outbound : "";
  outbound.value = currentOutbound || props.defaultOutbound || props.outbounds[0] || "";
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

function submit(): void {
  if (validation.value) return;
  const rule = props.rule ? cloneRouteValue(props.rule) : {};
  rule.action = action.value;
  const selectedOptions = selectedIds.value.flatMap((id) => {
    const option = props.options.find((candidate) => candidate.id === id);
    return option ? [option] : [];
  });
  const tags = [...new Set(selectedOptions.map((option) => option.tag))];
  if (tags.length) rule.rule_set = tags;
  else delete rule.rule_set;
  if (action.value === "route") rule.outbound = outbound.value;
  else delete rule.outbound;
  emit("submit", {
    rule,
    resourceIds: selectedOptions.flatMap((option) => option.resourceId ? [option.resourceId] : [])
  });
}
</script>

<template>
  <AppDialog class="route-rule-editor-dialog" :open="open" :title="rule ? '编辑规则' : '新增规则'" description="选择匹配的规则集与命中后的处理方式" initial-focus="input[type=search]" @close="emit('close')">
    <form id="route-rule-editor-form" class="route-rule-editor-form" @submit.prevent="submit">
      <div class="editor-row"><span class="editor-label">处理方式<small>action</small></span><div class="action-control">
        <label v-for="item in actionOptions" :key="item.value"><input v-model="action" type="radio" name="route-action" :value="item.value" /><span><AppIcon :name="item.icon" :size="13" />{{ item.label }}</span></label>
      </div></div>
      <label v-if="action === 'route'" class="editor-row"><span class="editor-label">目标出站<small>outbound</small></span><select v-model="outbound" aria-label="目标出站"><option value="" disabled>请选择出站</option><option v-for="tag in outbounds" :key="tag" :value="tag">{{ tag }}</option></select></label>

      <div class="ruleset-divider"><span>全部规则集</span><strong>{{ options.length }} 项 · 已选 {{ selectedIds.length }} 项</strong></div>
      <label class="ruleset-search"><AppIcon name="search" :size="14" /><input v-model="query" type="search" aria-label="搜索规则集" placeholder="搜索规则集名称或 tag" /><span>{{ filteredOptions.length }} / {{ options.length }}</span></label>
      <div class="ruleset-options" aria-label="规则集选择">
        <label v-for="option in filteredOptions" :key="option.id" class="ruleset-option" :class="{ disabled: option.disabled && !selectedSet.has(option.id), missing: option.kind === 'missing' }">
          <input v-model="selectedIds" type="checkbox" :value="option.id" :disabled="option.disabled && !selectedSet.has(option.id)" :aria-label="`选择规则集 ${option.name}`" />
          <span class="ruleset-option-content"><span class="ruleset-copy"><strong>{{ option.name }}</strong><small v-if="option.tag !== option.name">{{ option.tag }}</small><em>{{ option.disabled ? "已停用 · " : "" }}{{ option.detail }}</em></span><AppIcon name="check" :size="13" /></span>
        </label>
        <p v-if="!filteredOptions.length" class="ruleset-empty">{{ options.length ? "没有匹配的规则集" : "尚无规则集，请先到规则集中心导入资源" }}</p>
      </div>
      <p v-if="rule" class="preserve-note"><AppIcon name="info" :size="14" />未在此表单展示的原有匹配字段会继续保留。</p>
      <p v-if="validation" class="validation" role="alert"><AppIcon name="info" :size="14" />{{ validation }}</p>
    </form>
    <template #footer>
      <button class="secondary-button" type="button" @click="emit('close')">取消</button>
      <button class="primary-button" type="submit" form="route-rule-editor-form" :disabled="Boolean(validation)"><AppIcon :name="rule ? 'save' : 'plus'" :size="14" />{{ rule ? "保存规则" : "确认添加" }}</button>
    </template>
  </AppDialog>
</template>

<style scoped>
:global(.route-rule-editor-dialog) { width: min(900px, calc(100vw - 32px)); }.route-rule-editor-form { display: grid; }.editor-row { display: grid; min-width: 0; grid-template-columns: 122px minmax(0, 1fr); align-items: center; gap: 18px; padding: 7px 0; }.editor-label { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; color: var(--text); font-size: 11px; font-weight: 680; }.editor-label small { color: var(--text-faint); font-size: 8px; font-weight: 520; }.action-control { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); overflow: hidden; padding: 3px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-soft); }.action-control input, .ruleset-option > input { position: absolute; width: 1px; height: 1px; overflow: hidden; opacity: 0; }.action-control label > span { display: flex; min-height: 34px; align-items: center; justify-content: center; gap: 6px; border-radius: 7px; color: var(--text-faint); font-size: 9px; font-weight: 650; cursor: pointer; }.action-control input:checked + span { color: var(--primary); background: var(--surface); box-shadow: 0 2px 9px rgb(34 49 83 / 9%); }.action-control input:focus-visible + span, .ruleset-option input:focus-visible + span { outline: 3px solid color-mix(in srgb, var(--primary) 22%, transparent); }.ruleset-divider { display: flex; min-height: 41px; align-items: center; gap: 12px; margin-top: 12px; color: var(--text-soft); font-size: 9px; font-weight: 680; }.ruleset-divider::before, .ruleset-divider::after { height: 1px; content: ""; background: var(--line); }.ruleset-divider::before { width: 22px; }.ruleset-divider::after { flex: 1; }.ruleset-divider strong { order: 3; color: var(--primary); font-size: 8px; }.ruleset-search { display: flex; min-height: 38px; align-items: center; gap: 7px; margin-bottom: 8px; padding: 0 10px; border: 1px solid var(--line); border-radius: 9px; color: var(--text-faint); background: var(--surface-soft); }.ruleset-search:focus-within { border-color: var(--primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 14%, transparent); }.ruleset-search input { min-width: 0; min-height: 34px; flex: 1; padding: 0; border: 0; background: transparent; font-size: 9px; }.ruleset-search input:focus { outline: 0; }.ruleset-search > span { font-size: 8px; font-variant-numeric: tabular-nums; }.ruleset-options { display: grid; max-height: min(390px, 48vh); grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 7px; overflow-y: auto; padding: 1px; scrollbar-width: thin; }.ruleset-option { min-width: 0; }.ruleset-option-content { position: relative; display: flex; min-width: 0; min-height: 64px; align-items: center; padding: 8px 28px 8px 10px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface); cursor: pointer; transition: 150ms ease; }.ruleset-option-content:hover { border-color: color-mix(in srgb, var(--primary) 34%, var(--line)); transform: translateY(-1px); }.ruleset-copy { min-width: 0; }.ruleset-copy strong, .ruleset-copy small, .ruleset-copy em { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.ruleset-copy strong { color: var(--text); font-size: 9px; }.ruleset-copy small { margin-top: 2px; color: var(--text-soft); font: 8px ui-monospace, monospace; }.ruleset-copy em { margin-top: 3px; color: var(--text-faint); font-size: 8px; font-style: normal; }.ruleset-option-content > .icon { position: absolute; top: 7px; right: 7px; color: transparent; }.ruleset-option input:checked + .ruleset-option-content { border-color: color-mix(in srgb, var(--primary) 45%, var(--line)); background: var(--primary-soft); }.ruleset-option input:checked + .ruleset-option-content > .icon { color: var(--primary); }.ruleset-option.disabled .ruleset-option-content { opacity: .55; cursor: not-allowed; }.ruleset-option.missing .ruleset-option-content { border-style: dashed; }.ruleset-empty { min-height: 100px; grid-column: 1 / -1; display: grid; place-items: center; margin: 0; color: var(--text-faint); border: 1px dashed var(--line-strong); border-radius: 10px; background: var(--surface-soft); font-size: 9px; }.preserve-note, .validation { display: flex; align-items: flex-start; gap: 7px; margin: 10px 0 0; padding: 9px 10px; border-radius: 9px; font-size: 9px; }.preserve-note { color: var(--text-soft); border: 1px solid var(--line); background: var(--surface-soft); }.preserve-note .icon { color: var(--primary); }.validation { color: var(--red); background: var(--red-soft); }.secondary-button, .primary-button { display: inline-flex; min-height: 36px; align-items: center; gap: 6px; padding: 7px 11px; border: 1px solid var(--line); font-size: 10px; }.secondary-button { color: var(--text-soft); background: var(--surface); }.primary-button { color: #fff; border-color: var(--primary); background: var(--primary); }
@media (max-width: 720px) { :global(.route-rule-editor-dialog) { width: 100%; }.editor-row { grid-template-columns: 1fr; gap: 6px; padding: 8px 0; }.editor-label { justify-content: flex-start; }.ruleset-options { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 430px) { .action-control, .ruleset-options { grid-template-columns: 1fr; }.ruleset-divider strong { max-width: 48%; text-align: right; } }
</style>
