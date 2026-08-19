<script setup lang="ts">
import { computed, ref } from "vue";
import { useDraftStore } from "../../../stores/draft";
import { setAtPath } from "../model";

const draft = useDraftStore();

const experimentalConfig = computed(
  () => (draft.draft?.content.config.experimental as Record<string, unknown>) ?? {},
);

const clashApi = computed(
  () => (experimentalConfig.value.clash_api as Record<string, unknown>) ?? {},
);
const cacheFile = computed(
  () => (experimentalConfig.value.cache_file as Record<string, unknown>) ?? {},
);

const secretRevealed = ref(false);
let maskTimer: ReturnType<typeof setTimeout> | null = null;

function update(path: string[], value: unknown): void {
  const config = draft.draft?.content.config ?? {};
  draft.replaceConfig(setAtPath(config, ["experimental", ...path], value));
}

function onText(path: string[], event: Event): void {
  const value = (event.target as HTMLInputElement).value;
  update(path, value === "" ? undefined : value);
}

function onCheckbox(path: string[], event: Event): void {
  update(path, (event.target as HTMLInputElement).checked);
}

function toggleSecret(): void {
  secretRevealed.value = !secretRevealed.value;
  if (maskTimer !== null) {
    clearTimeout(maskTimer);
  }
  if (secretRevealed.value) {
    maskTimer = setTimeout(() => {
      secretRevealed.value = false;
    }, 30_000);
  }
}

function onSecretBlur(): void {
  if (maskTimer !== null) {
    clearTimeout(maskTimer);
  }
  maskTimer = setTimeout(() => {
    secretRevealed.value = false;
  }, 30_000);
}

const secretInputType = computed(() => (secretRevealed.value ? "text" : "password"));
</script>

<template>
  <form class="advanced-editor" @submit.prevent>
    <fieldset class="editor-section">
      <legend class="section-title">Clash API</legend>
      <label class="form-row">
        <span class="form-label">外部控制器</span>
        <input
          aria-label="Clash API 外部控制器"
          type="text"
          :value="(clashApi.external_controller as string) ?? ''"
          @input="onText(['clash_api', 'external_controller'], $event)"
        />
      </label>
      <label class="form-row">
        <span class="form-label">Clash API Secret</span>
        <input
          aria-label="Clash API Secret"
          :type="secretInputType"
          :value="(clashApi.secret as string) ?? ''"
          @input="onText(['clash_api', 'secret'], $event)"
          @blur="onSecretBlur"
        />
        <button class="btn btn-sm" type="button" name="临时显示 Clash API Secret" @click="toggleSecret">
          临时显示
        </button>
      </label>
      <label class="form-row">
        <span class="form-label">默认模式</span>
        <input
          aria-label="Clash API 默认模式"
          type="text"
          :value="(clashApi.default_mode as string) ?? ''"
          @input="onText(['clash_api', 'default_mode'], $event)"
        />
      </label>
      <label class="form-row">
        <span class="form-label">外部 UI</span>
        <input
          aria-label="Clash API 外部 UI"
          type="text"
          :value="(clashApi.external_ui as string) ?? ''"
          @input="onText(['clash_api', 'external_ui'], $event)"
        />
      </label>
      <label class="form-row">
        <span class="form-label">外部 UI 下载地址</span>
        <input
          aria-label="Clash API 外部 UI 下载地址"
          type="text"
          :value="(clashApi.external_ui_download_url as string) ?? ''"
          @input="onText(['clash_api', 'external_ui_download_url'], $event)"
        />
      </label>
      <label class="form-row">
        <span class="form-label">允许 CORS</span>
        <input
          aria-label="Clash API 允许 CORS"
          type="checkbox"
          :checked="clashApi.cors === true"
          @change="onCheckbox(['clash_api', 'cors'], $event)"
        />
      </label>
      <label class="form-row">
        <span class="form-label">允许私网访问</span>
        <input
          aria-label="Clash API 允许私网访问"
          type="checkbox"
          :checked="clashApi.allow_lan === true"
          @change="onCheckbox(['clash_api', 'allow_lan'], $event)"
        />
      </label>
      <label class="form-row">
        <span class="form-label">出站</span>
        <input
          aria-label="Clash API 出站"
          type="text"
          :value="(clashApi.detour as string) ?? ''"
          @input="onText(['clash_api', 'detour'], $event)"
        />
      </label>
    </fieldset>

    <fieldset class="editor-section">
      <legend class="section-title">缓存文件</legend>
      <label class="form-row">
        <span class="form-label">启用缓存文件</span>
        <input
          aria-label="启用缓存文件"
          type="checkbox"
          :checked="cacheFile.enabled === true"
          @change="onCheckbox(['cache_file', 'enabled'], $event)"
        />
      </label>
      <label class="form-row">
        <span class="form-label">缓存文件路径</span>
        <input
          aria-label="缓存文件路径"
          type="text"
          :value="(cacheFile.path as string) ?? ''"
          @input="onText(['cache_file', 'path'], $event)"
        />
      </label>
      <label class="form-row">
        <span class="form-label">缓存 ID</span>
        <input
          aria-label="缓存 ID"
          type="text"
          :value="(cacheFile.cache_id as string) ?? ''"
          @input="onText(['cache_file', 'cache_id'], $event)"
        />
      </label>
      <label class="form-row">
        <span class="form-label">缓存存储 FakeIP</span>
        <input
          aria-label="缓存存储 FakeIP"
          type="checkbox"
          :checked="cacheFile.store_fakeip === true"
          @change="onCheckbox(['cache_file', 'store_fakeip'], $event)"
        />
      </label>
    </fieldset>
  </form>
</template>

<style scoped>
.advanced-editor {
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-width: 640px;
}

.editor-section {
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.section-title {
  padding: 0 6px;
  color: var(--text-soft);
  font-size: 13px;
}

.form-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.form-label {
  flex: 0 0 160px;
  color: var(--text-soft);
  font-size: 13px;
}

.form-row input[type="text"],
.form-row input[type="password"] {
  flex: 1;
  min-width: 0;
  padding: 8px 10px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  font: inherit;
}
</style>
