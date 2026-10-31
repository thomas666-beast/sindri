<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { RouterLink, useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import { useAuthStore } from '../stores/auth';
import { usePanelStore } from '../stores/panel';
import { useChatStore } from '../stores/chatStore';
import NotificationsBell from './NotificationsBell.vue';
import { useBrand } from '../composables/useBrand'

const brand = useBrand()
const { t } = useI18n();
const auth = useAuthStore();
const panel = usePanelStore();
const chatStore = useChatStore();
const router = useRouter();

const emit = defineEmits<{
  (e: 'toggle-sidebar'): void;
}>();

const menuOpen = ref(false);

async function logout() {
  await auth.logout();
  panel.reset();
  router.push({ name: 'login' });
}

function toggleMenu() {
  menuOpen.value = !menuOpen.value;
}

function handleClickOutside(e: MouseEvent) {
  const target = e.target as HTMLElement;
  if (!target.closest('.user-menu')) menuOpen.value = false;
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside);
});

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside);
});
</script>

<template>
  <header class="topbar">
    <button
      v-if="auth.user"
      class="hamburger"
      @click="emit('toggle-sidebar')"
      aria-label="menu"
    >
      <span></span>
      <span></span>
      <span></span>
    </button>

    <RouterLink :to="auth.user ? '/welcome' : '/'" class="brand">
    <span class="brand-mark" aria-hidden="true">
      <svg viewBox="0 0 64 64" width="20" height="20">
        <path d="M16 22 L28 32 L16 42"
              fill="none" stroke="currentColor" stroke-width="4.5"
              stroke-linecap="round" stroke-linejoin="round"/>
        <line x1="34" y1="42" x2="48" y2="42"
              stroke="currentColor" stroke-width="4.5" stroke-linecap="round"/>
      </svg>
    </span>
    <span class="brand-name">{{ brand.name }}</span>
    </RouterLink>

    <span v-if="panel.mode === 'admin'" class="admin-badge">admin mode</span>

    <div class="spacer"></div>

    <template v-if="auth.user">
      <!-- CHAT BADGE -->
      <RouterLink to="/chat" class="chat-badge" v-if="chatStore.unreadMessages > 0">
        <span class="icon">💬</span>
        <span class="badge">{{ chatStore.unreadMessages > 99 ? '99+' : chatStore.unreadMessages }}</span>
      </RouterLink>

      <NotificationsBell />

      <div class="user-menu">
        <button class="user-btn" :class="{ active: menuOpen }" @click="toggleMenu">
          <span class="user">@{{ auth.user.username }}</span>
          <span class="chevron">▾</span>
        </button>

        <ul v-if="menuOpen" class="dropdown">
          <li>
            <RouterLink to="/profile" class="dropdown-item" @click="menuOpen = false">
              {{ t('nav.profile') }}
            </RouterLink>
          </li>
          <li>
            <button class="dropdown-item danger" @click="logout">
              {{ t('common.logout') }}
            </button>
          </li>
        </ul>
      </div>
    </template>

    <template v-else>
      <RouterLink to="/login" class="topbar-link">{{ t('common.login') }}</RouterLink>
      <RouterLink to="/register" class="topbar-link primary">
        {{ t('common.register') }}
      </RouterLink>
    </template>
  </header>
</template>

<style scoped>
.topbar {
  position: relative;
  z-index: 100;
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: 0 var(--space-5);
  height: 52px;
  background: #0a0e14;
  border-bottom: 1px solid var(--border);
  overflow: visible;
}
.hamburger {
  display: none;
  flex-direction: column;
  justify-content: center;
  gap: 3px;
  width: 28px;
  height: 28px;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  cursor: pointer;
  padding: 0;
  transition: border-color 0.1s;
}
.hamburger:hover {
  border-color: var(--accent);
}
.hamburger span {
  display: block;
  width: 12px;
  height: 1.5px;
  margin: 0 auto;
  background: var(--text-secondary);
  transition: background 0.1s;
}
.hamburger:hover span {
  background: var(--accent);
}
.brand {
  display: inline-flex;
  align-items: baseline;
  gap: var(--space-2);
  text-decoration: none;
  font-weight: 600;
  font-size: 0.95rem;
  letter-spacing: -0.02em;
  color: var(--text);
}
.brand-mark {
  display: inline-flex;
  align-items: center;
  color: var(--accent);
  transition: color 0.1s;
}
.brand:hover .brand-mark {
  color: var(--accent-hover);
}
.brand-name {
  color: var(--text);
}
.brand:hover .brand-name {
  color: var(--accent);
}
.admin-badge {
  background: rgba(255, 180, 84, 0.1);
  color: #ffb454;
  border: 1px solid #ffb454;
  border-radius: var(--radius-sm);
  padding: 0.15rem 0.5rem;
  font-size: 0.65rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.spacer {
  flex: 1;
}

.chat-badge {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #8a95a8;
  text-decoration: none;
  padding: 0.35rem;
  transition: color 0.1s;
}
.chat-badge:hover {
  color: #00d9a3;
}
.chat-badge .icon {
  font-size: 1.1rem;
  line-height: 1;
}
.chat-badge .badge {
  position: absolute;
  top: -2px;
  right: -4px;
  background: #ff5c4e;
  color: #0a0e14;
  font-size: 0.6rem;
  font-weight: bold;
  padding: 0.05rem 0.35rem;
  border-radius: 8px;
  min-width: 16px;
  text-align: center;
}

.user-menu {
  position: relative;
}
.user-btn {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  background: transparent;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  color: var(--text-secondary);
  font-size: 0.85rem;
  cursor: pointer;
  transition: color 0.1s, background 0.1s, border-color 0.1s;
}
.user-btn:hover,
.user-btn.active {
  color: var(--text);
  background: var(--surface);
  border-color: var(--border);
}
.user-handle {
  color: inherit;
}
.chevron {
  font-size: 0.65rem;
  opacity: 0.6;
}
.dropdown {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  min-width: 180px;
  background: var(--surface-elevated);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  list-style: none;
  padding: var(--space-1) 0;
  margin: 0;
  z-index: 200;
  box-shadow: var(--shadow-dropdown);
}
.dropdown-item {
  display: block;
  width: 100%;
  text-align: left;
  padding: var(--space-2) var(--space-4);
  color: var(--text-secondary);
  font-family: inherit;
  font-size: 0.85rem;
  background: transparent;
  border: none;
  cursor: pointer;
  transition: color 0.1s, background 0.1s;
}
.dropdown-item:hover {
  color: var(--text);
  background: var(--surface-hover);
}
.dropdown-item.danger:hover {
  color: var(--danger);
}
.topbar-link {
  color: var(--text-secondary);
  font-size: 0.85rem;
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-sm);
  transition: color 0.1s, background 0.1s;
}
.topbar-link:hover {
  color: var(--text);
  background: var(--surface);
}
.topbar-link.primary {
  color: var(--accent);
  border: 1px solid var(--accent);
}
.topbar-link.primary:hover {
  color: var(--accent-hover);
  border-color: var(--accent-hover);
  background: var(--accent-dim);
}
@media (max-width: 900px) {
  .hamburger {
    display: flex;
  }
  .topbar {
    padding: 0 var(--space-4);
  }
}
</style>
