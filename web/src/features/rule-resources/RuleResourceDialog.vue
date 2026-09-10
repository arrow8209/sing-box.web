<script setup lang="ts">
import { computed, reactive, watch } from "vue";
import AppDialog from "@/components/AppDialog.vue";
import AppIcon from "@/layout/AppIcon.vue";
import type { RuleResourceDetail, RuleResourceInput, RuleSourceType } from "./types";

const props = withDefaults(defineProps<{ open: boolean; resource?: RuleResourceDetail | null; busy?: boolean }>(), { resource: null, busy: false });
const emit = defineEmits<{ close: []; submit: [input: RuleResourceInput]; delete: [resource: RuleResourceDetail] }>();

const manualExample = `{
  "version": 3,
  "rules": [
    {
      "domain_suffix": [
        "example.com",
        "example.org"
      ]
    }
  ]
}`;
const form = reactive({ sourceType: "remote" as RuleSourceType, name: "", url: "", updateInterval: "24h", disabled: false, content: manualExample });

function reset(): void {
  const resource = props.resource;
  Object.assign(form, resource ? {
    sourceType: resource.sourceType,
    name: resource.name,
    url: resource.url,
    updateInterval: resource.updateInterval,
    disabled: resource.disabled,
    content: resource.sourceType === "manual" ? JSON.stringify(resource.content, null, 2) : manualExample
  } : { sourceType: "remote", name: "", url: "", updateInterval: "24h", disabled: false, content: manualExample });
}
watch(() => [props.open, props.resource] as const, reset, { immediate: true });

const manualValidation = computed(() => {
  if (form.sourceType !== "manual") return { valid: true, message: "" };
  try {
    const value = JSON.parse(form.content) as { version?: unknown; rules?: unknown };
    if (!value || typeof value !== "object" || Array.isArray(value)) return { valid: false, message: "手工规则必须是 JSON 对象。" };
    if (!Number.isInteger(value.version) || Number(value.version) < 1) return { valid: false, message: "必须包含有效的整数 version。" };
    if (!Array.isArray(value.rules) || value.rules.length === 0) return { valid: false, message: "rules 必须至少包含一个规则块。" };
    if (value.rules.some((rule) => !rule || typeof rule !== "object" || Array.isArray(rule) || Object.keys(rule).length === 0)) return { valid: false, message: "每个规则块都必须是非空对象。" };
    const count = value.rules.reduce<number>((total, rule) => total + Object.values(rule as Record<string, unknown>).reduce<number>((subtotal, item) => subtotal + (Array.isArray(item) ? item.length : item === null || item === "" ? 0 : 1), 0), 0);
    if (count === 0) return { valid: false, message: "手工规则中没有可用的匹配内容。" };
    return { valid: true, message: `JSON 有效 · ${value.rules.length} 个规则块 · ${count} 条规则信息` };
  } catch (cause) {
    return { valid: false, message: `JSON 语法错误：${cause instanceof Error ? cause.message : "无法解析"}` };
  }
});
const canSubmit = computed(() => Boolean(form.name.trim()) && (form.sourceType === "manual" ? manualValidation.value.valid : Boolean(form.url.trim())) && !props.busy);

function formatManual(): void {
  if (!manualValidation.value.valid) return;
  form.content = JSON.stringify(JSON.parse(form.content), null, 2);
}
function submit(): void {
  if (!canSubmit.value) return;
  if (form.sourceType === "manual") {
    emit("submit", { sourceType: "manual", name: form.name.trim(), url: "", format: "source", updateInterval: "", disabled: form.disabled, content: JSON.parse(form.content) });
    return;
  }
  emit("submit", { sourceType: "remote", name: form.name.trim(), url: form.url.trim(), format: "binary", updateInterval: form.updateInterval, disabled: form.disabled });
}
</script>

