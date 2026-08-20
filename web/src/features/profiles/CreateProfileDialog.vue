<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { useProfilesStore } from "../../stores/profiles";

const emit = defineEmits<{ close: [] }>();
const profiles = useProfilesStore();
const router = useRouter();

const name = ref("");
const slug = ref("");
const errorMessage = ref("");

async function submit(): Promise<void> {
  errorMessage.value = "";
  try {
    await profiles.create({ name: name.value, slug: slug.value }, router);
    name.value = "";
    slug.value = "";
    emit("close");
  } catch {
    errorMessage.value = "创建失败，请检查名称与 slug";
  }
}
</script>

<template>
  <div class="modal-backdrop" role="dialog" aria-label="创建配置档案" aria-modal="true">
    <div class="modal-card">
      <h2 class="modal-title">创建配置档案</h2>
      <label class="field">
        <span class="field-label">显示名称</span>
        <input v-model="name" class="input" type="text" autocomplete="off" />
      </label>
      <label class="field">
        <span class="field-label">slug</span>
        <input v-model="slug" class="input" type="text" autocomplete="off" />
      </label>
      <p v-if="errorMessage" class="modal-error" role="alert">{{ errorMessage }}</p>
      <div class="modal-actions">
        <button class="btn" type="button" @click="emit('close')">取消</button>
        <button class="btn btn-primary" name="创建" type="button" @click="submit">创建</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: rgb(10 18 38 / 45%);
}

.modal-card {
  width: min(420px, 100%);
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 22px;
  border-radius: var(--radius-lg);
  background: var(--surface);
  box-shadow: var(--shadow-lg);
}

.modal-title {
  margin: 0;
  font-size: 17px;
  color: var(--text);
}

.modal-error {
  margin: 0;
  color: var(--red);
  font-size: 13px;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field-label {
  font-size: 13px;
  color: var(--text-soft);
}

.input {
  padding: 9px 12px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  font: inherit;
}
</style>
