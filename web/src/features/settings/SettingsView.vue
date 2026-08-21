<script setup lang="ts">
import AppearanceSettings from "./AppearanceSettings.vue";
import PasswordSettings from "./PasswordSettings.vue";
</script>

<template><section class="settings-page"><header class="page-intro"><h1>设置</h1><p>界面偏好只保存在当前浏览器；认证和配置事实始终来自服务端。</p></header><div class="settings-layout"><nav class="settings-nav" aria-label="设置分类"><a href="#appearance">外观与布局</a><a href="#security">管理密码</a></nav><div class="settings-content"><section id="appearance"><AppearanceSettings /></section><section id="security"><PasswordSettings /></section></div></div></section></template>

<style scoped>
.settings-page { display: grid; gap: 16px; max-width: 1040px; }.page-intro h1 { margin: 0; font-size: 22px; letter-spacing: -.4px; }.page-intro p { margin: 5px 0 0; color: var(--text-soft); font-size: 12px; }.settings-layout { display: grid; grid-template-columns: 210px minmax(0, 1fr); gap: 14px; }.settings-nav { position: sticky; top: 80px; display: grid; align-content: start; height: fit-content; gap: 3px; padding: 8px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.settings-nav a { padding: 9px 10px; border-radius: 8px; color: var(--text-soft); font-size: 11px; font-weight: 650; text-decoration: none; }.settings-nav a:hover { color: var(--primary-strong); background: var(--primary-soft); }.settings-content { display: grid; gap: 14px; }.settings-content section { scroll-margin-top: 82px; }
@media (max-width: 760px) { .settings-layout { grid-template-columns: 1fr; }.settings-nav { position: static; grid-template-columns: repeat(2, minmax(0, 1fr)); }.settings-nav a { text-align: center; } }
</style>
