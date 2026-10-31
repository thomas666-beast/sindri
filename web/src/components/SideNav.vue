<script setup lang="ts">
import { computed } from 'vue';
import { RouterLink, useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import { useAuthStore } from '../stores/auth';
import { usePanelStore } from '../stores/panel';

const props = defineProps<{
  open: boolean;
}>();

const emit = defineEmits<{
  (e: 'close'): void;
}>();

const { t } = useI18n();
const auth = useAuthStore();
const panel = usePanelStore();
const router = useRouter();

interface Section {
  heading: string;
  items: { label: string; to: string; admin?: boolean }[];
}

const userSections = computed<Section[]>(() => {
  if (!auth.user) return [];
  return [
    {
      heading: t('nav.home'),
      items: [{ label: t('nav.welcome'), to: '/welcome' }],
    },
    {
      heading: t('nav.discover'),
      items: [{ label: t('nav.geeks'), to: '/geeks' }],
    },
    {
      heading: t('nav.wallet'),
      items: [{ label: t('nav.wallet'), to: '/wallet' }],
    },
    {
      heading: t('nav.market'),
      items: [
        { label: t('dashboard.heading'), to: '/dashboard' }, 
        { label: t('nav.products'), to: '/products' },
        { label: t('nav.my_products'), to: '/my-products' },
        { label: t('nav.orders'), to: '/orders' },
        { label: t('nav.invoices'), to: '/invoices' },
      ],
    },
    {
      heading: t('nav.chat'),
      items: [{ label: t('nav.chat'), to: '/chat' }],
    },
    {
      heading: t('nav.support'),
      items: [{ label: t('nav.tickets'), to: '/tickets' }],
    },
    {
      heading: t('nav.profile'),
      items: [{ label: t('nav.profile'), to: '/profile' }],
    },
  ];
});

const adminSections = computed<Section[]>(() => {
  if (!auth.user?.is_admin) return [];
  return [
    {
      heading: t('nav.admin'),
      items: [
        { label: t('nav.admin'), to: '/admin', admin: true },
        { label: t('nav.users_admin'), to: '/admin/users', admin: true },
        { label: t('nav.order_look_up_admin'), to: '/admin/lookup', admin: true }, // <-- ADD THIS
        { label: t('nav.disputes_admin'), to: '/admin/orders', admin: true },
        { label: t('nav.withdrawals_admin'), to: '/admin/withdrawals', admin: true },
        { label: t('nav.tickets_admin'), to: '/admin/tickets', admin: true },
        { label: t('nav.transactions_admin'), to: '/admin/transactions', admin: true },
      ],
    },
  ];
});

const sections = computed<Section[]>(() => {
  if (!auth.user) return [];
  return panel.mode === 'admin' ? adminSections.value : userSections.value;
});

function switchToAdmin() {
  panel.switchToAdmin();
  router.push({ name: 'admin' });
  emit('close');
}

function switchToUser() {
  panel.switchToUser();
  router.push({ name: 'wallet' });
  emit('close');
}
</script>

<template>
  <aside class="sidebar" :class="{ open: props.open }">
    <nav class="sections">
      <section v-for="section in sections" :key="section.heading" class="section">
        <h3 class="section-heading">{{ section.heading }}</h3>
        <ul class="items">
          <li v-for="item in section.items" :key="item.to">
            <RouterLink
              :to="item.to"
              class="item"
              :class="{ 'admin-item': item.admin }"
              active-class="active"
              @click="emit('close')"
            >
              {{ item.label }}
            </RouterLink>
          </li>
        </ul>
      </section>
    </nav>

    <div v-if="auth.user?.is_admin" class="panel-switch">
      <button v-if="panel.mode === 'user'" class="switch-btn admin" @click="switchToAdmin">
        {{ t('nav.switch_to_admin') }}
      </button>
      <button v-else class="switch-btn user" @click="switchToUser">
        {{ t('nav.switch_to_user') }}
      </button>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  background: #0a0e14;
  border-right: 1px solid var(--border);
  overflow-y: auto;
  overflow-x: hidden;
  padding: var(--space-4) var(--space-2);
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.sections {
  display: flex;
  flex-direction: column;
  gap: var(--space-5);
  flex: 1;
}
.section-heading {
  color: var(--text-tertiary);
  font-size: 0.65rem;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  font-weight: 500;
  margin: 0 0 var(--space-2) var(--space-3);
}
.items {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.item {
  display: block;
  color: var(--text-secondary);
  padding: 6px var(--space-3);
  text-decoration: none;
  font-size: 0.85rem;
  border-radius: var(--radius-sm);
  transition: color 0.1s, background 0.1s;
  position: relative;
}
.item:hover {
  color: var(--text);
  background: var(--surface);
}
.item.active {
  color: var(--text);
  background: var(--surface);
}
.item.active::before {
  content: '';
  position: absolute;
  left: 0;
  top: 6px;
  bottom: 6px;
  width: 2px;
  background: var(--accent);
  border-radius: 1px;
}
.item.admin-item {
  color: var(--warning);
}
.item.admin-item:hover {
  color: var(--warning);
  background: rgba(255, 180, 84, 0.05);
}
.item.admin-item.active {
  color: var(--warning);
  background: rgba(255, 180, 84, 0.08);
}
.item.admin-item.active::before {
  background: var(--warning);
}
.panel-switch {
  padding-top: var(--space-3);
  border-top: 1px solid var(--border);
  margin-top: var(--space-3);
}
.switch-btn {
  width: 100%;
  border-radius: var(--radius-sm);
  font-family: inherit;
  font-size: 0.8rem;
  font-weight: 600;
  padding: 0.5rem;
  cursor: pointer;
  border: 1px solid transparent;
}
.switch-btn.admin {
  background: rgba(255, 180, 84, 0.1);
  color: var(--warning);
  border-color: var(--warning);
}
.switch-btn.admin:hover {
  background: rgba(255, 180, 84, 0.15);
}
.switch-btn.user {
  background: rgba(0, 229, 160, 0.1);
  color: var(--accent);
  border-color: var(--accent);
}
.switch-btn.user:hover {
  background: rgba(0, 229, 160, 0.15);
}
@media (max-width: 900px) {
  .sidebar {
    position: fixed;
    top: 52px;
    left: 0;
    width: 216px;
    height: calc(100vh - 52px);
    z-index: 15;
    transform: translateX(-100%);
    transition: transform 0.18s ease;
    box-shadow: 4px 0 24px rgba(0, 0, 0, 0.7);
  }
  .sidebar.open {
    transform: translateX(0);
  }
}
</style>
