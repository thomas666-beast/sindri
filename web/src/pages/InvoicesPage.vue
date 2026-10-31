<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';
import { RouterLink } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as invoicesApi from '../api/invoices';
import * as walletApi from '../api/wallet';
import { fmtAmount, symbolFor } from '../lib/money';
import SearchById from '../components/SearchById.vue';

const { t } = useI18n();

const searchId = ref('')

const invoices = ref<invoicesApi.InvoiceItem[]>([]);
const currencies = ref<walletApi.Currency[]>([]);
const total = ref(0);
const loading = ref(true);
const error = ref<string | null>(null);

const role = ref<'' | 'buyer' | 'seller'>('buyer');
const status = ref<'' | 'pending' | 'held' | 'completed' | 'refunded' | 'cancelled'>('');
const limit = ref(30);
const offset = ref(0);
const loadingMore = ref(false);
const hasMore = ref(false);

async function load(reset = true) {
  if (reset) {
    loading.value = true;
    error.value = null;
    invoices.value = [];
    offset.value = 0;
  } else {
    loadingMore.value = true;
  }
  try {
    const r = await invoicesApi.list({
      role: role.value,
      status: status.value,
      id: searchId.value,
      limit: limit.value,
      offset: reset ? 0 : offset.value,
    });
    invoices.value = reset ? r.data : [...invoices.value, ...r.data];
    total.value = r.total;
    offset.value = (reset ? 0 : offset.value) + r.data.length;
    hasMore.value = invoices.value.length < r.total;
  } catch {
    error.value = t('invoices.load_failed');
  } finally {
    loading.value = false;
    loadingMore.value = false;
  }
}

async function loadCurrencies() {
  try {
    currencies.value = await walletApi.listCurrencies();
  } catch {
    // non-fatal
  }
}

function setRole(r: '' | 'buyer' | 'seller') {
  if (role.value === r) return;
  role.value = r;
  load(true);
}

function setStatus(s: '' | 'pending' | 'held' | 'completed' | 'refunded' | 'cancelled') {
  if (status.value === s) return;
  status.value = s;
  load(true);
}

function statusLabel(s: string): string {
  switch (s) {
    case 'pending': return t('invoices.status_pending');
    case 'held': return t('orders.status_held');
    case 'completed': return t('orders.status_completed');
    case 'refunded': return t('orders.status_refunded');
    case 'cancelled': return t('invoices.status_cancelled');
    default: return s;
  }
}

function fmtDate(unix: number): string {
  return new Date(unix * 1000).toISOString().slice(0, 10);
}

function counterparty(o: invoicesApi.InvoiceItem): string {
  return role.value === 'seller' ? (o.buyer_username ?? '') : (o.seller_username ?? '');
}

watch(searchId, () => load(true));

onMounted(() => {
  load(true);
  loadCurrencies();
});
</script>

