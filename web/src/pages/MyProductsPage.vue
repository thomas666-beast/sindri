<script setup lang="ts">
import { RouterLink } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as productsApi from '../api/products';
import * as walletApi from '../api/wallet';
import { fmtAmount, symbolFor, getCurrencies } from '../lib/money';
import { useAuthStore } from '../stores/auth';
import { ref, watch } from 'vue';

const { t } = useI18n();
const auth = useAuthStore();

const products = ref<productsApi.Product[]>([]);
const currencies = ref<walletApi.Currency[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);
const removingId = ref<string | null>(null);
const actionError = ref<string | null>(null);

async function load() {
  if (!auth.user) return;
  loading.value = true;
  error.value = null;
  try {
    const result = await productsApi.search({
      seller: auth.user.id,
      limit: 100,
      offset: 0,
    });
    products.value = result.results;
  } catch (e: any) {
    const body = e.response?.data;
    error.value = body?.error ?? t('products.load_failed');
  } finally {
    loading.value = false;
  }
}

async function loadCurrencies() {
  try {
    currencies.value = await getCurrencies();
  } catch {
    // non-fatal
  }
}

async function remove(p: productsApi.Product) {
  if (!confirm(t('products.confirm_remove'))) return;
  actionError.value = null;
  removingId.value = p.id;
  try {
    await productsApi.deactivate(p.id);
    await load();
  } catch (e: any) {
    actionError.value = e.response?.data?.error ?? t('products.remove_failed');
  } finally {
    removingId.value = null;
  }
}

function fmtDate(unix: number): string {
  return new Date(unix * 1000).toISOString().slice(0, 10);
}

watch(
  () => auth.user?.id,
  (id) => {
    if (id) {
      load();
      loadCurrencies();
    }
  },
  { immediate: true }
);
</script>

<template>
  <div>
    <header class="page-head">
      <h1 class="heading">{{ t('products.my_heading') }}</h1>
      <RouterLink to="/products/new" class="btn-primary">
        {{ t('products.list_yours') }}
      </RouterLink>
    </header>

    <p v-if="actionError" class="err">! {{ actionError }}</p>
    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="err">! {{ error }}</p>
    <p v-else-if="products.length === 0" class="empty">
      {{ t('products.my_empty') }}
    </p>

    <div v-else class="grid">
      <div v-for="p in products" :key="p.id" class="card-wrap">
        <RouterLink
          :to="{ name: 'product-detail', params: { id: p.id } }"
          class="card"
        >
          <!-- Image Header -->
          <div class="card-img-wrap">
            <img v-if="p.cover_image" :src="p.cover_image" alt="Cover" class="cover-img" />
            <div v-else class="placeholder-img">
              {{ p.title.charAt(0).toUpperCase() }}
            </div>
          </div>

          <div class="card-body">
            <span class="title">{{ p.title }}</span>
            <p v-if="p.description" class="desc">{{ p.description }}</p>
            <div class="price">
              {{ symbolFor(currencies, p.currency_code) }}
              {{ fmtAmount(currencies, p.price, p.currency_code) }}
              <span class="cur">{{ p.currency_code }}</span>
            </div>
            <footer class="card-foot">
              <span class="meta">{{ fmtDate(p.created_at_unix) }}</span>
            </footer>
          </div>
        </RouterLink>
        
        <div class="card-actions">
          <RouterLink
            :to="{ name: 'product-edit', params: { id: p.id } }"
            class="btn-edit"
          >
            {{ t('products.edit') }}
          </RouterLink>
          <button
            class="btn-deactivate"
            :disabled="removingId === p.id"
            @click="remove(p)"
          >
            {{ removingId === p.id ? '…' : t('products.remove') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 1.25rem;
}
.heading {
  font-size: 1.1rem;
  color: #e2e5e9;
  margin: 0;
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
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 1rem;
}

/* Card Wrap */
.card-wrap {
  display: flex;
  flex-direction: column;
  border: 1px solid #1e2229;
  background: rgba(15, 17, 21, 0.85);
  border-radius: 6px;
  overflow: hidden;
  transition: border-color 0.1s;
}
.card-wrap:hover {
  border-color: #2a3039;
}

/* Card Link */
.card {
  display: flex;
  flex-direction: column;
  text-decoration: none;
  color: inherit;
  flex: 1;
}

/* Image */
.card-img-wrap {
  height: 160px;
  background: #0a0e14;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-bottom: 1px solid #1e2229;
}
.cover-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.placeholder-img {
  font-size: 3rem;
  font-weight: bold;
  color: #1e2229;
}

/* Body */
.card-body {
  padding: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  flex: 1;
}
.title {
  color: #e2e5e9;
  font-weight: 600;
  font-size: 1rem;
  text-decoration: none;
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
  padding-top: 0.5rem;
  margin-top: 0.5rem;
  border-top: 1px solid #1e2229;
}
.meta {
  color: #3a4150;
  font-size: 0.7rem;
}

/* Actions */
.card-actions {
  display: flex;
  gap: 0.375rem;
  padding: 0.75rem 1rem;
  background: #0a0e14;
  border-top: 1px solid #1e2229;
}
.btn-edit {
  background: transparent;
  color: #4a9eff;
  border: 1px solid #1e2229;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.75rem;
  padding: 0.3rem 0.8rem;
  cursor: pointer;
  text-decoration: none;
}
.btn-edit:hover {
  border-color: #4a9eff;
}
.btn-deactivate {
  background: transparent;
  color: #6b7a90;
  border: 1px solid #1e2229;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.75rem;
  padding: 0.3rem 0.8rem;
  cursor: pointer;
}
.btn-deactivate:hover:not(:disabled) {
  color: #ff5c5c;
  border-color: #ff5c5c;
}
</style>
