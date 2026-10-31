<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import * as chatApi from '../api/chatApi';
import { useChatSocket } from '../lib/ws';
import { useAuthStore } from '../stores/auth';
import { useRoute } from 'vue-router';
import UserSearchInput from '../components/UserSearchInput.vue';
import { useChatStore } from '../stores/chatStore';
import { getErrorMessage } from '../api';

const chatStore = useChatStore();
const { t } = useI18n();
const auth = useAuthStore();
const socket = useChatSocket();
const route = useRoute();
const messageInputRef = ref<HTMLInputElement | null>(null);

const conversations = ref<chatApi.Conversation[]>([]);
const activeId = ref<string>('');
const messages = ref<chatApi.Message[]>([]);
const nextCursor = ref<string>('');
const hasMore = ref(false);
const loadingConvs = ref(true);
const loadingMessages = ref(false);
const loadingMore = ref(false);
const error = ref<string | null>(null);
const sendBody = ref('');
const messagesEl = ref<HTMLElement | null>(null);
const fileInput = ref<HTMLInputElement | null>(null);
const typingUsers = ref<Set<string>>(new Set());
const downloadError = ref<string | null>(null);

const dmModal = ref(false);
const dmUserId = ref('');
const groupModal = ref(false);
const groupTitle = ref('');
const groupMembers = ref('');
const modalBusy = ref(false);
const modalError = ref<string | null>(null);

const localTyping = ref(false);
let typingTimer: number | null = null;

function notifyTyping() {
  if (!activeId.value) return;
  if (!localTyping.value) {
    localTyping.value = true;
    socket.sendTyping(activeId.value, true);
  }
  if (typingTimer) window.clearTimeout(typingTimer);
  typingTimer = window.setTimeout(() => {
    localTyping.value = false;
    socket.sendTyping(activeId.value, false);
  }, 2000);
}

function syncGlobalUnread() {
  let total = 0;
  for (const c of conversations.value) {
    if (c.id === activeId.value) continue;
    total += c.unread_count;
  }
  chatStore.unreadMessages = total;
}

async function loadConversations(silent = false) {
  if (!silent) loadingConvs.value = true;
  error.value = null;
  try {
    conversations.value = await chatApi.listConversations();
    
    if (activeId.value) {
      const activeConv = conversations.value.find(c => c.id === activeId.value);
      if (activeConv) activeConv.unread_count = 0;
    }

    syncGlobalUnread();
    
    const openId = route.query.open as string | undefined;
    if (openId && !activeId.value) {
      const found = conversations.value.find((c) => c.id === openId);
      if (found) await openConversation(openId);
    }
  } catch (e: any) {
    error.value = getErrorMessage(e);
  } finally {
    if (!silent) loadingConvs.value = false;
  }
}

async function openConversation(id: string) {
  if (activeId.value === id) return;
  if (activeId.value) socket.unsubscribe(activeId.value);
  
  activeId.value = id;
  socket.subscribe(id); 

  const conv = conversations.value.find(c => c.id === id);
  if (conv) {
    conv.unread_count = 0;
    syncGlobalUnread();
  }
  
  messages.value = [];
  nextCursor.value = '';
  hasMore.value = false;
  loadingMessages.value = true;
  typingUsers.value = new Set();
  downloadError.value = null; // Reset error on chat switch

  try {
    const page = await chatApi.listMessages(id);
    messages.value = [...page.messages].reverse();
    nextCursor.value = page.next_cursor ?? '';
    hasMore.value = page.has_more;

    await nextTick();
    scrollToBottom();
    
    chatApi.markRead(id).catch(() => {});
    await nextTick();
    messageInputRef.value?.focus();
  } catch (e) {
    error.value = t('chat.load_failed');
  } finally {
    loadingMessages.value = false;
  }
}

async function loadOlder() {
  if (!hasMore.value || !nextCursor.value || loadingMore.value) return;
  loadingMore.value = true;
  const el = messagesEl.value;
  const prevHeight = el?.scrollHeight ?? 0;
  try {
    const page = await chatApi.listMessages(activeId.value, nextCursor.value, 50);
    messages.value = [...page.messages].reverse().concat(messages.value);
    nextCursor.value = page.next_cursor ?? '';
    hasMore.value = page.has_more;
    await nextTick();
    if (el) el.scrollTop = el.scrollHeight - prevHeight;
  } catch (e) {
  } finally {
    loadingMore.value = false;
  }
}

