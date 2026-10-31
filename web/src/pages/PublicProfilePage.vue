<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as profileApi from '../api/profile';
import * as chatApi from '../api/chatApi';
import * as reviewsApi from '../api/reviews';
import * as productsApi from '../api/products';
import * as walletApi from '../api/wallet';
import { useAuthStore } from '../stores/auth';
import Avatar from '../components/Avatar.vue';
import ReviewList from '../components/ReviewList.vue';
import { fmtAmount, symbolFor, getCurrencies } from '../lib/money';

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const auth = useAuthStore();

const profile = ref<profileApi.Profile | null>(null);
const stats = ref<profileApi.GeekStats | null>(null);
const products = ref<productsApi.Product[]>([]);
const reviews = ref<reviewsApi.Review[]>([]);
const summary = ref<reviewsApi.Summary | null>(null);
const currencies = ref<walletApi.Currency[]>([]);

const loading = ref(true);
const error = ref<string | null>(null);
const dmBusy = ref(false);

async function load() {
  loading.value = true;
  error.value = null;
  const username = route.params.username as string;
  
  try {
    // 1. Fetch Profile
    profile.value = await profileApi.getPublic(username);
    
    // 2. Fetch Stats, Products, Reviews in parallel
    const [statsRes, prodRes, revPage, revSum, currRes] = await Promise.all([
      profileApi.getStats(username),
      productsApi.search({ seller: profile.value.user_id, limit: 10 }),
      reviewsApi.listBySeller(profile.value.user_id, 5, 0),
      reviewsApi.sellerSummary(profile.value.user_id),
      getCurrencies(),
    ]);
    
    stats.value = statsRes;
    products.value = prodRes.results;
    reviews.value = revPage.reviews;
    summary.value = revSum;
    currencies.value = currRes;
    
  } catch (e: any) {
    error.value = e.response?.data?.error ?? t('profile.not_found');
  } finally {
    loading.value = false;
  }
}

async function message() {
  if (!profile.value || !auth.user) {
    router.push({ name: 'login' });
    return;
  }
  if (profile.value.user_id === auth.user.id) return;
  dmBusy.value = true;
  try {
    const conv = await chatApi.startDMByUsername(profile.value.username);
    router.push({ name: 'chat', query: { open: conv.id } });
  } catch (e: any) {
    error.value = e.response?.data?.error ?? 'could not open chat';
  } finally {
    dmBusy.value = false;
  }
}

function fmtDate(unix: number): string {
  return new Date(unix * 1000).toISOString().slice(0, 10);
}

watch(() => route.params.username, load);
onMounted(load);
</script>

<template>
  <div class="profile-page">
    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="err">! {{ error }}</p>

    <template v-else-if="profile && stats">
      <!-- Hero Section -->
      <header class="hero-panel">
        <div class="hero-left">
          <Avatar :username="profile.username" :src="profile.avatar_url" :size="96" />
          <div class="identity">
            <h1 class="display-name">{{ profile.display_name }}</h1>
            <div class="username">@{{ profile.username }}</div>
            
            <div class="stats-row">
              <div class="stat">
                <span class="stat-value">⭐ {{ stats.avg_rating.toFixed(1) }}</span>
                <span class="stat-label">{{ stats.review_count }} reviews</span>
              </div>
              <div class="stat">
                <span class="stat-value">{{ stats.completed_orders }}</span>
                <span class="stat-label">completed orders</span>
              </div>
              <div class="stat">
                <span class="stat-value">{{ stats.active_products }}</span>
                <span class="stat-label">active listings</span>
              </div>
            </div>
            <div class="meta">Member since {{ fmtDate(profile.member_since_unix) }}</div>
          </div>
        </div>
        
        <div class="hero-actions">
          <button v-if="auth.user && auth.user.id !== profile.user_id" class="primary" :disabled="dmBusy" @click="message">
            {{ dmBusy ? t('common.loading') : `> ${t('profile.message')}` }}
          </button>
          <RouterLink v-else-if="!auth.user" :to="{ name: 'login' }" class="primary">
            {{ t('common.login') }}
          </RouterLink>
          <RouterLink v-else to="/profile" class="ghost">
            {{ t('profile.edit_yours') }}
          </RouterLink>
        </div>
      </header>

      <!-- About & Skills -->
      <section class="section card">
        <h2 class="section-title">About</h2>
        <p v-if="profile.bio" class="bio">{{ profile.bio }}</p>
        <p v-else class="muted">No bio provided.</p>

        <div v-if="profile.skills.length > 0" class="chips-group">
          <span class="label-caps">Skills</span>
          <div class="chips">
            <span v-for="s in profile.skills" :key="s" class="chip primary">{{ s }}</span>
          </div>
        </div>
        
        <div v-if="profile.interests.length > 0" class="chips-group">
          <span class="label-caps">Interests</span>
          <div class="chips">
            <span v-for="s in profile.interests" :key="s" class="chip secondary">{{ s }}</span>
          </div>
        </div>
      </section>

      <!-- Services & Products -->
      <section class="section">
        <h2 class="section-title">{{ t('products.heading') }}</h2>
        <div v-if="products.length === 0" class="empty-card">
          No active listings.
        </div>
        <div v-else class="grid">
          <RouterLink
            v-for="p in products"
            :key="p.id"
            :to="{ name: 'product-detail', params: { id: p.id } }"
            class="product-card"
          >
            <span class="prod-title">{{ p.title }}</span>
            <p class="prod-desc">{{ p.description }}</p>
            <div class="prod-price">
              {{ symbolFor(currencies, p.currency_code) }}
              {{ fmtAmount(currencies, p.price, p.currency_code) }}
              <span class="cur">{{ p.currency_code }}</span>
            </div>
          </RouterLink>
        </div>
      </section>

      <!-- Reviews -->
      <section class="section">
        <h2 class="section-title">Reviews</h2>
        <ReviewList :reviews="reviews" :summary="summary" :loading="false" />
      </section>
    </template>
  </div>
