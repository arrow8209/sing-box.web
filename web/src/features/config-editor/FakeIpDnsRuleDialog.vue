<script setup lang="ts">
import { computed, reactive, watch } from "vue";
import AppDialog from "@/components/AppDialog.vue";
import AppIcon from "@/layout/AppIcon.vue";
import {
  createDefaultFakeIpDnsRule,
  dnsRuleServer,
  dnsServerType,
  fakeIpDnsRuleParts,
  isDnsObject,
  parseFakeIpDomainSuffixes,
  type DnsRule,
  type DnsRuleEditorResult,
  type DnsServer
} from "./dns";

const props = defineProps<{
  open: boolean;
  rule: DnsRule | null;
  enabled: boolean;
  servers: DnsServer[];
}>();
const emit = defineEmits<{ close: []; submit: [result: DnsRuleEditorResult] }>();

const form = reactive({
  suffixes: "",
  queryTypes: [] as string[],
  server: ""
});

const serverOptions = computed(() => props.servers.flatMap((server) => {
  const tag = typeof server.tag === "string" ? server.tag.trim() : "";
  return dnsServerType(server) === "fakeip" && tag ? [{
    tag,
    inet4Range: typeof server.inet4_range === "string" && server.inet4_range ? server.inet4_range : "未配置",
    inet6Range: typeof server.inet6_range === "string" && server.inet6_range ? server.inet6_range : "未配置"
  }] : [];
}));
const selectedServer = computed(() => serverOptions.value.find((server) => server.tag === form.server));
const normalizedSuffixes = computed(() => parseFakeIpDomainSuffixes(form.suffixes));
const selectedQueryTypes = computed(() => ["A", "AAAA"].filter((type) => form.queryTypes.includes(type)));
const validation = computed(() => {
  if (!normalizedSuffixes.value.length) return "请至少保留一个不进入 Fake-IP 的域名后缀。";
  if (!selectedQueryTypes.value.length) return "请至少选择 A 或 AAAA 一种查询类型。";
  if (!selectedServer.value) return "请先配置并选择一个 Fake-IP DNS 服务器。";
  return "";
});
const validationSummary = computed(() => validation.value || `规则有效 · ${normalizedSuffixes.value.length} 个排除后缀 · ${selectedQueryTypes.value.join(" / ")} → ${form.server}`);

watch(() => props.open, (open, wasOpen) => {
  if (open && !wasOpen) initialize();
}, { immediate: true });

function initialize(): void {
  const parts = fakeIpDnsRuleParts(props.rule) ?? fakeIpDnsRuleParts(createDefaultFakeIpDnsRule());
  form.suffixes = parts?.suffixes.join("\n") ?? "";
  form.queryTypes = ["A", "AAAA"].filter((type) => parts?.queryTypes.includes(type));
  const currentServer = dnsRuleServer(props.rule);
  form.server = serverOptions.value.some((server) => server.tag === currentServer)
    ? currentServer
    : serverOptions.value[0]?.tag ?? "";
}

function toggleQueryType(type: string): void {
  form.queryTypes = form.queryTypes.includes(type)
    ? form.queryTypes.filter((item) => item !== type)
    : [...form.queryTypes, type];
}

function submit(): void {
  if (validation.value || !selectedServer.value) return;
  const rule = createDefaultFakeIpDnsRule(normalizedSuffixes.value, selectedQueryTypes.value, form.server);
  if (!isDnsObject(rule)) return;
  emit("submit", { rule, text: JSON.stringify(rule, null, 2), resourceIds: [] });
}
</script>

