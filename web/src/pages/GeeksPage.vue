<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';
import { RouterLink, useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as usersApi from '../api/users';
import * as chatApi from '../api/chatApi';
import { useAuthStore } from '../stores/auth';
import Avatar from '../components/Avatar.vue';

const { t } = useI18n();
const auth = useAuthStore();
const router = useRouter();

const users = ref<usersApi.UserHit[]>([]);
const total = ref(0);
const loading = ref(true);
const loadingMore = ref(false);
const error = ref<string | null>(null);

const query = ref('');
const offset = ref(0);
const limit = 30;
let debounceTimer: number | null = null;

const dmBusy = ref<string | null>(null);

async function load(reset = true) {
  if (reset) {
    loading.value = true;
    error.value = null;
    users.value = [];
    offset.value = 0;
  } else {
    loadingMore.value = true;
  }

  try {
    const page = await usersApi.search(
      query.value.trim(),
      limit,
      reset ? 0 : offset.value
    );
    users.value = reset ? page.results : [...users.value, ...page.results];
    total.value = page.total;
    offset.value = (reset ? 0 : offset.value) + page.results.length;
  } catch {
    error.value = t('geeks.load_failed');
  } finally {
    loading.value = false;
    loadingMore.value = false;
  }
}

function onInput() {
  if (debounceTimer) window.clearTimeout(debounceTimer);
  debounceTimer = window.setTimeout(() => {
    load(true);
  }, 300);
}

async function message(u: usersApi.UserHit) {
  if (!auth.user) {
    router.push({ name: 'login' });
    return;
  }
  if (u.id === auth.user.id) return;
  dmBusy.value = u.id;
  try {
    const conv = await chatApi.startDMByUsername(u.username);
    router.push({ name: 'chat', query: { open: conv.id } });
  } catch {
    error.value = t('geeks.message_failed');
  } finally {
    dmBusy.value = null;
  }
}

function isMe(u: usersApi.UserHit): boolean {
  return auth.user?.id === u.id;
}

const hasMore = () => users.value.length < total.value;

onMounted(() => {
  load(true);
});

watch(query, onInput);
</script>

<template>
  <div>
    <header class="page-head">
      <h1 class="heading">{{ t('geeks.heading') }}</h1>
      <p class="subtitle">{{ t('geeks.subtitle') }}</p>
    </header>

    <input
      v-model="query"
      type="search"
      :placeholder="t('geeks.search_placeholder')"
      class="search"
    />

    <p v-if="error" class="err">! {{ error }}</p>
    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="users.length === 0 && query" class="empty">
      {{ t('geeks.empty_search') }}
    </p>
    <p v-else-if="users.length === 0" class="empty">
      {{ t('geeks.empty') }}
    </p>

    <div v-else class="grid">
      <article v-for="u in users" :key="u.id" class="card">
        <RouterLink
          :to="{ name: 'public-profile', params: { username: u.username } }"
          class="card-link"
        >
          <Avatar :username="u.username" :src="u.avatar_url" :size="56" />
          <div class="identity">
            <div class="display-name">{{ u.display_name }}</div>
            <div class="username">@{{ u.username }}</div>
          </div>
        </RouterLink>

        <p v-if="u.bio" class="bio">{{ u.bio }}</p>

        <div v-if="u.skills.length > 0" class="chips">
          <span v-for="s in u.skills.slice(0, 3)" :key="s" class="chip">{{ s }}</span>
          <span v-if="u.skills.length > 3" class="chip more">+{{ u.skills.length - 3 }}</span>
        </div>

        <footer class="actions">
          <button
            v-if="!isMe(u)"
            class="primary"
            :disabled="dmBusy === u.id"
            @click="message(u)"
          >
            {{ dmBusy === u.id ? t('common.loading') : t('geeks.message') }}
          </button>
          <RouterLink
            v-if="!isMe(u)"
            :to="{ name: 'public-profile', params: { username: u.username } }"
            class="ghost"
          >
            {{ t('geeks.profile') }}
          </RouterLink>
          <span v-else class="you">{{ t('geeks.you') }}</span>
        </footer>
      </article>
    </div>

    <div v-if="hasMore() && !loading" class="more">
      <button class="load-more" :disabled="loadingMore" @click="load(false)">
        {{ loadingMore ? t('common.loading') : t('geeks.load_more') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.page-head {
  margin-bottom: 1.25rem;
}
.heading {
  font-size: 1.1rem;
  color: #e2e5e9;
  margin: 0 0 0.25rem;
}
.subtitle {
  color: #6b7a90;
  font-size: 0.8rem;
  margin: 0;
}
.search {
  width: 100%;
  background: #0a0e14;
  border: 1px solid #1e2229;
  color: #c8d3e0;
  padding: 0.6rem 0.9rem;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.9rem;
  margin-bottom: 1rem;
}
.search:focus {
  outline: none;
  border-color: #00d9a3;
  box-shadow: 0 0 0 2px rgba(0, 229, 160, 0.25);
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
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 0.75rem;
}
.card {
  border: 1px solid #1e2229;
  background: rgba(15, 17, 21, 0.85);
  border-radius: 6px;
  padding: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.card-link {
  display: flex;
  gap: 0.75rem;
  align-items: center;
  text-decoration: none;
  color: inherit;
}
.identity {
  min-width: 0;
}
.display-name {
  color: #e2e5e9;
  font-weight: 600;
  font-size: 0.95rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.username {
  color: #00d9a3;
  font-size: 0.8rem;
}
.bio {
  color: #6b7a90;
  font-size: 0.8rem;
  margin: 0;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  line-height: 1.4;
}
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
}
.chip {
  background: rgba(0, 229, 160, 0.08);
  color: #00e5a0;
  border: 1px solid rgba(0, 229, 160, 0.3);
  padding: 0.1rem 0.5rem;
  border-radius: 4px;
  font-size: 0.7rem;
}
.chip.more {
  color: #6b7a90;
  border-color: #1e2229;
  background: transparent;
}
.actions {
  display: flex;
  gap: 0.375rem;
  margin-top: auto;
  padding-top: 0.5rem;
}
.primary {
  background: #00d9a3;
  color: #0a0e14;
  border: none;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.8rem;
  font-weight: 600;
  padding: 0.4rem 0.9rem;
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
  font-size: 0.8rem;
  padding: 0.4rem 0.9rem;
  text-decoration: none;
  cursor: pointer;
}
.ghost:hover {
  border-color: #00d9a3;
  color: #00d9a3;
}
.you {
  color: #6b7a90;
  font-size: 0.8rem;
}
.more {
  margin-top: 0.75rem;
}
.load-more {
  background: transparent;
  border: 1px solid #1e2229;
  color: #6b7a90;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.8rem;
  padding: 0.4rem 1rem;
  cursor: pointer;
  width: 100%;
}
.load-more:hover:not(:disabled) {
  border-color: #00d9a3;
  color: #00d9a3;
}
.load-more:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
