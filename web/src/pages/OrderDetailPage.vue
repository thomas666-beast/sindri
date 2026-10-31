<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { RouterLink, useRoute } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as ordersApi from '../api/orders';
import * as adminApi from '../api/admin';
import * as walletApi from '../api/wallet';
import * as reviewsApi from '../api/reviews';
import { fmtAmount, symbolFor } from '../lib/money';
import { useAuthStore } from '../stores/auth';
import ReviewForm from '../components/ReviewForm.vue';
import CopyId from '../components/CopyId.vue';

const { t } = useI18n();
const route = useRoute();
const auth = useAuthStore();

const order = ref<ordersApi.Order | null>(null);
const currencies = ref<walletApi.Currency[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);
const confirming = ref(false);
const actionError = ref<string | null>(null);
const actionSuccess = ref<string | null>(null);

// Tunnel State
const deliverables = ref<ordersApi.Deliverable[]>([]);
const uploading = ref(false);
const sealing = ref(false);

const existingReview = ref<reviewsApi.Review | null>(null);
const showReviewForm = ref(false);
const checkingReview = ref(false);

// dispute modal
const disputeOpen = ref(false);
const disputeReason = ref('');
const disputeBusy = ref(false);
const disputeError = ref<string | null>(null);

async function load() {
  loading.value = true;
  error.value = null;
  try {
    try {
      order.value = await ordersApi.get(route.params.id as string);
    } catch (e: any) {
      if (e.response?.status === 403 && auth?.user?.is_admin) {
        order.value = await adminApi.getOrder(route.params.id as string);
      } else {
        throw e;
      }
    }
    await Promise.all([checkReview(), loadDeliverables()]);
  } catch (e: any) {
    const body = e.response?.data;
    error.value = body?.error ?? t('orders.load_one_failed');
  } finally {
    loading.value = false;
  }
}

async function loadDeliverables() {
  if (!order.value) return;
  try {
    deliverables.value = await ordersApi.listDeliverables(order.value.id);
  } catch {}
}

async function checkReview() {
  if (!order.value) return;
  if (order.value.my_role !== 'buyer' || order.value.status !== 'completed') return;
  checkingReview.value = true;
  try {
    if (order.value.product_id) {
      const page = await reviewsApi.listByProduct(order.value.product_id, 100, 0);
      const found = page.reviews.find(
        (r) => r.order_id === order.value?.id && r.buyer_id === auth?.user?.id
      );
      existingReview.value = found ?? null;
    }
  } catch {
  } finally {
    checkingReview.value = false;
  }
}

async function loadCurrencies() {
  try {
    currencies.value = await walletApi.listCurrencies();
  } catch {}
}

async function confirm() {
  if (!order.value) return;
  actionError.value = null;
  actionSuccess.value = null;
  confirming.value = true;
  try {
    order.value = await ordersApi.confirm(order.value.id);
    actionSuccess.value = t('orders.confirm_ok');
  } catch (e: any) {
    const body = e.response?.data;
    actionError.value = body?.errors
      ? Object.values(body.errors).join(', ')
      : body?.error ?? t('orders.confirm_failed');
  } finally {
    confirming.value = false;
  }
}

async function handleFileUpload(e: Event) {
  if (!order.value) return;
  const file = (e.target as HTMLInputElement).files?.[0];
  if (!file) return;
  uploading.value = true;
  actionError.value = null;
  try {
    await ordersApi.uploadDeliverable(order.value.id, file);
    await loadDeliverables();
  } catch (err: any) {
    actionError.value = err.response?.data?.error ?? 'Failed to upload file';
  } finally {
    uploading.value = false;
    (e.target as HTMLInputElement).value = '';
  }
}

async function seal() {
  if (!order.value || deliverables.value.length === 0) return;
  sealing.value = true;
  actionError.value = null;
  try {
    await ordersApi.sealDelivery(order.value.id);
    order.value = {
      ...order.value,
      status: 'delivered'
    };
    actionSuccess.value = t('orders.sealed_ok');
  } catch (err: any) {
    actionError.value = err.response?.data?.error ?? 'Failed to seal delivery';
  } finally {
    sealing.value = false;
  }
}

