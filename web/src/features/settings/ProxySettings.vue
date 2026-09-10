<script setup lang="ts">
import { onMounted, ref } from "vue";
import { proxySettingsApi, type ProxySettingsApi } from "./api";

const props = withDefaults(defineProps<{ api?: ProxySettingsApi }>(), {
  api: () => proxySettingsApi
});

const proxyUrl = ref("");
const effectiveProxyUrl = ref("");
const loading = ref(true);
const saving = ref(false);
const error = ref("");
const saved = ref(false);

onMounted(async () => {
  try {
    const result = await props.api.get();
    proxyUrl.value = result.proxyUrl;
    effectiveProxyUrl.value = result.proxyUrl;
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "读取代理设置失败";
  } finally {
    loading.value = false;
  }
});

async function submit(): Promise<void> {
  error.value = "";
  saved.value = false;
  saving.value = true;
  try {
    const result = await props.api.update({ proxyUrl: proxyUrl.value.trim() });
    proxyUrl.value = result.proxyUrl;
    effectiveProxyUrl.value = result.proxyUrl;
    saved.value = true;
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "保存代理设置失败";
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <section class="panel" aria-labelledby="proxy-settings-title">
    <div class="panel-heading">
      <div>
        <h2 id="proxy-settings-title">代理设置</h2>
        <p>用于服务端发起的 HTTP/HTTPS 下载，不会改变 sing-box 核心的代理流量。</p>
      </div>
      <span class="status-badge" :class="effectiveProxyUrl ? 'status-badge--success' : 'status-badge--neutral'">
        {{ effectiveProxyUrl ? "使用代理" : "直连" }}
      </span>
    </div>
    <form @submit.prevent="submit">
      <label for="download-proxy-url">
        代理服务器 URL
        <input
          id="download-proxy-url"
          v-model="proxyUrl"
          @input="saved = false"
          :disabled="loading || saving"
          inputmode="url"
          type="url"
          placeholder="http://127.0.0.1:7890"
          autocomplete="off"
        >
        <small>支持 http:// 和 https:// 地址；留空保存后恢复直连。URL 中不能包含账号密码。</small>
      </label>
      <p class="scope-note">适用于核心版本查询与下载、校验文件、规则集目录、预览以及远程规则下载。</p>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <div class="actions">
        <button :disabled="loading || saving" type="submit">
          {{ loading ? "正在读取" : saving ? "正在保存" : "保存代理设置" }}
        </button>
        <span v-if="saved" class="saved" role="status">代理设置已保存并即时生效</span>
      </div>
    </form>
  </section>
</template>

<style scoped>
.panel { display: grid; gap: var(--section-gap); padding: var(--card-padding); }
.panel-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 14px; }
h2 { margin: 0; font-size: 15px; }
.panel-heading p { margin: 4px 0 0; color: var(--text-soft); font-size: 11px; }
form { display: grid; gap: var(--section-gap); min-width: 0; }
label { display: grid; gap: 6px; color: var(--text-soft); font-size: 11px; font-weight: 650; }
input { width: 100%; min-height: 38px; font-size: 11px; }
small { font-weight: 500; }
.scope-note { margin: 0; padding: 9px 11px; border-radius: 8px; color: var(--text-soft); background: var(--surface-soft); font-size: 11px; }
.actions { display: flex; align-items: center; gap: 12px; }
button { width: fit-content; min-height: 35px; padding: 7px 12px; border-radius: 8px; font-size: 11px; }
.saved { color: var(--green); font-size: 11px; font-weight: 650; }
.error { margin: 0; color: var(--red); font-size: 11px; }
@media (max-width: 520px) {
  .panel-heading { align-items: stretch; flex-direction: column; }
  .status-badge { width: fit-content; }
  .actions { align-items: flex-start; flex-direction: column; }
}
</style>
