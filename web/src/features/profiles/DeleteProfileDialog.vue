<script setup lang="ts">
import { computed, ref } from "vue";
import { useRouter } from "vue-router";
import { apiClient } from "../../api/client";
import { useProfilesStore } from "../../stores/profiles";

const props = defineProps<{ profileId: string }>();
const emit = defineEmits<{ close: [] }>();

const profiles = useProfilesStore();
const router = useRouter();

const confirmText = ref("");
const errorMessage = ref("");
const deleting = ref(false);

const profile = computed(() => profiles.items.find((item) => item.id === props.profileId));

const canDelete = computed(() => profile.value !== undefined && confirmText.value === profile.value.name);

async function submit(): Promise<void> {
  if (profile.value === undefined) {
    return;
  }
  errorMessage.value = "";
  deleting.value = true;
  try {
    const { data } = await apiClient.request<{ warning: string }>(`/profiles/${props.profileId}`, {
      method: "DELETE",
      body: JSON.stringify({ confirmName: profile.value.name }),
    });
    void data.warning;
    await profiles.remove(props.profileId);
    emit("close");
    await router.replace("/dashboard");
  } catch {
    errorMessage.value = "删除失败，请确认档案名称";
  } finally {
    deleting.value = false;
  }
}
</script>

<template>
  <div class="modal-backdrop" role="dialog" aria-label="删除配置档案" aria-modal="true">
    <div class="modal-card">
      <h2 class="modal-title">删除配置档案</h2>
      <p class="modal-copy">
        删除「{{ profile?.name ?? "" }}」将移除其草稿与已发布配置。一期无法判断外部核心是否正在使用该配置路径。
      </p>
      <label class="field">
        <span class="field-label">请输入档案名称以确认</span>
        <input
          v-model="confirmText"
          aria-label="档案名称确认"
          class="input"
          type="text"
          autocomplete="off"
        />
      </label>
      <p v-if="errorMessage" class="modal-error" role="alert">{{ errorMessage }}</p>
      <div class="modal-actions">
        <button class="btn" type="button" @click="emit('close')">取消</button>
        <button
          class="btn btn-danger"
          name="确认删除档案"
          type="button"
          :disabled="!canDelete || deleting"
          @click="submit"
        >
          确认删除档案
        </button>
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
  width: min(440px, 100%);
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

.modal-copy {
  margin: 0;
  color: var(--text-soft);
  font-size: 14px;
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
