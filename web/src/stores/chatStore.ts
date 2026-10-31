import { defineStore } from 'pinia';
import { ref } from 'vue';
import { api } from '../api';

export const useChatStore = defineStore('chat', () => {
  const unreadMessages = ref(0);

  async function refreshUnread() {
    try {
      const { data } = await api.get<{ data: number }>('/chat/unread');
      unreadMessages.value = data.data;
    } catch {}
  }

  function incrementUnread() {
    unreadMessages.value++;
  }

  function clearUnread() {
    unreadMessages.value = 0;
  }

  return { unreadMessages, refreshUnread, incrementUnread, clearUnread };
});
