<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { RouterLink } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as productsApi from '../api/products';
import * as walletApi from '../api/wallet';
import * as categoriesApi from '../api/categories';
import { fmtAmount, getCurrencies, symbolFor } from '../lib/money';
import { useAuthStore } from '../stores/auth';
import SearchById from '../components/SearchById.vue';

const { t } = useI18n();
const auth = useAuthStore();

const products = ref<productsApi.Product[]>([]);
const currencies = ref<walletApi.Currency[]>([]);
const topCats = ref<categoriesApi.Category[]>([]);
const childCats = ref<Record<string, categoriesApi.Category[]>>({});
const loading = ref(true);
const error = ref<string | null>(null);

const activeSlug = ref<string>('');
const activeChildId = ref<string>('');
const searchQuery = ref('');
const searchId = ref(''); // <-- NEW: Tracks the ID search

const PAGE_SIZE = 30;
const offset = ref(0);
const hasMore = ref(true);
const total = ref(0);

let debounceTimer: number | null = null;

const filtered = computed(() => {
  // Note: The text filtering now happens in the backend, so we just return the list
  return products.value;
});

async function loadCurrencies() {
  try {
    currencies.value = await getCurrencies();
  } catch {
    // non-fatal
  }
}

async function loadCategories() {
  try {
    topCats.value = await categoriesApi.listTop();
    for (const parent of topCats.value) {
      const children = await categoriesApi.listChildren(parent.slug);
      childCats.value[parent.slug] = children;
    }
  } catch {
    // non-fatal
  }
}

async function loadProducts(reset = true) {
  loading.value = true;
  error.value = null;
  if (reset) {
    offset.value = 0;
    hasMore.value = true;
  }
  try {
    const result = await productsApi.search({
      q: searchQuery.value.trim(),
      category: activeChildId.value || '',
      id: searchId.value.trim(),
      limit: PAGE_SIZE,
      offset: offset.value,
    });
    if (reset) {
      products.value = result.results;
    } else {
      products.value.push(...result.results);
    }
    offset.value += result.results.length;
    total.value = result.total;
    hasMore.value = products.value.length < result.total;
  } catch (e: any) {
    // keep your existing error handling here
  } finally {
    loading.value = false;
  }
}

function onInput() {
  if (debounceTimer) window.clearTimeout(debounceTimer);
  debounceTimer = window.setTimeout(() => {
    loadProducts();
  }, 300);
}

// <-- NEW: Trigger a reload when the ID changes
watch(searchId, () => {
  loadProducts();
});

function selectCategory(parentSlug: string) {
  if (activeSlug.value === parentSlug) {
    activeSlug.value = '';
    activeChildId.value = '';
  } else {
    activeSlug.value = parentSlug;
    activeChildId.value = '';
  }
  loadProducts();
}

function selectChild(id: string) {
  activeChildId.value = activeChildId.value === id ? '' : id;
  loadProducts();
}

function fmtDate(unix: number): string {
  return new Date(unix * 1000).toISOString().slice(0, 10);
}

const isMine = (p: productsApi.Product) => auth.user?.id === p.seller_id;

onMounted(() => {
  loadCurrencies();
  loadCategories();
  loadProducts();
});
</script>

