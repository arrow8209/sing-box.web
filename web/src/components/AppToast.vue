<script setup lang="ts">
withDefaults(defineProps<{ message: string; tone?: "info" | "error" | "success" }>(), { tone: "info" });
</script>
<template><div class="app-toast-region" :aria-live="tone === 'error' ? 'assertive' : 'polite'" aria-atomic="true"><p class="app-toast" :class="`app-toast--${tone}`" :role="tone === 'error' ? 'alert' : 'status'">{{ message }}</p></div></template>
