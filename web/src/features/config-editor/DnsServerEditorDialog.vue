<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import AppDialog from "@/components/AppDialog.vue";
import AppIcon from "@/layout/AppIcon.vue";
import { setJsonPath } from "./json";
import {
  dnsServerTypeOptions,
  isDnsObject,
  type DnsServer,
  type DnsServerEditorResult
} from "./dns";

const props = defineProps<{
  open: boolean;
  server: DnsServer | null;
  serverText?: string;
  reservedTags: string[];
  outbounds: string[];
  dnsServers: string[];
}>();
const emit = defineEmits<{ close: []; submit: [result: DnsServerEditorResult] }>();

const baseText = ref("{}");
const form = reactive({
  tag: "",
  type: "https",
  server: "",
  port: "443",
  path: "/dns-query",
  domainResolver: "",
  detour: "",
  inet4Range: "198.18.0.0/15",
  inet6Range: "fc00::/18",
  networkInterface: "",
  endpoint: ""
});

const endpointTypes = new Set(["udp", "tcp", "tls", "https", "h3", "quic"]);
const pathTypes = new Set(["https", "h3"]);
const endpointType = computed(() => endpointTypes.has(form.type));
const pathType = computed(() => pathTypes.has(form.type));
const fakeIpType = computed(() => form.type === "fakeip");
const domainResolverOptions = computed(() => [...new Set([
  ...(form.domainResolver && !props.dnsServers.includes(form.domainResolver) ? [form.domainResolver] : []),
  ...props.dnsServers.filter((tag) => tag !== form.tag)
])]);
const detourOptions = computed(() => [...new Set([
  ...(form.detour && !props.outbounds.includes(form.detour) ? [form.detour] : []),
  ...props.outbounds
])]);
const validation = computed(() => {
  const tag = form.tag.trim();
  if (!tag) return "服务器标签不能为空。";
  if (Array.from(tag).length > 128) return "服务器标签不能超过 128 个字符。";
  if (props.reservedTags.includes(tag)) return `服务器标签已存在：${tag}`;
  if (endpointType.value && !form.server.trim()) return "服务器地址不能为空。";
  if (endpointType.value) {
    const port = Number(form.port);
    if (!Number.isInteger(port) || port < 1 || port > 65535) return "服务器端口必须是 1 到 65535 的整数。";
  }
  if (pathType.value && form.path.trim() && !form.path.trim().startsWith("/")) return "请求路径必须以 / 开头。";
  if (fakeIpType.value && !form.inet4Range.trim() && !form.inet6Range.trim()) return "Fake-IP 至少需要一个地址段。";
  return "";
});

watch(() => props.open, (open, wasOpen) => {
  if (open && !wasOpen) initialize();
}, { immediate: true });