function openDispute() {
  disputeReason.value = '';
  disputeError.value = null;
  disputeOpen.value = true;
}

function closeDispute() {
  disputeOpen.value = false;
  disputeReason.value = '';
  disputeError.value = null;
}

async function submitDispute() {
  if (!order.value) return;
  disputeError.value = null;
  if (disputeReason.value.trim().length < 3) {
    disputeError.value = t('orders.dispute_reason_required');
    return;
  }
  disputeBusy.value = true;
  try {
    order.value = await ordersApi.dispute(order.value.id, disputeReason.value.trim());
    actionSuccess.value = t('orders.dispute_ok');
    closeDispute();
  } catch (e: any) {
    const body = e.response?.data;
    if (body?.errors) {
      disputeError.value = Object.values(body.errors).join(', ');
    } else {
      disputeError.value = body?.error ?? t('orders.dispute_failed');
    }
  } finally {
    disputeBusy.value = false;
  }
}

function onReviewSubmitted(review: reviewsApi.Review) {
  existingReview.value = review;
  showReviewForm.value = false;
}

function statusLabel(s: ordersApi.Order['status']): string {
  switch (s) {
    case 'held': return t('orders.status_held');
    case 'delivered': return 'Delivered';
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

const isBuyer = computed(() => order.value?.buyer_id === auth?.user?.id);
const isSeller = computed(() => order.value?.seller_id === auth?.user?.id);

const canReview = computed(
  () =>
    order.value?.my_role === 'buyer' &&
    order.value?.status === 'completed' &&
    existingReview.value === null &&
    !checkingReview.value
);

const canDispute = computed(
  () => order.value?.my_role === 'buyer' && (order.value?.status === 'held' || order.value?.status === 'delivered')
);

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
    <!-- TOP NAVIGATION WITH SECURE ADMIN BACK BUTTON -->
    <div class="top-nav">
      <RouterLink to="/orders" class="back">{{ t('orders.back') }}</RouterLink>
      
      <!-- SAFE CHECK: Only show if auth is loaded AND user is admin -->
      <RouterLink v-if="auth?.user?.is_admin" to="/admin" class="back admin-back">
        ← Back to Admin Dashboard
      </RouterLink>
    </div>

    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="err">! {{ error }}</p>

    <article v-else-if="order" class="detail">
      <header class="head">
        <h1 class="heading">{{ t('orders.detail_heading') }}</h1>
        <div class="head-right">
          <CopyId :id="order.id" label="order id" />
          <span class="status" :class="order.status">{{ statusLabel(order.status) }}</span>
        </div>
      </header>

      <div class="price-block">
        <div class="amount">
          {{ symbolFor(currencies, order.currency_code) }}
          {{ fmtAmount(currencies, order.amount, order.currency_code) }}
        </div>
        <div class="cur">{{ order.currency_code }}</div>
      </div>

      <div v-if="order.invoice_title" class="invoice">
        <div class="invoice-label">invoice</div>
        <div class="invoice-title">{{ order.invoice_title }}</div>
        <div v-if="order.invoice_description" class="invoice-desc">
          {{ order.invoice_description }}
        </div>
      </div>

      <!-- DELIVERY TUNNEL SECTION -->
      <section class="tunnel-section">
        <h2 class="section-title">{{ t('orders.tunnel_heading') }}</h2>
        
        <ul class="file-list">
          <li v-for="f in deliverables" :key="f.id" class="file-item">
            <span class="file-name">📎 {{ f.filename }} ({{ (f.size_bytes / 1024).toFixed(1) }} KB)</span>
            <a :href="ordersApi.getDeliverableDownloadUrl(order.id, f.id)" class="download-btn" download>{{ t('orders.tunnel_download') }}</a>
          </li>
          <li v-if="deliverables.length === 0" class="muted">{{ t('orders.tunnel_no_files') }}</li>
        </ul>

        <div v-if="deliverables.length > 0" class="expiry-warning">
          ⚠️ {{ t('orders.tunnel_warning') }}
        </div>

        <!-- Seller Controls -->
        <div v-if="isSeller && order.status === 'held'" class="seller-controls">
          <label class="upload-btn" :class="{ busy: uploading }">
            {{ uploading ? t('orders.tunnel_uploading') : t('orders.tunnel_attach') }}
            <input type="file" @change="handleFileUpload" style="display: none" />
          </label>
          
          <button 
            v-if="deliverables.length > 0"
            class="seal-btn" 
            :disabled="sealing" 
            @click="seal"
          >
            {{ sealing ? t('orders.tunnel_sealing') : t('orders.tunnel_seal') }}
          </button>
        </div>
      </section>

      <dl class="meta">
        <div class="meta-row">
          <dt>{{ t('orders.role_buyer') }}</dt>
          <dd>
            <RouterLink
              v-if="order.buyer_username"
              :to="{ name: 'public-profile', params: { username: order.buyer_username } }"
              class="userlink"
            >@{{ order.buyer_username }}</RouterLink>
            <span v-else>{{ order.buyer_id.slice(0, 8) }}</span>
            <span v-if="isBuyer" class="you">({{ t('orders.role_you') }})</span>
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
            <span v-else>{{ order.seller_id.slice(0, 8) }}</span>
            <span v-if="isSeller" class="you">({{ t('orders.role_you') }})</span>
          </dd>
        </div>
        <div class="meta-row">
          <dt>{{ t('orders.amount') }}</dt>
          <dd>
            {{ symbolFor(currencies, order.currency_code) }}
            {{ fmtAmount(currencies, order.amount, order.currency_code) }}
          </dd>
        </div>
        <div v-if="isSeller || auth?.user?.is_admin" class="meta-row">
          <dt>{{ t('orders.commission') }} ({{ order.commission_percent }}%)</dt>
          <dd>
            − {{ symbolFor(currencies, order.currency_code) }}
            {{ fmtAmount(currencies, order.commission_amount, order.currency_code) }}
          </dd>
        </div>
        <div v-if="isSeller" class="meta-row net-row">
          <dt>{{ t('orders.you_receive') }}</dt>
          <dd>
            {{ symbolFor(currencies, order.currency_code) }}
            {{ fmtAmount(currencies, order.amount - order.commission_amount, order.currency_code) }}
          </dd>
        </div>
        <div class="meta-row">
          <dt>{{ t('orders.created') }}</dt>
          <dd>{{ fmtDate(order.created_at_unix) }}</dd>
        </div>
      </dl>

      <!-- Status Notices -->
      <p v-if="order.status === 'held' && isSeller" class="notice seller">
        {{ t('orders.notice_seller_upload') }}
      </p>
      <p v-if="order.status === 'held' && isBuyer" class="notice buyer">
        {{ t('orders.notice_buyer_waiting') }}
      </p>
      <p v-if="order.status === 'delivered' && isBuyer" class="notice buyer">
        {{ t('orders.notice_buyer_review') }}
      </p>
      <p v-if="order.status === 'disputed'" class="notice disputed">
        {{ t('orders.notice_disputed') }}
      </p>

      <!-- Action Buttons -->
      <div class="actions">
        <button 
          v-if="isBuyer && order.status === 'delivered'" 
          class="primary" 
          :disabled="confirming" 
          @click="confirm"
        >
          {{ confirming ? t('orders.confirming') : `> ${t('orders.accept_delivery')}` }}
        </button>

        <button v-if="canDispute" class="danger" @click="openDispute">
          {{ t('orders.dispute') }}
        </button>
      </div>

      <p v-if="actionError" class="err">! {{ actionError }}</p>
      <p v-if="actionSuccess" class="ok">{{ actionSuccess }}</p>

      <!-- Review Section -->
      <section v-if="order.status === 'completed' && order.my_role === 'buyer'" class="review-section">
        <div v-if="existingReview" class="existing-review">
          <h3 class="section-title">{{ t('reviews.your_review') }}</h3>
          <div class="review-stars">{{ '★'.repeat(existingReview.rating) }}{{ '☆'.repeat(5 - existingReview.rating) }}</div>
          <p v-if="existingReview.comment" class="review-comment">{{ existingReview.comment }}</p>
        </div>

        <button
          v-else-if="canReview && !showReviewForm"
          class="primary"
          @click="showReviewForm = true"
        >
          {{ t('reviews.leave_review') }}
        </button>

        <ReviewForm
          v-if="showReviewForm"
          :order-id="order.id"
          @submitted="onReviewSubmitted"
          @cancel="showReviewForm = false"
        />
      </section>
    </article>

    <!-- Dispute Modal -->
    <div v-if="disputeOpen" class="modal-bg" @click.self="closeDispute">
      <div class="modal">
        <h2 class="modal-title">{{ t('orders.dispute_title') }}</h2>
        <p class="modal-hint">{{ t('orders.dispute_hint') }}</p>
        <label class="field">
          <span class="label">{{ t('orders.dispute_reason') }}</span>
          <textarea v-model="disputeReason" rows="4" maxlength="2000" />
        </label>
        <p v-if="disputeError" class="err">! {{ disputeError }}</p>
        <div class="modal-actions">
          <button class="danger" :disabled="disputeBusy" @click="submitDispute">
            {{ disputeBusy ? t('common.loading') : `> ${t('orders.dispute')}` }}
          </button>
          <button class="ghost" @click="closeDispute">{{ t('common.cancel') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* TOP NAV */
.top-nav {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1.5rem;
}

.back {
  display: inline-block;
  color: #6b7a90;
  text-decoration: none;
  font-size: 0.85rem;
  font-weight: 500;
  padding: 0.4rem 0.8rem;
  border-radius: 4px;
  border: 1px solid transparent;
  transition: all 0.15s;
}
.back:hover {
  color: #00d9a3;
  background: rgba(0, 217, 163, 0.05);
  border-color: rgba(0, 217, 163, 0.2);
}

.admin-back {
  color: #ffb454;
  border-color: rgba(255, 180, 84, 0.2);
}
.admin-back:hover {
  color: #ffb454;
  background: rgba(255, 180, 84, 0.05);
  border-color: rgba(255, 180, 84, 0.5);
}

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
  flex-wrap: wrap;
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
  position: relative;
}
.heading {
  color: #e2e5e9;
  font-size: 1.1rem;
  font-weight: 600;
  margin: 0;
}
.status {
  font-size: 0.75rem;
  padding: 0.15rem 0.6rem;
  border: 1px solid #1e2229;
  border-radius: 4px;
  color: #6b7a90;
}
.status.held {
  color: #ffb454;
  border-color: #ffb454;
}
.status.delivered {
  color: #4a9eff;
  border-color: #4a9eff;
}
.status.completed {
  color: #00d9a3;
  border-color: #00d9a3;
}
.status.refunded {
  color: #ff5c5c;
  border-color: #ff5c5c;
}
.status.disputed {
  color: #ff5c5c;
  border-color: #ff5c5c;
}
.price-block {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  margin-bottom: 1.25rem;
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
.invoice {
  border-left: 2px solid #4a9eff;
  background: rgba(74, 158, 255, 0.05);
  padding: 0.75rem 1rem;
  margin-bottom: 1.25rem;
  border-radius: 0 4px 4px 0;
}
.invoice-label {
  color: #4a9eff;
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 0.25rem;
}
.invoice-title {
  color: #e2e5e9;
  font-weight: 600;
  font-size: 0.95rem;
}
.invoice-desc {
  color: #6b7a90;
  font-size: 0.85rem;
  margin-top: 0.25rem;
  white-space: pre-wrap;
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
  flex-shrink: 0;
}
.meta-row dd {
  color: #e2e5e9;
  margin: 0;
  text-align: right;
  word-break: break-all;
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
.dim {
  color: #6b7a90;
}
.notice {
  border-left: 2px solid #ffb454;
  padding: 0.5rem 0.75rem;
  background: rgba(255, 180, 84, 0.05);
  color: #ffb454;
  font-size: 0.8rem;
  margin: 0 0 1rem;
  border-radius: 0 4px 4px 0;
}
.notice.buyer {
  border-left-color: #4a9eff;
  background: rgba(74, 158, 255, 0.05);
  color: #4a9eff;
}
.notice.disputed {
  border-left-color: #ff5c5c;
  background: rgba(255, 92, 92, 0.05);
  color: #ff5c5c;
}
.actions {
  display: flex;
  gap: 0.75rem;
  margin-top: 0.5rem;
}
.primary {
  background: #00d9a3;
  color: #0a0e14;
  border: none;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.9rem;
  font-weight: bold;
  padding: 0.55rem 1.25rem;
  cursor: pointer;
}
.primary:hover:not(:disabled) {
  background: #00f5aa;
}
.primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.danger {
  background: transparent;
  color: #ff5c5c;
  border: 1px solid #ff5c5c;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.9rem;
  font-weight: bold;
  padding: 0.55rem 1.25rem;
  cursor: pointer;
}
.danger:hover:not(:disabled) {
  background: rgba(255, 92, 92, 0.1);
}
.danger:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.review-section {
  margin-top: 1.5rem;
  padding-top: 1rem;
  border-top: 1px solid #1e2229;
}
.section-title {
  color: #6b7a90;
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 500;
  margin: 0 0 0.5rem;
}
.existing-review {
  background: #0a0e14;
  border: 1px solid #1e2229;
  padding: 0.75rem 1rem;
  border-radius: 4px;
}
.review-stars {
  color: #00d9a3;
  font-size: 1rem;
  letter-spacing: 0.1em;
  margin-bottom: 0.25rem;
}
.review-comment {
  color: #c8d3e0;
  font-size: 0.9rem;
  margin: 0;
  white-space: pre-wrap;
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
  color: #ff5c5c;
  font-size: 1rem;
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

/* TUNNEL SPECIFIC STYLES */
.tunnel-section {
  background: #0a0e14;
  border: 1px solid #1e2229;
  border-radius: 6px;
  padding: 1.5rem;
  margin: 1.5rem 0;
}
.file-list {
  list-style: none;
  padding: 0;
  margin-bottom: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.file-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.5rem 0.75rem;
  background: #11151c;
  border-radius: 4px;
  font-size: 0.85rem;
}
.download-btn {
  color: #00d9a3;
  text-decoration: none;
  font-weight: bold;
}
.expiry-warning {
  font-size: 0.75rem;
  color: #ffb454;
  background: rgba(255, 180, 84, 0.1);
  padding: 0.5rem;
  border-radius: 4px;
  border-left: 2px solid #ffb454;
  margin-top: 1rem;
}
.upload-btn {
  display: inline-block;
  padding: 0.5rem 1rem;
  border: 1px dashed #00d9a3;
  color: #00d9a3;
  border-radius: 4px;
  cursor: pointer;
  text-align: center;
  font-size: 0.85rem;
  transition: background 0.1s;
}
.upload-btn:hover:not(.busy) {
  background: rgba(0, 217, 163, 0.1);
}
.upload-btn.busy {
  opacity: 0.5;
  cursor: wait;
}
.seal-btn {
  background: #00d9a3;
  color: #0a0e14;
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 4px;
  font-weight: bold;
  margin-left: 1rem;
  cursor: pointer;
  font-size: 0.85rem;
}
.seal-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.net-row dt,
.net-row dd {
  color: #00d9a3;
  font-weight: 600;
}
</style>