<template>
  <div>
    <header class="page-head">
      <h1 class="heading">{{ t('invoices.heading') }}</h1>
      <RouterLink to="/invoices/new" class="btn-primary">
        {{ t('invoices.list_yours') }}
      </RouterLink>
    </header>

    <div class="filters">
      <div class="filter-group">
        <button
          class="filter"
          :class="{ active: role === 'buyer' }"
          @click="setRole('buyer')"
        >
          {{ t('invoices.role_buyer') }}
        </button>
        <button
          class="filter"
          :class="{ active: role === 'seller' }"
          @click="setRole('seller')"
        >
          {{ t('invoices.role_seller') }}
        </button>
      </div>

      <div class="filter-group">
        <button
          class="filter"
          :class="{ active: status === '' }"
          @click="setStatus('')"
        >
          {{ t('invoices.status_all') }}
        </button>
        <button
          class="filter"
          :class="{ active: status === 'pending' }"
          @click="setStatus('pending')"
        >
          {{ t('invoices.status_pending') }}
        </button>
        <button
          class="filter"
          :class="{ active: status === 'held' }"
          @click="setStatus('held')"
        >
          {{ t('orders.status_held') }}
        </button>
        <button
          class="filter"
          :class="{ active: status === 'completed' }"
          @click="setStatus('completed')"
        >
          {{ t('orders.status_completed') }}
        </button>
        <button
          class="filter"
          :class="{ active: status === 'cancelled' }"
          @click="setStatus('cancelled')"
        >
          {{ t('invoices.status_cancelled') }}
        </button>
      </div>

    <div class="search-wrap">
      <SearchById v-model="searchId" />
    </div>

    </div>

    <p v-if="error" class="err">! {{ error }}</p>
    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="invoices.length === 0" class="empty">
      {{ t('invoices.empty') }}
    </p>

    <div v-else class="list">
      <RouterLink
        v-for="inv in invoices"
        :key="inv.id"
        :to="{ name: 'invoice-detail', params: { id: inv.id } }"
        class="row"
        :class="inv.status"
      >
        <div class="row-head">
          <span class="title">{{ inv.invoice_title }}</span>
          <span class="status-badge" :class="inv.status">{{ statusLabel(inv.status) }}</span>
        </div>
        <div class="row-main">
          <span class="counterparty">
            {{ role === 'seller' ? t('invoices.to') : t('invoices.from') }}
            @{{ counterparty(inv) }}
          </span>
          <span class="amount">
            {{ symbolFor(currencies, inv.currency_code) }}
            {{ fmtAmount(currencies, inv.amount, inv.currency_code) }}
            <span class="cur">{{ inv.currency_code }}</span>
          </span>
        </div>
        <div class="row-foot">
          <span class="meta">{{ fmtDate(inv.created_at_unix) }}</span>
        </div>
      </RouterLink>
    </div>

    <div v-if="hasMore && !loading" class="more">
      <button class="load-more" :disabled="loadingMore" @click="load(false)">
        {{ loadingMore ? t('common.loading') : t('invoices.load_more') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.page-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 1.25rem;
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
  font-weight: 600;
  font-size: 0.85rem;
  padding: 0.5rem 0.9rem;
  cursor: pointer;
  text-decoration: none;
}
.btn-primary:hover {
  background: #00f5aa;
}
.filters {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 1.5rem;
}

.search-wrapper {
  margin-left: auto; /* This pushes the search input to the far right */
}
.filter-group {
  display: flex;
  gap: 0.25rem;
  flex-wrap: wrap;
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
  border-radius: 4px;
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
  border: 1px solid #1e2229;
  background: rgba(15, 17, 21, 0.85);
  border-radius: 4px;
  padding: 0.75rem 1rem;
  text-decoration: none;
  color: inherit;
  display: block;
  transition: border-color 0.1s;
}
.row:hover {
  border-color: #2a3039;
}
.row.cancelled {
  opacity: 0.6;
}
.row-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 0.5rem;
  margin-bottom: 0.375rem;
}
.title {
  color: #e2e5e9;
  font-weight: 600;
  font-size: 0.95rem;
}
.status-badge {
  font-size: 0.7rem;
  padding: 0.1rem 0.5rem;
  border-radius: 4px;
  border: 1px solid #1e2229;
  color: #6b7a90;
  white-space: nowrap;
}
.status-badge.pending {
  color: #ffb454;
  border-color: #ffb454;
}
.status-badge.held {
  color: #4a9eff;
  border-color: #4a9eff;
}
.status-badge.completed {
  color: #00d9a3;
  border-color: #00d9a3;
}
.status-badge.refunded {
  color: #ff5c5c;
  border-color: #ff5c5c;
}
.status-badge.cancelled {
  color: #6b7a90;
  border-color: #6b7a90;
}
.row-main {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 0.75rem;
}
.counterparty {
  color: #6b7a90;
  font-size: 0.8rem;
}
.amount {
  color: #e2e5e9;
  font-weight: 600;
}
.cur {
  color: #6b7a90;
  font-size: 0.7rem;
  font-weight: normal;
  margin-left: 0.25rem;
}
.row-foot {
  margin-top: 0.375rem;
}
.meta {
  color: #3a4150;
  font-size: 0.7rem;
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