<template>
  <AppDialog
    class="fakeip-rule-dialog"
    :open="open"
    title="编辑内置 Fake-IP 规则"
    description="依据 sing-box FakeIP 服务器与 DNS 逻辑规则设计"
    initial-focus="textarea[name=fakeip-domain-suffixes]"
    @close="emit('close')"
  >
    <form id="dns-fakeip-rule-form" class="fakeip-rule-form" @submit.prevent="submit">
      <div class="fakeip-rule-hero">
        <span class="fakeip-rule-hero-icon"><AppIcon name="sparkles" :size="19" /></span>
        <div><span class="fakeip-rule-kicker">系统内置 · {{ enabled ? "当前启用" : "当前停用" }}</span><strong>仅为公网地址查询分配 Fake-IP</strong><p>保留域名继续真实解析，其余 A / AAAA 查询路由到 Fake-IP 地址池。</p></div>
        <span class="fakeip-rule-lock"><AppIcon name="lock" :size="11" />不可删除</span>
      </div>

      <div class="fakeip-rule-flow" aria-label="Fake-IP 规则匹配逻辑">
        <div><em>条件 1</em><strong>域名不在排除清单</strong><span>{{ normalizedSuffixes.length }} 个排除后缀</span></div>
        <b>AND</b>
        <div><em>条件 2</em><strong>查询地址记录</strong><span>{{ selectedQueryTypes.join(" / ") || "未选择" }}</span></div>
        <i><AppIcon name="chevron" :size="14" /></i>
        <div class="target"><em>动作</em><strong>路由至 Fake-IP</strong><span>{{ form.server || "未配置" }}</span></div>
      </div>

      <div class="fakeip-rule-editor-grid">
        <section class="fakeip-rule-panel fakeip-domain-panel">
          <header><span><AppIcon name="shield" :size="14" /></span><div><strong>真实解析域名后缀</strong><small>命中这些后缀时反选条件失败，不进入 Fake-IP。</small></div><b>{{ normalizedSuffixes.length }} 个排除后缀</b></header>
          <textarea v-model="form.suffixes" name="fakeip-domain-suffixes" spellcheck="false" aria-label="不进入 Fake-IP 的域名后缀" />
          <p>每行填写一个后缀；省略开头的 <code>.</code> 时会自动补全。</p>
        </section>

        <section class="fakeip-rule-panel">
          <header><span><AppIcon name="dns" :size="14" /></span><div><strong>查询类型</strong><small>Fake-IP 只响应地址记录查询。</small></div></header>
          <div class="fakeip-query-options">
            <label v-for="option in [{ type: 'A', detail: 'IPv4 地址记录' }, { type: 'AAAA', detail: 'IPv6 地址记录' }]" :key="option.type">
              <input type="checkbox" :checked="form.queryTypes.includes(option.type)" :aria-label="`Fake-IP 查询类型 ${option.type}`" @change="toggleQueryType(option.type)" />
              <span><b>{{ option.type }}</b><small>{{ option.detail }}</small><AppIcon name="check" :size="13" /></span>
            </label>
          </div>
        </section>

        <section class="fakeip-rule-panel">
          <header><span><AppIcon name="server" :size="14" /></span><div><strong>目标 Fake-IP 服务器</strong><small>仅展示 type 为 fakeip 的 DNS 服务器。</small></div></header>
          <select v-model="form.server" aria-label="目标 Fake-IP 服务器" :disabled="!serverOptions.length"><option v-if="!serverOptions.length" value="">没有可用的 Fake-IP 服务器</option><option v-for="server in serverOptions" :key="server.tag" :value="server.tag">{{ server.tag }}</option></select>
          <div class="fakeip-range-grid"><span><small>IPv4 地址池</small><code>{{ selectedServer?.inet4Range ?? "未配置" }}</code></span><span><small>IPv6 地址池</small><code>{{ selectedServer?.inet6Range ?? "未配置" }}</code></span></div>
        </section>
      </div>

      <div class="fakeip-rule-structure"><AppIcon name="lock" :size="15" /><div><strong>固定结构</strong><p><code>type: logical</code>、<code>mode: and</code>、域名条件 <code>invert: true</code> 与 <code>action: route</code> 由界面保护，避免破坏 Fake-IP 匹配语义。</p></div><a href="https://sing-box.sagernet.org/zh/configuration/dns/server/fakeip/" target="_blank" rel="noreferrer">sing-box 官方说明<AppIcon name="external" :size="11" /></a></div>
      <div class="fakeip-validation" :class="validation ? 'invalid' : 'valid'" role="status"><AppIcon :name="validation ? 'info' : 'check'" :size="14" /><span>{{ validationSummary }}</span></div>
    </form>

    <template #footer>
      <span class="fakeip-footer-note"><AppIcon name="lock" :size="12" />内置规则可停用，但不能删除</span>
      <span class="footer-spacer" />
      <button class="secondary-button" type="button" @click="emit('close')">取消</button>
      <button class="primary-button" type="submit" form="dns-fakeip-rule-form" :disabled="Boolean(validation)"><AppIcon name="save" :size="14" />保存 Fake-IP 规则</button>
    </template>
  </AppDialog>
</template>

