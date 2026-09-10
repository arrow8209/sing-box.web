<script setup lang="ts">
import { computed, reactive, ref, toRaw, watch } from "vue";
import AppDialog from "@/components/AppDialog.vue";
import AppIcon from "@/layout/AppIcon.vue";
import { editorType, inboundTypes, legacyInboundFields, uniqueInboundTag, validateInboundJson } from "./model";
import type { InboundConfig, InboundDetail, InboundEditorType, InboundInput } from "./types";

const props = withDefaults(defineProps<{
  open: boolean;
  resource: InboundDetail | null;
  existingTags: string[];
  busy?: boolean;
  kind?: "library" | "profile";
  allowDelete?: boolean;
}>(), { busy: false, kind: "library", allowDelete: true });
const emit = defineEmits<{ close: []; submit: [input: InboundInput]; delete: [] }>();

interface EditorForm {
  tag: string;
  listen: string;
  listenPort: string;
  tcpFastOpen: boolean;
  tcpMultiPath: boolean;
  udpFragment: boolean;
  users: string[];
  udpTimeout: string;
  network: string;
  overrideAddress: string;
  overridePort: string;
  interfaceName: string;
  mtu: string;
  stack: string;
  autoRoute: boolean;
  strictRoute: boolean;
  endpointIndependentNat: boolean;
  address: string[];
  routeAddress: string[];
  routeExcludeAddress: string[];
}

const customExample = `{
  "type": "mixed",
  "tag": "custom-in",
  "listen": "127.0.0.1",
  "listen_port": 7891
}`;
const defaultPorts: Record<string, number> = { mixed: 7891, http: 8080, socks: 1080, direct: 5353 };
const selected = ref<InboundEditorType>("mixed");
const customSource = ref(customExample);
const baseConfig = ref<InboundConfig | null>(null);
const originalType = ref("");
const form = reactive<EditorForm>({
  tag: "", listen: "127.0.0.1", listenPort: "7891", tcpFastOpen: false, tcpMultiPath: false,
  udpFragment: false, users: [], udpTimeout: "5m",
  network: "", overrideAddress: "", overridePort: "", interfaceName: "", mtu: "9000", stack: "mixed",
  autoRoute: true, strictRoute: true, endpointIndependentNat: false,
  address: ["172.18.0.1/30", "fdfe:dcba:9876::1/126"], routeAddress: [], routeExcludeAddress: []
});

const title = computed(() => props.kind === "library"
  ? props.resource ? "编辑入站配置" : "新建入站配置"
  : "编辑独立入站副本");
const description = computed(() => props.kind === "library"
  ? "维护可在多个自定义配置中重复导入的源配置"
  : "此处内容只属于当前自定义配置，不与资源库联动");
const originalTag = computed(() => props.resource?.config.tag ?? "");
const customValidation = computed(() => validateInboundJson(customSource.value, props.existingTags, originalTag.value));
const structuredError = computed(() => {
  const tag = form.tag.trim();
  if (!tag) return "入站名称不能为空。";
  if (Array.from(tag).length > 128) return "入站名称不能超过 128 个字符。";
  if (tag !== originalTag.value && props.existingTags.includes(tag)) return `入站名称已存在：${tag}`;
  if (selected.value === "tun") {
    if (!form.address.some((item) => item.trim())) return "请至少添加一个 TUN IPv4 或 IPv6 前缀。";
    const mtu = Number(form.mtu);
    if (!Number.isInteger(mtu) || mtu <= 0) return "最大传输单元必须是正整数。";
    return "";
  }
  if (!form.listen.trim()) return "监听地址不能为空。";
  const port = Number(form.listenPort);
  if (!Number.isInteger(port) || port < 1 || port > 65535) return "监听端口必须是 1 到 65535 的整数。";
  if (form.overridePort) {
    const overridePort = Number(form.overridePort);
    if (!Number.isInteger(overridePort) || overridePort < 1 || overridePort > 65535) return "覆盖目标端口必须是 1 到 65535 的整数。";
  }
  return "";
});
const valid = computed(() => selected.value === "custom" ? customValidation.value.valid : !structuredError.value);

watch(() => [props.open, props.resource] as const, ([open]) => { if (open) initialize(); }, { immediate: true });