<template>
  <AppDialog
    :open="open"
    :title="resource ? '编辑规则集' : '添加规则集'"
    :description="form.sourceType === 'manual' ? '直接维护完整的手工规则 JSON' : '配置网络地址与服务端更新周期'"
    :close-disabled="busy"
    class="rule-resource-dialog"
    @close="emit('close')"
  >
    <form id="rule-resource-form" class="resource-form" @submit.prevent="submit">
      <section class="kind-section">
        <span class="field-label">规则集来源</span>
        <div class="segmented two" role="radiogroup" aria-label="规则集来源">
          <label :class="{ active: form.sourceType === 'remote' }"><input v-model="form.sourceType" type="radio" value="remote" /><AppIcon name="globe" :size="15" />网络规则集</label>
          <label :class="{ active: form.sourceType === 'manual' }"><input v-model="form.sourceType" type="radio" value="manual" /><AppIcon name="code" :size="15" />纯手工规则</label>
        </div>
      </section>

      <label class="form-row"><span class="field-label">规则集名称<small>name</small></span><input v-model.trim="form.name" required maxlength="80" placeholder="例如 geosite-cn" /></label>

      <template v-if="form.sourceType === 'remote'">
        <div class="form-row"><span class="field-label">保存格式<small>format</small></span><span class="fixed-format"><AppIcon name="database" :size="14" /><strong>Binary</strong><small>.srs · 网络规则固定格式</small></span></div>
        <label class="form-row"><span class="field-label">网络地址<small>url</small></span><input v-model.trim="form.url" type="url" required placeholder="https://example.com/rules.srs" /></label>
        <label class="form-row"><span class="field-label">定时更新<small>update interval</small></span><select v-model="form.updateInterval"><option value="">仅手动更新</option><option value="15m">每 15 分钟</option><option value="1h">每 1 小时</option><option value="6h">每 6 小时</option><option value="12h">每 12 小时</option><option value="24h">每天</option><option value="168h">每 7 天</option></select></label>
        <div class="automation-callout"><AppIcon name="clock" :size="16" /><div><strong>服务端自动更新</strong><span>浏览器关闭后计划仍会继续执行；内容变化后需要重新应用相关档案。</span></div></div>
      </template>

      <template v-else>
        <div class="manual-editor">
          <div class="editor-toolbar"><div><b>JSON</b><span>sing-box source 规则</span></div><div><button type="button" @click="form.content = manualExample">示例</button><button type="button" :disabled="!manualValidation.valid" @click="formatManual"><AppIcon name="code" :size="13" />格式化</button></div></div>
          <textarea v-model="form.content" :aria-invalid="!manualValidation.valid" spellcheck="false" />
        </div>
        <p class="validation" :class="manualValidation.valid ? 'valid' : 'invalid'"><AppIcon :name="manualValidation.valid ? 'check' : 'info'" :size="14" />{{ manualValidation.message }}</p>
        <div class="manual-help"><AppIcon name="info" :size="15" /><span>手工规则固定使用 Source JSON；内容由服务端校验并保存到固定本地文件。</span></div>
      </template>

      <label class="disabled-row"><input v-model="form.disabled" type="checkbox" />暂时禁用此规则集</label>
      <p v-if="resource" class="resource-path">固定路径：<code>{{ resource.localPath }}</code></p>
    </form>
    <template #footer>
      <button v-if="resource" type="button" class="danger-button" :disabled="busy" @click="emit('delete', resource)"><AppIcon name="trash" :size="14" />删除</button>
      <span class="footer-spacer" />
      <button type="button" class="secondary-button" :disabled="busy" @click="emit('close')">取消</button>
      <button type="submit" form="rule-resource-form" class="primary-button" :disabled="!canSubmit"><AppIcon :name="resource ? 'save' : 'plus'" :size="14" />{{ busy ? "保存中…" : resource ? "保存规则集" : "添加规则集" }}</button>
    </template>
  </AppDialog>
</template>