function scrollToBottom() {
  const el = messagesEl.value;
  if (el) el.scrollTop = el.scrollHeight;
}

async function handleFileUpload(e: Event) {
  const target = e.target as HTMLInputElement;
  const file = target.files?.[0];
  if (!file || !activeId.value) return;

  target.value = '';

  const tempId = 'temp-file-' + Date.now();
  const optimisticMsg: chatApi.Message = {
    id: tempId,
    conversation_id: activeId.value,
    author_id: auth.user!.id,
    body: `Uploading ${file.name}...`,
    created_at_unix: Math.floor(Date.now() / 1000),
  };
  messages.value.push(optimisticMsg);
  nextTick(() => scrollToBottom());

  try {
    const msg = await chatApi.uploadAttachment(activeId.value, file);
    const exists = messages.value.some(m => m.id === msg.id);
    
    if (exists) {
      messages.value = messages.value.filter(m => m.id !== tempId);
    } else {
      const idx = messages.value.findIndex(m => m.id === tempId);
      if (idx !== -1) {
        messages.value[idx] = msg;
      } else {
        messages.value.push(msg);
      }
    }
    
    nextTick(() => scrollToBottom());
    loadConversations(true);
  } catch {
    error.value = 'File upload failed';
    messages.value = messages.value.filter(m => m.id !== tempId);
  }
}

// SILENT DOWNLOAD HANDLER
async function downloadAttachment(attachment: chatApi.Attachment) {
  downloadError.value = null;
  try {
    const url = chatApi.getAttachmentDownloadUrl(attachment.id);
    const res = await fetch(url, { credentials: 'include' });

    if (!res.ok) {
      if (res.status === 404 || res.status === 500) {
        downloadError.value = "file expired or deleted by server";
      } else {
        downloadError.value = "download failed";
      }
      setTimeout(() => { downloadError.value = null; }, 4000);
      return;
    }

    const blob = await res.blob();
    const blobUrl = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = blobUrl;
    a.download = attachment.filename;
    document.body.appendChild(a);
    a.click();
    a.remove();
    window.URL.revokeObjectURL(blobUrl);
  } catch (err) {
    downloadError.value = "network error";
    setTimeout(() => { downloadError.value = null; }, 4000);
  }
}

function sendMessage() {
  const body = sendBody.value.trim();
  if (!body || !activeId.value) return;

  if (body === '/help') {
    let helpText = '';

    if (activeConv.value?.kind === 'dm') {
      helpText = `Available commands for DMs:
/help - show this message
/clear - clear your chat history`;
    } else {
      helpText = `Available commands for Groups:
/help - show this message
/members - list all members in this group
/invite @username - add user to group
/remove @username - remove user from group
/clear - clear your chat history
/leave - leave the group`;
    }
    
    messages.value.push({
      id: 'sys-help-' + Date.now(),
      conversation_id: activeId.value,
      author_id: auth.user!.id,
      body: helpText,
      created_at_unix: Math.floor(Date.now() / 1000),
    });
    
    sendBody.value = '';
    nextTick(() => scrollToBottom());
    return;
  }

  if (body === '/members' && activeConv.value?.kind === 'group') {
    const membersList = activeConv.value.members
      .map((m) => `@${m.username}`)
      .join('\n');
      
    messages.value.push({
      id: 'sys-members-' + Date.now(),
      conversation_id: activeId.value,
      author_id: auth.user!.id,
      body: `Group Members:\n${membersList}`,
      created_at_unix: Math.floor(Date.now() / 1000),
    });
    
    sendBody.value = '';
    nextTick(() => scrollToBottom());
    return;
  }

  const tempId = 'temp-' + Date.now();
  const optimisticMsg: chatApi.Message = {
    id: tempId,
    conversation_id: activeId.value,
    author_id: auth.user!.id,
    body: body,
    created_at_unix: Math.floor(Date.now() / 1000),
  };
  messages.value.push(optimisticMsg);
  sendBody.value = '';
  nextTick(() => scrollToBottom());

  const ok = socket.send(activeId.value, body);
  if (!ok) {
    error.value = 'not connected — try again';
    return;
  }
  
  if (typingTimer) window.clearTimeout(typingTimer);
  localTyping.value = false;
  socket.sendTyping(activeId.value, false);
}

