<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { RouterLink } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as walletApi from '../api/wallet';
import * as withdrawalsApi from '../api/withdrawals';
import { digitsFor, fmtAmount, getCurrencies, parseAmount, symbolFor } from '../lib/money';
import CopyId from '../components/CopyId.vue';

const { t } = useI18n();

const wallets = ref<walletApi.Balance[]>([]);
const currencies = ref<walletApi.Currency[]>([]);
const transactions = ref<walletApi.Transaction[]>([]);
const pendingWithdrawals = ref<withdrawalsApi.Withdrawal[]>([]);
const loading = ref(true);
const loadingMore = ref(false);
const nextCursor = ref('');
const hasMore = ref(false);
const error = ref<string | null>(null);

// deposit (input in dollars, sent as cents)
const depCurrency = ref('USD');
const depAmountStr = ref('');
const depSubmitting = ref(false);
const depError = ref<string | null>(null);
const depSuccess = ref<string | null>(null);

// withdrawal request (input in dollars, sent as cents)
const wdCurrency = ref('USD');
const wdAmountStr = ref('');
const wdSubmitting = ref(false);
const wdError = ref<string | null>(null);
const wdSuccess = ref<string | null>(null);

const depPreview = computed(() => {
  const cents = parseAmount(depAmountStr.value, digitsFor(currencies.value, depCurrency.value));
  if (cents == null || cents <= 0) return '';
  return fmtAmount(currencies.value, cents, depCurrency.value);
});

const wdPreview = computed(() => {
  const cents = parseAmount(wdAmountStr.value, digitsFor(currencies.value, wdCurrency.value));
  if (cents == null || cents <= 0) return '';
  return fmtAmount(currencies.value, cents, wdCurrency.value);
});

async function loadWallets() {
  try {
    wallets.value = await walletApi.listWallets();
  } catch {
    error.value = t('wallet.load_failed');
  }
}

async function loadCurrencies() {
  try {
    currencies.value = await getCurrencies();
    if (currencies.value.length && !currencies.value.find((c) => c.code === 'USD')) {
      const first = currencies.value[0].code;
      depCurrency.value = first;
      wdCurrency.value = first;
    }
  } catch {
    // non-fatal
  }
}

async function loadTransactions(reset = true) {
  if (reset) {
    transactions.value = [];
    nextCursor.value = '';
    hasMore.value = false;
  } else {
    loadingMore.value = true;
  }
  try {
    const page = await walletApi.listTransactions('', reset ? '' : nextCursor.value, 30);
    transactions.value = reset ? page.transactions : [...transactions.value, ...page.transactions];
    nextCursor.value = page.next_cursor ?? '';
    hasMore.value = page.has_more;
  } catch {
    // non-fatal
  } finally {
    loadingMore.value = false;
  }
}

async function loadPendingWithdrawals() {
  try {
    const all = await withdrawalsApi.listMine(50, 0);
    pendingWithdrawals.value = all.filter((w) => w.status === 'pending');
  } catch {
    // non-fatal
  }
}

async function doDeposit() {
  depError.value = null;
  depSuccess.value = null;
  const cents = parseAmount(depAmountStr.value, digitsFor(currencies.value, depCurrency.value));
  if (cents == null || cents <= 0) {
    depError.value = t('wallet.invalid_amount');
    return;
  }
  depSubmitting.value = true;
  try {
    await walletApi.deposit(depCurrency.value, cents);
    depSuccess.value = t('wallet.deposit_ok');
    depAmountStr.value = '';
    await loadWallets();
    await loadTransactions(true);
  } catch (e: any) {
    const body = e.response?.data;
    depError.value = body?.errors
      ? Object.values(body.errors).join(', ')
      : body?.error ?? t('wallet.deposit_failed');
  } finally {
    depSubmitting.value = false;
  }
}