<style scoped>
:global(.rule-resource-dialog) { width: min(700px, calc(100vw - 32px)); }.resource-form { display: grid; gap: 14px; }.kind-section { padding-bottom: 14px; border-bottom: 1px solid var(--line); }.field-label { display: flex; min-width: 0; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 2px 8px; color: var(--text); font-size: var(--font-size-base); font-weight: 680; }.field-label small { max-width: 100%; overflow-wrap: anywhere; color: var(--text-faint); font: var(--font-size-sm) ui-monospace, monospace; }.kind-section > .field-label { margin-bottom: 8px; }.segmented { display: grid; gap: 3px; padding: 3px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-soft); }.segmented.two { grid-template-columns: repeat(2, minmax(0, 1fr)); }.segmented label { display: flex; min-height: 39px; align-items: center; justify-content: center; gap: 7px; border-radius: 7px; color: var(--text-soft); font-size: var(--font-size-base); font-weight: 670; cursor: pointer; }.segmented label.active { color: var(--primary); background: var(--surface); box-shadow: 0 3px 10px rgb(30 45 80 / 8%); }.segmented input { position: absolute; opacity: 0; pointer-events: none; }.form-row { display: grid; grid-template-columns: 145px minmax(0, 1fr); align-items: center; gap: 14px; }.form-row > input, .form-row > select { width: 100%; min-height: 40px; background: var(--surface-soft); font-size: var(--font-size-base); }.automation-callout, .manual-help { display: flex; align-items: flex-start; gap: 9px; padding: 10px 12px; border: 1px solid color-mix(in srgb, var(--cyan) 18%, var(--line)); border-radius: 10px; color: var(--cyan); background: var(--cyan-soft); }.automation-callout strong, .automation-callout span { display: block; }.automation-callout strong { color: var(--text); font-size: var(--font-size-base); }.automation-callout span, .manual-help span { margin-top: 2px; color: var(--text-soft); font-size: var(--font-size-xs); }.manual-editor { overflow: hidden; border: 1px solid var(--line); border-radius: 11px; background: #10182a; }.editor-toolbar { display: flex; min-height: 39px; align-items: center; justify-content: space-between; gap: 10px; padding: 7px 10px; border-bottom: 1px solid rgb(255 255 255 / 9%); color: #91a0bb; font-size: var(--font-size-xs); }.editor-toolbar b { margin-right: 7px; color: #8aa2ff; font-size: var(--font-size-xs); }.editor-toolbar button { min-height: 27px; padding: 4px 7px; color: #b8c4d9; background: transparent; font-size: var(--font-size-sm); }.editor-toolbar button + button { margin-left: 3px; }.editor-toolbar button:hover { background: rgb(255 255 255 / 8%); }.manual-editor textarea { width: 100%; min-height: 300px; padding: 14px 16px; border: 0; border-radius: 0; color: #d4def0; background: #10182a; font: var(--font-size-base)/1.65 ui-monospace, SFMono-Regular, Consolas, monospace; resize: vertical; }.validation { display: flex; align-items: center; gap: 7px; margin: -5px 0 0; padding: 8px 10px; border-radius: 8px; font-size: var(--font-size-xs); }.validation.valid { color: var(--green); background: var(--green-soft); }.validation.invalid { color: var(--red); background: var(--red-soft); }.disabled-row { display: flex; align-items: center; gap: 8px; color: var(--text-soft); font-size: var(--font-size-base); }.disabled-row input { width: 15px; min-height: 15px; }.resource-path { margin: -3px 0 0; color: var(--text-faint); font-size: var(--font-size-xs); }.resource-path code { color: var(--primary-strong); }.footer-spacer { flex: 1; }.secondary-button, .danger-button, .primary-button { display: inline-flex; min-height: 36px; align-items: center; gap: 6px; padding: 7px 11px; border: 1px solid var(--line); font-size: var(--font-size-sm); }.secondary-button { color: var(--text-soft); background: var(--surface); }.danger-button { color: var(--red); border-color: color-mix(in srgb, var(--red) 20%, var(--line)); background: var(--red-soft); }.primary-button { color: #fff; border-color: var(--primary); background: var(--primary); }
.fixed-format { display: flex; min-height: 40px; align-items: center; gap: 7px; padding: 8px 11px; border: 1px solid var(--line); border-radius: 8px; color: var(--primary); background: var(--primary-soft); font-size: var(--font-size-sm); }.fixed-format strong { color: var(--text); }.fixed-format small { margin-left: auto; color: var(--text-faint); font-size: var(--font-size-sm); }
@media (max-width: 680px) { :global(.rule-resource-dialog) { width: 100%; }.form-row { grid-template-columns: 1fr; gap: 6px; }.manual-editor textarea { min-height: 42dvh; }.footer-spacer { display: none; }.danger-button { margin-right: auto; } }
</style>
