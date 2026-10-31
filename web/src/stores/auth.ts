import { defineStore } from 'pinia';
import { ref } from 'vue';
import * as api from '../api';
import { getErrorMessage } from '../api';

export const useAuthStore = defineStore('auth', () => {
  const user = ref<api.User | null>(null);
  const loading = ref(false);
  const error = ref<string | null>(null);
  const initialized = ref(false);

  async function refresh() {
    try {
      user.value = await api.me();
    } catch {
      user.value = null;
    } finally {
      initialized.value = true;
    }
  }

  async function login(email: string, password: string) {
    loading.value = true;
    error.value = null;
    try {
      const { user: u } = await api.login({ email, password });
      user.value = u;
    } catch (e: any) {
      error.value = getErrorMessage(e);
      throw e;
    } finally {
      loading.value = false;
    }
  }

  async function register(email: string, username: string, password: string) {
    loading.value = true;
    error.value = null;
    try {
      const { user: u } = await api.register({ email, username, password });
      user.value = u;
    } catch (e: any) {
      error.value = getErrorMessage(e);
      throw e;
    } finally {
      loading.value = false;
    }
  }

  async function logout() {
    try {
      await api.logout();
    } catch {
      // Even if the server call fails, clear local state.
    }
    user.value = null;
  }

  return { user, loading, error, initialized, refresh, login, register, logout };
});
