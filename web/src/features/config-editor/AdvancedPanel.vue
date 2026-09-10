<script setup lang="ts">
import AppIcon from "@/layout/AppIcon.vue";
import type { Draft } from "@/features/profiles/types";
import { isJsonObject } from "./json";
import { advancedConfigSections, type ConfigModule, type ConfigSection } from "./module-map";

const props = defineProps<{ draft: Draft; errors?: Record<string, string> }>();
const emit = defineEmits<{ editJson: [module: ConfigModule] }>();

function configured(section: ConfigSection): boolean {
  const value = props.draft.config[section.key];
  if (Array.isArray(value)) return value.length > 0;
  return isJsonObject(value) && Object.keys(value).length > 0;
}

function summary(section: ConfigSection): string {
  if (props.errors?.[section.key]) return "需修正";
  const value = props.draft.config[section.key];
  if (Array.isArray(value) && value.length > 0) return `${value.length} 个项目`;
  if (isJsonObject(value) && Object.keys(value).length > 0) return `${Object.keys(value).length} 个字段`;
  return "未配置";
}
</script>

<template>
  <section class="advanced-panel">
    <div class="config-layout">
      <div class="config-main">
        <article class="card advanced-card">
          <header class="card-head">
            <div><h2 class="card-title">高级与共享模块</h2><p class="card-subtitle">平台相关配置以完整 JSON 片段维护</p></div>
            <span class="type-badge">{{ advancedConfigSections.length }} modules</span>
          </header>
          <div class="advanced-section-list">
            <button
              v-for="section in advancedConfigSections"
              :key="section.key"
              type="button"
              class="advanced-section-row"
              :aria-label="`编辑${section.label} JSON，${summary(section)}`"
              @click="emit('editJson', section.key)"
            >
              <span class="advanced-section-icon"><AppIcon :name="section.icon" :size="14" /></span>
              <span class="advanced-section-copy"><strong>{{ section.label }}</strong><code>{{ section.key }}</code><small>{{ errors?.[section.key] || section.description }}</small></span>
              <span class="advanced-section-status" :class="{ 'is-configured': configured(section), 'has-error': errors?.[section.key] }"><i />{{ summary(section) }}</span>
              <AppIcon class="advanced-section-chevron" name="chevron" :size="13" />
            </button>
          </div>
        </article>
      </div>

      <aside class="config-aside">
        <article class="card compact-guide-card">
          <header class="card-head"><div><h2 class="card-title">编辑策略</h2><p class="card-subtitle">完整性优先</p></div></header>
          <div class="card-body compact-guide-list">
            <div><AppIcon name="check" :size="15" /><span><strong>字段无损</strong><small>未知与未来版本字段原样保存</small></span></div>
            <div><AppIcon name="shield" :size="15" /><span><strong>类型校验</strong><small>保存前检查对象或数组</small></span></div>
            <div><AppIcon name="code" :size="15" /><span><strong>标准 JSON</strong><small>可直接作为 sing-box 配置片段</small></span></div>
          </div>
        </article>
        <div class="info-callout"><AppIcon name="info" :size="15" /><span>不同核心版本、平台与编译标签支持的字段可能不同；部署前请以实际核心的检查结果为准。</span></div>
      </aside>
    </div>
  </section>
</template>

