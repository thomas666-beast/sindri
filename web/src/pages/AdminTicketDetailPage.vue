<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { RouterLink, useRoute } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as ticketsApi from '../api/tickets';

const { t } = useI18n();
const route = useRoute();

const ticket = ref<ticketsApi.Ticket | null>(null);
const messages = ref<ticketsApi.TicketMessage[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);
const replyBody = ref('');
const sending = ref(false);
const actionError = ref<string | null>(null);

async function load() {
  loading.value = true;
  error.value = null;
  actionError.value = null;
  try {
    const id = route.params.id as string;
    ticket.value = await ticketsApi.get(id);
    messages.value = await ticketsApi.listMessages(id);
  } catch (e: any) {
    error.value = e.response?.data?.error ?? t('tickets.load_failed');
  } finally {
    loading.value = false;
  }
}

async function sendReply() {
  if (!ticket.value) return;
  const body = replyBody.value.trim();
  if (!body) return;
  
  sending.value = true;
  actionError.value = null; // clear previous errors
  
  try {
    const msg = await ticketsApi.addMessage(ticket.value.id, body);
    messages.value.push(msg);
    replyBody.value = '';
    // Backend automatically sets status to 'answered' when staff replies
    ticket.value.status = 'answered';
  } catch (e: any) {
    actionError.value = e.response?.data?.error ?? 'failed to send reply';
  } finally {
    sending.value = false;
  }
}

async function setStatus(s: 'open' | 'answered' | 'closed') {
  if (!ticket.value) return;
  actionError.value = null;
  
  try {
    ticket.value = await ticketsApi.adminSetStatus(ticket.value.id, s);
  } catch (e: any) {
    actionError.value = e.response?.data?.error ?? 'failed to change status';
  }
}

const isClosed = computed(() => ticket.value?.status === 'closed');

function fmtDate(unix: number): string {
  return new Date(unix * 1000).toISOString().replace('T', ' ').slice(0, 19);
}

watch(() => route.params.id, () => load());
onMounted(load);
</script>

<template>
  <div>
    <RouterLink to="/admin/tickets" class="back">← back to admin tickets</RouterLink>

    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="err">! {{ error }}</p>

    <template v-else-if="ticket">
      <article class="detail">
        <header class="head">
          <div>
            <h1 class="heading">{{ ticket.subject }}</h1>
            <div class="meta">
              Ticket ID: {{ ticket.id }} · from {{ ticket.user_id.slice(0, 8) }} · {{ ticket.priority }} priority
            </div>
          </div>
          <span class="status-badge" :class="ticket.status">{{ ticket.status }}</span>
        </header>

        <!-- ERROR BANNER -->
        <p v-if="actionError" class="err" style="margin-bottom: 1rem;">! {{ actionError }}</p>

        <!-- Admin Controls -->
        <div class="admin-controls">
          <button 
            :disabled="ticket.status === 'open'" 
            class="btn-sm" 
            @click="setStatus('open')"
          >
            mark open
          </button>
          <button 
            :disabled="ticket.status === 'answered'" 
            class="btn-sm" 
            @click="setStatus('answered')"
          >
            mark answered
          </button>
          <button 
            :disabled="ticket.status === 'closed'" 
            class="btn-sm danger" 
            @click="setStatus('closed')"
          >
            close ticket
          </button>
        </div>

        <div class="thread">
          <div
            v-for="m in messages"
            :key="m.id"
            class="msg"
            :class="{ staff: m.is_staff }"
          >
            <div class="msg-head">
              <span class="author">{{ m.is_staff ? 'You (staff)' : 'User' }}</span>
              <span class="when">{{ fmtDate(m.created_at_unix) }}</span>
            </div>
            <div class="msg-body">{{ m.body }}</div>
          </div>
        </div>

        <div v-if="!isClosed" class="composer">
          <textarea
            v-model="replyBody"
            rows="3"
            placeholder="write a reply as staff…"
          />
          <button class="primary" :disabled="sending || !replyBody.trim()" @click="sendReply">
            {{ sending ? t('common.loading') : `> send reply` }}
          </button>
        </div>
        <p v-else class="closed-notice">this ticket is closed. change status to 'open' or 'answered' to reply.</p>
      </article>
    </template>
  </div>
</template>

<style scoped>
.back { display: inline-block; color: #6b7a90; text-decoration: none; font-size: 0.85rem; margin-bottom: 1rem; }
.back:hover { color: #00d9a3; }
.muted { color: #6b7a90; }
.err { color: #ff5c5c; font-size: 0.85rem; }
.detail { border: 1px solid #1e2229; background: rgba(15, 17, 21, 0.85); border-radius: 6px; padding: 1.5rem; }
.head { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 1rem; }
.heading { font-size: 1.2rem; color: #e2e5e9; margin: 0 0 0.25rem; }
.meta { color: #6b7a90; font-size: 0.75rem; font-family: monospace; }
.status-badge { font-size: 0.75rem; padding: 0.15rem 0.6rem; border-radius: 4px; border: 1px solid #1e2229; color: #6b7a90; text-transform: uppercase; }
.status-badge.open { color: #4a9eff; border-color: #4a9eff; }
.status-badge.answered { color: #ffb454; border-color: #ffb454; }
.admin-controls { display: flex; gap: 0.5rem; margin-bottom: 1.5rem; padding-bottom: 1rem; border-bottom: 1px solid #1e2229; }
.btn-sm { background: transparent; border: 1px solid #1e2229; color: #6b7a90; padding: 0.25rem 0.75rem; border-radius: 4px; font-size: 0.75rem; cursor: pointer; }
.btn-sm:hover:not(:disabled) { border-color: #00d9a3; color: #00d9a3; }
.btn-sm.danger:hover:not(:disabled) { border-color: #ff5c5c; color: #ff5c5c; }
.btn-sm:disabled { opacity: 0.3; cursor: not-allowed; }
.thread { display: flex; flex-direction: column; gap: 0.75rem; margin-bottom: 1.5rem; }
.msg { border-left: 2px solid #1e2229; padding: 0.5rem 0.75rem; background: #0a0e14; border-radius: 0 4px 4px 0; }
.msg.staff { border-left-color: #ffb454; background: rgba(255, 180, 84, 0.05); }
.msg-head { display: flex; justify-content: space-between; font-size: 0.75rem; color: #6b7a90; margin-bottom: 0.25rem; }
.msg.staff .author { color: #ffb454; font-weight: bold; }
.msg-body { color: #e2e5e9; font-size: 0.9rem; white-space: pre-wrap; }
.composer { display: flex; flex-direction: column; gap: 0.5rem; }
textarea { background: #0a0e14; border: 1px solid #1e2229; color: #c8d3e0; padding: 0.5rem; border-radius: 4px; font-family: inherit; font-size: 0.9rem; resize: vertical; }
textarea:focus { outline: none; border-color: #00d9a3; }
.primary { background: #00d9a3; color: #0a0e14; border: none; padding: 0.5rem 1rem; border-radius: 4px; font-weight: bold; cursor: pointer; align-self: flex-start; }
.primary:disabled { opacity: 0.5; cursor: not-allowed; }
.closed-notice { color: #6b7a90; font-style: italic; text-align: center; padding: 1rem; }
</style>
