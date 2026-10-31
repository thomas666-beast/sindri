<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { RouterLink } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as productsApi from '../api/products';
import * as walletApi from '../api/wallet';
import { fmtAmount, symbolFor, getCurrencies } from '../lib/money';

const { t } = useI18n();
const products = ref<productsApi.Product[]>([]);
const currencies = ref<walletApi.Currency[]>([]);
const loading = ref(true);

onMounted(async () => {
  try {
    const [res, curr] = await Promise.all([
      productsApi.search({ limit: 12 }),
      getCurrencies()
    ]);
    products.value = res.results;
    currencies.value = curr;
  } catch (e) {}
  loading.value = false;
});
</script>

<template>
  <div class="home">
    <!-- Hero Section -->
    <header class="hero">
      <div class="hero-content">
        <h1 class="title">
          {{ t('home.hero_title') }} <span class="accent">{{ t('home.hero_accent') }}</span>
        </h1>
        <p class="subtitle">
          {{ t('home.hero_subtitle') }}
        </p>
        <div class="hero-actions">
          <RouterLink to="/geeks" class="primary">{{ t('home.find_geeks') }}</RouterLink>
          <RouterLink to="/products" class="ghost">{{ t('home.browse_services') }}</RouterLink>
        </div>
      </div>
    </header>

    <!-- Featured Products -->
    <section class="section">
      <div class="section-head">
        <h2 class="section-title">{{ t('home.fresh_market') }}</h2>
        <RouterLink to="/products" class="view-all">{{ t('home.view_all') }}</RouterLink>
      </div>

      <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
      <p v-else-if="products.length === 0" class="empty">{{ t('home.empty_products') }}</p>
      
      <div v-else class="grid">
        <RouterLink
          v-for="p in products"
          :key="p.id"
          :to="{ name: 'product-detail', params: { id: p.id } }"
          class="card"
        >
          <div class="card-img-wrap">
            <img v-if="p.cover_image" :src="p.cover_image" alt="Cover" class="cover-img" />
            <div v-else class="placeholder-img">
              {{ p.title.charAt(0).toUpperCase() }}
            </div>
          </div>
          <div class="card-body">
            <span class="title">{{ p.title }}</span>
            <p class="desc">{{ p.description }}</p>
            <div class="price">
              {{ symbolFor(currencies, p.currency_code) }}
              {{ fmtAmount(currencies, p.price, p.currency_code) }}
            </div>
          </div>
        </RouterLink>
      </div>
    </section>
  </div>
</template>

<style scoped>
.home { width: 100%; display: flex; flex-direction: column; gap: 4rem; padding-bottom: 4rem; }

/* Hero */
.hero {
  padding: 6rem 2rem;
  background: radial-gradient(circle at top right, rgba(0, 217, 163, 0.1), transparent 50%);
  text-align: center;
  border-bottom: 1px solid #1e2229;
}
.hero-content { max-width: 800px; margin: 0 auto; }
.title { font-size: 3rem; color: #e2e5e9; margin: 0 0 1rem; line-height: 1.1; letter-spacing: -0.02em; }
.accent { color: #00d9a3; }
.subtitle { color: #8a95a8; font-size: 1.1rem; max-width: 600px; margin: 0 auto 2rem; line-height: 1.5; }
.hero-actions { display: flex; gap: 1rem; justify-content: center; }

.primary {
  background: #00d9a3; color: #0a0e14; padding: 0.75rem 1.5rem; border-radius: 4px;
  font-weight: 600; text-decoration: none; transition: background 0.1s;
}
.primary:hover { background: #00f5aa; }
.ghost {
  background: transparent; color: #e2e5e9; padding: 0.75rem 1.5rem; border-radius: 4px;
  border: 1px solid #1e2229; text-decoration: none;
}
.ghost:hover { border-color: #00d9a3; color: #00d9a3; }

/* Section */
.section { padding: 0 2rem; max-width: 1200px; margin: 0 auto; width: 100%; }
.section-head { display: flex; justify-content: space-between; align-items: baseline; margin-bottom: 1.5rem; }
.section-title { font-size: 1.5rem; color: #e2e5e9; margin: 0; }
.view-all { color: #00d9a3; text-decoration: none; font-size: 0.9rem; }

/* Grid */
.grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 1.5rem; }
.card {
  background: rgba(15, 17, 21, 0.85); border: 1px solid #1e2229; border-radius: 8px;
  overflow: hidden; text-decoration: none; display: flex; flex-direction: column;
  transition: border-color 0.1s, transform 0.1s;
}
.card:hover { border-color: #2a3039; transform: translateY(-2px); }

.card-img-wrap { height: 160px; background: #0a0e14; display: flex; align-items: center; justify-content: center; overflow: hidden; }
.cover-img { width: 100%; height: 100%; object-fit: cover; }
.placeholder-img { font-size: 3rem; font-weight: bold; color: #1e2229; }

.card-body { padding: 1.25rem; display: flex; flex-direction: column; gap: 0.5rem; flex: 1; }
.card-body .title { color: #e2e5e9; font-weight: 600; font-size: 1.1rem; }
.desc { color: #6b7a90; font-size: 0.85rem; margin: 0; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.price { color: #00d9a3; font-weight: 600; font-size: 1.1rem; margin-top: auto; }

.muted { color: #6b7a90; }
.empty { padding: 3rem; text-align: center; color: #6b7a90; border: 1px dashed #1e2229; border-radius: 8px; }
</style>