function text(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}
function numberText(value: unknown, fallback: number | "" = ""): string {
  return typeof value === "number" && Number.isFinite(value) ? String(value) : String(fallback);
}
function bool(value: unknown, fallback = false): boolean {
  return typeof value === "boolean" ? value : fallback;
}
function stringList(value: unknown, fallback: string[] = []): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string") : [...fallback];
}
function users(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((item) => {
    if (!item || typeof item !== "object" || Array.isArray(item)) return [];
    const user = item as Record<string, unknown>;
    const username = text(user.username);
    const password = text(user.password);
    return username || password ? [`${username}:${password}`] : [];
  });
}
function cloneConfig(config: InboundConfig): InboundConfig { return structuredClone(toRaw(config)); }

function initialize(): void {
  const config = props.resource?.config;
  baseConfig.value = config ? cloneConfig(config) : null;
  originalType.value = config?.type ?? "";
  selected.value = config ? editorType(config, props.resource?.mode) : "mixed";
  const used = props.existingTags.filter((tag) => tag !== config?.tag);
  form.tag = config?.tag ?? uniqueInboundTag("mixed-in", used);
  form.listen = text(config?.listen, "127.0.0.1");
  form.listenPort = numberText(config?.listen_port, defaultPorts[selected.value] ?? 7891);
  form.tcpFastOpen = bool(config?.tcp_fast_open);
  form.tcpMultiPath = bool(config?.tcp_multi_path);
  form.udpFragment = bool(config?.udp_fragment);
  form.users = users(config?.users);
  form.udpTimeout = text(config?.udp_timeout, "5m");
  form.network = text(config?.network);
  form.overrideAddress = text(config?.override_address);
  form.overridePort = numberText(config?.override_port);
  form.interfaceName = text(config?.interface_name);
  form.mtu = numberText(config?.mtu, 9000);
  form.stack = text(config?.stack, "mixed");
  form.autoRoute = bool(config?.auto_route, true);
  form.strictRoute = bool(config?.strict_route, true);
  form.endpointIndependentNat = bool(config?.endpoint_independent_nat);
  form.address = stringList(config?.address, ["172.18.0.1/30", "fdfe:dcba:9876::1/126"]);
  form.routeAddress = stringList(config?.route_address);
  form.routeExcludeAddress = stringList(config?.route_exclude_address);
  customSource.value = selected.value === "custom" && config ? JSON.stringify(config, null, 2) : customExample;
}

function chooseType(next: InboundEditorType): void {
  if (next === selected.value) return;
  if (next === "custom") {
    const candidate = selected.value === "custom" ? null : buildStructuredConfig();
    customSource.value = candidate ? JSON.stringify(candidate, null, 2) : customExample;
  } else {
    const previous = selected.value;
    if (previous === "custom") {
      const parsed = customValidation.value;
      if (parsed.valid && parsed.config.type === next) {
        baseConfig.value = cloneConfig(parsed.config);
        originalType.value = next;
        form.tag = parsed.config.tag;
      }
    }
    if (!form.tag.trim() || previous === "custom") form.tag = uniqueInboundTag(`${next}-in`, props.existingTags);
    form.listenPort = String(defaultPorts[next] ?? 7891);
  }
  selected.value = next;
}

function removeKeys(config: Record<string, unknown>, keys: string[]): void {
  for (const key of keys) delete config[key];
}
function cleanList(values: string[]): string[] { return values.map((item) => item.trim()).filter(Boolean); }

