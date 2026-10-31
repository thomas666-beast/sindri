<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as notificationsApi from '../api/notifications';
import { useNotificationsStore } from '../stores/notifications';

const { t } = useI18n();
const notifStore = useNotificationsStore();
const router = useRouter();

const notifications = ref<notificationsApi.Notification[]>([]);
const loading = ref(true);
const loadingMore = ref(false);
const error = ref<string | null>(null);
const offset = ref(0);
const limit = 30;
const hasMore = ref(false);

async function load(reset = true) {
  if (reset) {
    loading.value = true;
    error.value = null;
    notifications.value = [];
    offset.value = 0;
  } else {
    loadingMore.value = true;
  }

  try {
    const page = await notificationsApi.list(false, limit, reset ? 0 : offset.value);
    notifications.value = reset ? page.notifications : [...notifications.value, ...page.notifications];
    offset.value = (reset ? 0 : offset.value) + page.notifications.length;
    hasMore.value = page.notifications.length === limit;
    notifStore.refreshUnread();
  } catch {
    error.value = t('notifications.load_failed');
  } finally {
    loading.value = false;
    loadingMore.value = false;
  }
}

async function markOne(n: notificationsApi.Notification) {
  if (!n.read_at_unix) {
    await notifStore.markRead(n.id);
    const idx = notifications.value.findIndex((x) => x.id === n.id);
    if (idx !== -1) {
      notifications.value[idx].read_at_unix = Math.floor(Date.now() / 1000);
    }
  }
}

async function markAll() {
  await notifStore.markAll();
  const now = Math.floor(Date.now() / 1000);
  for (const n of notifications.value) {
    if (!n.read_at_unix) n.read_at_unix = now;
  }
}

function fmtDate(unix: number): string {
  return new Date(unix * 1000).toISOString().replace('T', ' ').slice(0, 16);
}

// ── NEW: Safely parse metadata (it might be a JSON string) ──
function parseMeta(n: notificationsApi.Notification): any {
  if (!n.metadata) return {};
  if (typeof n.metadata === 'string') {
    try { return JSON.parse(n.metadata); } catch { return {}; }
  }
  return n.metadata;
}

function getNotificationLink(n: notificationsApi.Notification): string | null {
  const meta = parseMeta(n);
  if (!meta.order_id) return null;
  if (n.kind.startsWith('invoice.')) return `/invoices/${meta.order_id}`;
  return `/orders/${meta.order_id}`;
}

function handleNotificationClick(n: notificationsApi.Notification) {
  markOne(n);
  const link = getNotificationLink(n);
  if (link) {
    router.push(link);
  }
}

onMounted(() => {
  load(true);
});
</script>

<template>
  <div>
    <header class="page-head">
      <h1 class="heading">{{ t('notifications.heading') }}</h1>
      <button
        v-if="notifStore.unread > 0"
        class="btn-mark-all"
        @click="markAll"
      >
        {{ t('notifications.mark_all_read') }}
      </button>
    </header>

    <p v-if="error" class="err">! {{ error }}</p>
    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="notifications.length === 0" class="empty">
      {{ t('notifications.empty') }}
    </p>

    <div v-else class="list">
      <div
        v-for="n in notifications"
        :key="n.id"
        class="row"
        :class="{ unread: !n.read_at_unix }"
        :style="{ cursor: getNotificationLink(n) ? 'pointer' : 'default' }"
        @click="handleNotificationClick(n)"
      >
        <div class="row-main">
          <div class="title">{{ n.title }}</div>
          <div class="meta">{{ fmtDate(n.created_at_unix) }}</div>
        </div>
        
        <div class="body">
          {{ n.body }}
        </div>
      </div>
    </div>

    <button
      v-if="hasMore && !loading"
      class="load-more"
      :disabled="loadingMore"
      @click="load(false)"
    >
      {{ loadingMore ? t('common.loading') : t('notifications.load_more') }}
    </button>
  </div>
</template>

<style scoped>
.page-head { display: flex; justify-content: space-between; align-items: baseline; margin-bottom: 1.5rem; }
.heading { font-size: 1.1rem; color: #e2e5e9; margin: 0; }
.btn-mark-all { background: transparent; border: 1px solid #1e2229; color: #6b7a90; border-radius: 4px; font-family: inherit; font-size: 0.75rem; padding: 0.4rem 0.8rem; cursor: pointer; }
.btn-mark-all:hover { border-color: #00d9a3; color: #00d9a3; }
.muted { color: #6b7a90; }
.err { color: #ff5c5c; font-size: 0.85rem; }
.empty { border: 1px dashed #1e2229; padding: 2rem; text-align: center; color: #6b7a90; border-radius: 4px; }
.list { display: flex; flex-direction: column; gap: 0.375rem; }
.row { border: 1px solid #1e2229; background: rgba(15, 17, 21, 0.85); border-radius: 4px; padding: 1rem; transition: border-color 0.1s, background 0.1s; }
.row:hover { border-color: #2a3039; }
.row.unread { border-left: 3px solid #00d9a3; background: rgba(0, 217, 163, 0.03); }
.row-main { display: flex; justify-content: space-between; align-items: baseline; margin-bottom: 0.25rem; gap: 1rem; }
.title { color: #e2e5e9; font-weight: 600; font-size: 0.95rem; }
.row.unread .title { color: #00d9a3; }
.meta { color: #6b7a90; font-size: 0.75rem; white-space: nowrap; }
.body { color: #8a95a8; font-size: 0.85rem; margin: 0; line-height: 1.5; }
.load-more { background: transparent; border: 1px solid #1e2229; color: #6b7a90; border-radius: 4px; font-family: inherit; font-size: 0.8rem; padding: 0.5rem 1rem; cursor: pointer; margin-top: 1rem; width: 100%; }
.load-more:hover:not(:disabled) { border-color: #00d9a3; color: #00d9a3; }
.load-more:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
