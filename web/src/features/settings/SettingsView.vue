<script setup lang="ts">
import AppearanceSettings from "./AppearanceSettings.vue";
import PasswordSettings from "./PasswordSettings.vue";
</script>

<template><section class="page"><header><h1>设置</h1><p>界面偏好只保存在当前浏览器；认证和配置事实始终来自服务端。</p></header><AppearanceSettings /><PasswordSettings /></section></template>

<style scoped>
.page { display: grid; gap: 18px; max-width: 860px; } h1 { margin: 0; } header p { color: var(--text-soft); }
</style>
