<script setup lang="ts">
import { ref } from "vue";
import { RouterView } from "vue-router";
import AppSidebar from "./AppSidebar.vue";
import AppTopbar from "./AppTopbar.vue";

const menuOpen = ref(false);
</script>

<template>
  <AppSidebar :open="menuOpen" @close="menuOpen = false" />
  <section class="app-main">
    <AppTopbar @toggle-menu="menuOpen = !menuOpen" />
    <main class="app-content"><RouterView /></main>
  </section>
</template>