function buildStructuredConfig(): InboundConfig {
  const type = selected.value === "custom" ? "mixed" : selected.value;
  const sameStructuredType = props.resource?.mode === "structured" && originalType.value === type;
  const config: Record<string, unknown> = sameStructuredType && baseConfig.value ? cloneConfig(baseConfig.value) : {};
  removeKeys(config, [...legacyInboundFields]);
  config.type = type;
  config.tag = form.tag.trim();
  const listenKeys = ["listen", "listen_port", "tcp_fast_open", "tcp_multi_path", "udp_fragment", "users", "udp_timeout", "network", "override_address", "override_port"];
  const tunKeys = ["interface_name", "mtu", "stack", "auto_route", "strict_route", "endpoint_independent_nat", "address", "route_address", "route_exclude_address"];
  if (type === "tun") {
    removeKeys(config, listenKeys);
    if (form.interfaceName.trim()) config.interface_name = form.interfaceName.trim(); else delete config.interface_name;
    config.mtu = Number(form.mtu);
    config.stack = form.stack;
    config.auto_route = form.autoRoute;
    config.strict_route = form.strictRoute;
    config.endpoint_independent_nat = form.endpointIndependentNat;
    config.address = cleanList(form.address);
    const routeAddress = cleanList(form.routeAddress);
    const routeExcludeAddress = cleanList(form.routeExcludeAddress);
    if (routeAddress.length) config.route_address = routeAddress; else delete config.route_address;
    if (routeExcludeAddress.length) config.route_exclude_address = routeExcludeAddress; else delete config.route_exclude_address;
    return config as InboundConfig;
  }
  removeKeys(config, tunKeys);
  config.listen = form.listen.trim();
  config.listen_port = Number(form.listenPort);
  config.tcp_fast_open = form.tcpFastOpen;
  config.tcp_multi_path = form.tcpMultiPath;
  config.udp_fragment = form.udpFragment;
  if (type === "direct") {
    delete config.users;
    delete config.udp_timeout;
    if (form.network) config.network = form.network; else delete config.network;
    if (form.overrideAddress.trim()) config.override_address = form.overrideAddress.trim(); else delete config.override_address;
    if (form.overridePort) config.override_port = Number(form.overridePort); else delete config.override_port;
    return config as InboundConfig;
  }
  delete config.network;
  delete config.override_address;
  delete config.override_port;
  if (type === "socks") {
    if (form.udpTimeout.trim()) config.udp_timeout = form.udpTimeout.trim(); else delete config.udp_timeout;
  } else {
    delete config.udp_timeout;
  }
  const credentials = cleanList(form.users).map((credential) => {
    const separator = credential.indexOf(":");
    return separator < 0
      ? { username: credential, password: "" }
      : { username: credential.slice(0, separator), password: credential.slice(separator + 1) };
  });
  if (credentials.length) config.users = credentials; else delete config.users;
  return config as InboundConfig;
}

function submit(): void {
  if (!valid.value) return;
  if (selected.value === "custom") {
    if (customValidation.value.valid) emit("submit", { mode: "custom", config: cloneConfig(customValidation.value.config) });
    return;
  }
  emit("submit", { mode: "structured", config: buildStructuredConfig() });
}

function formatCustom(): void {
  if (customValidation.value.valid) customSource.value = JSON.stringify(customValidation.value.config, null, 2);
}

function insertIndent(event: KeyboardEvent): void {
  if (event.key === "Tab") {
    event.preventDefault();
    const target = event.currentTarget as HTMLTextAreaElement;
    const start = target.selectionStart;
    const end = target.selectionEnd;
    customSource.value = `${customSource.value.slice(0, start)}  ${customSource.value.slice(end)}`;
    requestAnimationFrame(() => { target.selectionStart = target.selectionEnd = start + 2; });
  }
  if ((event.metaKey || event.ctrlKey) && event.key === "Enter" && valid.value && !props.busy) {
    event.preventDefault();
    submit();
  }
}
</script>