function stringValue(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

function defaultPort(type: string): string {
  if (type === "tls") return "853";
  if (type === "https" || type === "h3" || type === "quic") return "443";
  return "53";
}

function uniqueTag(preferred: string): string {
  if (!props.reservedTags.includes(preferred)) return preferred;
  let suffix = 2;
  while (props.reservedTags.includes(`${preferred}-${suffix}`)) suffix++;
  return `${preferred}-${suffix}`;
}

function initialize(): void {
  const server = props.server ?? {};
  baseText.value = props.serverText?.trim() || JSON.stringify(server, null, 2) || "{}";
  form.type = stringValue(server.type, "https");
  form.tag = stringValue(server.tag, uniqueTag("DNS-Server"));
  form.server = stringValue(server.server);
  form.port = typeof server.server_port === "number" || typeof server.server_port === "string" ? String(server.server_port) : defaultPort(form.type);
  form.path = stringValue(server.path, "/dns-query");
  form.domainResolver = stringValue(server.domain_resolver, isDnsObject(server.domain_resolver) ? stringValue(server.domain_resolver.server) : "");
  form.detour = stringValue(server.detour);
  form.inet4Range = stringValue(server.inet4_range, "198.18.0.0/15");
  form.inet6Range = stringValue(server.inet6_range, "fc00::/18");
  form.networkInterface = stringValue(server.interface);
  form.endpoint = stringValue(server.endpoint);
}

function chooseType(type: string): void {
  const previousDefault = defaultPort(form.type);
  form.type = type;
  if (!form.port || form.port === previousDefault) form.port = defaultPort(type);
}

function updateDomainResolver(text: string, value: string): string {
  const current = props.server?.domain_resolver;
  if (!isDnsObject(current)) return setJsonPath(text, ["domain_resolver"], value || undefined);
  if (!value && Object.keys(current).every((key) => key === "server")) return setJsonPath(text, ["domain_resolver"], undefined);
  return setJsonPath(text, ["domain_resolver", "server"], value || undefined);
}

function buildText(): string {
  let text = baseText.value.trim() || "{}";
  text = setJsonPath(text, ["tag"], form.tag.trim());
  text = setJsonPath(text, ["type"], form.type);
  text = setJsonPath(text, ["server"], endpointType.value ? form.server.trim() : undefined);
  text = setJsonPath(text, ["server_port"], endpointType.value ? Number(form.port) : undefined);
  text = setJsonPath(text, ["path"], pathType.value ? form.path.trim() || undefined : undefined);
  text = updateDomainResolver(text, endpointType.value ? form.domainResolver : "");
  text = setJsonPath(text, ["detour"], endpointType.value ? form.detour || undefined : undefined);
  text = setJsonPath(text, ["inet4_range"], fakeIpType.value ? form.inet4Range.trim() || undefined : undefined);
  text = setJsonPath(text, ["inet6_range"], fakeIpType.value ? form.inet6Range.trim() || undefined : undefined);
  text = setJsonPath(text, ["interface"], form.type === "dhcp" ? form.networkInterface.trim() || undefined : undefined);
  text = setJsonPath(text, ["endpoint"], form.type === "tailscale" ? form.endpoint.trim() || undefined : undefined);
  return text;
}

function submit(): void {
  if (validation.value) return;
  const text = buildText();
  const server = JSON.parse(text) as DnsServer;
  emit("submit", { server, text });
}
</script>

<template>
  <AppDialog class="dns-server-dialog" :open="open" :title="server ? '编辑 DNS 服务器' : '新增 DNS 服务器'" :description="server ? '更新解析器参数，未展示字段会原样保留' : '配置上游解析器或 Fake-IP 地址池'" initial-focus="input[name=tag]" @close="emit('close')">
    <form id="dns-server-form" class="dns-server-form" @submit.prevent="submit">
      <label class="editor-row"><span class="editor-label">服务器标签<small>tag</small></span><input v-model="form.tag" name="tag" placeholder="例如 Local-DNS" required /></label>
      <div class="editor-row"><span class="editor-label">服务器类型<small>type</small></span><div class="type-control">
        <label v-for="option in dnsServerTypeOptions" :key="option.value"><input type="radio" name="dns-server-type" :value="option.value" :checked="form.type === option.value" @change="chooseType(option.value)" /><span>{{ option.label }}</span></label>
      </div></div>

      <template v-if="endpointType">
        <label class="editor-row"><span class="editor-label">服务器地址<small>server</small></span><input v-model="form.server" name="server" placeholder="例如 223.5.5.5" required /></label>
        <label class="editor-row"><span class="editor-label">服务器端口<small>server_port</small></span><input v-model="form.port" name="server-port" type="number" min="1" max="65535" required /></label>
        <label v-if="pathType" class="editor-row"><span class="editor-label">请求路径<small>path</small></span><input v-model="form.path" name="path" placeholder="/dns-query" /></label>
        <label class="editor-row"><span class="editor-label">域名解析器<small>domain_resolver</small></span><select v-model="form.domainResolver" name="domain-resolver"><option value="">无需前置解析</option><option v-for="tag in domainResolverOptions" :key="tag" :value="tag">{{ tag }}</option></select></label>
        <label class="editor-row"><span class="editor-label">出站路由<small>detour</small></span><select v-model="form.detour" name="detour"><option value="">默认出站</option><option v-for="tag in detourOptions" :key="tag" :value="tag">{{ tag }}</option></select></label>
      </template>

      <template v-else-if="fakeIpType">
        <label class="editor-row"><span class="editor-label">IPv4 地址段<small>inet4_range</small></span><input v-model="form.inet4Range" name="inet4-range" placeholder="198.18.0.0/15" /></label>
        <label class="editor-row"><span class="editor-label">IPv6 地址段<small>inet6_range</small></span><input v-model="form.inet6Range" name="inet6-range" placeholder="fc00::/18" /></label>
      </template>

      <label v-else-if="form.type === 'dhcp'" class="editor-row"><span class="editor-label">网络接口<small>interface</small></span><input v-model="form.networkInterface" name="interface" placeholder="留空自动检测" /></label>
      <label v-else-if="form.type === 'tailscale'" class="editor-row"><span class="editor-label">Tailscale 端点<small>endpoint</small></span><input v-model="form.endpoint" name="endpoint" placeholder="端点标签" /></label>
      <div v-else class="no-options"><AppIcon name="dns" :size="28" /><strong>该类型无需常用参数</strong><span>仍可在“编辑全部 JSON”中维护高级字段。</span></div>

      <p class="preserve-note"><AppIcon name="info" :size="14" />表单仅更新当前显示的字段，其他 sing-box 字段与大整数文本会继续保留。</p>
      <p v-if="validation" class="validation" role="alert"><AppIcon name="info" :size="14" />{{ validation }}</p>
    </form>
    <template #footer>
      <button class="secondary-button" type="button" @click="emit('close')">取消</button>
      <button class="primary-button" type="submit" form="dns-server-form" :disabled="Boolean(validation)"><AppIcon :name="server ? 'save' : 'plus'" :size="14" />{{ server ? "保存" : "确认添加" }}</button>
    </template>
  </AppDialog>
</template>

<style scoped>
:global(.dns-server-dialog) { width: min(880px, calc(100vw - 32px)); }.dns-server-form { display: grid; }.editor-row { display: grid; min-width: 0; grid-template-columns: 138px minmax(0, 1fr); align-items: center; gap: 18px; padding: 7px 0; }.editor-label { display: flex; min-width: 0; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 2px 8px; color: var(--text); font-size: var(--font-size-base); font-weight: 680; }.editor-label small { max-width: 100%; overflow-wrap: anywhere; color: var(--text-faint); font-size: var(--font-size-sm); font-weight: 520; }.editor-row input, .editor-row select { width: 100%; min-width: 0; }.type-control { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 3px; padding: 3px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-soft); }.type-control label { min-width: 0; }.type-control input { position: absolute; width: 1px; height: 1px; overflow: hidden; opacity: 0; }.type-control label > span { display: flex; min-height: 33px; align-items: center; justify-content: center; padding: 5px; border-radius: 7px; color: var(--text-faint); font-size: var(--font-size-base); font-weight: 650; cursor: pointer; transition: 150ms ease; }.type-control label > span:hover { color: var(--text); }.type-control input:checked + span { color: var(--primary); background: var(--surface); box-shadow: 0 2px 9px rgb(34 49 83 / 9%); }.type-control input:focus-visible + span { outline: 3px solid color-mix(in srgb, var(--primary) 22%, transparent); }.no-options { display: grid; min-height: 145px; place-items: center; align-content: center; gap: 6px; margin-top: 8px; border: 1px dashed var(--line-strong); border-radius: 11px; color: var(--primary); background: var(--surface-soft); text-align: center; }.no-options strong { color: var(--text); font-size: var(--font-size-base); }.no-options span { color: var(--text-faint); font-size: var(--font-size-sm); }.preserve-note, .validation { display: flex; align-items: flex-start; gap: 7px; margin: 10px 0 0; padding: 9px 10px; border-radius: 9px; font-size: var(--font-size-xs); }.preserve-note { color: var(--text-soft); border: 1px solid var(--line); background: var(--surface-soft); }.preserve-note .icon { color: var(--primary); }.validation { color: var(--red); background: var(--red-soft); }.secondary-button, .primary-button { display: inline-flex; min-height: 36px; align-items: center; gap: 6px; padding: 7px 11px; border: 1px solid var(--line); font-size: var(--font-size-sm); }.secondary-button { color: var(--text-soft); background: var(--surface); }.primary-button { color: #fff; border-color: var(--primary); background: var(--primary); }
@media (max-width: 760px) { :global(.dns-server-dialog) { width: 100%; }.editor-row { grid-template-columns: 1fr; gap: 6px; padding: 8px 0; }.editor-label { justify-content: flex-start; }.type-control { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
@media (max-width: 430px) { .type-control { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
