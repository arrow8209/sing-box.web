<script setup lang="ts">
defineProps<{ modelValue: string; label: string; error?: string; rows?: number; dataOutboundJson?: boolean }>();
const emit = defineEmits<{ "update:modelValue": [value: string] }>();
</script>
<template>
  <div class="json-editor">
    <label class="sr-only" :for="`json-editor-${label}`">{{ label }}</label>
    <textarea :id="`json-editor-${label}`" :value="modelValue" :rows="rows ?? 16" spellcheck="false" :data-outbound-json="dataOutboundJson || undefined" :aria-invalid="Boolean(error)" :aria-describedby="error ? `json-error-${label}` : undefined" @input="emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)" />
    <p v-if="error" :id="`json-error-${label}`" class="json-editor__error" role="alert">{{ error }}</p>
  </div>
</template>
