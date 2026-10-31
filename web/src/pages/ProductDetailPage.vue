<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';
import { RouterLink, useRoute, useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as productsApi from '../api/products';
import * as ordersApi from '../api/orders';
import * as reviewsApi from '../api/reviews';
import * as categoriesApi from '../api/categories';
import { fmtAmount, symbolFor, getCurrencies } from '../lib/money';
import { useAuthStore } from '../stores/auth';
import ReviewList from '../components/ReviewList.vue';
import CopyId from '../components/CopyId.vue';
import type { Currency } from '../api/wallet';
import * as chatApi from '../api/chatApi';

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const auth = useAuthStore();

const product = ref<productsApi.Product | null>(null);
const currencies = ref<Currency[]>([]);
const reviews = ref<reviewsApi.Review[]>([]);
const summary = ref<reviewsApi.Summary | null>(null);
const categoryPath = ref<string>('');
const loading = ref(true);
const reviewsLoading = ref(true);
const error = ref<string | null>(null);
const buying = ref(false);
const actionError = ref<string | null>(null);
const messaging = ref(false);

const images = ref<string[]>([]);
const activeImage = ref<string | null>(null);

async function load() {
  loading.value = true;
  error.value = null;
  try {
    product.value = await productsApi.get(route.params.id as string);
    await loadCategoryPath();
  } catch (e: any) {
    error.value = e.response?.data?.error ?? t('products.load_failed');
  } finally {
    loading.value = false;
  }
}

async function loadImages() {
  try {
    images.value = await productsApi.getImages(route.params.id as string);
    if (images.value.length > 0) activeImage.value = images.value[0];
  } catch {}
}

async function loadCurrencies() {
  try {
    currencies.value = await getCurrencies();
  } catch {}
}

async function loadReviews() {
  reviewsLoading.value = true;
  try {
    const page = await reviewsApi.listByProduct(route.params.id as string, 3, 0);
    reviews.value = page.reviews;
    summary.value = await reviewsApi.productSummary(route.params.id as string);
  } catch {
  } finally {
    reviewsLoading.value = false;
  }
}

async function loadCategoryPath() {
  if (!product.value?.category_id) return;
  try {
    const all = await categoriesApi.listAll();
    const cat = all.find((c) => c.id === product.value?.category_id);
    if (!cat) return;
    if (cat.parent_id) {
      const parent = all.find((c) => c.id === cat.parent_id);
      categoryPath.value = parent ? `${parent.name} / ${cat.name}` : cat.name;
    } else {
      categoryPath.value = cat.name;
    }
  } catch {}
}

async function buy() {
  if (!auth.user) return router.push({ name: 'login' });
  if (!product.value) return;
  actionError.value = null;
  buying.value = true;
  try {
    const order = await ordersApi.create(product.value.id);
    router.push({ name: 'order-detail', params: { id: order.id } });
  } catch (e: any) {
    const body = e.response?.data;
    actionError.value = body?.errors 
      ? Object.values(body.errors).join(', ') 
      : body?.error ?? t('orders.buy_failed');
  } finally {
    buying.value = false;
  }
}

async function messageSeller() {
  if (!auth.user) return router.push({ name: 'login' });
  if (!product.value?.seller_username) return;
  messaging.value = true;
  try {
    const conv = await chatApi.startDMByUsername(product.value.seller_username);
    router.push({ name: 'chat', query: { open: conv.id } });
  } catch (e: any) {
    actionError.value = e.response?.data?.error ?? 'could not open chat';
  } finally {
    messaging.value = false;
  }
}

const isMine = () => !!product.value && !!auth.user && auth.user.id === product.value.seller_id;

watch(() => route.params.id, () => {
  load();
  loadImages();
  loadReviews();
});

onMounted(() => {
  load();
  loadImages();
  loadCurrencies();
  loadReviews();
});
</script>

<template>
  <div class="page">
    <div class="top-nav">
      <RouterLink to="/products" class="back">← {{ t('products.back') }}</RouterLink>
      <CopyId :id="route.params.id as string" label="product id" />
    </div>

    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="err">! {{ error }}</p>

    <template v-else-if="product">
      
      <!-- MAIN CONTAINER: Holds Everything -->
      <div class="main-container">
        
        <!-- Top Section: Image + Details -->
        <div class="product-header-grid">
          
          <!-- Left: Image -->
          <div class="image-section">
            <div class="main-image">
              <img v-if="activeImage" :src="activeImage" alt="Product" />
              <div v-else class="placeholder">No image provided</div>
            </div>
            <div v-if="images.length > 1" class="thumbnails">
              <img 
                v-for="img in images" 
                :key="img"
                :src="img"
                class="thumb"
                :class="{ active: activeImage === img }"
                @click="activeImage = img"
              />
            </div>
          </div>

          <!-- Right: Details -->
          <div class="info-section">
            <div class="category-badge" v-if="categoryPath">{{ categoryPath }}</div>
            <h1 class="title">{{ product.title }}</h1>
            <div class="seller">
              {{ t('products.seller') }}
              <RouterLink
                :to="{ name: 'public-profile', params: { username: product.seller_username } }"
                class="seller-link"
              >
                @{{ product.seller_username }}
              </RouterLink>
            </div>

            <div class="divider"></div>

            <div class="description">
              <h3>About this service</h3>
              <p>{{ product.description || 'No description provided.' }}</p>
            </div>

            <div class="divider"></div>

            <div class="checkout-box">
              <div class="price">
                {{ symbolFor(currencies, product.currency_code) }}
                {{ fmtAmount(currencies, product.price, product.currency_code) }}
                <span class="cur">{{ product.currency_code }}</span>
              </div>
              
              <div class="actions">
                <template v-if="!isMine()">
                  <button class="btn-primary" :disabled="buying" @click="buy">
                    {{ buying ? t('common.loading') : `> ${t('products.buy_now')}` }}
                  </button>
                  <button class="btn-ghost" :disabled="messaging" @click="messageSeller">
                    {{ messaging ? t('common.loading') : t('products.message_seller') }}
                  </button>
                </template>
                
                <template v-else>
                  <RouterLink :to="{ name: 'product-edit', params: { id: product.id } }" class="btn-primary">
                    {{ t('products.edit') }}
                  </RouterLink>
                  <RouterLink to="/my-products" class="btn-ghost">
                    {{ t('products.back_to_mine') }}
                  </RouterLink>
                </template>
              </div>
              
              <p v-if="actionError" class="err" style="margin-top: 1rem;">! {{ actionError }}</p>
              
              <div class="escrow-notice" v-if="!isMine()">
                {{ t('products.escrow_notice') }}
              </div>
            </div>
          </div>
        </div>

        <!-- Bottom Section: Reviews -->
        <div class="reviews-section">
          <div class="reviews-header">
            <h2 class="section-title">Reviews ({{ summary?.review_count || 0 }})</h2>
            <RouterLink 
              v-if="summary && summary.review_count > 3"
              :to="{ name: 'public-profile', params: { username: product.seller_username } }"
              class="see-all-link"
            >
              See all reviews →
            </RouterLink>
          </div>
          
          <!-- Wrapped to remove inner borders -->
          <div class="reviews-clean-wrapper">
            <ReviewList :reviews="reviews" :summary="summary" :loading="reviewsLoading" />
          </div>
        </div>

      </div>
    </template>
  </div>
</template>

<style scoped>
.page {
  /* Removed max-width: 1100px */
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.top-nav {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.back { color: #6b7a90; text-decoration: none; font-size: 0.85rem; }
.back:hover { color: #00d9a3; }

.muted { color: #6b7a90; font-size: 0.9rem; }
.err { color: #ff5c5c; font-size: 0.85rem; }

/* MAIN CONTAINER */
.main-container {
  background: rgba(15, 17, 21, 0.85);
  border: 1px solid #1e2229;
  border-radius: 8px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

/* TOP GRID */
.product-header-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
}

@media (max-width: 800px) {
  .product-header-grid { grid-template-columns: 1fr; }
}

/* IMAGE SECTION */
.image-section {
  padding: 2rem;
  background: #0a0e14;
  border-right: 1px solid #1e2229;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.main-image {
  width: 100%;
  aspect-ratio: 4 / 3;
  background: #000;
  border-radius: 6px;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
}

.main-image img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.placeholder {
  color: #6b7a90;
  font-style: italic;
}

.thumbnails {
  display: flex;
  gap: 0.5rem;
  overflow-x: auto;
}

.thumb {
  width: 60px;
  height: 60px;
  object-fit: cover;
  border-radius: 4px;
  border: 2px solid transparent;
  cursor: pointer;
  opacity: 0.6;
}
.thumb:hover, .thumb.active {
  border-color: #00d9a3;
  opacity: 1;
}

/* INFO SECTION */
.info-section {
  padding: 2rem;
  display: flex;
  flex-direction: column;
}

.category-badge {
  align-self: flex-start;
  background: rgba(0, 217, 163, 0.1);
  color: #00d9a3;
  padding: 0.2rem 0.6rem;
  border-radius: 4px;
  font-size: 0.75rem;
  margin-bottom: 0.75rem;
}

.title {
  font-size: 1.8rem;
  font-weight: 600;
  color: #e2e5e9;
  margin: 0 0 0.5rem;
  line-height: 1.2;
}

.seller {
  font-size: 0.9rem;
  color: #6b7a90;
  margin-bottom: 1rem;
}

.seller-link {
  color: #4a9eff;
  text-decoration: none;
}
.seller-link:hover { text-decoration: underline; }

.divider {
  height: 1px;
  background: #1e2229;
  margin: 1rem 0;
}

.description h3 {
  font-size: 0.85rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: #6b7a90;
  margin: 0 0 0.5rem;
}

.description p {
  color: #c8d3e0;
  font-size: 0.95rem;
  line-height: 1.6;
  margin: 0;
  white-space: pre-wrap;
}

/* CHECKOUT BOX */
.checkout-box {
  margin-top: auto;
  background: #0a0e14;
  border: 1px solid #1e2229;
  border-radius: 6px;
  padding: 1.25rem;
}

.price {
  font-size: 2rem;
  font-weight: 700;
  color: #e2e5e9;
  margin-bottom: 1rem;
}

.cur {
  font-size: 0.9rem;
  color: #6b7a90;
  font-weight: normal;
}

.actions {
  display: flex;
  gap: 0.75rem;
}

.btn-primary {
  flex: 1;
  background: #00d9a3;
  color: #0a0e14;
  border: none;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.95rem;
  font-weight: bold;
  padding: 0.7rem;
  cursor: pointer;
  text-align: center;
  text-decoration: none;
}
.btn-primary:hover:not(:disabled) { background: #00f5aa; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }

.btn-ghost {
  flex: 1;
  background: transparent;
  color: #e2e5e9;
  border: 1px solid #1e2229;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.95rem;
  padding: 0.7rem;
  cursor: pointer;
  text-align: center;
  text-decoration: none;
}
.btn-ghost:hover { border-color: #00d9a3; color: #00d9a3; }

.escrow-notice {
  font-size: 0.75rem;
  color: #4a9eff;
  margin-top: 1rem;
  padding-left: 0.75rem;
  border-left: 2px solid #4a9eff;
  line-height: 1.4;
}

/* BOTTOM REVIEWS SECTION */
.reviews-section {
  padding: 2rem;
  border-top: 1px solid #1e2229;
  background: rgba(15, 17, 21, 0.85);
}

.reviews-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 1.5rem;
}

.section-title {
  font-size: 1.2rem;
  color: #e2e5e9;
  margin: 0;
}

.see-all-link {
  color: #00d9a3;
  text-decoration: none;
  font-size: 0.85rem;
  font-weight: 500;
}
.see-all-link:hover { text-decoration: underline; }

/* IMPORTANT: Remove the inner border/background of ReviewList so it merges into the page */
.reviews-clean-wrapper :deep(.reviews) {
  border-top: none !important;
  padding-top: 0 !important;
  margin-top: 0 !important;
}
</style>
