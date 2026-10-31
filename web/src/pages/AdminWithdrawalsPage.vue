<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import * as adminApi from '../api/admin';
import * as walletApi from '../api/wallet';
import { fmtAmount, symbolFor } from '../lib/money';
import CopyId from '../components/CopyId.vue';

const { t } = useI18n();

const currentTab = ref<'' | 'pending' | 'approved' | 'rejected'>('pending');
const searchId = ref('');

const withdrawals = ref<adminApi.Withdrawal[]>([]);
const currencies = ref<walletApi.Currency[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);
const actionError = ref<string | null>(null);
const actionSuccess = ref<string | null>(null);
const busyId = ref<string | null>(null);

// Pagination State
const limit = 15;
const offset = ref(0);
const hasMore = ref(true);
const loadingMore = ref(false);

// Modals State
const rejectModal = ref(false);
const rejectTarget = ref<adminApi.Withdrawal | null>(null);
const rejectReason = ref('');
const rejectBusy = ref(false);
const rejectError = ref<string | null>(null);

const viewReasonModal = ref(false);
const viewReasonTarget = ref<adminApi.Withdrawal | null>(null);

async function load(reset = true) {
  if (reset) {
    loading.value = true;
    error.value = null;
    withdrawals.value = [];
    offset.value = 0;
    hasMore.value = true;
  } else {
    loadingMore.value = true;
  }

  try {
    const res = await adminApi.listWithdrawals(
      currentTab.value, 
      searchId.value, 
      limit, 
      offset.value
    );
    
    if (reset) {
      withdrawals.value = res;
    } else {
      withdrawals.value = [...withdrawals.value, ...res];
    }

    if (res.length < limit) {
      hasMore.value = false;
    } else {
      offset.value += limit;
    }
  } catch {
    error.value = t('withdrawals.load_failed');
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

function setFilter(tab: '' | 'pending' | 'approved' | 'rejected') {
  if (currentTab.value === tab) return;
  currentTab.value = tab;
  load(true);
}

async function approve(w: adminApi.Withdrawal) {
  actionError.value = null;
  actionSuccess.value = null;
  busyId.value = w.id;
  try {
    await adminApi.approveWithdrawal(w.id);
    actionSuccess.value = t('withdrawals.approve_ok');
    await load(true); 
  } catch (e: any) {
    actionError.value = e.response?.data?.error ?? t('withdrawals.approve_failed');
  } finally {
    busyId.value = null;
  }
}

function openReject(w: adminApi.Withdrawal) {
  rejectTarget.value = w;
  rejectReason.value = '';
  rejectError.value = null;
  rejectModal.value = true;
}

function closeReject() {
  rejectModal.value = false;
  rejectTarget.value = null;
  rejectReason.value = '';
  rejectError.value = null;
}

async function confirmReject() {
  if (!rejectTarget.value) return;
  rejectError.value = null;
  if (rejectReason.value.trim().length < 3) {
    rejectError.value = t('withdrawals.reason_required');
    return;
  }
  rejectBusy.value = true;
  try {
    await adminApi.rejectWithdrawal(rejectTarget.value.id, rejectReason.value.trim());
    actionSuccess.value = t('withdrawals.reject_ok');
    closeReject();
    await load(true);
  } catch (e: any) {
    rejectError.value = e.response?.data?.error ?? t('withdrawals.reject_failed');
  } finally {
    rejectBusy.value = false;
  }
}

function openViewReason(w: adminApi.Withdrawal) {
  viewReasonTarget.value = w;
  viewReasonModal.value = true;
}

function closeViewReason() {
  viewReasonModal.value = false;
  viewReasonTarget.value = null;
}

function fmtDate(unix: number): string {
  return new Date(unix * 1000).toISOString().slice(0, 10);
}

onMounted(() => {
  load(true);
  loadCurrencies();
});
</script>

<template>
  <div>
    <h1 class="heading">{{ t('withdrawals.admin_heading') }}</h1>

    <!-- Filter & Search Bar -->
    <div class="filters-bar">
      <div class="filters">
        <button class="filter" :class="{ active: currentTab === 'pending' }" @click="setFilter('pending')">
          {{ t('withdrawals.status_pending') }}
        </button>
        <button class="filter" :class="{ active: currentTab === 'approved' }" @click="setFilter('approved')">
          {{ t('withdrawals.status_approved') }}
        </button>
        <button class="filter" :class="{ active: currentTab === 'rejected' }" @click="setFilter('rejected')">
          {{ t('withdrawals.status_rejected') }}
        </button>
        <button class="filter" :class="{ active: currentTab === '' }" @click="setFilter('')">
          {{ t('withdrawals.status_all') }}
        </button>
      </div>

      <div class="search-box">
        <input 
          v-model="searchId" 
          type="text" 
          placeholder="search by id..." 
          @keydown.enter="load(true)"
        />
        <button class="btn-ghost" @click="load(true)" title="search">↻</button>
      </div>
    </div>

    <p v-if="actionError" class="err">! {{ actionError }}</p>
    <p v-if="actionSuccess" class="ok">{{ actionSuccess }}</p>

    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="err">! {{ error }}</p>
    <p v-else-if="withdrawals.length === 0" class="empty">
      {{ t('withdrawals.admin_empty') }}
    </p>

    <div v-else class="list">
      <article v-for="w in withdrawals" :key="w.id" class="row">
        <div class="row-main">
          
          <!-- ID Component -->
          <CopyId :id="w.id" label="id" />

          <span class="amount">
            {{ symbolFor(currencies, w.currency_code) }}
            {{ fmtAmount(currencies, w.amount, w.currency_code) }}
            <span class="cur">{{ w.currency_code }}</span>
          </span>
          
          <span class="user">@{{ w.username }}</span>

          <span v-if="w.status !== 'pending'" class="badge" :class="w.status">
            {{ t(`withdrawals.status_${w.status}`) }}
          </span>
          
          <button v-if="w.status === 'rejected'" class="text-link" @click="openViewReason(w)">
            {{ t('withdrawals.view_reason') }}
          </button>
        </div>

        <div class="row-sub">
          <span class="meta">{{ t('withdrawals.requested') }}: {{ fmtDate(w.created_at_unix) }}</span>
          
          <span v-if="w.reviewed_at_unix" class="meta">
            · {{ t('withdrawals.reviewed') }}: {{ fmtDate(w.reviewed_at_unix) }} 
            <span v-if="w.reviewed_by_username">by @{{ w.reviewed_by_username }}</span>
          </span>

          <div class="actions" v-if="w.status === 'pending'">
            <button
              class="btn-approve"
              :disabled="busyId === w.id"
              @click="approve(w)"
            >
              {{ busyId === w.id ? t('common.loading') : t('withdrawals.approve') }}
            </button>
            <button class="btn-reject" @click="openReject(w)">
              {{ t('withdrawals.reject') }}
            </button>
          </div>
        </div>
      </article>

      <!-- PAGINATION -->
      <div v-if="hasMore" class="load-more-wrap">
        <button class="load-more-btn" :disabled="loadingMore" @click="load(false)">
          {{ loadingMore ? t('common.loading') : t('withdrawals.load_more') }}
        </button>
      </div>
    </div>

    <!-- Reject Modal -->
    <div v-if="rejectModal" class="modal-bg" @click.self="closeReject">
      <div class="modal">
        <h2 class="modal-title">{{ t('withdrawals.reject_title') }}</h2>
        <label class="field">
          <span class="label">{{ t('withdrawals.reason') }}</span>
          <textarea v-model="rejectReason" rows="3" maxlength="1000" />
        </label>
        <p v-if="rejectError" class="err">! {{ rejectError }}</p>
        <div class="modal-actions">
          <button class="btn-reject" :disabled="rejectBusy" @click="confirmReject">
            {{ rejectBusy ? t('common.loading') : `> ${t('withdrawals.reject')}` }}
          </button>
          <button class="btn-ghost" @click="closeReject">{{ t('common.cancel') }}</button>
        </div>
      </div>
    </div>

    <!-- View Reason Modal -->
    <div v-if="viewReasonModal" class="modal-bg" @click.self="closeViewReason">
      <div class="modal">
        <h2 class="modal-title" style="color: #ff5c5c">{{ t('withdrawals.rejection_reason') }}</h2>
        <div class="reason-box">
          {{ viewReasonTarget?.rejection_reason || t('withdrawals.no_reason') }}
        </div>
        <div class="modal-actions" style="justify-content: flex-end;">
          <button class="btn-ghost" @click="closeViewReason">{{ t('common.close') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.heading {
  color: #00d9a3;
  font-size: 1.25rem;
  font-weight: normal;
  margin: 0 0 1.5rem;
}

/* FILTERS & SEARCH */
.filters-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
  gap: 1rem;
  flex-wrap: wrap;
}
.filters {
  display: flex;
  gap: 0.25rem;
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
  transition: all 0.15s;
}
.filter:hover {
  color: #c8d3e0;
}
.filter.active {
  color: #00d9a3;
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
  width: 220px;
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
.ok { color: #00d9a3; font-size: 0.85rem; margin-bottom: 1rem; }
.empty {
  border: 1px dashed #1e2229;
  padding: 2rem;
  text-align: center;
  color: #6b7a90;
  border-radius: 4px;
}

/* LIST & ROWS */
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
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}
.row-main {
  display: flex;
  gap: 0.5rem;
  align-items: center;
  flex-wrap: wrap;
}
.amount {
  color: #e2e5e9;
  font-weight: 600;
  font-size: 0.95rem;
}
.cur {
  color: #6b7a90;
  font-size: 0.7rem;
  font-weight: normal;
  margin-left: 0.15rem;
}
.user {
  color: #6b7a90;
  font-size: 0.85rem;
}

/* BADGES */
.badge {
  font-size: 0.65rem;
  padding: 0.05rem 0.4rem;
  border: 1px solid #1e2229;
  color: #6b7a90;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  border-radius: 3px;
}
.badge.pending { color: #ffb454; border-color: #ffb454; }
.badge.approved { color: #00d9a3; border-color: #00d9a3; }
.badge.rejected { color: #ff5c5c; border-color: #ff5c5c; }

/* TEXT LINK */
.text-link {
  background: transparent;
  border: none;
  color: #4a9eff;
  font-family: inherit;
  font-size: 0.75rem;
  padding: 0;
  cursor: pointer;
  text-decoration: underline;
  margin-left: auto; /* Pushes to the right */
}
.text-link:hover {
  color: #71b3ff;
}

/* SUB-ROW (Dates and Buttons) */
.row-sub {
  display: flex;
  gap: 0.5rem;
  font-size: 0.7rem;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
}
.meta {
  color: #6b7a90;
}

/* ACTION BUTTONS */
.actions {
  display: flex;
  gap: 0.5rem;
  margin-left: auto; /* Pushes to the far right */
}
.btn-approve {
  background: #00d9a3;
  color: #0a0e14;
  border: none;
  font-family: inherit;
  font-size: 0.7rem;
  font-weight: 600;
  padding: 0.25rem 0.6rem;
  border-radius: 3px;
  cursor: pointer;
}
.btn-approve:hover:not(:disabled) {
  background: #00f5aa;
}
.btn-approve:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.btn-reject {
  background: transparent;
  color: #ff5c5c;
  border: 1px solid #ff5c5c;
  font-family: inherit;
  font-size: 0.7rem;
  padding: 0.25rem 0.6rem;
  border-radius: 3px;
  cursor: pointer;
}
.btn-reject:hover {
  background: rgba(255, 92, 92, 0.1);
}

/* LOAD MORE */
.load-more-wrap {
  display: flex;
  justify-content: center;
  margin-top: 1rem;
}
.load-more-btn {
  background: transparent;
  border: 1px solid #1e2229;
  color: #6b7a90;
  padding: 0.5rem 2rem;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.75rem;
  cursor: pointer;
}
.load-more-btn:hover:not(:disabled) {
  border-color: #00d9a3;
  color: #00d9a3;
}
.load-more-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* MODALS */
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
  padding: 1.5rem;
  border-radius: 6px;
  width: 460px;
  max-width: 90vw;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
}
.modal-title {
  color: #e2e5e9;
  font-size: 1rem;
  font-weight: 600;
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
  font-family: inherit;
  font-size: 0.9rem;
  border-radius: 4px;
  resize: vertical;
}
textarea:focus {
  outline: none;
  border-color: #00d9a3;
}
.modal-actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 1rem;
}
.reason-box {
  background: rgba(255, 92, 92, 0.05);
  border: 1px solid rgba(255, 92, 92, 0.2);
  color: #c8d3e0;
  padding: 1rem;
  border-radius: 4px;
  font-size: 0.9rem;
  line-height: 1.5;
  white-space: pre-wrap;
  max-height: 300px;
  overflow-y: auto;
}
</style>
