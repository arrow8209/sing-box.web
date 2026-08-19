<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../../stores/auth";

const auth = useAuthStore();
const router = useRouter();

const darkMode = ref(false);
const newPassword = ref("");
const confirmPassword = ref("");
const passwordError = ref("");
const changing = ref(false);

onMounted(() => {
  const stored = localStorage.getItem("singbox-theme");
  darkMode.value = stored === "dark";
});

function onThemeChange(event: Event): void {
  darkMode.value = (event.target as HTMLInputElement).checked;
  localStorage.setItem("singbox-theme", darkMode.value ? "dark" : "light");
}

async function changePassword(): Promise<void> {
  passwordError.value = "";
  if (newPassword.value !== confirmPassword.value) {
    passwordError.value = "两次输入的密码不一致";
    return;
  }
  changing.value = true;
  try {
    await auth.changePassword(newPassword.value);
    newPassword.value = "";
    confirmPassword.value = "";
    // 密码修改使所有会话失效，回到登录页。
    auth.session = null;
    await router.replace("/login");
  } catch {
    passwordError.value = "密码修改失败，请确认新密码至少 12 个字符";
  } finally {
    changing.value = false;
  }
}
</script>

<template>
  <section class="settings-view">
    <h2 class="settings-title">系统设置</h2>

    <fieldset class="settings-section">
      <legend class="section-title">界面</legend>
      <label class="form-row">
        <span class="form-label">深色模式</span>
        <input
          aria-label="深色模式"
          type="checkbox"
          :checked="darkMode"
          @change="onThemeChange"
        />
      </label>
    </fieldset>

    <fieldset class="settings-section">
      <legend class="section-title">管理员</legend>
      <label class="form-row">
        <span class="form-label">新密码</span>
        <input
          v-model="newPassword"
          aria-label="新密码"
          class="input"
          type="password"
          autocomplete="new-password"
        />
      </label>
      <label class="form-row">
        <span class="form-label">确认新密码</span>
        <input
          v-model="confirmPassword"
          aria-label="确认新密码"
          class="input"
          type="password"
          autocomplete="new-password"
        />
      </label>
      <p v-if="passwordError" class="password-error" role="alert">{{ passwordError }}</p>
      <button
        class="btn btn-primary"
        name="修改管理员密码"
        type="button"
        :disabled="changing"
        @click="changePassword"
      >
        修改管理员密码
      </button>
    </fieldset>
  </section>
</template>

<style scoped>
.settings-view {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-width: 560px;
}

.settings-title {
  margin: 0;
  font-size: 18px;
  color: var(--text);
}

.settings-section {
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.section-title {
  padding: 0 6px;
  color: var(--text-soft);
  font-size: 13px;
}

.form-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.form-label {
  flex: 0 0 110px;
  color: var(--text-soft);
  font-size: 13px;
}

.input {
  flex: 1;
  min-width: 0;
  padding: 8px 10px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  font: inherit;
}

.password-error {
  margin: 0;
  color: var(--red);
  font-size: 13px;
}
</style>
