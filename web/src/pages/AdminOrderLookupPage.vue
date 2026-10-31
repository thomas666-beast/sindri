<script setup lang="ts">
import { ref } from 'vue';
import { RouterLink } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as adminApi from '../api/admin';
import * as walletApi from '../api/wallet';
import type { Order } from '../api/orders';
import { fmtAmount, symbolFor } from '../lib/money';

const { t } = useI18n();

const searchId = ref('');
const order = ref<Order | null>(null);
const currencies = ref<walletApi.Currency[]>([]);
const loading = ref(false);
const error = ref<string | null>(null);

async function loadCurrencies() {
  try {
    currencies.value = await walletApi.listCurrencies();
  } catch {
    // non-fatal
  }
}

async function lookup() {
  const id = searchId.value.trim();
  if (!id) return;

  loading.value = true;
  error.value = null;
  order.value = null;

  try {
    order.value = await adminApi.getOrder(id);
  } catch (e: any) {
    if (e.response?.status === 404) {
      error.value = t('admin.lookup_not_found');
    } else {
      error.value = e.response?.data?.error ?? t('admin.lookup_failed');
    }
  } finally {
    loading.value = false;
  }
}

function statusLabel(s: string): string {
  switch (s) {
    case 'held': return t('orders.status_held');
    case 'completed': return t('orders.status_completed');
    case 'refunded': return t('orders.status_refunded');
    case 'pending': return t('orders.status_pending');
    case 'cancelled': return t('orders.status_cancelled');
    case 'disputed': return t('orders.status_disputed');
  }
  return s;
}

function fmtDate(unix: number): string {
  return new Date(unix * 1000).toISOString().replace('T', ' ').slice(0, 19);
}

loadCurrencies();
</script>

<template>
  <div>
    <header class="page-head">
      <h1 class="heading">{{ t('admin.lookup_heading') }}</h1>
    </header>

    <div class="search-bar">
      <input
        v-model="searchId"
        type="text"
        :placeholder="t('admin.lookup_placeholder')"
        @keydown.enter="lookup"
      />
      <button class="primary" :disabled="loading || !searchId.trim()" @click="lookup">
        {{ loading ? t('common.loading') : t('admin.lookup_btn') }}
      </button>
    </div>

    <p v-if="error" class="err">! {{ error }}</p>

    <article v-if="order" class="detail">
      <header class="head">
        <h2 class="order-id">{{ order.id }}</h2>
        <span class="status" :class="order.status">{{ statusLabel(order.status) }}</span>
      </header>

      <div class="price-block">
        <div class="amount">
          {{ symbolFor(currencies, order.currency_code) }}
          {{ fmtAmount(currencies, order.amount, order.currency_code) }}
        </div>
        <div class="cur">{{ order.currency_code }}</div>
      </div>

      <dl class="meta">
        <div class="meta-row">
          <dt>{{ t('orders.role_buyer') }}</dt>
          <dd>
            <RouterLink
              v-if="order.buyer_username"
              :to="{ name: 'public-profile', params: { username: order.buyer_username } }"
              class="userlink"
            >@{{ order.buyer_username }}</RouterLink>
          </dd>
        </div>
        <div class="meta-row">
          <dt>{{ t('orders.role_seller') }}</dt>
          <dd>
            <RouterLink
              v-if="order.seller_username"
              :to="{ name: 'public-profile', params: { username: order.seller_username } }"
              class="userlink"
            >@{{ order.seller_username }}</RouterLink>
          </dd>
        </div>
        <div v-if="order.product_id" class="meta-row">
          <dt>product id</dt>
          <dd>
            <RouterLink
              :to="{ name: 'product-detail', params: { id: order.product_id } }"
              class="userlink"
            >
              {{ order.product_id }}
            </RouterLink>
          </dd>
        </div>
        <div class="meta-row">
          <dt>{{ t('orders.amount') }}</dt>
          <dd>{{ order.amount }}</dd>
        </div>
        <div class="meta-row">
          <dt>{{ t('orders.commission') }}</dt>
          <dd>{{ order.commission_amount }} ({{ order.commission_percent }}%)</dd>
        </div>
        <div class="meta-row">
          <dt>{{ t('orders.created') }}</dt>
          <dd>{{ fmtDate(order.created_at_unix) }}</dd>
        </div>
      </dl>

      <div class="actions">
        <RouterLink
          :to="{ name: 'order-detail', params: { id: order.id }, query: { from: 'admin' } }"
          class="ghost"
        >
          {{ t('admin.lookup_view_full') }}
        </RouterLink>
      </div>
    </article>
  </div>
</template>

<style scoped>
.page-head {
  margin-bottom: 1.5rem;
}
.heading {
  font-size: 1.1rem;
  color: #e2e5e9;
  margin: 0;
}
.search-bar {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1.5rem;
}
.search-bar input {
  flex: 1;
  background: #0a0e14;
  border: 1px solid #1e2229;
  color: #c8d3e0;
  padding: 0.6rem 0.9rem;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.9rem;
}
.search-bar input:focus {
  outline: none;
  border-color: #00d9a3;
}
.primary {
  background: #00d9a3;
  color: #0a0e14;
  border: none;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.9rem;
  font-weight: bold;
  padding: 0 1.5rem;
  cursor: pointer;
}
.primary:hover:not(:disabled) {
  background: #00f5aa;
}
.primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.err {
  color: #ff5c5c;
  font-size: 0.85rem;
  margin-bottom: 1rem;
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
  margin-bottom: 1rem;
  gap: 1rem;
}
.order-id {
  font-family: monospace;
  font-size: 1rem;
  color: #4a9eff;
  margin: 0;
  word-break: break-all;
}
.status {
  font-size: 0.75rem;
  padding: 0.15rem 0.6rem;
  border-radius: 4px;
  border: 1px solid #1e2229;
  color: #6b7a90;
}
.status.held { color: #ffb454; border-color: #ffb454; }
.status.disputed { color: #ff5c5c; border-color: #ff5c5c; }
.status.completed { color: #00d9a3; border-color: #00d9a3; }
.price-block {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  margin-bottom: 1.5rem;
}
.amount {
  font-size: 2rem;
  font-weight: 600;
  color: #e2e5e9;
}
.cur {
  color: #6b7a90;
  font-size: 0.85rem;
}
.meta {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  border-top: 1px solid #1e2229;
  padding-top: 1rem;
  margin-bottom: 1.5rem;
}
.meta-row {
  display: flex;
  justify-content: space-between;
  font-size: 0.85rem;
  gap: 1rem;
}
.meta-row dt {
  color: #6b7a90;
}
.meta-row dd {
  color: #e2e5e9;
  margin: 0;
  text-align: right;
  word-break: break-all;
}
.userlink {
  color: #4a9eff;
  text-decoration: none;
}
.userlink:hover {
  text-decoration: underline;
}
.ghost {
  color: #6b7a90;
  text-decoration: none;
  font-size: 0.85rem;
  padding: 0.5rem 1rem;
  border: 1px solid #1e2229;
  border-radius: 4px;
  display: inline-block;
}
.ghost:hover {
  border-color: #00d9a3;
  color: #00d9a3;
}
</style>