<template>
  <div class="products-layout">
    <aside class="cat-sidebar">
      <button
        class="cat-item top-item"
        :class="{ active: activeSlug === '' }"
        @click="selectCategory('')"
      >
        {{ t('products.all') }}
      </button>
      <div v-for="parent in topCats" :key="parent.id" class="cat-group">
        <button
          class="cat-item top-item"
          :class="{ active: activeSlug === parent.slug }"
          @click="selectCategory(parent.slug)"
        >
          {{ parent.name }}
        </button>
        <div v-if="activeSlug === parent.slug && childCats[parent.slug]?.length" class="children">
          <button
            v-for="child in childCats[parent.slug]"
            :key="child.id"
            class="cat-item child-item"
            :class="{ active: activeChildId === child.id }"
            @click="selectChild(child.id)"
          >
            {{ child.name }}
          </button>
        </div>
      </div>
    </aside>

    <section class="main-area">
      <header class="page-head">
        <h1 class="heading">
          {{ t('products.heading') }}
          <span v-if="!loading && products.length" class="count">({{ products.length }} / {{ total }})</span>
        </h1>
        <div class="head-actions">
          <SearchById v-model="searchId" />
          <input
            v-model="searchQuery"
            type="search"
            :placeholder="t('products.search')"
            class="search"
            @input="onInput"
          />
          <RouterLink v-if="auth.user" to="/products/new" class="btn-primary">
            {{ t('products.list_yours') }}
          </RouterLink>
        </div>
      </header>

      <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
      <p v-else-if="error" class="err">! {{ error }}</p>
      <div v-else-if="filtered.length === 0" class="empty">
        {{ t('products.empty') }}
      </div>

      <div v-else class="grid">
        <RouterLink
          v-for="p in filtered"
          :key="p.id"
          :to="{ name: 'product-detail', params: { id: p.id } }"
          class="card"
          :class="{ mine: isMine(p) }"
        >
          <div class="card-img-wrap">
            <img
              v-if="p.cover_image"
              :src="p.cover_image"
              alt="Cover"
              class="cover-img"
              loading="lazy"
              decoding="async"
            />
            <div v-else class="placeholder-img">
              {{ p.title.charAt(0).toUpperCase() }}
            </div>
          </div>
          <span class="title">{{ p.title }}</span>
          <p v-if="p.description" class="desc">{{ p.description }}</p>
          <div class="price">
            {{ symbolFor(currencies, p.currency_code) }}
            {{ fmtAmount(currencies, p.price, p.currency_code) }}
            <span class="cur">{{ p.currency_code }}</span>
          </div>
          <footer class="card-foot">
            <span class="meta">{{ fmtDate(p.created_at_unix) }}</span>
            <span v-if="isMine(p)" class="badge-you">{{ t('products.you') }}</span>
            <span v-else class="view-hint">{{ t('products.click_to_view') }} →</span>
          </footer>
        </RouterLink>
      </div>
      <div v-if="hasMore" class="load-more-wrap">
        <button
          class="load-more"
          :disabled="loading"
          @click="loadProducts(false)"
        >
          {{ loading ? t('common.loading') : t('products.load_more') }}
        </button>
      </div>
    </section>
  </div>
</template>

