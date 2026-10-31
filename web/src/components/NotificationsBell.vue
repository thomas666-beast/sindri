<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import { useNotificationsStore } from '../stores/notifications';
import { useAuthStore } from '../stores/auth';
import type { Notification } from '../api/notifications';

const { t } = useI18n();
const auth = useAuthStore();
const notif = useNotificationsStore();
const router = useRouter();

const open = ref(false);
const pollTimer = ref<number | null>(null);

const recentUnread = computed(() => {
  return notif.items.filter((n) => !n.read_at_unix).slice(0, 5);
});

async function toggle() {
  open.value = !open.value;
  if (open.value) {
    await notif.load();
  }
}

async function markOne(n: Notification) {
  if (!n.read_at_unix) {
    await notif.markRead(n.id);
  }
}

function showAll() {
  open.value = false;
  router.push({ name: 'notifications' });
}

function handleClickOutside(e: MouseEvent) {
  if (!open.value) return;
  const target = e.target as HTMLElement;
  if (!target.closest('.bell-wrap')) {
    open.value = false;
  }
}

function startPolling() {
  stopPolling();
  pollTimer.value = window.setInterval(() => {
    if (auth.user) notif.refreshUnread();
  }, 30000);
}

function stopPolling() {
  if (pollTimer.value) {
    window.clearInterval(pollTimer.value);
    pollTimer.value = null;
  }
}

watch(() => auth.user, (u) => {
  if (u) {
    notif.refreshUnread();
    startPolling();
  } else {
    notif.reset();
    stopPolling();
  }
}, { immediate: true });

onMounted(() => document.addEventListener('click', handleClickOutside));
onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside);
  stopPolling();
});

const badge = computed(() => {
  if (notif.unread === 0) return '';
  return notif.unread > 99 ? '99+' : String(notif.unread);
});

function fmtDate(unix: number): string {
  const d = new Date(unix * 1000);
  const now = new Date();
  const diffMs = now.getTime() - d.getTime();
  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 1) return t('notifications.just_now');
  if (diffMin < 60) return `${diffMin}m`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h`;
  const diffDay = Math.floor(diffHr / 24);
  if (diffDay < 7) return `${diffDay}d`;
  return d.toISOString().slice(0, 10);
}

// ── NEW: Safely parse metadata (it might be a JSON string) ──
function parseMeta(n: Notification): any {
  if (!n.metadata) return {};
  if (typeof n.metadata === 'string') {
    try { return JSON.parse(n.metadata); } catch { return {}; }
  }
  return n.metadata;
}

function getNotificationLink(n: Notification): string | null {
  const meta = parseMeta(n);
  if (!meta.order_id) return null;
  if (n.kind.startsWith('invoice.')) return `/invoices/${meta.order_id}`;
  return `/orders/${meta.order_id}`;
}

function handleNotificationClick(n: Notification) {
  markOne(n);
  const link = getNotificationLink(n);
  
  // Debug log to check what is happening
  console.log("Clicked notification:", n.kind, "Link:", link, "Metadata:", parseMeta(n));

  if (link) {
    router.push(link);
    open.value = false;
  }
}
</script>

<template>
  <div v-if="auth.user" class="bell-wrap">
    <button class="bell" :class="{ active: open }" @click="toggle">
      <span class="icon">▲</span>
      <span v-if="badge" class="badge">{{ badge }}</span>
    </button>

    <div v-if="open" class="panel">
      <header class="panel-head">
        <span class="panel-title">{{ t('notifications.title') }}</span>
      </header>

      <p v-if="notif.loading" class="muted">{{ t('common.loading') }}</p>
      <p v-else-if="notif.error" class="err">! {{ notif.error }}</p>
      <p v-else-if="recentUnread.length === 0" class="muted empty">
        {{ t('notifications.empty') }}
      </p>

      <ul v-else class="list">
        <li
          v-for="n in recentUnread"
          :key="n.id"
          class="item unread"
          :style="{ cursor: getNotificationLink(n) ? 'pointer' : 'default' }"
          @click="handleNotificationClick(n)"
        >
          <div class="item-head">
            <span class="title">{{ n.title }}</span>
            <span class="when">{{ fmtDate(n.created_at_unix) }}</span>
          </div>
          
          <div class="body">
            {{ n.body }}
          </div>

        </li>
      </ul>

      <div class="panel-foot">
        <button class="show-all" @click="showAll">
          {{ t('notifications.show_all') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* (Your existing bell styles - unchanged) */
.bell-wrap { position: relative; }
.bell { position: relative; background: transparent; border: 1px solid transparent; color: #8a95a8; font-family: inherit; font-size: 0.85rem; cursor: pointer; padding: 0.35rem 0.65rem; line-height: 1.2; transition: color 0.1s, border-color 0.1s; }
.bell:hover, .bell.active { color: #00d9a3; border-color: #1f2633; }
.icon { display: inline-block; font-size: 0.9rem; transform: rotate(180deg); line-height: 1; }
.badge { position: absolute; top: -2px; right: -4px; background: #ff5c5c; color: #0a0e14; font-size: 0.6rem; font-weight: bold; padding: 0.05rem 0.35rem; border-radius: 8px; line-height: 1.3; min-width: 16px; text-align: center; }
.panel { position: absolute; top: calc(100% + 6px); right: 0; width: 340px; background: #11151c; border: 1px solid #1f2633; z-index: 50; display: flex; flex-direction: column; }
.panel-head { padding: 0.75rem 1rem; border-bottom: 1px solid #1f2633; }
.panel-title { color: #00d9a3; font-size: 0.85rem; font-weight: bold; }
.panel-foot { padding: 0.5rem 1rem; border-top: 1px solid #1f2633; text-align: center; }
.show-all { background: transparent; border: none; color: #8a95a8; font-family: inherit; font-size: 0.75rem; cursor: pointer; padding: 0.25rem; }
.show-all:hover { color: #00d9a3; }
.muted { color: #6b7a90; font-size: 0.85rem; padding: 1rem; text-align: center; }
.err { color: #ff5c5c; font-size: 0.85rem; padding: 1rem; }
.empty { font-style: italic; }
.list { list-style: none; padding: 0; margin: 0; max-height: 280px; overflow-y: auto; }
.item { padding: 0.75rem 1rem; border-bottom: 1px solid #0a0e14; cursor: pointer; transition: background 0.1s; border-left: 2px solid #00d9a3; background: rgba(0, 217, 163, 0.03); }
.item:hover { background: #0a0e14; }
.item-head { display: flex; justify-content: space-between; align-items: baseline; gap: 0.5rem; margin-bottom: 0.25rem; }
.title { color: #00d9a3; font-size: 0.85rem; font-weight: bold; }
.when { color: #6b7a90; font-size: 0.7rem; white-space: nowrap; }
.body { color: #8a95a8; font-size: 0.8rem; margin: 0; line-height: 1.4; }
</style>
