<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { RouterLink, useRoute, useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as productsApi from '../api/products';
import * as walletApi from '../api/wallet';
import * as categoriesApi from '../api/categories';
import { digitsFor, fmtAmount, formatForInput, parseAmount, symbolFor } from '../lib/money';

const { t } = useI18n();
const route = useRoute();
const router = useRouter();

const loading = ref(true);
const loadError = ref<string | null>(null);
const submitting = ref(false);
const submitError = ref<string | null>(null);

const product = ref<productsApi.Product | null>(null);
const currencies = ref<walletApi.Currency[]>([]);
const topCats = ref<categoriesApi.Category[]>([]);
const childCats = ref<Record<string, categoriesApi.Category[]>>({});

const title = ref('');
const description = ref('');
const priceStr = ref('');
const parentSlug = ref('');
const childId = ref('');

const images = ref<string[]>([]);
const uploading = ref(false);

async function load() {
  loading.value = true;
  loadError.value = null;
  try {
    const p = await productsApi.get(route.params.id as string);
    product.value = p;
    title.value = p.title;
    description.value = p.description;

    // Resolve category into parent/child.
    if (p.category_id) {
      const all = await categoriesApi.listAll();
      const cat = all.find((c) => c.id === p.category_id);
      if (cat) {
        if (cat.parent_id) {
          const parent = all.find((c) => c.id === cat.parent_id);
          if (parent) {
            parentSlug.value = parent.slug;
            childId.value = cat.id;
          }
        } else {
          parentSlug.value = cat.slug;
        }
      }
    }
  } catch (e: any) {
    loadError.value = e.response?.data?.error ?? t('products.load_failed');
  } finally {
    loading.value = false;
  }
}

async function loadImages() {
  try {
    images.value = await productsApi.getImages(route.params.id as string);
  } catch {}
}

async function handleImageUpload(e: Event) {
  const target = e.target as HTMLInputElement;
  const file = target.files?.[0];
  if (!file) return;

  uploading.value = true;
  try {
    const url = await productsApi.uploadImage(route.params.id as string, file);
    images.value.push(url);
    target.value = '';
  } catch {
    submitError.value = 'Failed to upload image';
  } finally {
    uploading.value = false;
  }
}

async function loadCurrencies() {
  try {
    currencies.value = await walletApi.listCurrencies();
    if (product.value) {
      priceStr.value = formatForInput(
        product.value.price,
        digitsFor(currencies.value, product.value.currency_code)
      );
    }
  } catch {
    // non-fatal
  }
}

async function loadCategories() {
  try {
    topCats.value = await categoriesApi.listTop();
    for (const parent of topCats.value) {
      childCats.value[parent.slug] = await categoriesApi.listChildren(parent.slug);
    }
  } catch {
    // non-fatal
  }
}

const preview = computed(() => {
  const cur = product.value?.currency_code ?? 'USD';
  const cents = parseAmount(priceStr.value, digitsFor(currencies.value, cur));
  if (cents == null || cents <= 0) return '';
  return fmtAmount(currencies.value, cents, cur);
});

function onParentChange() {
  childId.value = '';
}

async function submit() {
  submitError.value = null;
  if (!title.value.trim()) {
    submitError.value = 'title is required';
    return;
  }

  const cur = product.value?.currency_code ?? 'USD';
  const priceCents = parseAmount(priceStr.value, digitsFor(currencies.value, cur));
  if (priceCents == null || priceCents <= 0) {
    submitError.value = 'price must be a positive amount';
    return;
  }

  const categoryId =
    childId.value ||
    (parentSlug.value
      ? (topCats.value.find((c) => c.slug === parentSlug.value)?.id ?? '')
      : '');

  submitting.value = true;
  try {
    await productsApi.update(route.params.id as string, {
      title: title.value.trim(),
      description: description.value,
      price: priceCents,
      category_id: categoryId,
    });
    router.push({ name: 'my-products' });
  } catch (e: any) {
    const body = e.response?.data;
    if (body?.errors) {
      submitError.value = Object.values(body.errors).join(', ');
    } else {
      submitError.value = body?.error ?? t('products.update_failed');
    }
  } finally {
    submitting.value = false;
  }
}

onMounted(async () => {
  await load();
  await loadCurrencies();
  await loadCategories();
  await loadImages();
});
</script>

<template>
  <div class="page">
    <RouterLink to="/my-products" class="back">{{ t('products.back') }}</RouterLink>

    <h1 class="heading">{{ t('products.edit_heading') }}</h1>

    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="loadError" class="err">! {{ loadError }}</p>

    <form v-else-if="product" @submit.prevent="submit" class="form">
      <div class="readonly-row">
        <span class="ro-label">{{ t('products.currency') }}</span>
        <span class="ro-value">{{ product.currency_code }}</span>
        <span class="ro-hint">{{ t('products.currency_locked') }}</span>
      </div>

      <label class="field">
        <span class="label">{{ t('products.title') }}</span>
        <input
          v-model="title"
          type="text"
          required
          maxlength="200"
          autocomplete="off"
        />
      </label>

      <label class="field">
        <span class="label">{{ t('products.description') }}</span>
        <textarea v-model="description" rows="4" />
      </label>

      <div class="row">
        <label class="field">
          <span class="label">{{ t('products.category') }}</span>
          <select v-model="parentSlug" @change="onParentChange">
            <option value="">{{ t('products.category_none') }}</option>
            <option v-for="c in topCats" :key="c.id" :value="c.slug">
              {{ c.name }}
            </option>
          </select>
        </label>
        <label v-if="parentSlug && childCats[parentSlug]?.length" class="field">
          <span class="label">{{ t('products.subcategory') }}</span>
          <select v-model="childId">
            <option value="">{{ t('products.subcategory_none') }}</option>
            <option v-for="c in childCats[parentSlug]" :key="c.id" :value="c.id">
              {{ c.name }}
            </option>
          </select>
        </label>
      </div>

      <label class="field">
        <span class="label">{{ t('products.price') }}</span>
        <input
          v-model="priceStr"
          type="text"
          inputmode="decimal"
          placeholder="0.00"
          required
        />
        <span class="hint">{{ t('products.price_hint') }}</span>
      </label>

      <p v-if="preview" class="preview">
        <span class="dim">=</span>
        {{ symbolFor(currencies, product.currency_code) }} {{ preview }}
      </p>

      <!-- Image Uploader -->
      <div class="field">
        <span class="label">Images</span>
        <div class="image-grid">
          <div v-for="(img, idx) in images" :key="idx" class="image-preview">
            <img :src="img" alt="Product image" />
          </div>
          
          <label class="image-upload-btn" :class="{ busy: uploading }">
            {{ uploading ? '...' : '+ Add Image' }}
            <input type="file" accept="image/*" @change="handleImageUpload" style="display: none" />
          </label>
        </div>
      </div>

      <p v-if="submitError" class="err">! {{ submitError }}</p>

      <div class="actions">
        <button type="submit" class="primary" :disabled="submitting">
          {{ submitting ? t('common.loading') : `> ${t('products.save')}` }}
        </button>
        <RouterLink to="/my-products" class="ghost">{{ t('common.cancel') }}</RouterLink>
      </div>
    </form>
  </div>
</template>

<style scoped>
.page {
  max-width: 560px;
  margin: 0 auto;
}
.back {
  display: inline-block;
  color: #6b7a90;
  text-decoration: none;
  font-size: 0.85rem;
  margin-bottom: 1rem;
}
.back:hover {
  color: #00d9a3;
}
.heading {
  font-size: 1.1rem;
  color: #e2e5e9;
  margin: 0 0 1.5rem;
}
.muted {
  color: #6b7a90;
}
.err {
  color: #ff5c5c;
  font-size: 0.85rem;
  margin: 0;
}
.form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.readonly-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.5rem 0.75rem;
  background: #0a0e14;
  border: 1px solid #1e2229;
  border-radius: 4px;
  font-size: 0.85rem;
}
.ro-label {
  color: #6b7a90;
  font-size: 0.7rem;
  text-transform: lowercase;
}
.ro-value {
  color: #e2e5e9;
  font-weight: 600;
}
.ro-hint {
  margin-left: auto;
  color: #6b7a90;
  font-size: 0.7rem;
  font-style: italic;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}