<style scoped>
.config-layout { display: grid; grid-template-columns: minmax(0, 1.68fr) minmax(220px, .54fr); gap: var(--section-gap); align-items: start; }
.config-main, .config-aside { display: grid; min-width: 0; gap: var(--section-gap); }
.card { overflow: hidden; }
.card-head { display: flex; min-height: 46px; align-items: center; justify-content: space-between; gap: var(--section-gap); padding: 8px var(--card-padding); border-bottom: 1px solid var(--line); }
.card-title, .card-subtitle { margin: 0; }
.card-title { font-size: var(--font-size-heading); font-weight: 710; letter-spacing: -.15px; }
.card-subtitle { margin-top: 2px; color: var(--text-faint); font-size: var(--font-size-sm); }
.card-body { padding: var(--card-padding); }
.type-badge { display: inline-flex; min-height: 22px; align-items: center; padding: 3px 8px; border-radius: 999px; color: var(--text-soft); background: var(--surface-soft); font-size: var(--font-size-xs); font-weight: 660; white-space: nowrap; }
.advanced-section-list { display: grid; padding: 5px 12px 10px; }
.advanced-section-row { display: grid; min-width: 0; min-height: 46px; grid-template-columns: auto minmax(0, 1fr) auto auto; align-items: center; gap: 9px; padding: 8px 3px; border-bottom: 1px solid var(--line); border-radius: 0; color: var(--text-soft); background: transparent; text-align: left; }
.advanced-section-row:last-child { border-bottom: 0; }
.advanced-section-row:hover { color: var(--primary); }
.advanced-section-icon { display: grid; width: 30px; height: 30px; flex: 0 0 auto; place-items: center; border-radius: 8px; color: var(--primary); background: var(--primary-soft); }
.advanced-section-copy { display: grid; min-width: 0; grid-template-columns: auto minmax(0, 1fr); align-items: baseline; gap: 2px 7px; }
.advanced-section-copy strong { color: var(--text); font-size: var(--font-size-base); }
.advanced-section-copy code { overflow: hidden; color: var(--primary); font-size: var(--font-size-sm); text-overflow: ellipsis; white-space: nowrap; }
.advanced-section-copy small { grid-column: 1 / -1; overflow: hidden; color: var(--text-faint); font-size: var(--font-size-sm); text-overflow: ellipsis; white-space: nowrap; }
.advanced-section-status { display: inline-flex; max-width: 120px; align-items: center; gap: 5px; overflow: hidden; color: var(--text-faint); font-size: var(--font-size-sm); text-overflow: ellipsis; white-space: nowrap; }
.advanced-section-status i { width: 5px; height: 5px; flex: 0 0 auto; border-radius: 50%; background: var(--line-strong); }
.advanced-section-status.is-configured { color: var(--green); }
.advanced-section-status.is-configured i { background: var(--green); }
.advanced-section-status.has-error { color: var(--red); }
.advanced-section-status.has-error i { background: var(--red); }
.advanced-section-chevron { color: var(--text-faint); }
.compact-guide-list { display: grid; gap: 9px; }
.compact-guide-list > div { display: grid; grid-template-columns: auto minmax(0, 1fr); align-items: center; gap: 8px; }
.compact-guide-list > div > :deep(.icon) { color: var(--primary); }
.compact-guide-list strong, .compact-guide-list small { display: block; }
.compact-guide-list strong { font-size: var(--font-size-base); }
.compact-guide-list small { margin-top: 1px; color: var(--text-faint); font-size: var(--font-size-sm); }
.info-callout { display: flex; gap: 10px; padding: 11px 12px; border: 1px solid color-mix(in srgb, var(--primary) 14%, var(--line)); border-radius: 10px; color: var(--text-soft); background: var(--primary-soft); font-size: var(--font-size-sm); }
.info-callout :deep(.icon) { flex: 0 0 auto; color: var(--primary); }

@media (max-width: 1080px) {
  .config-layout { grid-template-columns: minmax(0, 1fr); }
  .config-aside { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 768px) {
  .config-aside { grid-template-columns: 1fr; }
  .advanced-section-list { padding: 4px 9px 8px; }
  .advanced-section-row { min-height: 46px; grid-template-columns: auto minmax(0, 1fr) auto; gap: 7px; }
  .advanced-section-status { display: none; }
}
@media (max-width: 420px) {
  .card-head { align-items: flex-start; }
  .advanced-section-copy strong { font-size: var(--font-size-base); }
  .advanced-section-copy code, .advanced-section-copy small { font-size: var(--font-size-xs); }
}
</style>