const offMessage = socket.onMessage((msg) => {
  if (msg.type === 'message' && msg.message) {
    if (msg.message.conversation_id === activeId.value) {
      const tempIdx = messages.value.findIndex((m) => m.id.startsWith('temp-') && m.body === msg.message!.body);
      if (tempIdx !== -1) {
        messages.value[tempIdx] = msg.message;
      } else {
        const exists = messages.value.some((m) => m.id === msg.message!.id);
        if (!exists) {
          if (msg.message.id === 'sys' && msg.message.body === 'Chat cleared.') {
             messages.value = [msg.message];
          } else {
             messages.value.push(msg.message);
          }
          nextTick(() => scrollToBottom());
          
          if (document.hasFocus()) {
            chatApi.markRead(activeId.value).catch(() => {});
          }
        }
      }
    }
    loadConversations(true); 
  } else if (msg.type === 'chat.unread') {
    if (msg.conversation_id === activeId.value) {
      return;
    }
    const conv = conversations.value.find(c => c.id === msg.conversation_id);
    if (conv) {
      conv.unread_count++;
      syncGlobalUnread(); 
    } else {
      loadConversations(true);
    }
  } else if (msg.type === 'typing' && msg.conversation_id === activeId.value && msg.user_id) {
    if (msg.user_id === auth.user?.id) return;
    const next = new Set(typingUsers.value);
    if (msg.is_typing) {
      next.add(msg.user_id);
    } else {
      next.delete(msg.user_id);
    }
    typingUsers.value = next;
  } else if (msg.type === 'error') {
    error.value = msg.error ?? 'socket error';
  }
});

onBeforeUnmount(() => {
  offMessage();
  if (typingTimer) window.clearTimeout(typingTimer);
});

function closeModals() {
  dmModal.value = false;
  groupModal.value = false;
  dmUserId.value = '';
  groupTitle.value = '';
  groupMembers.value = '';
  modalError.value = null;
}

async function submitDm() {
  modalError.value = null;
  const username = dmUserId.value.trim();
  if (!username) {
    modalError.value = 'username required';
    return;
  }
  modalBusy.value = true;
  try {
    const conv = await chatApi.startDMByUsername(username);
    await loadConversations();
    closeModals();
    await openConversation(conv.id);
  } catch (e: any) {
    modalError.value = e.response?.data?.error ?? t('chat.dm_failed');
  } finally {
    modalBusy.value = false;
  }
}

async function submitGroup() {
  modalError.value = null;
  const title = groupTitle.value.trim();
  const usernames = groupMembers.value
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);
  if (!title) {
    modalError.value = 'title required';
    return;
  }
  if (usernames.length === 0) {
    modalError.value = 'at least one member required';
    return;
  }
  modalBusy.value = true;
  try {
    const conv = await chatApi.createGroupByUsernames(title, usernames);
    await loadConversations();
    closeModals();
    await openConversation(conv.id);
  } catch (e: any) {
    modalError.value = e.response?.data?.error ?? t('chat.group_failed');
  } finally {
    modalBusy.value = false;
  }
}

function fmtTime(unix: number): string {
  const d = new Date(unix * 1000);
  const today = new Date();
  const sameDay =
    d.getFullYear() === today.getFullYear() &&
    d.getMonth() === today.getMonth() &&
    d.getDate() === today.getDate();
  if (sameDay) {
    return d.toISOString().slice(11, 16);
  }
  return d.toISOString().slice(5, 16).replace('T', ' ');
}

function fmtDate(unix?: number): string {
  if (!unix) return '';
  const d = new Date(unix * 1000);
  const today = new Date();
  const sameDay = d.getDate() === today.getDate() && d.getMonth() === today.getMonth() && d.getFullYear() === today.getFullYear();
  if (sameDay) return d.toISOString().slice(11, 16);
  return d.toISOString().slice(5, 10);
}

function convTitle(c: chatApi.Conversation): string {
  if (c.kind === 'group' && c.title) return c.title;
  const other = c.members.find((m) => m.id !== auth.user?.id);
  if (other) return `@${other.username}`;
  return `dm · ${c.id.slice(0, 8)}`;
}

const membersById = computed(() => {
  const map: Record<string, chatApi.Member> = {};
  if (activeConv.value) {
    for (const m of activeConv.value.members) map[m.id] = m;
  }
  return map;
});