<style scoped>
:global(.fakeip-rule-dialog) { width: min(980px, calc(100vw - 32px)); }.fakeip-rule-form { display: grid; gap: 12px; }.fakeip-rule-hero { display: flex; min-width: 0; align-items: center; gap: 12px; padding: 13px 14px; border: 1px solid color-mix(in srgb, var(--orange) 19%, var(--line)); border-radius: 12px; background: linear-gradient(120deg, var(--orange-soft), color-mix(in srgb, var(--surface) 92%, var(--orange-soft))); }.fakeip-rule-hero-icon { display: grid; width: 39px; height: 39px; flex: 0 0 auto; place-items: center; border-radius: 11px; color: var(--orange); background: var(--surface); box-shadow: var(--shadow-sm); }.fakeip-rule-hero > div { min-width: 0; flex: 1; }.fakeip-rule-kicker, .fakeip-rule-hero strong, .fakeip-rule-hero p { display: block; }.fakeip-rule-kicker { margin-bottom: 2px; color: var(--orange); font-size: var(--font-size-sm); font-weight: 720; }.fakeip-rule-hero strong { font-size: 12px; }.fakeip-rule-hero p { margin: 3px 0 0; color: var(--text-soft); font-size: 9px; }.fakeip-rule-lock { display: inline-flex; min-height: 26px; flex: 0 0 auto; align-items: center; gap: 4px; padding: 4px 8px; border-radius: 999px; color: var(--orange); background: var(--surface); font-size: var(--font-size-sm); font-weight: 680; }.fakeip-rule-flow { display: grid; grid-template-columns: minmax(0, 1fr) auto minmax(0, .8fr) auto minmax(0, .8fr); align-items: stretch; gap: 7px; padding: 9px; border: 1px solid var(--line); border-radius: 12px; background: var(--surface-soft); }.fakeip-rule-flow > div { display: grid; min-width: 0; min-height: 63px; align-content: center; padding: 9px 10px; border: 1px solid var(--line); border-radius: 9px; background: var(--surface); }.fakeip-rule-flow > div.target { border-color: color-mix(in srgb, var(--orange) 24%, var(--line)); background: var(--orange-soft); }.fakeip-rule-flow em, .fakeip-rule-flow strong, .fakeip-rule-flow span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.fakeip-rule-flow em { color: var(--text-faint); font-size: var(--font-size-xs); font-style: normal; font-weight: 700; text-transform: uppercase; }.fakeip-rule-flow strong { margin-top: 2px; font-size: 9px; }.fakeip-rule-flow span { margin-top: 3px; color: var(--text-faint); font-size: var(--font-size-sm); }.fakeip-rule-flow > b, .fakeip-rule-flow > i { display: grid; width: 28px; place-items: center; align-self: center; color: var(--primary); font-size: var(--font-size-xs); font-style: normal; }.fakeip-rule-flow > b { height: 28px; border-radius: 50%; background: var(--primary-soft); }.fakeip-rule-editor-grid { display: grid; grid-template-columns: minmax(0, 1.08fr) minmax(270px, .92fr); gap: 9px; }.fakeip-rule-panel { min-width: 0; padding: 11px; border: 1px solid var(--line); border-radius: 11px; background: var(--surface-soft); }.fakeip-domain-panel { grid-row: span 2; }.fakeip-rule-panel header { display: flex; min-width: 0; align-items: center; gap: 8px; margin-bottom: 9px; }.fakeip-rule-panel header > span { display: grid; width: 29px; height: 29px; flex: 0 0 auto; place-items: center; border-radius: 8px; color: var(--primary); background: var(--primary-soft); }.fakeip-rule-panel header > div { min-width: 0; flex: 1; }.fakeip-rule-panel header strong, .fakeip-rule-panel header small { display: block; }.fakeip-rule-panel header strong { font-size: 10px; }.fakeip-rule-panel header small { margin-top: 1px; color: var(--text-faint); font-size: var(--font-size-sm); }.fakeip-rule-panel header > b { flex: 0 0 auto; color: var(--primary); font-size: var(--font-size-sm); }.fakeip-domain-panel textarea { width: 100%; min-height: 172px; resize: vertical; background: var(--surface); font: 9px/1.6 ui-monospace, SFMono-Regular, Consolas, monospace; }.fakeip-domain-panel > p { margin: 7px 1px 0; color: var(--text-faint); font-size: var(--font-size-sm); }.fakeip-domain-panel > p code { color: var(--primary); }.fakeip-query-options { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 7px; }.fakeip-query-options input { position: absolute; width: 1px; height: 1px; opacity: 0; }.fakeip-query-options label > span { position: relative; display: block; min-height: 53px; padding: 9px 27px 8px 9px; border: 1px solid var(--line); border-radius: 9px; background: var(--surface); cursor: pointer; }.fakeip-query-options b, .fakeip-query-options small { display: block; }.fakeip-query-options b { font: 10px ui-monospace, monospace; }.fakeip-query-options small { margin-top: 3px; color: var(--text-faint); font-size: var(--font-size-xs); }.fakeip-query-options .icon { position: absolute; top: 9px; right: 9px; color: transparent; }.fakeip-query-options input:checked + span { color: var(--primary); border-color: color-mix(in srgb, var(--primary) 36%, var(--line)); background: var(--primary-soft); }.fakeip-query-options input:checked + span .icon { color: var(--primary); }.fakeip-query-options input:focus-visible + span { outline: 3px solid color-mix(in srgb, var(--primary) 20%, transparent); }.fakeip-rule-panel > select { width: 100%; background: var(--surface); }.fakeip-range-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px; margin-top: 7px; }.fakeip-range-grid > span { min-width: 0; padding: 7px 8px; border: 1px solid var(--line); border-radius: 8px; background: var(--surface); }.fakeip-range-grid small, .fakeip-range-grid code { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.fakeip-range-grid small { color: var(--text-faint); font-size: var(--font-size-xs); }.fakeip-range-grid code { margin-top: 3px; color: var(--orange); font-size: var(--font-size-sm); }.fakeip-rule-structure { display: flex; align-items: flex-start; gap: 9px; padding: 10px 11px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-soft); }.fakeip-rule-structure > .icon { flex: 0 0 auto; margin-top: 1px; color: var(--orange); }.fakeip-rule-structure > div { min-width: 0; flex: 1; }.fakeip-rule-structure strong { display: block; font-size: 9px; }.fakeip-rule-structure p { margin: 2px 0 0; color: var(--text-faint); font-size: var(--font-size-sm); line-height: 1.55; }.fakeip-rule-structure code { color: var(--primary); font-family: ui-monospace, SFMono-Regular, Consolas, monospace; }.fakeip-rule-structure a { display: inline-flex; flex: 0 0 auto; align-items: center; gap: 4px; color: var(--primary); font-size: var(--font-size-sm); font-weight: 650; }.fakeip-validation { display: flex; align-items: center; gap: 7px; padding: 9px 10px; border-radius: 9px; font-size: 9px; }.fakeip-validation.valid { color: var(--green); background: var(--green-soft); }.fakeip-validation.invalid { color: var(--red); background: var(--red-soft); }.fakeip-footer-note { display: inline-flex; align-items: center; gap: 5px; color: var(--orange); font-size: 9px; font-weight: 650; }.footer-spacer { flex: 1; }.secondary-button, .primary-button { display: inline-flex; min-height: 36px; align-items: center; gap: 6px; padding: 7px 11px; border: 1px solid var(--line); font-size: 10px; }.secondary-button { color: var(--text-soft); background: var(--surface); }.primary-button { color: #fff; border-color: var(--primary); background: var(--primary); }
@media (max-width: 760px) { :global(.fakeip-rule-dialog) { width: 100%; }.fakeip-rule-editor-grid { grid-template-columns: 1fr; }.fakeip-domain-panel { grid-row: auto; } }
@media (max-width: 560px) { .fakeip-rule-hero { align-items: flex-start; flex-wrap: wrap; padding: 11px; }.fakeip-rule-hero > div { flex-basis: calc(100% - 52px); }.fakeip-rule-lock { margin-left: 51px; }.fakeip-rule-flow { grid-template-columns: 1fr; }.fakeip-rule-flow > b, .fakeip-rule-flow > i { width: 100%; height: 20px; }.fakeip-rule-flow > b { border-radius: 6px; }.fakeip-rule-flow > i .icon { transform: rotate(90deg); }.fakeip-rule-structure { flex-wrap: wrap; }.fakeip-rule-structure a { margin-left: 24px; }.fakeip-footer-note { display: none; } }
@media (max-width: 380px) { .fakeip-query-options, .fakeip-range-grid { grid-template-columns: 1fr; } }
</style>