async function requestWithdrawal() {
  wdError.value = null;
  wdSuccess.value = null;
  const cents = parseAmount(wdAmountStr.value, digitsFor(currencies.value, wdCurrency.value));
  if (cents == null || cents <= 0) {
    wdError.value = t('wallet.invalid_amount');
    return;
  }
  wdSubmitting.value = true;
  try {
    await withdrawalsApi.request({
      currency: wdCurrency.value,
      amount: cents,
    });
    wdSuccess.value = t('wallet.withdraw_request_ok');
    wdAmountStr.value = '';
    await loadWallets();
    await loadPendingWithdrawals();
    await loadTransactions(true);
  } catch (e: any) {
    const body = e.response?.data;
    wdError.value = body?.errors
      ? Object.values(body.errors).join(', ')
      : body?.error ?? t('wallet.withdraw_failed');
  } finally {
    wdSubmitting.value = false;
  }
}

function txLabel(type: string): string {
  const map: Record<string, string> = {
    deposit: t('wallet.tx_deposit'),
    withdraw: t('wallet.tx_withdraw'),
    escrow_hold: t('wallet.tx_escrow_hold'),
    escrow_release: t('wallet.tx_escrow_release'),
    escrow_refund: t('wallet.tx_escrow_refund'),
    commission: t('wallet.tx_commission'),
    admin_adjustment: t('wallet.tx_admin'),
  };
  return map[type] ?? type;
}

function fmtDate(unix: number): string {
  return new Date(unix * 1000).toISOString().slice(0, 16).replace('T', ' ');
}

onMounted(async () => {
  loading.value = true;
  await Promise.all([
    loadCurrencies(),
    loadWallets(),
    loadPendingWithdrawals(),
    loadTransactions(true)
  ]);
  loading.value = false;
});
</script>

