<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';
import { RouterLink, useRoute } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as ticketsApi from '../api/tickets';
import { useAuthStore } from '../stores/auth';

const { t } = useI18n();
const route = useRoute();
const auth = useAuthStore();

const ticket = ref<ticketsApi.Ticket | null>(null);
const messages = ref<ticketsApi.TicketMessage[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);
const sending = ref(false);
const sendError = ref<string | null>(null);
const replyBody = ref('');

async function load() {
  loading.value = true;
  error.value = null;
  try {
    ticket.value = await ticketsApi.get(route.params.id as string);
    messages.value = await ticketsApi.listMessages(route.params.id as string);
  } catch (e: any) {
    error.value = e.response?.data?.error ?? t('tickets.load_failed');
  } finally {
    loading.value = false;
  }
}

async function sendReply() {
  if (!ticket.value) return;
  sendError.value = null;
  const body = replyBody.value.trim();
  if (body.length < 1) {
    sendError.value = t('tickets.message_required');
    return;
  }
  sending.value = true;
  try {
    const msg = await ticketsApi.addMessage(ticket.value.id, body);
    messages.value = [...messages.value, msg];
    replyBody.value = '';
    ticket.value = await ticketsApi.get(ticket.value.id);
  } catch (e: any) {
    sendError.value = e.response?.data?.error ?? t('tickets.send_failed');
  } finally {
    sending.value = false;
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
  return new Date(unix * 1000).toISOString().replace('T', ' ').slice(0, 19);
}

watch(
  () => route.params.id,
  () => load()
);

onMounted(load);
</script>

<template>
  <div>
    <RouterLink to="/tickets" class="back">{{ t('tickets.back') }}</RouterLink>

    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="err">! {{ error }}</p>

    <template v-else-if="ticket">
      <article class="detail">
        <header class="head">
          <h1 class="heading">{{ ticket.subject }}</h1>
          <span class="status-badge" :class="ticket.status">{{ statusLabel(ticket.status) }}</span>
        </header>

        <div class="meta">
          <span class="priority" :class="ticket.priority">{{ ticket.priority }}</span>
          <span class="dim">·</span>
          <span>{{ t('tickets.created') }} {{ fmtDate(ticket.created_at_unix) }}</span>
        </div>

        <div class="thread">
          <div
            v-for="m in messages"
            :key="m.id"
            class="msg"
            :class="{ staff: m.is_staff, mine: m.author_id === auth.user?.id && !m.is_staff }"
          >
            <div class="msg-head">
              <span class="author">
                {{ m.is_staff ? t('tickets.staff') : t('tickets.you') }}
              </span>
              <span class="when">{{ fmtDate(m.created_at_unix) }}</span>
            </div>
            <div class="msg-body">{{ m.body }}</div>
          </div>
        </div>

        <div v-if="ticket.status !== 'closed'" class="composer">
          <textarea
            v-model="replyBody"
            rows="4"
            maxlength="10000"
            :placeholder="t('tickets.reply_placeholder')"
          />
          <p v-if="sendError" class="err">! {{ sendError }}</p>
          <div class="actions">
            <button class="primary" :disabled="sending" @click="sendReply">
              {{ sending ? t('common.loading') : `> ${t('tickets.send')}` }}
            </button>
          </div>
        </div>
        <p v-else class="closed-notice">{{ t('tickets.closed_notice') }}</p>
      </article>
    </template>
  </div>
</template>

<style scoped>
.back {
  display: inline-block;
  color: #6b7a90;
  text-decoration: none;
  font-size: 0.85rem;
  margin-bottom: 1rem;
}
.back:hover {
  color: #00d9a3;
}
.muted {
  color: #6b7a90;
}
.err {
  color: #ff5c5c;
  font-size: 0.85rem;
}
.detail {
  border: 1px solid #1e2229;
  background: rgba(15, 17, 21, 0.85);
  border-radius: 6px;
  padding: 1.5rem;
}
.head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 1rem;
  margin-bottom: 0.75rem;
}
.heading {
  font-size: 1.2rem;
  color: #e2e5e9;
  margin: 0;
}
.status-badge {
  font-size: 0.75rem;
  padding: 0.15rem 0.6rem;
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
.meta {
  display: flex;
  gap: 0.5rem;
  align-items: baseline;
  color: #6b7a90;
  font-size: 0.75rem;
  margin-bottom: 1.5rem;
}
.priority {
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.priority.high {
  color: #ff5c5c;
}
.dim {
  color: #3a4150;
}
.thread {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin-bottom: 1.5rem;
}
.msg {
  border-left: 2px solid #1e2229;
  padding: 0.5rem 0.75rem;
  background: #0a0e14;
  border-radius: 0 4px 4px 0;
}
.msg.mine {
  border-left-color: #00d9a3;
}
.msg.staff {
  border-left-color: #ffb454;
}
.msg-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 0.25rem;
  gap: 0.5rem;
}
.author {
  font-size: 0.75rem;
  color: #6b7a90;
}
.msg.mine .author {
  color: #00d9a3;
}
.msg.staff .author {
  color: #ffb454;
}
.when {
  font-size: 0.7rem;
  color: #3a4150;
}
.msg-body {
  color: #e2e5e9;
  font-size: 0.9rem;
  white-space: pre-wrap;
  line-height: 1.55;
}
.composer {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  border-top: 1px solid #1e2229;
  padding-top: 1rem;
}
textarea {
  background: #0a0e14;
  border: 1px solid #1e2229;
  color: #c8d3e0;
  padding: 0.5rem 0.75rem;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.9rem;
  resize: vertical;
}
textarea:focus {
  outline: none;
  border-color: #00d9a3;
  box-shadow: 0 0 0 2px rgba(0, 229, 160, 0.25);
}
.actions {
  display: flex;
  gap: 0.5rem;
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
.closed-notice {
  color: #6b7a90;
  font-size: 0.85rem;
  font-style: italic;
  text-align: center;
  padding: 1rem;
  border-top: 1px solid #1e2229;
}
@media (max-width: 600px) {
  .meta-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.25rem;
  }
  .meta-row dd {
    text-align: left;
  }
  .price-block {
    flex-direction: column;
    gap: 0.25rem;
  }
}
</style>
