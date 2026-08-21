import { reactive, readonly } from "vue";
import { clearCSRFToken, request, setUnauthorizedHandler } from "@/api/client";
import type { LoginResult, Session } from "@/api/types";

export type AuthStatus = "loading" | "anonymous" | "authenticated";

const state = reactive<{ status: AuthStatus; expiresAt: string | null }>({
  status: "loading",
  expiresAt: null
});

function reset(): void {
  clearCSRFToken();
  state.status = "anonymous";
  state.expiresAt = null;
}

async function initialize(): Promise<void> {
  state.status = "loading";
  try {
    const { data } = await request<Session>("/auth/session");
    if (data.valid) {
      state.status = "authenticated";
      state.expiresAt = data.expiresAt;
      return;
    }
  } catch {
    // An unavailable or expired session is treated as anonymous at startup.
  }
  reset();
}

async function login(password: string): Promise<void> {
  await request<LoginResult>("/auth/login", { method: "POST", body: { password } });
  state.status = "authenticated";
  const { data } = await request<Session>("/auth/session");
  state.expiresAt = data.expiresAt;
}

async function logout(): Promise<void> {
  try {
    await request<void>("/auth/logout", { method: "POST" });
  } finally {
    reset();
  }
}

async function changePassword(currentPassword: string, newPassword: string): Promise<void> {
  try {
    await request<void>("/auth/password", { method: "PUT", body: { currentPassword, newPassword } });
  } finally {
    reset();
  }
}

setUnauthorizedHandler(reset);

export function useAuthStore() {
  return { state: readonly(state), initialize, login, logout, changePassword, reset };
}
