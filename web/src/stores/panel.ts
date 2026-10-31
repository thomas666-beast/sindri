import { defineStore } from 'pinia';
import { ref, watch } from 'vue';

export type PanelMode = 'user' | 'admin';

export const usePanelStore = defineStore('panel', () => {
  // 1. Load the initial state from localStorage (fallback to 'user')
  const storedMode = localStorage.getItem('barter.panel_mode') as PanelMode | null;
  const mode = ref<PanelMode>(storedMode === 'admin' ? 'admin' : 'user');

  // 2. Watch for changes and save them to localStorage
  watch(mode, (newMode) => {
    localStorage.setItem('barter.panel_mode', newMode);
  });

  function switchToAdmin() {
    mode.value = 'admin';
  }

  function switchToUser() {
    mode.value = 'user';
  }

  function reset() {
    mode.value = 'user';
  }

  return { mode, switchToAdmin, switchToUser, reset };
});
