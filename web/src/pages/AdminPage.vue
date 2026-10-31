<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { RouterLink } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as adminApi from '../api/admin';

const { t } = useI18n();

const users = ref<adminApi.AdminUser[]>([]);
const total = ref(0);
const search = ref('');
const status = ref<'' | 'active' | 'disabled'>('');
const loading = ref(true);
const error = ref<string | null>(null);

// Pagination
const limit = 15;
const offset = ref(0);
const hasMore = ref(true);
const loadingMore = ref(false);

async function load(reset = true) {
  if (reset) {
    loading.value = true;
    error.value = null;
    users.value = [];
    offset.value = 0;
    hasMore.value = true;
  } else {
    loadingMore.value = true;
  }

  try {
    const r = await adminApi.listUsers(search.value, status.value, limit, offset.value);
    
    if (reset) {
      users.value = r.users;
    } else {
      users.value = [...users.value, ...r.users];
    }
    
    total.value = r.total;
    
    if (r.users.length < limit) {
      hasMore.value = false;
    } else {
      offset.value += limit;
    }
  } catch {
    error.value = t('admin.load_failed');
  } finally {
    loading.value = false;
    loadingMore.value = false;
  }
}

function fmtDate(unix: number): string {
  return new Date(unix * 1000).toISOString().slice(0, 10);
}

onMounted(() => {
  load(true);
});
</script>

<template>
  <div>
    <header class="page-head">
      <h1 class="heading">{{ t('admin.tab_users') }}</h1>
    </header>

    <div class="filters">
      <input
        v-model="search"
        type="search"
        :placeholder="t('admin.search_placeholder')"
        class="search"
        @keydown.enter="load(true)"
      />
      <select v-model="status" class="status-select" @change="load(true)">
        <option value="">{{ t('admin.filter_all') }}</option>
        <option value="active">{{ t('admin.filter_active') }}</option>
        <option value="disabled">{{ t('admin.filter_disabled') }}</option>
      </select>
      <button class="btn-ghost" @click="load(true)">↻</button>
    </div>

    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="err">! {{ error }}</p>
    <p v-else-if="users.length === 0" class="empty">{{ t('admin.no_users') || 'No users found.' }}</p>

    <div v-else class="list">
      <RouterLink
        v-for="u in users"
        :key="u.id"
        :to="{ name: 'admin-user', params: { id: u.id } }"
        class="row"
      >
        <div class="row-main">
          <span class="uname">{{ u.username }}</span>
          <span v-if="u.is_admin" class="badge admin">admin</span>
          <span v-if="!u.is_active" class="badge disabled">disabled</span>
          <span class="badge kyc" :class="u.kyc_status">kyc:{{ u.kyc_status }}</span>
        </div>
        <div class="row-sub">
          <span class="meta">{{ u.email }}</span>
          <span class="meta">·</span>
          <span class="meta">{{ fmtDate(u.created_at_unix) }}</span>
        </div>
      </RouterLink>
    </div>

    <!-- PAGINATION -->
    <div v-if="hasMore && !loading" class="load-more-wrap">
      <button class="load-more-btn" :disabled="loadingMore" @click="load(false)">
        {{ loadingMore ? t('common.loading') : 'Load More Users' }}
      </button>
    </div>

    <p v-if="users.length > 0" class="total">
      showing {{ users.length }} of {{ total }}
    </p>
  </div>
</template>

<style scoped>
.page-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 1.5rem;
}
.heading {
  color: #00d9a3;
  font-size: 1.25rem;
  font-weight: normal;
  margin: 0;
}
.filters {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1rem;
}
.search {
  flex: 1;
  background: #11151c;
  border: 1px solid #1f2633;
  color: #c8d3e0;
  padding: 0.5rem 0.75rem;
  font-family: inherit;
  font-size: 0.85rem;
}
.search:focus {
  outline: none;
  border-color: #00d9a3;
}
.status-select {
  background: #11151c;
  border: 1px solid #1f2633;
  color: #c8d3e0;
  padding: 0.5rem 0.75rem;
  font-family: inherit;
  font-size: 0.85rem;
}
.status-select:focus {
  outline: none;
  border-color: #00d9a3;
}
.btn-ghost {
  background: transparent;
  border: 1px solid #1f2633;
  color: #6b7a90;
  font-family: inherit;
  font-size: 0.9rem;
  padding: 0.5rem 0.75rem;
  cursor: pointer;
}
.btn-ghost:hover {
  border-color: #00d9a3;
  color: #00d9a3;
}
.muted {
  color: #6b7a90;
}
.err {
  color: #ff5c5c;
  font-size: 0.85rem;
}
.empty {
  border: 1px dashed #1f2633;
  padding: 2rem;
  text-align: center;
  color: #6b7a90;
}
.list {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}
.row {
  border: 1px solid #1f2633;
  background: #11151c;
  padding: 0.75rem 1rem;
  text-decoration: none;
  color: inherit;
  display: block;
  transition: border-color 0.1s;
}
.row:hover {
  border-color: #00d9a3;
}
.row-main {
  display: flex;
  gap: 0.5rem;
  align-items: center;
  flex-wrap: wrap;
}
.uname {
  color: #c8d3e0;
  font-weight: bold;
}
.badge {
  font-size: 0.65rem;
  padding: 0.05rem 0.4rem;
  border: 1px solid #1f2633;
  color: #6b7a90;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}
.badge.admin {
  color: #00d9a3;
  border-color: #00d9a3;
}
.badge.disabled {
  color: #ff5c5c;
  border-color: #ff5c5c;
}
.badge.verified {
  color: #00d9a3;
  border-color: #00d9a3;
}
.badge.pending {
  color: #ffb454;
  border-color: #ffb454;
}
.badge.rejected {
  color: #ff5c5c;
  border-color: #ff5c5c;
}
.row-sub {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.25rem;
  font-size: 0.7rem;
  flex-wrap: wrap;
}
.meta {
  color: #6b7a90;
}
.total {
  color: #6b7a90;
  font-size: 0.75rem;
  margin-top: 1rem;
  text-align: right;
}

/* LOAD MORE */
.load-more-wrap {
  display: flex;
  justify-content: center;
  margin-top: 1rem;
}
.load-more-btn {
  background: transparent;
  border: 1px solid #1f2633;
  color: #6b7a90;
  padding: 0.6rem 2rem;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.85rem;
  cursor: pointer;
  transition: all 0.15s;
}
.load-more-btn:hover:not(:disabled) {
  border-color: #00d9a3;
  color: #00d9a3;
}
.load-more-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
