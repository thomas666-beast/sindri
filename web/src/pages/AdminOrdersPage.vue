<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';
import { RouterLink } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as adminApi from '../api/admin';
import * as walletApi from '../api/wallet';
import type { Order } from '../api/orders';
import { fmtAmount, symbolFor } from '../lib/money';
import SearchById from '../components/SearchById.vue';

const { t } = useI18n();

const searchId = ref('');
const orders = ref<Order[]>([]);
const currencies = ref<walletApi.Currency[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);
const actionError = ref<string | null>(null);
const actionSuccess = ref<string | null>(null);
const busyId = ref<string | null>(null);

// Pagination State
const limit = 10;
const offset = ref(0);
const hasMore = ref(true);
const loadingMore = ref(false);

// Modal State for Resolution
const resolveModal = ref(false);
const resolveTarget = ref<Order | null>(null);
const resolveAction = ref<'release' | 'refund'>('release');
const resolveBusy = ref(false);

async function load(reset = true) {
  if (reset) {
    loading.value = true;
    error.value = null;
    orders.value = [];
    offset.value = 0;
    hasMore.value = true;
  } else {
    loadingMore.value = true;
  }

  try {
    const res = await adminApi.listDisputedOrders(limit, offset.value, searchId.value);
    
    if (reset) {
      orders.value = res;
    } else {
      orders.value = [...orders.value, ...res];
    }

    // If we got fewer results than the limit, there are no more pages
    if (res.length < limit) {
      hasMore.value = false;
    } else {
      offset.value += limit;
    }
  } catch {
    error.value = t('admin.disputes_load_failed');
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

function openResolve(order: Order, action: 'release' | 'refund') {
  resolveTarget.value = order;
  resolveAction.value = action;
  resolveModal.value = true;
}

function closeResolve() {
  resolveModal.value = false;
  resolveTarget.value = null;
}

async function submitResolve() {
  if (!resolveTarget.value) return;
  resolveBusy.value = true;
  actionError.value = null;
  actionSuccess.value = null;
  
  try {
    if (resolveAction.value === 'release') {
      await adminApi.releaseOrder(resolveTarget.value.id);
      actionSuccess.value = t('admin.disputes_release_ok');
    } else {
      await adminApi.refundOrder(resolveTarget.value.id);
      actionSuccess.value = t('admin.disputes_refund_ok');
    }
    closeResolve();
    await load(true); // Reset the list after resolving
  } catch (e: any) {
    actionError.value = e.response?.data?.error ?? 'Action failed';
  } finally {
    resolveBusy.value = false;
  }
}

function fmtDate(unix: number): string {
  return new Date(unix * 1000).toISOString().replace('T', ' ').slice(0, 19);
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
      <h1 class="heading">{{ t('admin.disputes_heading') }}</h1>
      <SearchById v-model="searchId" />
    </header>

    <p v-if="actionError" class="err">! {{ actionError }}</p>
    <p v-if="actionSuccess" class="ok">{{ actionSuccess }}</p>

    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="err">! {{ error }}</p>
    <p v-else-if="orders.length === 0" class="empty">
      {{ t('admin.disputes_empty') }}
    </p>

    <div v-else class="list">
      <article v-for="o in orders" :key="o.id" class="card">
        
        <!-- Card Header -->
        <header class="card-head">
          <div class="header-left">
            <span class="amount">
              {{ symbolFor(currencies, o.currency_code) }}
              {{ fmtAmount(currencies, o.amount, o.currency_code) }}
              <span class="cur">{{ o.currency_code }}</span>
            </span>
            <span class="status-badge">{{ t('orders.status_disputed') }}</span>
          </div>
          <span class="meta">ID: {{ o.id.slice(0, 8) }} · {{ fmtDate(o.created_at_unix) }}</span>
        </header>

        <!-- Parties involved -->
        <div class="parties">
          <div class="party">
            <span class="label">{{ t('orders.role_buyer') }}</span>
            <RouterLink
              :to="{ name: 'public-profile', params: { username: o.buyer_username } }"
              class="user-link"
            >
              @{{ o.buyer_username }}
            </RouterLink>
          </div>
          <div class="party">
            <span class="label">{{ t('orders.role_seller') }}</span>
            <RouterLink
              :to="{ name: 'public-profile', params: { username: o.seller_username } }"
              class="user-link"
            >
              @{{ o.seller_username }}
            </RouterLink>
          </div>
        </div>

        <!-- Dispute Reason -->
        <div v-if="o.dispute_reason" class="dispute-box">
          <div class="dispute-head">
            <span class="dispute-label">{{ t('admin.disputes_reason') }}</span>
            <span v-if="o.dispute_opened_by_username" class="dispute-by">
              — opened by @{{ o.dispute_opened_by_username }}
            </span>
          </div>
          <p class="dispute-text">{{ o.dispute_reason }}</p>
        </div>

        <!-- Evidence (Links to Chat and Files) -->
        <div class="evidence-links">
          <RouterLink 
            :to="{ name: 'order-detail', params: { id: o.id }, query: { from: 'admin' } }" 
            class="evidence-btn"
          >
            📄 View Order & Files
          </RouterLink>
          <span class="evidence-hint">Check the delivered files before resolving.</span>
        </div>

        <!-- Resolution Actions -->
        <footer class="card-foot">
          <button
            class="btn-release"
            :disabled="busyId === o.id"
            @click="openResolve(o, 'release')"
          >
            {{ t('admin.disputes_release') }} (Seller Wins)
          </button>
          <button
            class="btn-refund"
            :disabled="busyId === o.id"
            @click="openResolve(o, 'refund')"
          >
            {{ t('admin.disputes_refund') }} (Buyer Wins)
          </button>
        </footer>
      </article>

      <!-- PAGINATION BUTTON -->
      <div v-if="hasMore" class="load-more-wrap">
        <button class="load-more-btn" :disabled="loadingMore" @click="load(false)">
          {{ loadingMore ? t('common.loading') : 'Load More Disputes' }}
        </button>
      </div>
    </div>

    <!-- Resolution Modal -->
    <div v-if="resolveModal" class="modal-bg" @click.self="closeResolve">
      <div class="modal">
        <h2 class="modal-title">
          {{ resolveAction === 'release' ? 'Release Funds to Seller' : 'Refund Buyer' }}
        </h2>
        <p class="modal-hint">
          Are you sure? This action cannot be undone and will close the dispute permanently.
        </p>
        
        <div v-if="resolveTarget" class="modal-summary">
          <div class="summary-row">
            <span>Amount:</span>
            <strong>{{ symbolFor(currencies, resolveTarget.currency_code) }} {{ fmtAmount(currencies, resolveTarget.amount, resolveTarget.currency_code) }}</strong>
          </div>
          <div class="summary-row">
            <span>Winner:</span>
            <strong>{{ resolveAction === 'release' ? `@${resolveTarget.seller_username}` : `@${resolveTarget.buyer_username}` }}</strong>
          </div>
        </div>

        <p v-if="actionError" class="err">! {{ actionError }}</p>

        <div class="modal-actions">
          <button 
            class="primary" 
            :class="{ danger: resolveAction === 'refund' }"
            :disabled="resolveBusy" 
            @click="submitResolve"
          >
            {{ resolveBusy ? t('common.loading') : 'Confirm Resolution' }}
          </button>
          <button class="ghost" @click="closeResolve">{{ t('common.cancel') }}</button>
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
  margin-bottom: 1.5rem;
  flex-wrap: wrap;
  gap: 1rem;
}

.heading {
  font-size: 1.25rem;
  color: #e2e5e9;
  margin: 0;
  font-weight: 600;
}

.muted { color: #6b7a90; }
.err { color: #ff5c5c; font-size: 0.85rem; margin-bottom: 1rem; }
.ok { color: #00d9a3; font-size: 0.85rem; margin-bottom: 1rem; }

.empty {
  border: 1px dashed #1e2229;
  padding: 3rem;
  text-align: center;
  color: #6b7a90;
  border-radius: 8px;
}

.list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

/* CARD DESIGN */
.card {
  border: 1px solid #1e2229;
  background: rgba(15, 17, 21, 0.85);
  border-radius: 8px;
  padding: 1.5rem;
  position: relative;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

.card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  width: 4px;
  height: 100%;
  background: #ff5c5c;
  border-radius: 8px 0 0 8px;
}

.card-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.25rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid #1e2229;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.amount {
  color: #e2e5e9;
  font-weight: 700;
  font-size: 1.3rem;
}

.cur {
  color: #6b7a90;
  font-size: 0.8rem;
  font-weight: normal;
  margin-left: 0.25rem;
}

.status-badge {
  font-size: 0.75rem;
  padding: 0.2rem 0.6rem;
  border: 1px solid #ff5c5c;
  color: #ff5c5c;
  border-radius: 4px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 600;
}

.meta {
  color: #6b7a90;
  font-size: 0.75rem;
  font-family: monospace;
}

/* PARTIES */
.parties {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1.5rem;
  margin-bottom: 1rem;
}

.party {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.party .label {
  color: #6b7a90;
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.user-link {
  color: #4a9eff;
  font-size: 1rem;
  font-weight: 600;
  text-decoration: none;
  width: fit-content;
}
.user-link:hover {
  text-decoration: underline;
}

/* DISPUTE REASON */
.dispute-box {
  background: rgba(255, 92, 92, 0.05);
  border: 1px solid rgba(255, 92, 92, 0.2);
  padding: 1rem;
  border-radius: 6px;
  margin-bottom: 1rem;
}

.dispute-head {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
}

.dispute-label {
  color: #ff5c5c;
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 600;
}

.dispute-by {
  color: #6b7a90;
  font-size: 0.75rem;
}

.dispute-text {
  color: #e2e5e9;
  font-size: 0.95rem;
  margin: 0;
  white-space: pre-wrap;
  line-height: 1.5;
}

/* EVIDENCE */
.evidence-links {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1.5rem;
}

.evidence-btn {
  background: #0a0e14;
  border: 1px solid #1e2229;
  color: #e2e5e9;
  padding: 0.6rem 1rem;
  border-radius: 6px;
  text-decoration: none;
  font-size: 0.85rem;
  font-weight: 500;
  transition: border-color 0.1s;
}
.evidence-btn:hover {
  border-color: #4a9eff;
  color: #4a9eff;
}

.evidence-hint {
  color: #6b7a90;
  font-size: 0.75rem;
  font-style: italic;
}

/* ACTIONS */
.card-foot {
  display: flex;
  gap: 1rem;
}

.btn-release, .btn-refund {
  flex: 1;
  padding: 0.75rem;
  border-radius: 6px;
  font-family: inherit;
  font-size: 0.9rem;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.15s;
}

.btn-release {
  background: #00d9a3;
  color: #0a0e14;
  border: none;
}
.btn-release:hover:not(:disabled) {
  background: #00f5aa;
}

.btn-refund {
  background: transparent;
  color: #ff5c5c;
  border: 1px solid #ff5c5c;
}
.btn-refund:hover:not(:disabled) {
  background: rgba(255, 92, 92, 0.1);
}

.btn-release:disabled, .btn-refund:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* LOAD MORE */
.load-more-wrap {
  display: flex;
  justify-content: center;
  margin-top: 1rem;
  margin-bottom: 2rem;
}

.load-more-btn {
  background: transparent;
  border: 1px solid #1e2229;
  color: #6b7a90;
  padding: 0.75rem 2rem;
  border-radius: 6px;
  font-family: inherit;
  font-size: 0.9rem;
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

/* MODAL */
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
  border-radius: 8px;
  padding: 2rem;
  width: 480px;
  max-width: 90vw;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.6);
}

.modal-title {
  color: #e2e5e9;
  font-size: 1.2rem;
  font-weight: 600;
  margin: 0 0 0.5rem;
}

.modal-hint {
  color: #6b7a90;
  font-size: 0.9rem;
  margin: 0 0 1.5rem;
  line-height: 1.5;
}

.modal-summary {
  background: #0a0e14;
  border: 1px solid #1e2229;
  border-radius: 6px;
  padding: 1rem;
  margin-bottom: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.summary-row {
  display: flex;
  justify-content: space-between;
  font-size: 0.9rem;
}

.summary-row span { color: #6b7a90; }
.summary-row strong { color: #e2e5e9; }

.modal-actions {
  display: flex;
  gap: 0.75rem;
}

.primary {
  flex: 1;
  background: #00d9a3;
  color: #0a0e14;
  border: none;
  padding: 0.75rem;
  border-radius: 6px;
  font-weight: bold;
  font-size: 0.95rem;
  cursor: pointer;
}

.primary.danger {
  background: #ff5c5c;
  color: #fff;
}
.primary.danger:hover:not(:disabled) {
  background: #ff4545;
}

.primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.ghost {
  background: transparent;
  color: #6b7a90;
  border: 1px solid #1e2229;
  padding: 0.75rem 1.25rem;
  border-radius: 6px;
  font-weight: 500;
  font-size: 0.95rem;
  cursor: pointer;
}
.ghost:hover {
  border-color: #e2e5e9;
  color: #e2e5e9;
}

@media (max-width: 700px) {
  .parties { grid-template-columns: 1fr; gap: 0.5rem; }
  .card-foot { flex-direction: column; }
}
</style>