<template>
  <AppDialog class="inbound-editor-dialog" :open="open" :title="title" :description="description" :close-disabled="busy" :initial-focus="selected === 'custom' ? 'textarea' : 'input[name=tag]'" @close="emit('close')">
    <form id="inbound-editor-form" class="inbound-editor-form" @submit.prevent="submit">
      <section class="kind-section"><label>入站类型<small>选择常用模板或完整 JSON</small></label><div class="type-grid">
        <label v-for="item in inboundTypes" :key="item.type"><input type="radio" name="inbound-type" :value="item.type" :checked="selected === item.type" @change="chooseType(item.type)" /><span><AppIcon :name="item.icon" :size="21" /><strong>{{ item.label }}</strong><small>{{ item.description }}</small></span></label>
      </div></section>

      <template v-if="selected !== 'custom'">
        <label class="field tag-field"><span>入站名称<small>对应 tag</small></span><input v-model="form.tag" name="tag" required /></label>
        <div class="divider"><span>类型专属配置</span></div>

        <template v-if="selected === 'tun'">
          <div class="toggle-grid">
            <label class="toggle-card"><span><strong>自动设置全局路由</strong><small>由 sing-box 自动写入系统路由</small></span><input v-model="form.autoRoute" type="checkbox" /></label>
            <label class="toggle-card"><span><strong>严格路由</strong><small>减少流量泄漏并强化路由约束</small></span><input v-model="form.strictRoute" type="checkbox" /></label>
            <label class="toggle-card"><span><strong>独立于端点的 NAT</strong><small>为 UDP 使用 endpoint-independent NAT</small></span><input v-model="form.endpointIndependentNat" type="checkbox" /></label>
          </div>
          <div class="form-grid">
            <label class="field"><span>TUN 网卡名称<small>interface_name</small></span><input v-model="form.interfaceName" placeholder="留空由系统自动命名" /></label>
            <label class="field"><span>最大传输单元<small>MTU</small></span><input v-model="form.mtu" type="number" min="1" /></label>
            <fieldset class="field full stack-field"><legend>TUN 模式堆栈 <small>stack</small></legend><div><label v-for="stack in ['system', 'gvisor', 'mixed']" :key="stack"><input v-model="form.stack" type="radio" :value="stack" /><span>{{ stack === 'gvisor' ? 'gVisor' : stack[0]?.toUpperCase() + stack.slice(1) }}</span></label></div></fieldset>
            <div class="field full list-field"><span>IPv4 和 IPv6 前缀<small>address</small></span><div v-for="(_, index) in form.address" :key="`address-${index}`" class="list-row"><input v-model="form.address[index]" :aria-label="`TUN 地址 ${index + 1}`" placeholder="例如 fd00::1/126" /><button type="button" :aria-label="`删除 TUN 地址 ${index + 1}`" @click="form.address.splice(index, 1)">×</button></div><button class="add-row" type="button" @click="form.address.push('')"><AppIcon name="plus" :size="13" />添加地址</button></div>
            <div class="field full list-field"><span>自定义路由<small>route_address</small></span><div v-for="(_, index) in form.routeAddress" :key="`route-${index}`" class="list-row"><input v-model="form.routeAddress[index]" :aria-label="`自定义路由 ${index + 1}`" placeholder="例如 0.0.0.0/1 或 ::/1" /><button type="button" :aria-label="`删除自定义路由 ${index + 1}`" @click="form.routeAddress.splice(index, 1)">×</button></div><button class="add-row" type="button" @click="form.routeAddress.push('')"><AppIcon name="plus" :size="13" />添加路由</button></div>
            <div class="field full list-field"><span>排除自定义路由<small>route_exclude_address</small></span><div v-for="(_, index) in form.routeExcludeAddress" :key="`exclude-${index}`" class="list-row"><input v-model="form.routeExcludeAddress[index]" :aria-label="`排除路由 ${index + 1}`" placeholder="例如 fc00::/7" /><button type="button" :aria-label="`删除排除路由 ${index + 1}`" @click="form.routeExcludeAddress.splice(index, 1)">×</button></div><button class="add-row" type="button" @click="form.routeExcludeAddress.push('')"><AppIcon name="plus" :size="13" />添加排除路由</button></div>
          </div>
          <p class="type-note"><AppIcon name="shield" :size="15" />TUN 地址与路由字段支持多个 IPv4/IPv6 CIDR。</p>
        </template>

        <template v-else>
          <div class="toggle-grid listen-toggles">
            <label class="toggle-card"><span><strong>TCP Fast Open</strong><small>减少 TCP 握手延迟</small></span><input v-model="form.tcpFastOpen" type="checkbox" /></label>
            <label class="toggle-card"><span><strong>TCP Multi Path</strong><small>允许单连接使用多条路径</small></span><input v-model="form.tcpMultiPath" type="checkbox" /></label>
            <label class="toggle-card"><span><strong>UDP 分片</strong><small>需要时发送分片 UDP 数据报</small></span><input v-model="form.udpFragment" type="checkbox" /></label>
          </div>
          <div class="form-grid">
            <label class="field"><span>监听地址<small>listen</small></span><input v-model="form.listen" required /></label>
            <label class="field"><span>监听端口<small>listen_port</small></span><input v-model="form.listenPort" type="number" min="1" max="65535" required /></label>
            <label v-if="selected === 'socks'" class="field"><span>UDP 超时<small>例如 5m</small></span><input v-model="form.udpTimeout" /></label>
            <template v-if="selected === 'direct'">
              <label class="field"><span>网络协议<small>network</small></span><select v-model="form.network"><option value="">TCP + UDP</option><option value="tcp">TCP</option><option value="udp">UDP</option></select></label>
              <label class="field"><span>覆盖目标地址<small>可选</small></span><input v-model="form.overrideAddress" /></label>
              <label class="field"><span>覆盖目标端口<small>可选</small></span><input v-model="form.overridePort" type="number" min="1" max="65535" /></label>
            </template>
            <div v-else class="field full list-field"><span>认证用户<small>users · 用户名:密码</small></span><div v-for="(_, index) in form.users" :key="`user-${index}`" class="list-row"><input v-model="form.users[index]" :aria-label="`认证用户 ${index + 1}`" placeholder="用户名:密码" /><button type="button" :aria-label="`删除认证用户 ${index + 1}`" @click="form.users.splice(index, 1)">×</button></div><button class="add-row" type="button" @click="form.users.push('')"><AppIcon name="plus" :size="13" />添加认证用户</button></div>
          </div>
          <p class="type-note"><AppIcon :name="selected === 'direct' ? 'route' : selected === 'http' ? 'globe' : 'inbound'" :size="15" />{{ selected === "direct" ? "Direct 入站适合透明转发或 DNS 劫持。" : selected === "http" ? "HTTP 入站接收标准 HTTP 代理与 CONNECT 请求。" : selected === "socks" ? "SOCKS 入站支持 SOCKS4、SOCKS4a 与 SOCKS5。" : "Mixed 入站在同一端口同时提供 HTTP 与 SOCKS 代理。" }}</p>
        </template>
        <p v-if="structuredError" class="validation invalid" role="alert"><AppIcon name="info" :size="14" />{{ structuredError }}</p>
      </template>

      <template v-else>
        <div class="divider"><span>完整入站对象</span></div>
        <div class="json-workbench"><div class="json-toolbar"><div><b>JSON</b><span>单个入站对象</span></div><div><button type="button" @click="customSource = customExample">示例</button><button type="button" :disabled="!customValidation.valid" @click="formatCustom"><AppIcon name="sparkles" :size="13" />格式化</button></div></div><textarea v-model="customSource" aria-label="入站 JSON" :aria-invalid="!customValidation.valid" spellcheck="false" @keydown="insertIndent" /></div>
        <p class="validation" :class="customValidation.valid ? 'valid' : 'invalid'" role="status"><AppIcon :name="customValidation.valid ? 'check' : 'info'" :size="14" />{{ customValidation.message }}</p>
        <p class="type-note"><AppIcon name="info" :size="15" />请输入一个完整的 sing-box 入站对象，而不是整个配置文件或 inbounds 数组。</p>
      </template>
    </form>
    <template #footer>
      <button v-if="resource && allowDelete" class="delete-button" type="button" :disabled="busy" @click="emit('delete')"><AppIcon name="trash" :size="14" />删除入站配置</button>
      <button class="secondary-button" type="button" :disabled="busy" @click="emit('close')">取消</button>
      <button class="primary-button" type="submit" form="inbound-editor-form" :disabled="!valid || busy"><AppIcon :name="resource ? 'save' : 'plus'" :size="14" />{{ busy ? "正在保存" : resource ? "保存入站配置" : "新建入站配置" }}</button>
    </template>
  </AppDialog>