</template>

<style scoped>
.profile-page {
  /* Removed max-width: 900px and margin: 0 auto */
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.muted { color: #6b7a90; font-size: 0.85rem; }
.err { color: #ff5c5c; font-size: 0.85rem; }

/* Hero Panel */
.hero-panel {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 2.5rem;
  background: rgba(15, 17, 21, 0.85);
  border: 1px solid #1e2229;
  border-radius: 8px;
  gap: 1.5rem;
  flex-wrap: wrap;
}

.hero-left {
  display: flex;
  gap: 1.5rem;
  align-items: flex-start;
}

.identity { display: flex; flex-direction: column; gap: 0.25rem; }

.display-name {
  font-size: 1.8rem;
  font-weight: 600;
  color: #e2e5e9;
  margin: 0;
  line-height: 1.2;
}

.username { color: #00d9a3; font-size: 1.1rem; font-weight: 500; }

.stats-row {
  display: flex;
  gap: 2rem;
  margin-top: 1.25rem;
  margin-bottom: 0.5rem;
}

.stat { display: flex; flex-direction: column; }

.stat-value { color: #e2e5e9; font-weight: 600; font-size: 1.25rem; }
.stat-label { color: #6b7a90; font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em; }

.meta { color: #6b7a90; font-size: 0.8rem; }

.hero-actions { display: flex; gap: 0.5rem; }

/* Buttons */
.primary {
  background: #00d9a3;
  color: #0a0e14;
  border: none;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.9rem;
  font-weight: 600;
  padding: 0.6rem 1.5rem;
  cursor: pointer;
  text-decoration: none;
}
.primary:hover:not(:disabled) { background: #00f5aa; }
.primary:disabled { opacity: 0.5; cursor: not-allowed; }

.ghost {
  background: transparent;
  color: #e2e5e9;
  border: 1px solid #1e2229;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.9rem;
  padding: 0.6rem 1.5rem;
  text-decoration: none;
}
.ghost:hover { border-color: #00d9a3; color: #00d9a3; }

/* Sections */
.section { display: flex; flex-direction: column; gap: 1rem; }

.section-title {
  color: #6b7a90;
  font-size: 0.8rem;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  font-weight: 500;
  margin: 0;
  border-bottom: 1px solid #1e2229;
  padding-bottom: 0.5rem;
}

.card {
  background: rgba(15, 17, 21, 0.85);
  border: 1px solid #1e2229;
  border-radius: 8px;
  padding: 2rem;
}

.bio { color: #c8d3e0; font-size: 1rem; line-height: 1.6; margin: 0 0 1.5rem; }

/* Chips */
.chips-group { margin-bottom: 1rem; }
.label-caps { display: block; margin-bottom: 0.5rem; color: #6b7a90; font-size: 0.7rem; text-transform: uppercase; }
.chips { display: flex; flex-wrap: wrap; gap: 0.375rem; }

.chip {
  padding: 0.25rem 0.75rem;
  border-radius: 4px;
  font-size: 0.8rem;
  border: 1px solid;
}
.chip.primary {
  background: rgba(0, 229, 160, 0.08);
  color: #00e5a0;
  border-color: rgba(0, 229, 160, 0.3);
}
.chip.secondary {
  background: rgba(74, 158, 255, 0.08);
  color: #4a9eff;
  border-color: rgba(74, 158, 255, 0.3);
}

/* Grid of Products */
.grid {
  display: grid;
  /* Changed minmax from 250px to 320px for wider cards */
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 1.25rem;
}

.product-card {
  border: 1px solid #1e2229;
  background: rgba(15, 17, 21, 0.85);
  border-radius: 6px;
  padding: 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  text-decoration: none;
  transition: border-color 0.1s;
}
.product-card:hover { border-color: #2a3039; }

.prod-title { color: #e2e5e9; font-weight: 600; font-size: 1.1rem; }
.prod-desc {
  color: #6b7a90;
  font-size: 0.85rem;
  margin: 0;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.prod-price { color: #e2e5e9; font-weight: 600; margin-top: auto; font-size: 1.25rem; }
.cur { color: #6b7a90; font-size: 0.8rem; font-weight: normal; margin-left: 0.25rem; }

.empty-card {
  padding: 3rem;
  text-align: center;
  color: #6b7a90;
  border: 1px dashed #1e2229;
  border-radius: 6px;
  font-size: 0.9rem;
}

@media (max-width: 700px) {
  .hero-panel { flex-direction: column; padding: 1.5rem; }
  .hero-left { flex-direction: column; align-items: center; text-align: center; }
  .stats-row { justify-content: center; }
  .hero-actions { width: 100%; justify-content: center; }
  .card { padding: 1.5rem; }
  .grid { grid-template-columns: 1fr; }
}
</style>

