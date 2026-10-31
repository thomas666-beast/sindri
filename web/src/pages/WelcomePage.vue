<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { RouterLink } from 'vue-router';
import { useI18n } from 'vue-i18n';
import { useAuthStore } from '../stores/auth';
import * as usersApi from '../api/users';
import * as productsApi from '../api/products';
import * as walletApi from '../api/wallet';
import { fmtAmount, symbolFor, getCurrencies } from '../lib/money';
import Avatar from '../components/Avatar.vue';

const { t } = useI18n();
const auth = useAuthStore();

const geeks = ref<usersApi.UserHit[]>([]);
const products = ref<productsApi.Product[]>([]);
const currencies = ref<walletApi.Currency[]>([]);
const loading = ref(true);

onMounted(async () => {
  try {
    const [geekRes, prodRes, currRes] = await Promise.all([
      usersApi.search('', 5, 0),
      productsApi.search({ limit: 4, offset: 0 }),
      getCurrencies()
    ]);
    geeks.value = geekRes.results;
    products.value = prodRes.results;
    currencies.value = currRes;
  } catch (e) {
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <div class="welcome-page">
    
    <!-- Hero Section -->
    <header class="hero">
      <div class="hero-content">
        <h1 class="greeting">
          {{ t('welcome.heading') }} <br/>
          <span class="accent">@{{ auth.user?.username }}</span>
        </h1>
        <p class="subtitle">{{ t('welcome.subtitle') }}</p>
        
        <!-- INLINE ACTION BAR -->
        <div class="action-bar">
          <RouterLink to="/geeks" class="action-btn primary">
            <span class="icon">🔍</span> {{ t('welcome.find_geek') }}
          </RouterLink>
          <RouterLink to="/orders" class="action-btn">
            <span class="icon">📦</span> {{ t('welcome.my_orders') }}
          </RouterLink>
          <RouterLink to="/chat" class="action-btn">
            <span class="icon">💬</span> {{ t('welcome.messages') }}
          </RouterLink>
          <RouterLink to="/dashboard" class="action-btn">
            <span class="icon">📊</span> {{ t('welcome.dashboard') }}
          </RouterLink>
        </div>
      </div>
    </header>

    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>

    <template v-else>
      <!-- Top Geeks -->
      <section class="section">
        <div class="section-head">
          <h2 class="section-title">{{ t('welcome.top_geeks') }}</h2>
          <RouterLink to="/geeks" class="view-all">{{ t('welcome.view_all') }}</RouterLink>
        </div>
        
        <div v-if="geeks.length === 0" class="empty">{{ t('welcome.empty_geeks') }}</div>
        <div v-else class="geeks-grid">
          <RouterLink
            v-for="g in geeks"
            :key="g.id"
            :to="{ name: 'public-profile', params: { username: g.username } }"
            class="geek-card"
          >
            <Avatar :username="g.username" :src="g.avatar_url" :size="48" />
            <div class="geek-info">
              <div class="geek-name">@{{ g.username }}</div>
              <div class="geek-skills">
                {{ g.skills.slice(0, 2).join(', ') || t('welcome.geek_fallback') }}
              </div>
            </div>
          </RouterLink>
        </div>
      </section>

      <!-- Latest Services -->
      <section class="section">
        <div class="section-head">
          <h2 class="section-title">{{ t('welcome.fresh_market') }}</h2>
          <RouterLink to="/products" class="view-all">{{ t('welcome.view_all') }}</RouterLink>
        </div>

        <div v-if="products.length === 0" class="empty">{{ t('welcome.empty_products') }}</div>
        <div v-else class="products-grid">
          <RouterLink
            v-for="p in products"
            :key="p.id"
            :to="{ name: 'product-detail', params: { id: p.id } }"
            class="product-card"
          >
            <div class="product-img">
              <img v-if="p.cover_image" :src="p.cover_image" alt="Cover" loading="lazy" />
              <div v-else class="placeholder">{{ p.title.charAt(0) }}</div>
            </div>
            <div class="product-info">
              <span class="prod-title">{{ p.title }}</span>
              <div class="prod-price">
                {{ symbolFor(currencies, p.currency_code) }}
                {{ fmtAmount(currencies, p.price, p.currency_code) }}
              </div>
            </div>
          </RouterLink>
        </div>
      </section>
    </template>

  </div>
</template>

<style scoped>
/* Lightweight, fast scrolling CSS */
.welcome-page {
  display: flex;
  flex-direction: column;
  gap: 3rem;
  padding-bottom: 2rem;
}

/* HERO */
.hero {
  position: relative;
  background: rgba(15, 17, 21, 0.85);
  border: 1px solid #1e2229;
  border-radius: 12px;
  padding: 3rem 2.5rem;
  overflow: hidden;
  /* Removed backdrop-filter and heavy shadows for performance */
}

.hero-content {
  position: relative;
  z-index: 2;
  max-width: 700px;
}

.greeting {
  font-size: 2.8rem;
  font-weight: 800;
  color: #ffffff;
  margin: 0 0 0.5rem;
  letter-spacing: -0.03em;
  line-height: 1.1;
}

.accent {
  color: #00d9a3;
}

.subtitle {
  color: #8a95a8;
  font-size: 1.15rem;
  margin: 0 0 2rem;
  line-height: 1.5;
}

/* INLINE ACTION BAR */
.action-bar {
  display: inline-flex;
  background: #0a0e14;
  border: 1px solid #1e2229;
  border-radius: 8px;
  padding: 0.35rem;
  gap: 0.25rem;
}

.action-btn {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  background: transparent;
  color: #a1aab8;
  padding: 0.6rem 1.25rem;
  border-radius: 6px;
  text-decoration: none;
  font-size: 0.9rem;
  font-weight: 500;
  transition: color 0.15s, background 0.15s;
}

.action-btn:hover {
  color: #ffffff;
  background: rgba(255, 255, 255, 0.05);
}

.action-btn.primary {
  background: #00d9a3;
  color: #0a0e14;
  font-weight: 600;
}

.action-btn.primary:hover {
  background: #00f5aa;
}

.icon {
  font-size: 1rem;
}

/* SECTIONS */
.section {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.section-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
}

.section-title {
  font-size: 1.25rem;
  color: #e2e5e9;
  margin: 0;
}

.view-all {
  color: #6b7a90;
  text-decoration: none;
  font-size: 0.85rem;
}
.view-all:hover {
  color: #00d9a3;
}

.muted { color: #6b7a90; font-size: 0.9rem; }
.empty {
  padding: 2rem;
  text-align: center;
  border: 1px dashed #1e2229;
  border-radius: 8px;
  color: #6b7a90;
}

/* GEEKS GRID */
.geeks-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 1rem;
}

.geek-card {
  display: flex;
  align-items: center;
  gap: 1rem;
  background: rgba(15, 17, 21, 0.85);
  border: 1px solid #1e2229;
  border-radius: 8px;
  padding: 1rem;
  text-decoration: none;
  transition: border-color 0.15s;
}
.geek-card:hover {
  border-color: #00d9a3;
}

.geek-info {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.geek-name {
  color: #e2e5e9;
  font-weight: 600;
  font-size: 0.95rem;
}

.geek-skills {
  color: #6b7a90;
  font-size: 0.75rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 140px;
}

/* PRODUCTS GRID */
.products-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 1rem;
}

.product-card {
  display: flex;
  flex-direction: column;
  background: rgba(15, 17, 21, 0.85);
  border: 1px solid #1e2229;
  border-radius: 8px;
  overflow: hidden;
  text-decoration: none;
  transition: border-color 0.15s;
}
.product-card:hover {
  border-color: #00d9a3;
}

.product-img {
  height: 120px;
  background: #0a0e14;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.product-img img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.product-img .placeholder {
  font-size: 2rem;
  font-weight: bold;
  color: #1e2229;
}

.product-info {
  padding: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.prod-title {
  color: #e2e5e9;
  font-weight: 600;
  font-size: 0.95rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.prod-price {
  color: #00d9a3;
  font-weight: 600;
  font-size: 0.9rem;
}

@media (max-width: 700px) {
  .hero { padding: 2rem 1.5rem; }
  .greeting { font-size: 2rem; }
  .action-bar { display: flex; flex-direction: column; width: 100%; }
  .action-btn { width: 100%; justify-content: center; }
}
</style>
