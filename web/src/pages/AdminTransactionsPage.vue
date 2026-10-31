<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { RouterLink } from 'vue-router';
import * as adminApi from '../api/admin';
import * as walletApi from '../api/wallet';
import { fmtAmount, symbolFor } from '../lib/money';
import CopyId from '../components/CopyId.vue';

const { t } = useI18n();

const transactions = ref<adminApi.AdminTransaction[]>([]);
const currencies = ref<walletApi.Currency[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);

// Filters
const typeFilter = ref('');
const searchId = ref('');
const searchUser = ref('');

// Pagination
const limit = 50;
const offset = ref(0);
const hasMore = ref(true);
const loadingMore = ref(false);

async function load(reset = true) {
  if (reset) {
    loading.value = true;
    error.value = null;
    transactions.value = [];
    offset.value = 0;
    hasMore.value = true;
  } else {
    loadingMore.value = true;
  }

  try {
    const res = await adminApi.listTransactions(
      typeFilter.value,
      searchId.value,
      searchUser.value,
      limit,
      offset.value
    );

    if (reset) {
      transactions.value = res;
    } else {
      transactions.value = [...transactions.value, ...res];
    }

    if (res.length < limit) {
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

async function loadCurrencies() {
  try {
    currencies.value = await walletApi.listCurrencies();
  } catch {}
}

function fmtDate(unix: number): string {
  return new Date(unix * 1000).toISOString().slice(0, 19).replace('T', ' ');
}

onMounted(() => {
  load(true);
  loadCurrencies();
});
</script>

<template>
  <div>
    <h1 class="heading">{{ t('admin.transactions_heading') }}</h1>

    <!-- Filter Bar -->
    <div class="filters-bar">
      <div class="filters">
        <select v-model="typeFilter" @change="load(true)">
          <option value="">{{ t('admin.search_type') }}</option>
          <option value="deposit">{{ t('wallet.tx_deposit') }}</option>
          <option value="withdraw">{{ t('wallet.tx_withdraw') }}</option>
          <option value="escrow_hold">{{ t('wallet.tx_escrow_hold') }}</option>
          <option value="escrow_release">{{ t('wallet.tx_escrow_release') }}</option>
          <option value="escrow_refund">{{ t('wallet.tx_escrow_refund') }}</option>
          <option value="commission">{{ t('wallet.tx_commission') }}</option>
          <option value="admin_adjustment">{{ t('wallet.tx_admin') }}</option>
        </select>
      </div>
      
      <div class="search-box">
        <input 
          v-model="searchId" 
          type="text" 
          :placeholder="t('admin.search_tx_id')" 
          @keydown.enter="load(true)"
        />
        <input 
          v-model="searchUser" 
          type="text" 
          :placeholder="t('admin.search_user_id')" 
          @keydown.enter="load(true)"
        />
        <button class="btn-ghost" @click="load(true)" title="search">↻</button>
      </div>
    </div>

    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="err">! {{ error }}</p>
    <p v-else-if="transactions.length === 0" class="empty">
      {{ t('admin.empty_transactions') }}
    </p>

    <div v-else class="list">
      <article v-for="tx in transactions" :key="tx.id" class="row">
        <div class="row-main">
          <CopyId :id="tx.id" label="tx id" />
          
          <span class="user">@{{ tx.username }}</span>
          
          <span class="badge" :class="tx.type">
            {{ t('wallet.tx_' + (tx.type === 'admin_adjustment' ? 'admin' : tx.type.replace('escrow_', 'escrow_'))) }}
          </span>
          
          <span class="amount" :class="tx.direction">
            {{ tx.direction === 'debit' ? '-' : '+' }}
            {{ symbolFor(currencies, tx.currency_code) }}
            {{ fmtAmount(currencies, tx.amount, tx.currency_code) }}
          </span>
        </div>
        
        <div class="row-sub">
          <span class="meta">{{ fmtDate(tx.created_at_unix) }}</span>
          <span class="meta">·</span>
          <span class="meta">{{ t('admin.tx_balance_after') }}: {{ fmtAmount(currencies, tx.balance_after, tx.currency_code) }}</span>
          
          <RouterLink
            v-if="tx.order_id"
            :to="{ name: 'order-detail', params: { id: tx.order_id } }"
            class="row-link"
          >
            {{ t('admin.tx_view_order') }}
          </RouterLink>
        </div>
      </article>

      <!-- PAGINATION -->
      <div v-if="hasMore" class="load-more-wrap">
        <button class="load-more-btn" :disabled="loadingMore" @click="load(false)">
          {{ loadingMore ? t('common.loading') : t('common.load_more') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.heading { color: #00d9a3; font-size: 1.25rem; font-weight: normal; margin: 0 0 1.5rem; }

/* FILTERS & SEARCH */
.filters-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
  gap: 1rem;
  flex-wrap: wrap;
}
.filters select {
  background: #11151c;
  border: 1px solid #1e2229;
  color: #c8d3e0;
  padding: 0.3rem 0.6rem;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.75rem;
}
.filters select:focus {
  outline: none;
  border-color: #00d9a3;
}
.search-box {
  display: flex;
  gap: 0.25rem;
}
.search-box input {
  background: #11151c;
  border: 1px solid #1e2229;
  color: #c8d3e0;
  padding: 0.3rem 0.6rem;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.75rem;
  width: 200px;
}
.search-box input:focus {
  outline: none;
  border-color: #00d9a3;
}
.btn-ghost {
  background: transparent;
  border: 1px solid #1e2229;
  color: #6b7a90;
  font-family: inherit;
  font-size: 0.85rem;
  padding: 0.3rem 0.6rem;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.15s;
}
.btn-ghost:hover {
  border-color: #00d9a3;
  color: #00d9a3;
}

.muted { color: #6b7a90; }
.err { color: #ff5c5c; font-size: 0.85rem; margin-bottom: 1rem; }
.empty { border: 1px dashed #1e2229; padding: 2rem; text-align: center; color: #6b7a90; border-radius: 4px; }

/* LIST & ROWS */
.list { display: flex; flex-direction: column; gap: 0.375rem; }
.row { border: 1px solid #1e2229; background: rgba(15, 17, 21, 0.85); border-radius: 4px; padding: 0.75rem 1rem; display: flex; flex-direction: column; gap: 0.375rem; }
.row-main { display: flex; gap: 0.5rem; align-items: center; flex-wrap: wrap; }
.user { color: #6b7a90; font-size: 0.85rem; }
.amount { font-weight: 600; font-size: 0.95rem; margin-left: auto; }
.amount.credit { color: #00d9a3; }
.amount.debit { color: #ff5c5c; }

/* BADGES (Types) */
.badge {
  font-size: 0.65rem;
  padding: 0.05rem 0.4rem;
  border: 1px solid #1e2229;
  color: #6b7a90;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  border-radius: 3px;
}
.badge.deposit { color: #4a9eff; border-color: #4a9eff; }
.badge.withdraw { color: #ff5c5c; border-color: #ff5c5c; }
.badge.escrow_hold { color: #ffb454; border-color: #ffb454; }
.badge.escrow_release { color: #00d9a3; border-color: #00d9a3; }
.badge.commission { color: #b482ff; border-color: #b482ff; }

/* SUB-ROW */
.row-sub { display: flex; gap: 0.4rem; font-size: 0.7rem; flex-wrap: wrap; align-items: center; }
.meta { color: #3a4150; }
.row-link { color: #4a9eff; text-decoration: none; margin-left: auto; }
.row-link:hover { text-decoration: underline; }

/* LOAD MORE */
.load-more-wrap { display: flex; justify-content: center; margin-top: 1rem; }
.load-more-btn { background: transparent; border: 1px solid #1e2229; color: #6b7a90; padding: 0.5rem 2rem; border-radius: 4px; font-family: inherit; font-size: 0.75rem; cursor: pointer; }
.load-more-btn:hover:not(:disabled) { border-color: #00d9a3; color: #00d9a3; }
.load-more-btn:disabled { opacity: 0.5; cursor: not-allowed; }

/* CopyId style fix */
.row-main :deep(.copy-id) {
  font-size: 0.65rem;
  color: #6b7a90;
}
</style>
