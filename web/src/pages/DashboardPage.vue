<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { RouterLink } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as dashboardApi from '../api/dashboard';
import * as walletApi from '../api/wallet';
import { fmtAmount, symbolFor, getCurrencies } from '../lib/money';

const { t } = useI18n();
const stats = ref<dashboardApi.DashboardStats | null>(null);
const currencies = ref<walletApi.Currency[]>([]);
const loading = ref(true);

onMounted(async () => {
  try {
    const [s, c] = await Promise.all([
      dashboardApi.getStats(),
      getCurrencies()
    ]);
    stats.value = s;
    currencies.value = c;
  } catch (e) {}
  loading.value = false;
});
</script>

<template>
  <div class="dashboard">
    <header class="page-head">
      <h1 class="heading">{{ t('dashboard.heading') }}</h1>
      <RouterLink to="/products/new" class="btn-primary">
        {{ t('dashboard.new_listing') }}
      </RouterLink>
    </header>

    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>

    <template v-else-if="stats">
      <!-- Stats Grid -->
      <div class="stats-grid">
        <div class="stat-card highlight">
          <div class="stat-label">{{ t('dashboard.total_earned') }}</div>
          <div class="stat-value">
            {{ symbolFor(currencies, 'USD') }}
            {{ fmtAmount(currencies, stats.total_earned, 'USD') }}
          </div>
          <div class="stat-hint">{{ t('dashboard.after_fees') }}</div>
        </div>

        <div class="stat-card">
          <div class="stat-label">{{ t('dashboard.active_orders') }}</div>
          <div class="stat-value">{{ stats.active_orders }}</div>
          <div class="stat-hint">{{ t('dashboard.awaiting_delivery') }}</div>
        </div>

        <div class="stat-card">
          <div class="stat-label">{{ t('dashboard.pending_withdrawals') }}</div>
          <div class="stat-value">{{ stats.pending_withdrawals }}</div>
          <div class="stat-hint">{{ t('dashboard.under_review') }}</div>
        </div>

        <div class="stat-card">
          <div class="stat-label">{{ t('dashboard.active_listings') }}</div>
          <div class="stat-value">{{ stats.active_products }}</div>
          <div class="stat-hint">{{ t('dashboard.live_market') }}</div>
        </div>
      </div>

      <!-- Quick Links -->
      <section class="section">
        <h2 class="section-title">{{ t('dashboard.manage_title') }}</h2>
        <div class="links-grid">
          <RouterLink to="/orders" class="link-card">
            <div class="link-icon">📦</div>
            <div class="link-content">
              <div class="link-title">{{ t('dashboard.orders') }}</div>
              <div class="link-desc">{{ t('dashboard.orders_desc') }}</div>
            </div>
          </RouterLink>

          <RouterLink to="/my-products" class="link-card">
            <div class="link-icon">🏷️</div>
            <div class="link-content">
              <div class="link-title">{{ t('dashboard.my_products') }}</div>
              <div class="link-desc">{{ t('dashboard.my_products_desc') }}</div>
            </div>
          </RouterLink>

          <RouterLink to="/wallet" class="link-card">
            <div class="link-icon">💳</div>
            <div class="link-content">
              <div class="link-title">{{ t('dashboard.wallet') }}</div>
              <div class="link-desc">{{ t('dashboard.wallet_desc') }}</div>
            </div>
          </RouterLink>

          <RouterLink to="/profile" class="link-card">
            <div class="link-icon">👤</div>
            <div class="link-content">
              <div class="link-title">{{ t('dashboard.profile') }}</div>
              <div class="link-desc">{{ t('dashboard.profile_desc') }}</div>
            </div>
          </RouterLink>
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.dashboard {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 2rem;
}

.page-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.heading {
  font-size: 1.5rem;
  color: #e2e5e9;
  margin: 0;
}

.btn-primary {
  background: #00d9a3;
  color: #0a0e14;
  font-weight: 600;
  font-size: 0.85rem;
  padding: 0.6rem 1.2rem;
  border-radius: 4px;
  text-decoration: none;
  transition: background 0.1s;
}
.btn-primary:hover {
  background: #00f5aa;
}

.muted { color: #6b7a90; }

/* Stats Grid */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 1rem;
}

.stat-card {
  background: rgba(15, 17, 21, 0.85);
  border: 1px solid #1e2229;
  border-radius: 8px;
  padding: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.stat-card.highlight {
  border-color: rgba(0, 217, 163, 0.3);
  background: linear-gradient(180deg, rgba(0, 217, 163, 0.05) 0%, rgba(15, 17, 21, 0.85) 100%);
}

.stat-label {
  color: #6b7a90;
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 500;
}

.stat-value {
  color: #e2e5e9;
  font-size: 2rem;
  font-weight: 700;
  margin: 0.25rem 0;
}

.stat-card.highlight .stat-value {
  color: #00d9a3;
}

.stat-hint {
  color: #6b7a90;
  font-size: 0.75rem;
}

/* Quick Links */
.section {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.section-title {
  font-size: 1.1rem;
  color: #e2e5e9;
  margin: 0;
}

.links-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 1rem;
}

.link-card {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
  background: rgba(15, 17, 21, 0.85);
  border: 1px solid #1e2229;
  border-radius: 8px;
  padding: 1.25rem;
  text-decoration: none;
  transition: border-color 0.1s, background 0.1s;
}

.link-card:hover {
  border-color: #2a3039;
  background: rgba(20, 24, 32, 0.9);
}

.link-icon {
  font-size: 1.5rem;
  background: #0a0e14;
  padding: 0.5rem;
  border-radius: 6px;
  border: 1px solid #1e2229;
}

.link-content {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.link-title {
  color: #e2e5e9;
  font-weight: 600;
  font-size: 0.95rem;
}

.link-desc {
  color: #6b7a90;
  font-size: 0.8rem;
  line-height: 1.4;
}
</style>