<style scoped>
.card {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  text-decoration: none;
  color: inherit;
  border: 1px solid #1e2229;
  background: rgba(15, 17, 21, 0.85);
  border-radius: 6px;
  padding: 1rem;
  transition: border-color 0.1s, background 0.1s;
}
.card:hover {
  border-color: #2a3039;
  background: rgba(20, 24, 32, 0.9);
}
.view-hint {
  color: #6b7a90;
  font-size: 0.7rem;
}
.products-layout {
  display: grid;
  grid-template-columns: 200px 1fr;
  gap: 1.5rem;
}
.cat-sidebar {
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
}
.cat-group {
  display: flex;
  flex-direction: column;
}
.cat-item {
  text-align: left;
  background: transparent;
  border: none;
  color: #8a95a8;
  font-family: inherit;
  font-size: 0.85rem;
  padding: 0.4rem 0.6rem;
  cursor: pointer;
  border-radius: 4px;
  transition: color 0.1s, background 0.1s;
}
.cat-item:hover {
  color: #e2e5e9;
  background: rgba(15, 17, 21, 0.9);
}
.cat-item.active {
  color: #00d9a3;
  background: rgba(0, 229, 160, 0.08);
}
.top-item {
  font-weight: 500;
}
.children {
  display: flex;
  flex-direction: column;
  padding-left: 0.75rem;
  margin: 0.125rem 0 0.375rem 0;
  border-left: 1px solid rgba(30, 34, 41, 0.6);
}
.child-item {
  font-size: 0.8rem;
  padding: 0.3rem 0.5rem;
}
.main-area {
  min-width: 0;
}
.page-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.25rem;
  gap: 0.75rem;
  flex-wrap: wrap;
}
.heading {
  font-size: 1.1rem;
  color: #e2e5e9;
}
.head-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}
.search {
  width: 200px;
}
.btn-primary {
  background: #00d9a3;
  color: #0a0e14;
  font-weight: bold;
  font-size: 0.85rem;
  padding: 0.5rem 0.9rem;
  border-radius: 4px;
  text-decoration: none;
}
.btn-primary:hover {
  color: #0a0e14;
  opacity: 0.9;
}
.muted {
  color: #6b7a90;
}
.err {
  color: #ff5c5c;
  font-size: 0.85rem;
}
.empty {
  border: 1px dashed #1f2633;
  padding: 2rem;
  text-align: center;
  color: #6b7a90;
  border-radius: 4px;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 1rem;
}
.card {
  border: 1px solid #1e2229;
  background: rgba(15, 17, 21, 0.85);
  border-radius: 6px;
  padding: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  position: relative;
  transition: border-color 0.1s;
}
.card:hover {
  border-color: #2a3039;
}
.card.mine {
  border-color: rgba(0, 229, 160, 0.3);
}
.title {
  color: #e2e5e9;
  font-weight: 600;
  font-size: 1rem;
  word-break: break-word;
}
.title:hover {
  color: #00d9a3;
}
.desc {
  color: #6b7a90;
  font-size: 0.8rem;
  margin: 0;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.price {
  font-size: 1.25rem;
  font-weight: 600;
  color: #e2e5e9;
  margin-top: auto;
}
.cur {
  color: #6b7a90;
  font-size: 0.75rem;
  font-weight: normal;
  margin-left: 0.25rem;
}
.card-foot {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 0.5rem;
  padding-top: 0.5rem;
  border-top: 1px solid #1e2229;
  gap: 0.5rem;
}
.meta {
  color: #3a4150;
  font-size: 0.7rem;
}
.btn-buy {
  background: transparent;
  color: #00e5a0;
  border: 1px solid #00e5a0;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.75rem;
  font-weight: 600;
  padding: 0.25rem 0.7rem;
  cursor: pointer;
}
.btn-buy:hover:not(:disabled) {
  background: rgba(0, 229, 160, 0.08);
}
.btn-buy:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
.badge-you {
  background: rgba(0, 229, 160, 0.08);
  color: #00e5a0;
  border: 1px solid rgba(0, 229, 160, 0.3);
  font-size: 0.7rem;
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  text-decoration: none;
}
.badge-you:hover {
  color: #00f5aa;
}
@media (max-width: 800px) {
  .products-layout {
    grid-template-columns: 1fr;
  }
  .cat-sidebar {
    flex-direction: row;
    flex-wrap: wrap;
    gap: 0.25rem;
    padding-bottom: 1rem;
    border-bottom: 1px solid #1e2229;
    margin-bottom: 1rem;
  }
  .children {
    display: none;
  }
}
@media (max-width: 700px) {
  .page-head {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.75rem;
  }
  .filters {
    flex-direction: column;
    align-items: flex-start;
  }
  .form-row, .row { 
    /* Ensure any side-by-side inputs stack vertically */
    grid-template-columns: 1fr !important;
    flex-direction: column;
  }
}
.card { 
  padding: 0; /* Remove old padding so image touches edges */
  overflow: hidden;
}
.card-img-wrap {
  height: 160px;
  background: #0a0e14;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-bottom: 1px solid #1e2229;
}
.cover-img { width: 100%; height: 100%; object-fit: cover; }
.placeholder-img { font-size: 2.5rem; font-weight: bold; color: #1e2229; }
/* You may need to add padding back to the text elements inside the card */
.title, .desc, .price, .card-foot {
  padding: 0 1rem; /* Or adjust to fit your layout */
}
.load-more-wrap {
  display: flex;
  justify-content: center;
  margin-top: 1.25rem;
}
.load-more {
  background: transparent;
  color: #00d9a3;
  border: 1px solid #00d9a3;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.85rem;
  font-weight: 600;
  padding: 0.5rem 1.25rem;
  cursor: pointer;
  transition: background 0.1s;
}
.load-more:hover:not(:disabled) {
  background: rgba(0, 217, 163, 0.08);
}
.load-more:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.count {
  color: #6b7a90;
  font-size: 0.75rem;
  font-weight: normal;
  margin-left: 0.5rem;
}
</style>
