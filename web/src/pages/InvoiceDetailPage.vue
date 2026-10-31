<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { RouterLink, useRoute, useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as invoicesApi from '../api/invoices';
import * as walletApi from '../api/wallet';
import { fmtAmount, symbolFor } from '../lib/money';
import { useAuthStore } from '../stores/auth';
import CopyId from '../components/CopyId.vue';

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const auth = useAuthStore();

const invoice = ref<invoicesApi.InvoiceItem | null>(null);
const currencies = ref<walletApi.Currency[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);
const actionError = ref<string | null>(null);
const actionSuccess = ref<string | null>(null);
const busy = ref(false);

const cancelOpen = ref(false);
const cancelReason = ref('');
const cancelError = ref<string | null>(null);

async function load() {
  loading.value = true;
  error.value = null;
  try {
    invoice.value = await invoicesApi.get(route.params.id as string);
    console.log(invoice.value)

  } catch (e: any) {
    error.value = e.response?.data?.error ?? t('invoices.load_failed');
  } finally {
    loading.value = false;
  }
}

async function loadCurrencies() {
  try {
    currencies.value = await walletApi.listCurrencies();
  } catch {
    // non-fatal
  }
}

async function pay() {
  if (!invoice.value) return;
  actionError.value = null;
  actionSuccess.value = null;
  busy.value = true;
  try {
    const o = await invoicesApi.pay(invoice.value.id);
    router.push({ name: 'order-detail', params: { id: o.id } });
  } catch (e: any) {
    actionError.value = e.response?.data?.error ?? t('invoices.pay_failed');
  } finally {
    busy.value = false;
  }
}

function openCancel() {
  cancelReason.value = '';
  cancelError.value = null;
  cancelOpen.value = true;
}

function closeCancel() {
  cancelOpen.value = false;
  cancelReason.value = '';
  cancelError.value = null;
}

async function confirmCancel() {
  if (!invoice.value) return;
  cancelError.value = null;
  if (cancelReason.value.trim().length < 3) {
    cancelError.value = t('invoices.cancel_reason_required');
    return;
  }
  busy.value = true;
  try {
    invoice.value = await invoicesApi.cancel(invoice.value.id, cancelReason.value.trim());
    actionSuccess.value = t('invoices.cancel_ok');
    closeCancel();
  } catch (e: any) {
    cancelError.value = e.response?.data?.error ?? t('invoices.cancel_failed');
  } finally {
    busy.value = false;
  }
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
  return new Date(unix * 1000).toISOString().replace('T', ' ').slice(0, 19);
}

const isBuyer = computed(() => invoice.value?.buyer_id === auth.user?.id);
const isSeller = computed(() => invoice.value?.seller_id === auth.user?.id);
const canPay = computed(() => isBuyer.value && invoice.value?.status === 'pending');
const canCancel = computed(() => invoice.value?.status === 'pending');
const canDeliver = computed(() => isSeller.value && invoice.value?.status === 'held');

watch(
  () => route.params.id,
  () => load()
);

onMounted(() => {
  load();
  loadCurrencies();
});
</script>

<template>
  <div>
    <RouterLink to="/invoices" class="back">{{ t('invoices.back') }}</RouterLink>

    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="err">! {{ error }}</p>

    <article v-else-if="invoice" class="detail">
      <header class="head">
        <h1 class="heading">{{ invoice.invoice_title }}</h1>
        <div class="head-right">
          <CopyId :id="invoice.id" label="invoice id" />
          <span class="status-badge" :class="invoice.status">{{ statusLabel(invoice.status) }}</span>
        </div>
      </header>

      <div class="price-block">
        <div class="amount">
          {{ symbolFor(currencies, invoice.currency_code) }}
          {{ fmtAmount(currencies, invoice.amount, invoice.currency_code) }}
        </div>
        <div class="cur">{{ invoice.currency_code }}</div>
      </div>

      <p v-if="invoice.invoice_description" class="description">
        {{ invoice.invoice_description }}
      </p>

      <dl class="meta">
        <div class="meta-row">
          <dt>{{ t('invoices.role_buyer') }}</dt>
          <dd>
            <RouterLink
              :to="{ name: 'public-profile', params: { username: invoice.buyer_username } }"
              class="userlink"
            >@{{ invoice.buyer_username }}</RouterLink>
            <span v-if="isBuyer" class="you">({{ t('orders.role_you') }})</span>
          </dd>
        </div>
        <div class="meta-row">
          <dt>{{ t('invoices.role_seller') }}</dt>
          <dd>
            <RouterLink
              :to="{ name: 'public-profile', params: { username: invoice.seller_username } }"
              class="userlink"
            >@{{ invoice.seller_username }}</RouterLink>
            <span v-if="isSeller" class="you">({{ t('orders.role_you') }})</span>
          </dd>
        </div>
        <div class="meta-row">
          <dt>{{ t('invoices.created') }}</dt>
          <dd>{{ fmtDate(invoice.created_at_unix) }}</dd>
        </div>
        <div v-if="invoice.cancel_reason" class="meta-row">
          <dt>{{ t('invoices.cancel_reason') }}</dt>
          <dd>{{ invoice.cancel_reason }}</dd>
        </div>
      </dl>

      <div v-if="canPay" class="notice">
        {{ t('invoices.escrow_notice') }}
      </div>

      <div class="actions">
        <button v-if="canPay" class="primary" :disabled="busy" @click="pay">
          {{ busy ? t('common.loading') : `> ${t('invoices.pay_now')}` }}
        </button>
        <button v-if="canCancel" class="danger" :disabled="busy" @click="openCancel">
          {{ t('invoices.cancel') }}
        </button>
        <RouterLink
          v-if="canDeliver"
          :to="{ name: 'order-detail', params: { id: invoice.id } }"
          class="primary link-btn"
        >
          {{ t('invoices.go_to_order_deliver') }}
        </RouterLink>
      </div>

      <p v-if="actionError" class="err">! {{ actionError }}</p>
      <p v-if="actionSuccess" class="ok">{{ actionSuccess }}</p>
    </article>

    <div v-if="cancelOpen" class="modal-bg" @click.self="closeCancel">
      <div class="modal">
        <h2 class="modal-title">{{ t('invoices.cancel_title') }}</h2>
        <p class="modal-hint">{{ t('invoices.cancel_hint') }}</p>
        <label class="field">
          <span class="label">{{ t('invoices.cancel_reason') }}</span>
          <textarea v-model="cancelReason" rows="4" maxlength="1000" />
        </label>
        <p v-if="cancelError" class="err">! {{ cancelError }}</p>
        <div class="modal-actions">
          <button class="danger" :disabled="busy" @click="confirmCancel">
            {{ busy ? t('common.loading') : `> ${t('invoices.cancel')}` }}
          </button>
          <button class="ghost" @click="closeCancel">{{ t('common.cancel') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.head-right {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 1rem;
  gap: 1rem;
  flex-wrap: wrap;
}
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
.ok {
  color: #00d9a3;
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
  margin-bottom: 1rem;
  gap: 1rem;
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
}
.price-block {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  margin-bottom: 1rem;
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
.description {
  color: #c8d3e0;
  font-size: 0.9rem;
  white-space: pre-wrap;
  margin: 0 0 1rem;
  line-height: 1.55;
}
.meta {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin: 0 0 1.25rem;
  border-top: 1px solid #1e2229;
  padding-top: 1rem;
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
}
.you {
  color: #00d9a3;
  font-size: 0.75rem;
  margin-left: 0.25rem;
}
.userlink {
  color: #4a9eff;
  text-decoration: none;
}
.userlink:hover {
  text-decoration: underline;
}
.notice {
  border-left: 2px solid #4a9eff;
  background: rgba(74, 158, 255, 0.05);
  color: #4a9eff;
  font-size: 0.85rem;
  padding: 0.6rem 0.75rem;
  border-radius: 0 4px 4px 0;
  margin-bottom: 1rem;
  line-height: 1.55;
}
.actions {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1rem;
  flex-wrap: wrap;
}
.primary {
  background: #00d9a3;
  color: #0a0e14;
  border: none;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.9rem;
  font-weight: 600;
  padding: 0.5rem 1.25rem;
  cursor: pointer;
}
.primary:hover:not(:disabled) {
  background: #00f5aa;
}
.primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.link-btn {
  text-decoration: none;
  display: inline-flex;
  align-items: center;
}
.danger {
  background: transparent;
  color: #ff5c5c;
  border: 1px solid #ff5c5c;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.85rem;
  padding: 0.5rem 1rem;
  cursor: pointer;
}
.danger:hover:not(:disabled) {
  background: rgba(255, 92, 92, 0.1);
}
.danger:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.ghost {
  background: transparent;
  color: #6b7a90;
  border: 1px solid #1e2229;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.85rem;
  padding: 0.5rem 1rem;
  cursor: pointer;
}
.ghost:hover {
  border-color: #00d9a3;
  color: #00d9a3;
}
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
  border-radius: 6px;
  padding: 1.5rem;
  width: 460px;
  max-width: 90vw;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
}
.modal-title {
  font-size: 1rem;
  color: #ff5c5c;
  font-weight: 600;
  margin: 0 0 0.5rem;
}
.modal-hint {
  color: #6b7a90;
  font-size: 0.85rem;
  margin: 0 0 1rem;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
  margin-bottom: 1rem;
}
.label {
  color: #6b7a90;
  font-size: 0.7rem;
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
  border-color: #ff5c5c;
  box-shadow: 0 0 0 2px rgba(255, 92, 92, 0.25);
}
.modal-actions {
  display: flex;
  gap: 0.5rem;
}
</style>
