<script setup lang="ts">
withDefaults(defineProps<{ message: string; tone?: "info" | "error" | "success" }>(), { tone: "info" });
</script>
<template><div class="app-toast-region" aria-live="polite" aria-atomic="true"><p class="app-toast" :class="`app-toast--${tone}`" role="status">{{ message }}</p></div></template>