.label {
  color: #6b7a90;
  font-size: 0.7rem;
  text-transform: lowercase;
}
.label::before {
  content: '> ';
  color: #00d9a3;
  opacity: 0.6;
}
.hint {
  color: #6b7a90;
  font-size: 0.7rem;
}
.row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1rem;
}
input,
textarea,
select {
  background: #0a0e14;
  border: 1px solid #1e2229;
  color: #c8d3e0;
  padding: 0.5rem 0.75rem;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.9rem;
  resize: vertical;
}
input:focus,
textarea:focus,
select:focus {
  outline: none;
  border-color: #00d9a3;
  box-shadow: 0 0 0 2px rgba(0, 229, 160, 0.25);
}
.preview {
  color: #c8d3e0;
  font-size: 0.85rem;
  margin: 0;
  padding: 0.5rem 0.75rem;
  background: #0a0e14;
  border-left: 2px solid #00d9a3;
  border-radius: 0 4px 4px 0;
}
.dim {
  color: #6b7a90;
}
.actions {
  display: flex;
  gap: 0.75rem;
  align-items: center;
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
  padding: 0.5rem 1.25rem;
  cursor: pointer;
}
.primary:hover:not(:disabled) {
  background: #00f5aa;
}
.primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.ghost {
  color: #6b7a90;
  text-decoration: none;
  font-size: 0.85rem;
  padding: 0.5rem 1rem;
  border: 1px solid #1e2229;
  border-radius: 4px;
}
.ghost:hover {
  border-color: #00d9a3;
  color: #00d9a3;
}

/* Image Grid */
.image-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(80px, 1fr));
  gap: 0.5rem;
  margin-top: 0.5rem;
}
.image-preview {
  aspect-ratio: 1;
  border: 1px solid #1e2229;
  border-radius: 4px;
  overflow: hidden;
}
.image-preview img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.image-upload-btn {
  aspect-ratio: 1;
  border: 1px dashed #1e2229;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #6b7a90;
  font-size: 0.75rem;
  cursor: pointer;
  background: #0a0e14;
  transition: all 0.1s;
  text-align: center;
}
.image-upload-btn:hover:not(.busy) {
  border-color: #00d9a3;
  color: #00d9a3;
}
.image-upload-btn.busy {
  opacity: 0.5;
  cursor: wait;
}
</style>
