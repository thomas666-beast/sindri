<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useAuthStore } from './stores/auth';
import { useNotificationsStore } from './stores/notifications';
import { useChatStore } from './stores/chatStore';
import { useChatSocket } from './lib/ws';
import MatrixRain from './components/MatrixRain.vue';
import TopBar from './components/TopBar.vue';
import SideNav from './components/SideNav.vue';
import { useRoute, useRouter } from 'vue-router';

const route = useRoute(); 
const router = useRouter();
const auth = useAuthStore();
const notifStore = useNotificationsStore();
const chatStore = useChatStore();
const socket = useChatSocket();

// This key is only for the inner page content
const pageKey = ref(0);

// GLOBAL LISTENER
socket.onMessage((msg) => {
  if (msg.type === 'notification' && msg.notification) {
    notifStore.receiveNotification(msg.notification);
    // MAGIC: If we are on the same page, silently refresh it
    pageKey.value++;
  }
  if (msg.type === 'chat.unread') {
    if (route.name !== 'chat') {
      chatStore.incrementUnread();
    }
  }
});

onMounted(() => {
  if (auth.user) {
    chatStore.refreshUnread();
  }
});

// --- THE GLOBAL SILENT REFRESH (FOR CLICKS) ---
router.beforeEach((to, from) => {
  if (to.path === from.path && to.name === from.name) {
    // If clicking the same link, bump the key to refresh the page content
    pageKey.value++;
    // Abort the default router navigation
    return false;
  }
});
</script>

<template>
  <MatrixRain v-if="!auth.user" />
  <div class="app-shell" :class="{ 'with-sidebar': !!auth.user }">
    <TopBar />
    <SideNav v-if="auth.user" :open="false" @close="() => {}" />
    <main class="main">
      <div class="content-scroll">
        <div class="content">
          <!-- THE FIX: Wrap the RouterView in a keyed div -->
          <div :key="pageKey" class="page-wrapper">
            <RouterView />
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: 1fr;
  grid-template-rows: 52px 1fr;
  grid-template-areas: 'topbar' 'main';
  height: 100vh;
  overflow: hidden;
}
.app-shell.with-sidebar {
  grid-template-columns: 216px 1fr;
  grid-template-areas: 'topbar topbar' 'sidebar main';
}
.app-shell > :deep(.topbar) { grid-area: topbar; }
.app-shell > :deep(.sidebar) { grid-area: sidebar; }
.main {
  grid-area: main;
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}
.content-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 32px;
  position: relative;
  z-index: 2;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.content {
  background: rgba(10, 12, 16, 0.72);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 32px;
  flex: 1;
  display: flex;
  flex-direction: column;
}

/* THE WRAPPER FIX */
.page-wrapper {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0; /* CRITICAL: Allows inner scrolling */
  width: 100%;
}

/* TARGET CHAT PAGE SPECIFICALLY */
.content:has(.chat) {
  padding: 0;
  overflow: hidden;
  height: 100%;
}
.content:has(.chat) .page-wrapper {
  overflow: hidden;
  height: 100%;
}

@media (max-width: 900px) {
  .app-shell.with-sidebar {
    grid-template-columns: 1fr;
    grid-template-areas: 'topbar' 'main';
  }
  .content-scroll { padding: 1rem; }
  .content { padding: 1.25rem; }
}
</style>

