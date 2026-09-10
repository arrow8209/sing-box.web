<script setup lang="ts">
import AppearanceSettings from "./AppearanceSettings.vue";
import CoreSettings from "./CoreSettings.vue";
import PasswordSettings from "./PasswordSettings.vue";
import ProxySettings from "./ProxySettings.vue";
</script>

<template>
  <section class="settings-page">
    <header class="page-intro">
      <h1>系统设置</h1>
      <p>管理核心行为、控制台偏好与安全访问方式。</p>
    </header>
    <div class="settings-content">
      <section id="core"><CoreSettings /></section>
      <section id="appearance"><AppearanceSettings /></section>
      <section id="proxy"><ProxySettings /></section>
      <section id="security"><PasswordSettings /></section>
    </div>
  </section>
</template>

<style scoped>
.settings-page { display: grid; gap: var(--section-gap); min-width: 0; }
.page-intro h1 { margin: 0; font-size: 22px; letter-spacing: -.4px; }
.page-intro p { margin: 5px 0 0; color: var(--text-soft); font-size: var(--font-size-base); }
.settings-content { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: start; gap: var(--section-gap); }
.settings-content section { min-width: 0; scroll-margin-top: calc(var(--topbar-height) + var(--section-gap)); }
@media (max-width: 1280px) {
  .settings-content { grid-template-columns: minmax(0, 1fr); }
}
</style>
