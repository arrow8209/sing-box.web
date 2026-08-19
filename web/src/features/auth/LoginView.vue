<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../../stores/auth";

const auth = useAuthStore();
const router = useRouter();

const password = ref("");
const errorMessage = ref("");
const submitting = ref(false);

async function submit(): Promise<void> {
  if (submitting.value) {
    return;
  }
  submitting.value = true;
  errorMessage.value = "";
  try {
    await auth.login(password.value);
    await router.replace("/dashboard");
  } catch {
    errorMessage.value = "用户名或密码错误";
  } finally {
    password.value = "";
    submitting.value = false;
  }
}
</script>

<template>
  <main class="login-view">
    <form class="login-card" @submit.prevent="submit">
      <h1 class="login-title">sing-box 控制台</h1>
      <p class="login-subtitle">登录以管理配置档案</p>
      <label class="field">
        <span class="field-label">用户名</span>
        <input aria-label="用户名" class="input" type="text" value="admin" readonly />
      </label>
      <label class="field">
        <span class="field-label">密码</span>
        <input
          v-model="password"
          aria-label="密码"
          class="input"
          type="password"
          autocomplete="current-password"
          autofocus
        />
      </label>
      <p v-if="errorMessage" class="login-error" role="alert">{{ errorMessage }}</p>
      <button class="btn btn-primary btn-block" name="登录" type="submit" :disabled="submitting">
        登录
      </button>
    </form>
  </main>
</template>

<style scoped>
.login-view {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: var(--bg);
}

.login-card {
  width: min(400px, 100%);
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-md);
  padding: 28px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.login-title {
  margin: 0;
  font-size: 22px;
  color: var(--text);
}

.login-subtitle {
  margin: -8px 0 4px;
  color: var(--text-soft);
  font-size: 14px;
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
  padding: 10px 12px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  font: inherit;
}

.input:focus {
  outline: 2px solid var(--primary);
  outline-offset: 1px;
}

.input[readonly] {
  background: var(--surface-soft);
  color: var(--text-faint);
}

.login-error {
  margin: 0;
  color: var(--red);
  font-size: 13px;
}

.btn-block {
  width: 100%;
}
</style>
