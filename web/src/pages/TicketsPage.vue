<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { RouterLink, useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as ticketsApi from '../api/tickets';

const { t } = useI18n();
const router = useRouter();

const tickets = ref<ticketsApi.Ticket[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);

const statusFilter = ref<'' | 'open' | 'answered' | 'closed'>('');

const newOpen = ref(false);
const newSubject = ref('');
const newPriority = ref<'low' | 'normal' | 'high'>('normal');
const newBody = ref('');
const newBusy = ref(false);
const newError = ref<string | null>(null);

const filtered = computed(() => {
  if (!statusFilter.value) return tickets.value;
  return tickets.value.filter((tk) => tk.status === statusFilter.value);
});

async function load() {
  loading.value = true;
  error.value = null;
  try {
    tickets.value = await ticketsApi.listMine(50, 0);
  } catch {
    error.value = t('tickets.load_failed');
  } finally {
    loading.value = false;
  }
}

function openNew() {
  newSubject.value = '';
  newPriority.value = 'normal';
  newBody.value = '';
  newError.value = null;
  newOpen.value = true;
}

function closeNew() {
  newOpen.value = false;
}

async function submitNew() {
  newError.value = null;
  if (newSubject.value.trim().length < 3) {
    newError.value = t('tickets.subject_required');
    return;
  }
  if (newBody.value.trim().length < 3) {
    newError.value = t('tickets.message_required');
    return;
  }
  newBusy.value = true;
  try {
    const tk = await ticketsApi.create({
      subject: newSubject.value.trim(),
      priority: newPriority.value,
    });
    await ticketsApi.addMessage(tk.id, newBody.value.trim());
    closeNew();
    router.push({ name: 'ticket-detail', params: { id: tk.id } });
  } catch (e: any) {
    const body = e.response?.data;
    if (body?.errors) {
      newError.value = Object.values(body.errors).join(', ');
    } else {
      newError.value = body?.error ?? t('tickets.create_failed');
    }
  } finally {
    newBusy.value = false;
  }
}

function statusLabel(s: ticketsApi.Ticket['status']): string {
  switch (s) {
    case 'open': return t('tickets.status_open');
    case 'answered': return t('tickets.status_answered');
    case 'closed': return t('tickets.status_closed');
  }
}

function fmtDate(unix: number): string {
  return new Date(unix * 1000).toISOString().slice(0, 10);
}

onMounted(load);
</script>

<template>
  <div>
    <header class="page-head">
      <h1 class="heading">{{ t('tickets.heading') }}</h1>
      <button class="btn-primary" @click="openNew">
        + {{ t('tickets.new') }}
      </button>
    </header>

    <div class="filters">
      <button class="filter" :class="{ active: statusFilter === '' }" @click="statusFilter = ''">
        {{ t('tickets.filter_all') }}
      </button>
      <button class="filter" :class="{ active: statusFilter === 'open' }" @click="statusFilter = 'open'">
        {{ t('tickets.status_open') }}
      </button>
      <button class="filter" :class="{ active: statusFilter === 'answered' }" @click="statusFilter = 'answered'">
        {{ t('tickets.status_answered') }}
      </button>
      <button class="filter" :class="{ active: statusFilter === 'closed' }" @click="statusFilter = 'closed'">
        {{ t('tickets.status_closed') }}
      </button>
    </div>

    <p v-if="error" class="err">! {{ error }}</p>
    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="filtered.length === 0" class="empty">
      {{ t('tickets.empty') }}
    </p>

    <div v-else class="list">
      <RouterLink
        v-for="tk in filtered"
        :key="tk.id"
        :to="{ name: 'ticket-detail', params: { id: tk.id } }"
        class="row"
      >
        <div class="row-head">
          <span class="subject">{{ tk.subject }}</span>
          <span class="status-badge" :class="tk.status">{{ statusLabel(tk.status) }}</span>
        </div>
        <div class="row-foot">
          <span class="priority" :class="tk.priority">{{ tk.priority }}</span>
          <span class="meta">{{ fmtDate(tk.created_at_unix) }}</span>
        </div>
      </RouterLink>
    </div>

    <div v-if="newOpen" class="modal-bg" @click.self="closeNew">
      <div class="modal">
        <h2 class="modal-title">{{ t('tickets.new') }}</h2>

        <label class="field">
          <span class="label">{{ t('tickets.subject') }}</span>
          <input v-model="newSubject" type="text" maxlength="300" />
        </label>

        <label class="field">
          <span class="label">{{ t('tickets.priority') }}</span>
          <select v-model="newPriority">
            <option value="low">{{ t('tickets.priority_low') }}</option>
            <option value="normal">{{ t('tickets.priority_normal') }}</option>
            <option value="high">{{ t('tickets.priority_high') }}</option>
          </select>
        </label>

        <label class="field">
          <span class="label">{{ t('tickets.first_message') }}</span>
          <textarea v-model="newBody" rows="5" maxlength="10000" />
        </label>

        <p v-if="newError" class="err">! {{ newError }}</p>

        <div class="modal-actions">
          <button class="primary" :disabled="newBusy" @click="submitNew">
            {{ newBusy ? t('common.loading') : `> ${t('tickets.send')}` }}
          </button>
          <button class="ghost" @click="closeNew">{{ t('common.cancel') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 1rem;
}
.heading {
  font-size: 1.1rem;
  color: #e2e5e9;
  margin: 0;
}
.btn-primary {
  background: #00d9a3;
  color: #0a0e14;
  border: none;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.85rem;
  font-weight: 600;
  padding: 0.5rem 0.9rem;
  cursor: pointer;
}
.btn-primary:hover {
  background: #00f5aa;
}
.filters {
  display: flex;
  gap: 0.25rem;
  flex-wrap: wrap;
  margin-bottom: 1rem;
}
.filter {
  background: transparent;
  border: 1px solid #1e2229;
  color: #6b7a90;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.75rem;
  padding: 0.3rem 0.7rem;
  cursor: pointer;
}
.filter:hover {
  color: #e2e5e9;
}
.filter.active {
  color: #00d9a3;
  border-color: #00d9a3;
}
.muted {
  color: #6b7a90;
}
.err {
  color: #ff5c5c;
  font-size: 0.85rem;
}
.empty {
  border: 1px dashed #1e2229;
  padding: 2rem;
  text-align: center;
  color: #6b7a90;
  border-radius: 4px;
}
.list {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}
.row {
  border: 1px solid #1e2229;
  background: rgba(15, 17, 21, 0.85);
  border-radius: 4px;
  padding: 0.75rem 1rem;
  text-decoration: none;
  color: inherit;
  display: block;
}
.row:hover {
  border-color: #2a3039;
}
.row-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 0.375rem;
  gap: 0.5rem;
}
.subject {
  color: #e2e5e9;
  font-weight: 600;
  font-size: 0.95rem;
  word-break: break-word;
}
.status-badge {
  font-size: 0.7rem;
  padding: 0.1rem 0.5rem;
  border-radius: 4px;
  border: 1px solid #1e2229;
  color: #6b7a90;
  white-space: nowrap;
}
.status-badge.open {
  color: #4a9eff;
  border-color: #4a9eff;
}
.status-badge.answered {
  color: #ffb454;
  border-color: #ffb454;
}
.status-badge.closed {
  color: #6b7a90;
}
.row-foot {
  display: flex;
  gap: 0.75rem;
  align-items: baseline;
}
.priority {
  font-size: 0.7rem;
  color: #6b7a90;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.priority.high {
  color: #ff5c5c;
}
.meta {
  color: #3a4150;
  font-size: 0.7rem;
  margin-left: auto;
}
.modal-bg {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.75);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 200;
}
.modal {
  background: #161a20;
  border: 1px solid #1e2229;
  border-radius: 6px;
  padding: 1.5rem;
  width: 500px;
  max-width: 90vw;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
}
.modal-title {
  font-size: 1rem;
  color: #e2e5e9;
  font-weight: 600;
  margin: 0 0 1rem;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
  margin-bottom: 0.75rem;
}
.label {
  color: #6b7a90;
  font-size: 0.7rem;
}
input,
textarea,
select {
  background: #0a0e14;
  border: 1px solid #1e2229;
  color: #c8d3e0;
  padding: 0.5rem 0.75rem;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.9rem;
  resize: vertical;
}
input:focus,
textarea:focus,
select:focus {
  outline: none;
  border-color: #00d9a3;
  box-shadow: 0 0 0 2px rgba(0, 229, 160, 0.25);
}
.modal-actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 1rem;
}
.primary {
  background: #00d9a3;
  color: #0a0e14;
  border: none;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.85rem;
  font-weight: 600;
  padding: 0.5rem 1rem;
  cursor: pointer;
}
.primary:hover:not(:disabled) {
  background: #00f5aa;
}
.primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.ghost {
  background: transparent;
  color: #6b7a90;
  border: 1px solid #1e2229;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.85rem;
  padding: 0.5rem 1rem;
  cursor: pointer;
}
.ghost:hover {
  border-color: #00d9a3;
  color: #00d9a3;
}
</style>
