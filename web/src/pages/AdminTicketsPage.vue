<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { RouterLink } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as ticketsApi from '../api/tickets';

const { t } = useI18n();

const tickets = ref<ticketsApi.AdminTicket[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);
const statusFilter = ref<'' | 'open' | 'answered' | 'closed'>('');

async function load() {
  loading.value = true;
  error.value = null;
  try {
    tickets.value = await ticketsApi.adminListTickets(statusFilter.value, 100, 0);
  } catch {
    error.value = t('admin.load_failed');
  } finally {
    loading.value = false;
  }
}

function setFilter(s: '' | 'open' | 'answered' | 'closed') {
  statusFilter.value = s;
  load();
}

function fmtDate(unix: number): string {
  return new Date(unix * 1000).toISOString().slice(0, 10);
}

onMounted(load);
</script>

<template>
  <div>
    <h1 class="heading">{{ t('admin.tickets_heading') }}</h1>

    <div class="filters">
      <button class="filter" :class="{ active: statusFilter === '' }" @click="setFilter('')">all</button>
      <button class="filter" :class="{ active: statusFilter === 'open' }" @click="setFilter('open')">open</button>
      <button class="filter" :class="{ active: statusFilter === 'answered' }" @click="setFilter('answered')">answered</button>
      <button class="filter" :class="{ active: statusFilter === 'closed' }" @click="setFilter('closed')">closed</button>
    </div>

    <p v-if="error" class="err">! {{ error }}</p>
    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="tickets.length === 0" class="empty">{{ t('admin.no_tickets') }}</p>

    <div v-else class="list">
      <RouterLink
        v-for="tk in tickets"
        :key="tk.id"
        :to="{ name: 'admin-ticket', params: { id: tk.id } }"
        class="row"
      >
        <div class="row-head">
          <span class="subject">{{ tk.subject }}</span>
          <span class="status-badge" :class="tk.status">{{ tk.status }}</span>
        </div>
        <div class="row-foot">
          <span class="priority" :class="tk.priority">{{ tk.priority }}</span>
          <span class="meta">from {{ tk.user_id.slice(0, 8) }} · {{ fmtDate(tk.created_at_unix) }}</span>
        </div>
      </RouterLink>
    </div>
  </div>
</template>

<style scoped>
/* (Copy the styles from the AdminTicketsPage we made previously, or use these simple ones) */
.heading { font-size: 1.1rem; color: #e2e5e9; margin: 0 0 1rem; }
.filters { display: flex; gap: 0.25rem; margin-bottom: 1rem; }
.filter { background: transparent; border: 1px solid #1e2229; color: #6b7a90; border-radius: 4px; font-size: 0.75rem; padding: 0.3rem 0.7rem; cursor: pointer; }
.filter.active { color: #00d9a3; border-color: #00d9a3; }
.muted { color: #6b7a90; }
.err { color: #ff5c5c; font-size: 0.85rem; }
.empty { border: 1px dashed #1e2229; padding: 2rem; text-align: center; color: #6b7a90; border-radius: 4px; }
.list { display: flex; flex-direction: column; gap: 0.375rem; }
.row { border: 1px solid #1e2229; background: rgba(15, 17, 21, 0.85); border-radius: 4px; padding: 0.75rem 1rem; text-decoration: none; color: inherit; }
.row:hover { border-color: #2a3039; }
.row-head { display: flex; justify-content: space-between; margin-bottom: 0.375rem; }
.subject { color: #e2e5e9; font-weight: 600; }
.status-badge { font-size: 0.7rem; padding: 0.1rem 0.5rem; border-radius: 4px; border: 1px solid #1e2229; color: #6b7a90; }
.status-badge.open { color: #4a9eff; border-color: #4a9eff; }
.status-badge.answered { color: #ffb454; border-color: #ffb454; }
.row-foot { display: flex; gap: 0.75rem; font-size: 0.7rem; color: #6b7a90; }
.priority.high { color: #ff5c5c; }
</style>
