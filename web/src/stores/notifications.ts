import { defineStore } from 'pinia';
import { ref } from 'vue';
import * as notificationsApi from '../api/notifications';

export const useNotificationsStore = defineStore('notifications', () => {
  const unread = ref(0);
  const items = ref<notificationsApi.Notification[]>([]);
  const loading = ref(false);
  const error = ref<string | null>(null);

  async function refreshUnread() {
    try {
      unread.value = await notificationsApi.unreadCount();
    } catch {}
  }

  async function load() {
    loading.value = true;
    error.value = null;
    try {
      const page = await notificationsApi.list(false, 30, 0);
      items.value = page.notifications;
    } catch {
      error.value = 'failed to load notifications';
    } finally {
      loading.value = false;
    }
  }

  async function markRead(id: string) {
    try {
      await notificationsApi.markRead(id);
      const item = items.value.find((n) => n.id === id);
      if (item && !item.read_at_unix) {
        item.read_at_unix = Math.floor(Date.now() / 1000);
        if (unread.value > 0) unread.value--;
      }
    } catch {}
  }

  async function markAll() {
    try {
      await notificationsApi.markAllRead();
      const now = Math.floor(Date.now() / 1000);
      for (const n of items.value) {
        if (!n.read_at_unix) n.read_at_unix = now;
      }
      unread.value = 0;
    } catch {}
  }

  function reset() {
    unread.value = 0;
    items.value = [];
    error.value = null;
  }

  // NEW: Called by App.vue when WebSocket receives a notification
  function receiveNotification(n: notificationsApi.Notification) {
    unread.value++;
    if (items.value.length > 0) {
      items.value.unshift(n);
      if (items.value.length > 50) items.value.pop();
    }
  }

  return { unread, items, loading, error, refreshUnread, load, markRead, markAll, reset, receiveNotification };
});
