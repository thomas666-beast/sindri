<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { RouterLink } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as adminApi from '../api/admin';
import * as walletApi from '../api/wallet';
import { fmtAmount, symbolFor } from '../lib/money';

const { t } = useI18n();

const stats = ref<adminApi.AdminStats | null>(null);
const currencies = ref<walletApi.Currency[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);

async function load() {
  loading.value = true;
  error.value = null;
  try {
    stats.value = await adminApi.stats();
  } catch {
    error.value = t('admin.stats_load_failed');
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

function orderCount(status: 'held' | 'disputed'): number {
  if (!stats.value) return 0;
  return status === 'held' ? stats.value.orders_held : stats.value.orders_disputed;
}

const commissionRows = () => {
  if (!stats.value) return [];
  return Object.entries(stats.value.commission_by_currency).map(([code, amount]) => ({
    code,
    amount,
  }));
};

onMounted(() => {
  load();
  loadCurrencies();
});
</script>

<template>
  <div>
    <header class="page-head">
      <h1 class="heading">{{ t('admin.dashboard_heading') }}</h1>
      <button class="btn-refresh" @click="load" :disabled="loading" title="refresh">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M21 12a9 9 0 1 1-3-6.7" />
          <path d="M21 3v6h-6" />
        </svg>
      </button>
    </header>

    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="err">! {{ error }}</p>

    <template v-else-if="stats">
      <!-- STAT GRID -->
      <div class="stat-grid">
        <div class="stat">
          <div class="stat-icon green">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
              <circle cx="9" cy="7" r="4" />
              <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
              <path d="M16 3.13a4 4 0 0 1 0 7.75" />
            </svg>
          </div>
          <div class="stat-value">{{ stats.users_active }}</div>
          <div class="stat-label">{{ t('admin.stat_users_active') }}</div>
        </div>

        <div class="stat">
          <div class="stat-icon red">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
              <circle cx="9" cy="7" r="4" />
              <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
              <line x1="1" y1="1" x2="23" y2="23" />
            </svg>
          </div>
          <div class="stat-value">{{ stats.users_disabled }}</div>
          <div class="stat-label">{{ t('admin.stat_users_disabled') }}</div>
        </div>

        <div class="stat">
          <div class="stat-icon blue">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="3" y="7" width="18" height="14" rx="2" />
              <path d="M3 7l9-4 9 4" />
              <path d="M12 7v14" />
            </svg>
          </div>
          <div class="stat-value">{{ orderCount('held') }}</div>
          <div class="stat-label">{{ t('admin.stat_orders_held') }}</div>
        </div>

        <div class="stat">
          <div class="stat-icon amber">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
              <line x1="12" y1="9" x2="12" y2="13" />
              <line x1="12" y1="17" x2="12.01" y2="17" />
            </svg>
          </div>
          <div class="stat-value">{{ orderCount('disputed') }}</div>
          <div class="stat-label">{{ t('admin.stat_orders_disputed') }}</div>
        </div>

        <div class="stat">
          <div class="stat-icon purple">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="12" y1="1" x2="12" y2="23" />
              <path d="M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6" />
            </svg>
          </div>
          <div class="stat-value">{{ stats.pending_withdrawals }}</div>
          <div class="stat-label">{{ t('admin.stat_withdrawals_pending') }}</div>
        </div>

        <div class="stat">
          <div class="stat-icon cyan">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
            </svg>
          </div>
          <div class="stat-value">{{ stats.open_tickets }}</div>
          <div class="stat-label">{{ t('admin.stat_tickets_open') }}</div>
        </div>
      </div>

      <!-- COMMISSION -->
      <section class="section">
        <h2 class="section-title">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="8" r="6" />
            <path d="M15.477 12.89L17 22l-5-3-5 3 1.523-9.11" />
          </svg>
          {{ t('admin.commission_earned') }}
        </h2>

        <div v-if="commissionRows().length === 0" class="empty">
          <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="9" />
            <path d="M12 8v4M12 16h.01" />
          </svg>
          <span>{{ t('admin.commission_empty') }}</span>
        </div>

        <div v-else class="commission-grid">
          <div v-for="r in commissionRows()" :key="r.code" class="commission-card">
            <div class="commission-head">
              <span class="commission-dot" />
              <span class="commission-cur">{{ r.code }}</span>
            </div>
            <div class="commission-amount">
              <span class="amount-symbol">{{ symbolFor(currencies, r.code) }}</span>
              {{ fmtAmount(currencies, r.amount, r.code) }}
            </div>
            <div class="commission-foot">{{ t('admin.commission_total') }}</div>
          </div>
        </div>
      </section>

      <!-- QUICK LINKS -->
      <section class="section">
        <h2 class="section-title">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
            <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
          </svg>
          {{ t('admin.quick_links') }}
        </h2>
        <div class="links-grid">
          <RouterLink to="/admin/users" class="link-card">
            <svg class="link-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
              <circle cx="9" cy="7" r="4" />
            </svg>
            <div class="link-body">
              <div class="link-label">{{ t('admin.tab_users') }}</div>
              <div class="link-hint">{{ t('admin.link_users_hint') }}</div>
            </div>
          </RouterLink>

          <RouterLink to="/admin/orders" class="link-card">
            <svg class="link-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
              <line x1="12" y1="9" x2="12" y2="13" />
              <line x1="12" y1="17" x2="12.01" y2="17" />
            </svg>
            <div class="link-body">
              <div class="link-label">{{ t('admin.tab_disputes') }}</div>
              <div class="link-hint">{{ t('admin.link_disputes_hint') }}</div>
            </div>
          </RouterLink>

          <RouterLink to="/admin/withdrawals" class="link-card">
            <svg class="link-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="12" y1="1" x2="12" y2="23" />
              <path d="M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6" />
            </svg>
            <div class="link-body">
              <div class="link-label">{{ t('admin.tab_withdrawals') }}</div>
              <div class="link-hint">{{ t('admin.link_withdrawals_hint') }}</div>
            </div>
          </RouterLink>

          <RouterLink to="/admin/tickets" class="link-card">
            <svg class="link-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
            </svg>
            <div class="link-body">
              <div class="link-label">{{ t('admin.tab_tickets') }}</div>
              <div class="link-hint">{{ t('admin.link_tickets_hint') }}</div>
            </div>
          </RouterLink>

          <RouterLink to="/admin/lookup" class="link-card">
            <svg class="link-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="11" cy="11" r="7" />
              <line x1="21" y1="21" x2="16.65" y2="16.65" />
            </svg>
            <div class="link-body">
              <div class="link-label">{{ t('admin.order_look_up_admin') }}</div>
              <div class="link-hint">search any order by its exact ID</div>
            </div>
          </RouterLink>
          <RouterLink to="/admin/settings" class="link-card">
            <svg class="link-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="3" />
              <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z" />
            </svg>
            <div class="link-body">
              <div class="link-label">settings</div>
              <div class="link-hint">manage platform fees and timers</div>
            </div>
          </RouterLink>
        </div>
      </section>
    </template>
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
  font-size: 1.1rem;
  color: #e2e5e9;
  margin: 0;
}
.muted { color: #6b7a90; }
.err { color: #ff5c5c; font-size: 0.85rem; }

.btn-refresh {
  background: transparent;
  border: 1px solid #1e2229;
  color: #6b7a90;
  padding: 0.4rem 0.55rem;
  border-radius: 4px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  transition: all 0.15s;
}
.btn-refresh:hover:not(:disabled) {
  color: #00d9a3;
  border-color: #00d9a3;
}
.btn-refresh:disabled { opacity: 0.5; cursor: wait; }

/* ---- STAT GRID ---- */
.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 0.75rem;
  margin-bottom: 2rem;
}
.stat {
  border: 1px solid #1e2229;
  background: linear-gradient(180deg, rgba(255,255,255,0.01), rgba(15, 17, 21, 0.85));
  border-radius: 8px;
  padding: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  transition: border-color 0.15s, transform 0.15s;
}
.stat:hover {
  border-color: #2a313d;
  transform: translateY(-1px);
}
.stat-icon {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 0.25rem;
}
.stat-icon svg {
  width: 14px;
  height: 14px;
}
.stat-icon.green  { background: rgba(0, 217, 163, 0.12); color: #00d9a3; }
.stat-icon.red    { background: rgba(255, 92, 92, 0.12);  color: #ff5c5c; }
.stat-icon.blue   { background: rgba(74, 158, 255, 0.12); color: #4a9eff; }
.stat-icon.amber  { background: rgba(255, 180, 84, 0.12); color: #ffb454; }
.stat-icon.purple { background: rgba(180, 130, 255, 0.12); color: #b482ff; }
.stat-icon.cyan   { background: rgba(74, 217, 255, 0.12); color: #4ad9ff; }
.stat-value {
  font-size: 1.75rem;
  font-weight: 600;
  color: #e2e5e9;
  line-height: 1.1;
  font-variant-numeric: tabular-nums;
}
.stat-label {
  color: #6b7a90;
  font-size: 0.72rem;
  letter-spacing: 0.02em;
}

/* ---- SECTION HEADER ---- */
.section { margin-bottom: 2rem; }
.section-title {
  color: #6b7a90;
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  font-weight: 500;
  margin: 0 0 0.75rem;
  display: flex;
  align-items: center;
  gap: 0.4rem;
}
.section-title svg {
  color: #00d9a3;
  flex-shrink: 0;
}

/* ---- COMMISSION ---- */
.empty {
  color: #6b7a90;
  font-size: 0.85rem;
  padding: 1.25rem 0.75rem;
  border: 1px dashed #1e2229;
  border-radius: 6px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.4rem;
}
.empty svg { opacity: 0.5; }

.commission-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 0.75rem;
}
.commission-card {
  border: 1px solid #1e2229;
  background: linear-gradient(180deg, rgba(0, 217, 163, 0.06), rgba(15, 17, 21, 0.85));
  border-radius: 8px;
  padding: 0.9rem 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  transition: border-color 0.15s;
}
.commission-card:hover { border-color: rgba(0, 217, 163, 0.5); }
.commission-head {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}
.commission-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #00d9a3;
  box-shadow: 0 0 8px rgba(0, 217, 163, 0.6);
}
.commission-cur {
  color: #00d9a3;
  font-weight: 700;
  font-size: 0.75rem;
  letter-spacing: 0.05em;
}
.commission-amount {
  color: #e2e5e9;
  font-size: 1.3rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  line-height: 1.2;
}
.amount-symbol {
  color: #6b7a90;
  font-size: 0.9rem;
  margin-right: 0.15rem;
}
.commission-foot {
  color: #6b7a90;
  font-size: 0.65rem;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

/* ---- QUICK LINKS ---- */
.links-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 0.6rem;
}
.link-card {
  border: 1px solid #1e2229;
  background: rgba(15, 17, 21, 0.85);
  border-radius: 8px;
  padding: 0.9rem 1rem;
  text-decoration: none;
  display: flex;
  align-items: center;
  gap: 0.75rem;
  transition: border-color 0.15s, background 0.15s;
}
.link-card:hover {
  border-color: #ffb454;
  background: rgba(255, 180, 84, 0.04);
}
.link-icon {
  width: 20px;
  height: 20px;
  color: #ffb454;
  flex-shrink: 0;
}
.link-body { display: flex; flex-direction: column; gap: 0.15rem; }
.link-label {
  color: #ffb454;
  font-weight: 600;
  font-size: 0.85rem;
}
.link-hint {
  color: #6b7a90;
  font-size: 0.72rem;
}
</style>