</template>

<style scoped>
:global(.inbound-editor-dialog) { width: min(780px, calc(100vw - 32px)); }.inbound-editor-form { display: grid; gap: 14px; }.kind-section { display: grid; gap: 8px; }.kind-section > label, .field > span, .stack-field legend { color: var(--text); font-size: var(--font-size-base); font-weight: 680; }.kind-section > label small, .field > span small, .stack-field legend small { display: block; margin-top: 1px; color: var(--text-faint); font-size: var(--font-size-sm); font-weight: 500; }.type-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }.type-grid label { min-width: 0; }.type-grid input { position: absolute; width: 1px; height: 1px; overflow: hidden; opacity: 0; }.type-grid label > span { display: grid; min-height: 66px; grid-template-columns: auto minmax(0, 1fr); grid-template-rows: auto auto; align-content: center; gap: 2px 9px; padding: 10px 11px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-soft); cursor: pointer; transition: 150ms ease; }.type-grid label > span:hover { border-color: color-mix(in srgb, var(--primary) 30%, var(--line)); transform: translateY(-1px); }.type-grid .icon { grid-row: 1 / 3; align-self: center; color: var(--primary); }.type-grid strong, .type-grid small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.type-grid strong { font-size: var(--font-size-base); }.type-grid small { color: var(--text-faint); font-size: var(--font-size-sm); }.type-grid input:checked + span { border-color: color-mix(in srgb, var(--primary) 48%, var(--line)); background: var(--primary-soft); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--primary) 8%, transparent); }.type-grid input:focus-visible + span { outline: 3px solid color-mix(in srgb, var(--primary) 22%, transparent); outline-offset: 1px; }.field { display: grid; min-width: 0; gap: 6px; }.tag-field { margin-top: 2px; }.form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }.field.full { grid-column: 1 / -1; }.divider { display: flex; align-items: center; gap: 10px; color: var(--text-faint); font-size: var(--font-size-xs); font-weight: 690; letter-spacing: .45px; }.divider::before, .divider::after { height: 1px; flex: 1; content: ""; background: var(--line); }.divider::before { max-width: 18px; }.toggle-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 9px; }.toggle-card, .toggle-line { display: flex; min-width: 0; min-height: 60px; align-items: center; justify-content: space-between; gap: 10px; padding: 10px 12px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-soft); }.toggle-card span, .toggle-card strong, .toggle-card small { display: block; }.toggle-card strong { font-size: var(--font-size-base); }.toggle-card small { margin-top: 2px; color: var(--text-faint); font-size: var(--font-size-sm); }.toggle-card input, .toggle-line input { width: 34px; min-height: 19px; accent-color: var(--primary); }.stack-field { padding: 0; border: 0; }.stack-field > div { display: grid; grid-template-columns: repeat(3, 1fr); gap: 6px; }.stack-field label { position: relative; }.stack-field input { position: absolute; opacity: 0; }.stack-field label span { display: grid; min-height: 38px; place-items: center; border: 1px solid var(--line); border-radius: 8px; color: var(--text-soft); background: var(--surface-soft); font-size: var(--font-size-base); cursor: pointer; }.stack-field input:checked + span { color: var(--primary); border-color: var(--primary); background: var(--primary-soft); }.list-field { gap: 7px; }.list-row { display: grid; grid-template-columns: minmax(0, 1fr) 36px; gap: 6px; }.list-row button { min-height: 36px; padding: 0; color: var(--text-faint); border: 1px solid var(--line); background: var(--surface-soft); }.list-row button:hover { color: var(--red); background: var(--red-soft); }.add-row { display: inline-flex; width: fit-content; min-height: 31px; align-items: center; gap: 5px; padding: 5px 8px; color: var(--primary); border: 1px dashed color-mix(in srgb, var(--primary) 30%, var(--line)); background: var(--primary-soft); font-size: var(--font-size-xs); }.type-note { display: flex; align-items: flex-start; gap: 8px; margin: 0; padding: 9px 10px; border: 1px solid var(--line); border-radius: 9px; color: var(--text-soft); background: var(--surface-soft); font-size: var(--font-size-xs); }.type-note .icon { color: var(--primary); }.json-workbench { overflow: hidden; border: 1px solid var(--line); border-radius: 11px; background: #10182a; }.json-toolbar { display: flex; min-height: 39px; align-items: center; justify-content: space-between; gap: 10px; padding: 7px 10px; border-bottom: 1px solid rgb(255 255 255 / 9%); color: #91a0bb; font-size: var(--font-size-xs); }.json-toolbar b { margin-right: 8px; color: #8aa2ff; }.json-toolbar button { display: inline-flex; min-height: 27px; align-items: center; gap: 4px; padding: 4px 7px; color: #b8c4d9; background: transparent; font-size: var(--font-size-sm); }.json-workbench textarea { display: block; width: 100%; min-height: 320px; padding: 14px 16px; border: 0; border-radius: 0; outline: 0; color: #d4def0; background: #10182a; font: var(--font-size-base)/1.62 ui-monospace, SFMono-Regular, Consolas, monospace; resize: vertical; }.json-workbench textarea[aria-invalid="true"] { box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--red) 75%, transparent); }.validation { display: flex; align-items: center; gap: 7px; margin: 0; padding: 8px 10px; border-radius: 8px; font-size: var(--font-size-xs); }.validation.valid { color: var(--green); background: var(--green-soft); }.validation.invalid { color: var(--red); background: var(--red-soft); }.secondary-button, .primary-button, .delete-button { display: inline-flex; min-height: 36px; align-items: center; gap: 6px; padding: 7px 11px; border: 1px solid var(--line); font-size: var(--font-size-sm); }.secondary-button { color: var(--text-soft); background: var(--surface); }.primary-button { color: #fff; border-color: var(--primary); background: var(--primary); }.delete-button { margin-right: auto; color: var(--red); border-color: color-mix(in srgb, var(--red) 20%, var(--line)); background: var(--red-soft); }
@media (max-width: 680px) { :global(.inbound-editor-dialog) { width: 100%; }.type-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }.toggle-grid, .form-grid { grid-template-columns: 1fr; }.field.full { grid-column: 1; }.json-workbench textarea { min-height: 40dvh; }.delete-button { width: 36px; justify-content: center; padding-inline: 0; overflow: hidden; color: transparent; gap: 0; }.delete-button :deep(.icon) { color: var(--red); } }
</style>