<template>
  <div>
    <h1 class="heading">{{ t('wallet.heading') }}</h1>

    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="err">! {{ error }}</p>

    <template v-else>
      <section class="section">
        <h2 class="section-title">{{ t('wallet.balances') }}</h2>
        <div v-if="wallets.length === 0" class="empty">
          {{ t('wallet.empty') }}
        </div>
        <div v-else class="grid">
          <div v-for="w in wallets" :key="w.currency_code" class="card">
            <div class="card-head">
              <span class="code">{{ w.currency_code }}</span>
              <span class="label">{{ t('wallet.available') }}</span>
            </div>
            <div class="amount">
              {{ symbolFor(currencies, w.currency_code) }}
              {{ fmtAmount(currencies, w.available, w.currency_code) }}
            </div>
            <div v-if="w.held > 0" class="held">
              <span class="dot"></span>
              {{ t('wallet.in_escrow') }}:
              {{ fmtAmount(currencies, w.held, w.currency_code) }}
            </div>
          </div>
        </div>
      </section>

      <section class="section">
        <h2 class="section-title">{{ t('wallet.deposit') }}</h2>
        <div class="form-row">
          <label class="field">
            <span class="label">{{ t('wallet.currency') }}</span>
            <select v-model="depCurrency">
              <option v-for="c in currencies" :key="c.code" :value="c.code">
                {{ c.code }}
              </option>
            </select>
          </label>
          <label class="field">
            <span class="label">{{ t('wallet.amount') }}</span>
            <input
              v-model="depAmountStr"
              type="text"
              inputmode="decimal"
              placeholder="0.00"
            />
          </label>
          <button class="primary" :disabled="depSubmitting" @click="doDeposit">
            {{ depSubmitting ? t('common.loading') : `> ${t('wallet.deposit')}` }}
          </button>
        </div>
        <p v-if="depPreview" class="preview">
          = {{ symbolFor(currencies, depCurrency) }} {{ depPreview }}
        </p>
        <p v-if="depError" class="err">! {{ depError }}</p>
        <p v-if="depSuccess" class="ok">{{ depSuccess }}</p>
      </section>

      <section class="section">
        <h2 class="section-title">{{ t('wallet.request_withdrawal') }}</h2>
        <p class="hint">{{ t('wallet.withdraw_hold_notice') }}</p>
        <div class="form-row">
          <label class="field">
            <span class="label">{{ t('wallet.currency') }}</span>
            <select v-model="wdCurrency">
              <option v-for="c in currencies" :key="c.code" :value="c.code">
                {{ c.code }}
              </option>
            </select>
          </label>
          <label class="field">
            <span class="label">{{ t('wallet.amount') }}</span>
            <input
              v-model="wdAmountStr"
              type="text"
              inputmode="decimal"
              placeholder="0.00"
            />
          </label>
          <button class="secondary" :disabled="wdSubmitting" @click="requestWithdrawal">
            {{ wdSubmitting ? t('common.loading') : `> ${t('wallet.request')}` }}
          </button>
        </div>
        <p v-if="wdPreview" class="preview">
          = {{ symbolFor(currencies, wdCurrency) }} {{ wdPreview }}
        </p>
        <p v-if="wdError" class="err">! {{ wdError }}</p>
        <p v-if="wdSuccess" class="ok">{{ wdSuccess }}</p>
      </section>

      <section v-if="pendingWithdrawals.length > 0" class="section">
        <h2 class="section-title">{{ t('wallet.pending_withdrawals') }}</h2>
        <div class="list">
          <div v-for="w in pendingWithdrawals" :key="w.id" class="row">
            <div class="row-main">
              <span class="row-amount">
                {{ symbolFor(currencies, w.currency_code) }}
                {{ fmtAmount(currencies, w.amount, w.currency_code) }}
                <span class="cur">{{ w.currency_code }}</span>
              </span>
              <span class="badge pending">{{ t('withdrawals.status_pending') }}</span>
            </div>
            
            <div class="row-sub">
              <div class="row-meta-group">
                <span class="meta">{{ fmtDate(w.created_at_unix) }}</span>
                <span class="meta-sep">·</span>
                <CopyId :id="w.id" label="id" />
              </div>
            </div>
          </div>
        </div>
      </section>

      <section class="section">
        <div class="section-head">
          <h2 class="section-title">{{ t('wallet.transactions') }}</h2>
          <RouterLink to="/orders" class="section-link">{{ t('wallet.view_orders') }} →</RouterLink>
        </div>
        <p v-if="transactions.length === 0" class="empty">
          {{ t('wallet.tx_empty') }}
        </p>
        <div v-else class="list">
          <div v-for="tx in transactions" :key="tx.id" class="row">
            <div class="row-main">
              <span class="row-type" :class="tx.type">{{ txLabel(tx.type) }}</span>
              <span class="row-amount" :class="tx.direction">
                {{ tx.direction === 'debit' ? '-' : '+' }}
                {{ symbolFor(currencies, tx.currency_code) }}
                {{ fmtAmount(currencies, tx.amount, tx.currency_code) }}
              </span>
            </div>
            
            <div class="row-sub">
              <div class="row-meta-group">
                <span class="meta">{{ fmtDate(tx.created_at_unix) }}</span>
                <span class="meta-sep">·</span>
                <CopyId :id="tx.id" label="tx id" />
              </div>

              <RouterLink
                v-if="tx.order_id"
                :to="{ name: 'order-detail', params: { id: tx.order_id } }"
                class="row-link"
              >
                {{ t('wallet.view_order') }} →
              </RouterLink>
            </div>
          </div>
        </div>
        <button
          v-if="hasMore"
          class="load-more"
          :disabled="loadingMore"
          @click="loadTransactions(false)"
        >
          {{ loadingMore ? t('common.loading') : t('wallet.load_more') }}
        </button>
      </section>
    </template>
  </div>
</template>