const activeConv = computed(() =>
  conversations.value.find((c) => c.id === activeId.value)
);

const canSend = computed(
  () => socket.connected.value && activeId.value && sendBody.value.trim().length > 0
);

const typingLabel = computed(() => {
  const names = Array.from(typingUsers.value)
    .map((id) => membersById.value[id]?.username)
    .filter(Boolean);
  if (names.length === 0) return '';
  if (names.length === 1) return `${names[0]} is typing…`;
  return `${names.length} people are typing…`;
});

watch(
  () => route.query.open,
  (id) => {
    if (typeof id === 'string' && id && id !== activeId.value) {
      openConversation(id);
    }
  }
);

onMounted(() => {
  loadConversations();
  chatStore.clearUnread();
  
  window.addEventListener('ws-reconnected', () => {
    loadConversations(true); 
    if (activeId.value) {
      openConversation(activeId.value);
    }
  });
});
</script>

<template>
  <div class="chat">
    <aside class="sidebar">
      <header class="sidebar-head">
        <h1 class="heading">{{ t('chat.heading') }}</h1>
        <span class="status" :class="{ ok: socket.connected.value }">
          {{ socket.connected.value ? t('chat.connected') : t('chat.connecting') }}
        </span>
      </header>

      <div class="new-buttons">
        <button class="btn-sm" @click="dmModal = true">{{ t('chat.new_dm') }}</button>
        <button class="btn-sm" @click="groupModal = true">{{ t('chat.new_group') }}</button>
      </div>

      <p v-if="loadingConvs" class="muted">{{ t('common.loading') }}</p>
      <p v-else-if="conversations.length === 0" class="muted empty">
        {{ t('chat.empty') }}
      </p>

      <ul v-else class="conv-list">
        <li
          v-for="c in conversations"
          :key="c.id"
          class="conv"
          :class="{ active: c.id === activeId }"
          @click="openConversation(c.id)"
        >
          <div class="conv-head">
            <span class="conv-title">{{ convTitle(c) }}</span>
            
            <span v-if="c.unread_count > 0" class="conv-badge">
              {{ c.unread_count > 99 ? '99+' : c.unread_count }}
            </span>
            
            <span class="conv-date">{{ fmtDate(c.last_message_at_unix) }}</span>
          </div>
          <div class="conv-preview">
            {{ c.last_message_text || 'No messages yet' }}
          </div>
        </li>
      </ul>
    </aside>

    <main class="main">
      <p v-if="!activeId" class="placeholder">{{ t('chat.pick_one') }}</p>

      <template v-else>
        <header class="main-head">
          <span class="conv-title">
            {{ activeConv ? convTitle(activeConv) : '' }}
          </span>
          <span v-if="activeConv" class="members">
            {{ activeConv.member_count }} members
          </span>
        </header>

        <div ref="messagesEl" class="messages">
          <button
            v-if="hasMore"
            class="load-older"
            :disabled="loadingMore"
            @click="loadOlder"
          >
            {{ loadingMore ? t('chat.loading_more') : t('chat.load_older') }}
          </button>

          <p v-if="loadingMessages" class="muted">
            {{ t('common.loading') }}
          </p>

          <div
            v-for="m in messages"
            :key="m.id"
            class="msg"
            :class="{ mine: m.author_id === auth.user?.id, temp: m.id.startsWith('temp-'), system: m.id.startsWith('sys') }"
          >
            <div class="msg-head" v-if="!m.id.startsWith('sys')">
              <span class="author">
                {{ m.author_id === auth.user?.id
                  ? t('chat.you')
                  : (membersById[m.author_id]?.username ?? m.author_id.slice(0, 8)) }}
              </span>
              <span class="when">{{ fmtTime(m.created_at_unix) }}</span>
            </div>
            <div class="msg-body">
              <!-- 1. Render Image Preview if it's an image -->
              <template v-if="m.attachment && m.attachment.content_type.startsWith('image/')">
                <a href="#" @click.prevent="downloadAttachment(m.attachment)" class="image-link">
                  <img 
                    :src="chatApi.getAttachmentDownloadUrl(m.attachment.id)" 
                    :alt="m.attachment.filename" 
                    class="chat-image"
                    loading="lazy"
                    @error="(e: any) => e.target.style.display = 'none'"
                  />
                </a>
                <div class="image-meta" @click="downloadAttachment(m.attachment)" style="cursor: pointer;">
                  📎 {{ m.attachment.filename }} ({{ (m.attachment.size_bytes / 1024).toFixed(1) }} KB)
                </div>
              </template>

              <!-- 2. Render File Link if it's a non-image attachment -->
              <template v-else-if="m.attachment">
                <a href="#" @click.prevent="downloadAttachment(m.attachment)" class="attachment-link">
                  📎 {{ m.attachment.filename }} 
                  <span style="opacity: 0.6; font-size: 0.8em;">({{ (m.attachment.size_bytes / 1024).toFixed(1) }} KB)</span>
                </a>
              </template>

              <!-- 3. Render Normal Text -->
              <template v-else>
                <span v-if="m.body && !m.body.includes('\u0000')">{{ m.body }}</span>
                <span v-else class="muted">[File attachment]</span>
              </template>
            </div>
          </div>
        </div>

        <p v-if="typingLabel" class="typing-label">{{ typingLabel }}</p>

        <footer class="composer">
          <!-- THE DOWNLOAD ERROR NOTICE -->
          <div v-if="downloadError" class="chat-error-toast">
            ⚠️ {{ downloadError }}
          </div>

          <input 
            type="file" 
            ref="fileInput" 
            style="display: none" 
            @change="handleFileUpload" 
          />
          <button class="attach-btn" @click="fileInput?.click()" title="Attach file">
            📎
          </button>
          <input
            ref="messageInputRef"
            v-model="sendBody"
            type="text"
            placeholder="type a message or command... (send /help for info)"
            @input="notifyTyping"
            @keydown.enter.prevent="sendMessage"
          />
          <button class="send" :disabled="!canSend" @click="sendMessage">
            {{ t('chat.send') }}
          </button>
        </footer>
      </template>

      <p v-if="error" class="err">! {{ error }}</p>
    </main>

    <div v-if="dmModal" class="modal-bg" @click.self="closeModals">
      <div class="modal">
        <h2 class="modal-title">{{ t('chat.dm_modal_title') }}</h2>
          <label class="field">
            <span class="label">{{ t('chat.dm_modal_user_id') }}</span>
            <UserSearchInput
              v-model="dmUserId"
              :placeholder="t('chat.dm_modal_user_id_hint')"
            />
          </label>
        <p v-if="modalError" class="err">! {{ modalError }}</p>
        <div class="modal-actions">
          <button class="btn-primary" :disabled="modalBusy" @click="submitDm">
            {{ modalBusy ? t('common.loading') : `> ${t('chat.new_dm')}` }}
          </button>
          <button class="btn-ghost" @click="closeModals">{{ t('chat.cancel') }}</button>
        </div>
      </div>
    </div>

    <div v-if="groupModal" class="modal-bg" @click.self="closeModals">
      <div class="modal">
        <h2 class="modal-title">{{ t('chat.group_modal_title') }}</h2>
        <label class="field">
          <span class="label">{{ t('chat.group_modal_title_field') }}</span>
          <input v-model="groupTitle" type="text" maxlength="200" />
        </label>
        <label class="field">
          <span class="label">{{ t('chat.group_modal_members') }}</span>
          <input v-model="groupMembers" type="text" autocomplete="off" />
        </label>
        <p v-if="modalError" class="err">! {{ modalError }}</p>
        <div class="modal-actions">
          <button class="btn-primary" :disabled="modalBusy" @click="submitGroup">
            {{ modalBusy ? t('common.loading') : `> ${t('chat.group_modal_create')}` }}
          </button>
          <button class="btn-ghost" @click="closeModals">{{ t('chat.cancel') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.chat {
  display: grid;
  grid-template-columns: 300px 1fr;
  gap: 0;
  height: 100%;
  min-height: 0;
  background: #0e1319;
  border-radius: var(--radius);
  overflow: hidden;
}

.sidebar {
  border-right: 1px solid #1f2633;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-height: 0;
}

.sidebar-head {
  padding: 0.75rem 1rem;
  border-bottom: 1px solid #1f2633;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-shrink: 0;
}

.heading {
  color: #00d9a3;
  font-size: 1rem;
  font-weight: normal;
  margin: 0;
}

.status {
  font-size: 0.7rem;
  color: #ff5c5c;
}
.status.ok {
  color: #00d9a3;
}

.new-buttons {
  padding: 0.5rem 1rem;
  display: flex;
  gap: 0.375rem;
  border-bottom: 1px solid #1f2633;
  flex-shrink: 0;
}

.btn-sm {
  background: transparent;
  border: 1px solid #1f2633;
  color: #c8d3e0;
  font-family: inherit;
  font-size: 0.7rem;
  padding: 0.25rem 0.6rem;
  cursor: pointer;
  flex: 1;
}
.btn-sm:hover {
  border-color: #00d9a3;
  color: #00d9a3;
}

.conv-list {
  list-style: none;
  padding: 0;
  margin: 0;
  overflow-y: auto;
  flex: 1;
  min-height: 0;
}

.conv {
  padding: 0.6rem 1rem;
  border-bottom: 1px solid #0a0e14;
  cursor: pointer;
  border-left: 3px solid transparent;
}
.conv:hover {
  background: #0a0e14;
}
.conv.active {
  background: #0a0e14;
  border-left-color: #00d9a3;
}

.conv-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 0.5rem;
  margin-bottom: 0.1rem;
}

.conv-title {
  color: #c8d3e0;
  font-size: 0.9rem;
  font-weight: bold;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.conv-date {
  color: #6b7a90;
  font-size: 0.65rem;
  white-space: nowrap;
}

.conv-preview {
  color: #6b7a90;
  font-size: 0.75rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.conv.active .conv-title {
  color: #00d9a3;
}

/* UNREAD BADGE */
.conv-badge {
  background: #ff5c5c;
  color: #0a0e14;
  font-size: 0.6rem;
  font-weight: bold;
  padding: 0.1rem 0.4rem;
  border-radius: 10px;
  min-width: 18px;
  text-align: center;
  margin-left: 0.5rem;
}

.main {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-height: 0;
}

.placeholder {
  color: #6b7a90;
  text-align: center;
  margin: auto;
  font-size: 0.85rem;
}

.main-head {
  padding: 0.75rem 1rem;
  border-bottom: 1px solid #1f2633;
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  flex-shrink: 0;
}

.members {
  color: #6b7a90;
  font-size: 0.75rem;
}

.messages {
  flex: 1;
  overflow-y: auto;
  padding: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  min-height: 0;
}

.load-older {
  align-self: center;
  background: transparent;
  border: 1px solid #1f2633;
  color: #6b7a90;
  font-family: inherit;
  font-size: 0.7rem;
  padding: 0.25rem 0.75rem;
  cursor: pointer;
  margin-bottom: 0.5rem;
  flex-shrink: 0;
}
.load-older:hover:not(:disabled) {
  border-color: #00d9a3;
  color: #00d9a3;
}

.msg {
  align-self: flex-start;
  max-width: 70%;
  background: #0a0e14;
  border: 1px solid #1f2633;
  padding: 0.375rem 0.75rem;
  border-radius: 4px;
}
.msg.temp {
  opacity: 0.7;
}
.msg.mine {
  align-self: flex-end;
  border-color: #00d9a3;
  background: rgba(0, 217, 163, 0.05);
}
.msg.system {
  align-self: center;
  background: transparent;
  border: none;
  color: #6b7a90;
  font-style: italic;
  font-size: 0.8rem;
  white-space: pre-wrap;
  text-align: center;
  max-width: 90%;
  line-height: 1.5;
  margin: 0.5rem 0;
}

.msg-head {
  display: flex;
  gap: 0.5rem;
  align-items: baseline;
  margin-bottom: 0.125rem;
}

.author {
  color: #6b7a90;
  font-size: 0.7rem;
}
.msg.mine .author {
  color: #00d9a3;
}

.when {
  color: #6b7a90;
  font-size: 0.65rem;
}

.msg-body {
  color: #c8d3e0;
  font-size: 0.9rem;
  white-space: pre-wrap;
  word-break: break-word;
}

.attachment-link {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  color: #00d9a3;
  text-decoration: none;
  background: rgba(0, 217, 163, 0.1);
  padding: 0.5rem 0.75rem;
  border-radius: 4px;
  font-size: 0.85rem;
  border: 1px solid rgba(0, 217, 163, 0.3);
}
.attachment-link:hover {
  background: rgba(0, 217, 163, 0.2);
  text-decoration: underline;
}

.typing-label {
  padding: 0.25rem 1rem;
  color: #6b7a90;
  font-size: 0.75rem;
  font-style: italic;
  flex-shrink: 0;
}

.composer {
  display: flex;
  gap: 0.5rem;
  padding: 0.75rem 1rem;
  border-top: 1px solid #1f2633;
  background: #0e1319;
  flex-shrink: 0;
}

.attach-btn {
  background: transparent;
  border: none;
  color: #6b7a90;
  font-size: 1.2rem;
  cursor: pointer;
  padding: 0 0.5rem;
}
.attach-btn:hover {
  color: #00d9a3;
}

.composer input[type="text"] {
  flex: 1;
  background: #0a0e14;
  border: 1px solid #1f2633;
  color: #c8d3e0;
  padding: 0.5rem 0.75rem;
  font-family: inherit;
  font-size: 0.9rem;
  border-radius: 4px;
}
.composer input[type="text"]:focus {
  outline: none;
  border-color: #00d9a3;
}

.send {
  background: #00d9a3;
  color: #0a0e14;
  border: none;
  font-family: inherit;
  font-size: 0.85rem;
  font-weight: bold;
  padding: 0.5rem 1rem;
  cursor: pointer;
  border-radius: 4px;
}
.send:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.muted {
  color: #6b7a90;
  padding: 1rem;
  text-align: center;
  font-size: 0.85rem;
}

.err {
  color: #ff5c5c;
  font-size: 0.85rem;
  padding: 0 1rem;
}

.modal-bg {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.modal {
  background: #11151c;
  border: 1px solid #1f2633;
  padding: 1.5rem;
  width: 400px;
  max-width: 90vw;
  border-radius: 6px;
}

.modal-title {
  color: #00d9a3;
  font-size: 1rem;
  font-weight: normal;
  margin: 0 0 1rem;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  margin-bottom: 0.75rem;
}

.label {
  color: #6b7a90;
  font-size: 0.7rem;
}

.field input {
  background: #0a0e14;
  border: 1px solid #1f2633;
  color: #c8d3e0;
  padding: 0.5rem 0.75rem;
  font-family: inherit;
  font-size: 0.9rem;
  border-radius: 4px;
}
.field input:focus {
  outline: none;
  border-color: #00d9a3;
}

.modal-actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 1rem;
}

.btn-primary {
  background: #00d9a3;
  color: #0a0e14;
  border: none;
  font-family: inherit;
  font-size: 0.85rem;
  font-weight: bold;
  padding: 0.5rem 1rem;
  cursor: pointer;
  border-radius: 4px;
}
.btn-primary:hover:not(:disabled) {
  box-shadow: 0 0 12px rgba(0, 217, 163, 0.4);
}
.btn-primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-ghost {
  background: transparent;
  border: 1px solid #1f2633;
  color: #6b7a90;
  font-family: inherit;
  font-size: 0.85rem;
  padding: 0.5rem 1rem;
  cursor: pointer;
  border-radius: 4px;
}
.btn-ghost:hover {
  border-color: #00d9a3;
  color: #00d9a3;
}

@media (max-width: 800px) {
  .chat {
    grid-template-columns: 1fr;
  }
  .sidebar {
    border-right: none;
    border-bottom: 1px solid #1f2633;
    max-height: 250px;
  }
}

.image-link {
  display: block;
  max-width: 300px;
  border-radius: 4px;
  overflow: hidden;
  border: 1px solid #1f2633;
  margin-bottom: 0.25rem;
}

.chat-image {
  width: 100%;
  height: auto;
  display: block;
  object-fit: cover;
  max-height: 300px;
}

.image-meta {
  color: #6b7a90;
  font-size: 0.7rem;
  display: flex;
  align-items: center;
  gap: 0.25rem;
}
.chat-error-toast {
  position: absolute;
  bottom: 80px; /* Above the input box */
  left: 50%;
  transform: translateX(-50%);
  background: rgba(255, 92, 92, 0.1);
  border: 1px solid #ff5c5c;
  color: #ff5c5c;
  padding: 0.4rem 0.8rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-family: inherit;
  z-index: 50;
  pointer-events: none;
  animation: fadeInOut 0.3s ease;
}

@keyframes fadeInOut {
  from { opacity: 0; transform: translate(-50%, 10px); }
  to { opacity: 1; transform: translate(-50%, 0); }
}
</style>
