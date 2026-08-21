<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { ApiError } from "@/api/client";
import { useAuthStore } from "./store";

const password = ref("");
const pending = ref(false);
const error = ref("");
const router = useRouter();
const auth = useAuthStore();

async function submit(): Promise<void> {
  if (!password.value || pending.value) return;
  pending.value = true;
  error.value = "";
  try {
    await auth.login(password.value);
    await router.replace((router.currentRoute.value.query.redirect as string | undefined) ?? "/");
  } catch (cause) {
    error.value = cause instanceof ApiError ? cause.message : "无法连接到服务，请稍后重试";
  } finally {
    pending.value = false;
  }
}
</script>

<template>
  <main class="login-page">
    <form class="login-card" @submit.prevent="submit">
      <p class="eyebrow">SING-BOX WEB</p>
      <h1>登录管理面板</h1>
      <p class="description">输入管理密码以继续。</p>
      <label for="password">管理密码</label>
      <input id="password" v-model="password" type="password" name="password" autocomplete="current-password" required />
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
      <button type="submit" :disabled="pending">{{ pending ? "正在登录…" : "登录" }}</button>
    </form>
  </main>
</template>

<style scoped>
.login-page { display: grid; min-height: 100vh; place-items: center; padding: 24px; }
.login-card { width: min(100%, 400px); padding: 32px; border: 1px solid var(--line); border-radius: var(--radius-lg); background: var(--surface); box-shadow: var(--shadow-lg); }
.eyebrow { margin: 0 0 8px; color: var(--primary); font-size: 12px; font-weight: 750; letter-spacing: .12em; }
h1 { margin: 0; font-size: 26px; }
.description { margin: 10px 0 24px; color: var(--text-soft); }
label { display: block; margin-bottom: 8px; font-weight: 650; }
input { width: 100%; margin-bottom: 16px; }
button { width: 100%; }
.form-error { margin: -4px 0 12px; color: var(--red); }
</style>