<style scoped>
.heading {
  font-size: 1.1rem;
  color: #e2e5e9;
  margin: 0 0 1.5rem;
}
.muted {
  color: #6b7a90;
}
.err {
  color: #ff5c5c;
  font-size: 0.85rem;
}
.ok {
  color: #00d9a3;
  font-size: 0.85rem;
}
.hint {
  color: #6b7a90;
  font-size: 0.7rem;
}
.empty {
  border: 1px dashed #1e2229;
  padding: 1.5rem;
  text-align: center;
  color: #6b7a90;
  border-radius: 4px;
  font-size: 0.85rem;
}
.section {
  margin-bottom: 2rem;
}
.section-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 0.75rem;
}
.section-title {
  font-size: 0.75rem;
  color: #6b7a90;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  font-weight: 500;
  margin: 0 0 0.75rem;
}
.section-head .section-title {
  margin: 0;
}
.section-link {
  color: #6b7a90;
  font-size: 0.75rem;
  text-decoration: none;
}
.section-link:hover {
  color: #00d9a3;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 0.75rem;
}
.card {
  border: 1px solid #1e2229;
  background: rgba(15, 17, 21, 0.85);
  border-radius: 6px;
  padding: 1rem;
}
.card-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 0.5rem;
}
.code {
  color: #00d9a3;
  font-weight: 600;
}
.label {
  color: #6b7a90;
  font-size: 0.7rem;
  text-transform: lowercase;
}
.amount {
  font-size: 1.5rem;
  font-weight: 600;
  color: #e2e5e9;
}
.held {
  color: #6b7a90;
  font-size: 0.75rem;
  margin-top: 0.5rem;
  display: flex;
  align-items: center;
  gap: 0.375rem;
}
.dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  background: #ffb454;
  border-radius: 50%;
}
.form-row {
  display: grid;
  grid-template-columns: 120px 1fr auto;
  gap: 0.5rem;
  align-items: end;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}
.preview {
  color: #c8d3e0;
  font-size: 0.85rem;
  margin: 0.5rem 0 0;
  padding: 0.4rem 0.75rem;
  background: #0a0e14;
  border-left: 2px solid #00d9a3;
  border-radius: 0 4px 4px 0;
}
.primary {
  background: #00d9a3;
  color: #0a0e14;
  border: none;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.85rem;
  font-weight: bold;
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
.secondary {
  background: transparent;
  color: #e2e5e9;
  border: 1px solid #1e2229;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.85rem;
  font-weight: 600;
  padding: 0.5rem 1rem;
  cursor: pointer;
}
.secondary:hover:not(:disabled) {
  border-color: #00d9a3;
  color: #00d9a3;
}
.secondary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
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
  padding: 0.6rem 0.75rem;
}
.row-main {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.75rem;
}

/* --- UPDATED: COLOR CODING FOR TRANSACTIONS --- */
.row-type {
  font-size: 0.85rem;
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: 0.4rem;
}
.row-type::before {
  content: '';
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
  box-shadow: 0 0 6px currentColor;
}

/* 1. Deposits & Escrow Release (Incoming Money) = Green */
.row-type.deposit,
.row-type.escrow_release {
  color: #00d9a3;
}

/* 2. Withdraws & Escrow Holds (Outgoing/Locked) = Red/Orange */
.row-type.withdraw {
  color: #ff5c5c;
}
.row-type.escrow_hold {
  color: #ffb454;
}

/* 3. Refunds = Blue */
.row-type.escrow_refund {
  color: #4a9eff;
}

/* 4. Commissions & Admin Adjustments = Purple */
.row-type.commission,
.row-type.admin_adjustment {
  color: #b482ff;
}

.row-amount {
  font-weight: 600;
  font-size: 0.9rem;
  color: #e2e5e9;
}
.row-amount.credit {
  color: #00d9a3;
}
.row-amount.debit {
  color: #ff5c5c;
}
.cur {
  color: #6b7a90;
  font-size: 0.7rem;
  font-weight: normal;
  margin-left: 0.25rem;
}
.badge {
  font-size: 0.7rem;
  padding: 0.1rem 0.5rem;
  border-radius: 4px;
  border: 1px solid #1e2229;
  color: #6b7a90;
}
.badge.pending {
  color: #ffb454;
  border-color: #ffb454;
}
.row-sub {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 0.4rem;
  gap: 0.75rem;
}
.row-meta-group {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}
.meta {
  color: #3a4150;
  font-size: 0.7rem;
}
.meta-sep {
  color: #3a4150;
  font-size: 0.7rem;
}
.row-meta-group :deep(.copy-id) {
  font-size: 0.65rem;
  color: #6b7a90;
  background: transparent;
  padding: 0;
}
.row-link {
  color: #4a9eff;
  font-size: 0.75rem;
  text-decoration: none;
}
.row-link:hover {
  text-decoration: underline;
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
  margin-top: 0.75rem;
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
  .form-row {
    grid-template-columns: 1fr;
  }
  .form-row button {
    width: 100%;
  }
}
</style>
