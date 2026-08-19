import { defineStore } from "pinia";
import { apiClient, APIClientError } from "../api/client";
import type { SessionResponse } from "../api/types";

interface AuthState {
  session: SessionResponse | null;
  loading: boolean;
}

export const useAuthStore = defineStore("auth", {
  state: (): AuthState => ({
    session: null,
    loading: false,
  }),
  actions: {
    async restore(): Promise<void> {
      this.loading = true;
      try {
        const { data } = await apiClient.request<SessionResponse>("/auth/session");
        this.session = data;
        apiClient.setCSRFToken(data.csrfToken);
      } catch (error) {
        if (error instanceof APIClientError && error.status === 401) {
          // A missing session is a normal logged-out state.
          this.session = null;
          apiClient.clearCSRFToken();
        } else {
          this.session = null;
          apiClient.clearCSRFToken();
          throw error;
        }
      } finally {
        this.loading = false;
      }
    },
    async login(password: string): Promise<void> {
      const { data } = await apiClient.request<SessionResponse>("/auth/login", {
        method: "POST",
        body: JSON.stringify({ username: "admin", password }),
      });
      this.session = data;
      apiClient.setCSRFToken(data.csrfToken);
    },
    async logout(): Promise<void> {
      try {
        await apiClient.request<null>("/auth/logout", { method: "POST" });
      } finally {
        this.session = null;
        apiClient.clearCSRFToken();
      }
    },
    async changePassword(password: string): Promise<void> {
      await apiClient.request<null>("/auth/password", {
        method: "PUT",
        body: JSON.stringify({ password }),
      });
    },
  },
});
