<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';
import { RouterLink } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as ordersApi from '../api/orders';
import * as walletApi from '../api/wallet';
import UserName from '../components/UserName.vue';
import SearchById from '../components/SearchById.vue';
import { fmtAmount, getCurrencies, symbolFor } from '../lib/money';

const { t } = useI18n();
const searchId = ref('');

const orders = ref<ordersApi.Order[]>([]);
const currencies = ref<walletApi.Currency[]>([]);
const role = ref<'' | 'buyer' | 'seller'>('');
const loading = ref(true);
const error = ref<string | null>(null);
const nextCursor = ref<string>('');
const hasMore = ref(false);
const loadingMore = ref(false);

async function load(reset = true) {
  if (reset) {
    loading.value = true;
    error.value = null;
    orders.value = [];
    nextCursor.value = '';
    hasMore.value = false;
  } else {
    loadingMore.value = true;
  }

  try {
    const page = await ordersApi.list(role.value, reset ? '' : nextCursor.value, 20, searchId.value);
    orders.value = reset ? page.orders : [...orders.value, ...page.orders];
    nextCursor.value = page.next_cursor ?? '';
    hasMore.value = page.has_more;
  } catch {
    error.value = t('orders.load_failed');
  } finally {
    loading.value = false;
    loadingMore.value = false;
  }
}

async function loadCurrencies() {
  try {
    currencies.value = await getCurrencies();
  } catch {
    // non-fatal
  }
}

function setRole(r: '' | 'buyer' | 'seller') {
  if (role.value === r) return;
  role.value = r;
  load(true);
}

function statusLabel(s: ordersApi.Order['status']): string {
  switch (s) {
    case 'held':      return t('orders.status_held');
    case 'completed': return t('orders.status_completed');
    case 'refunded':  return t('orders.status_refunded');
    case 'pending':   return t('orders.status_pending');
    case 'cancelled': return t('orders.status_cancelled');
    case 'disputed':  return t('orders.status_disputed');
  }
  return s;
}

function fmtDate(unix: number): string {
  return new Date(unix * 1000).toISOString().slice(0, 10);
}

onMounted(() => {
  load();
  loadCurrencies();
});

watch(searchId, () => {
  load(true);
});
</script>

<template>
  <div>
    <header class="page-head">
      <h1 class="heading">{{ t('orders.heading') }}</h1>
      <SearchById v-model="searchId" />
      <div class="filters">
        <button class="filter" :class="{ active: role === '' }" @click="setRole('')">
          {{ t('orders.filter_all') }}
        </button>
        <button class="filter" :class="{ active: role === 'buyer' }" @click="setRole('buyer')">
          {{ t('orders.filter_buyer') }}
        </button>
        <button class="filter" :class="{ active: role === 'seller' }" @click="setRole('seller')">
          {{ t('orders.filter_seller') }}
        </button>
      </div>
    </header>

    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="err">! {{ error }}</p>

    <div v-else-if="orders.length === 0" class="empty">
      {{ t('orders.empty') }}
    </div>

    <div v-else class="list">
      <RouterLink
        v-for="o in orders"
        :key="o.id"
        :to="{ name: 'order-detail', params: { id: o.id } }"
        class="row"
      >
        <div class="row-main">
          <span class="role" :class="o.my_role">{{ t(`orders.role_${o.my_role}`) }}</span>
          <span class="amount">
            {{ symbolFor(currencies, o.currency_code) }}
            {{ fmtAmount(currencies, o.amount, o.currency_code) }}
          </span>
          <span class="status" :class="o.status">{{ statusLabel(o.status) }}</span>
        </div>
        <div class="row-sub">
        <span class="meta">
            {{ o.my_role === 'buyer' ? 'to' : 'from' }}
        </span>
        <UserName
            :username="o.my_role === 'buyer' ? o.seller_username : o.buyer_username"
            prefix="at"
        />
        <span class="meta">·</span>
        <span class="meta">{{ t('orders.created') }} {{ fmtDate(o.created_at_unix) }}</span>
        <span class="meta">·</span>
        <span class="meta">{{ o.id.slice(0, 8) }}</span>
        </div>
      </RouterLink>
    </div>

    <div v-if="hasMore && !loading" class="more">
      <button class="btn-ghost" :disabled="loadingMore" @click="load(false)">
        {{ loadingMore ? t('common.loading') : 'load more' }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.page-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 1.5rem;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.heading {
  color: #00d9a3;
  font-size: 1.25rem;
  font-weight: normal;
  margin: 0;
}
.filters {
  display: flex;
  gap: 0.375rem;
}
.filter {
  background: transparent;
  border: 1px solid #1f2633;
  color: #6b7a90;
  font-family: inherit;
  font-size: 0.75rem;
  padding: 0.25rem 0.6rem;
  cursor: pointer;
}
.filter:hover {
  color: #c8d3e0;
}
.filter.active {
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
  gap: 0.5rem;
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
  align-items: baseline;
  gap: 0.75rem;
}
.role {
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  padding: 0.1rem 0.4rem;
  border: 1px solid #1f2633;
}
.role.buyer {
  color: #4ea3ff;
  border-color: #4ea3ff;
}
.role.seller {
  color: #ffb454;
  border-color: #ffb454;
}
.amount {
  font-size: 1.1rem;
  font-weight: bold;
  color: #c8d3e0;
}
.status {
  margin-left: auto;
  font-size: 0.75rem;
  padding: 0.1rem 0.5rem;
  border: 1px solid #1f2633;
  color: #6b7a90;
}
.status.held {
  color: #ffb454;
  border-color: #ffb454;
}
.status.completed {
  color: #00d9a3;
  border-color: #00d9a3;
}
.status.refunded {
  color: #ff5c5c;
  border-color: #ff5c5c;
}
.row-sub {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.375rem;
  font-size: 0.7rem;
  flex-wrap: wrap;
}
.meta {
  color: #6b7a90;
}
.more {
  margin-top: 1rem;
  text-align: center;
}
.btn-ghost {
  background: transparent;
  border: 1px solid #1f2633;
  color: #c8d3e0;
  font-family: inherit;
  font-size: 0.85rem;
  padding: 0.4rem 1rem;
  cursor: pointer;
}
.btn-ghost:hover:not(:disabled) {
  border-color: #00d9a3;
  color: #00d9a3;
}
.btn-ghost:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.search-wrap {
  margin-bottom: 1rem;
}
@media (max-width: 700px) {
  .page-head {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.75rem;
  }
  .filters {
    flex-direction: column;
    align-items: flex-start;
  }
  .form-row, .row { 
    /* Ensure any side-by-side inputs stack vertically */
    grid-template-columns: 1fr !important;
    flex-direction: column;
  }
}
</style>
