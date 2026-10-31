<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { RouterLink, useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as invoicesApi from '../api/invoices';
import * as walletApi from '../api/wallet';
import { digitsFor, fmtAmount, parseAmount, symbolFor } from '../lib/money';
import UserSearchInput from '../components/UserSearchInput.vue';

const { t } = useI18n();
const router = useRouter();

const buyerUsername = ref('');
const title = ref('');
const description = ref('');
const amountStr = ref('');
const currency = ref('USD');
const currencies = ref<walletApi.Currency[]>([]);

const submitting = ref(false);
const error = ref<string | null>(null);

async function loadCurrencies() {
  try {
    currencies.value = await walletApi.listCurrencies();
    if (currencies.value.length && !currencies.value.find((c) => c.code === 'USD')) {
      currency.value = currencies.value[0].code;
    }
  } catch {
    // non-fatal
  }
}

const preview = computed(() => {
  const cents = parseAmount(amountStr.value, digitsFor(currencies.value, currency.value));
  if (cents == null || cents <= 0) return '';
  return fmtAmount(currencies.value, cents, currency.value);
});

async function submit() {
  error.value = null;
  if (!buyerUsername.value.trim()) {
    error.value = 'buyer username is required';
    return;
  }
  if (!title.value.trim()) {
    error.value = 'title is required';
    return;
  }
  const amountCents = parseAmount(amountStr.value, digitsFor(currencies.value, currency.value));
  if (amountCents == null || amountCents <= 0) {
    error.value = 'amount must be a positive value';
    return;
  }
  submitting.value = true;
  try {
    await invoicesApi.create({
      buyer_username: buyerUsername.value.trim(),
      title: title.value.trim(),
      description: description.value,
      amount: amountCents,
      currency: currency.value,
    });
    router.push({ name: 'invoices' });
  } catch (e: any) {
    const body = e.response?.data;
    if (body?.errors) {
      error.value = Object.values(body.errors).join(', ');
    } else {
      error.value = body?.error ?? t('invoices.create_failed');
    }
  } finally {
    submitting.value = false;
  }
}

onMounted(loadCurrencies);
</script>

<template>
  <div class="page">
    <h1 class="heading">{{ t('invoices.new_heading') }}</h1>

    <form @submit.prevent="submit" class="form">
      <label class="field">
        <span class="label">{{ t('invoices.buyer_username') }}</span>
        <UserSearchInput
          v-model="buyerUsername"
          :placeholder="t('invoices.buyer_search_placeholder')"
        />
        <span class="hint">{{ t('invoices.buyer_username_hint') }}</span>
      </label>

      <label class="field">
        <span class="label">{{ t('invoices.title') }}</span>
        <input
          v-model="title"
          type="text"
          required
          maxlength="200"
          autocomplete="off"
        />
      </label>

      <label class="field">
        <span class="label">{{ t('invoices.description') }}</span>
        <textarea v-model="description" rows="4" />
      </label>

      <div class="row">
        <label class="field">
          <span class="label">{{ t('invoices.currency') }}</span>
          <select v-model="currency">
            <option v-for="c in currencies" :key="c.code" :value="c.code">
              {{ c.code }} — {{ c.name }}
            </option>
          </select>
        </label>
        <label class="field">
          <span class="label">{{ t('invoices.amount') }}</span>
          <input
            v-model="amountStr"
            type="text"
            inputmode="decimal"
            placeholder="0.00"
            required
          />
          <span class="hint">{{ t('invoices.amount_hint') }}</span>
        </label>
      </div>

      <p v-if="preview" class="preview">
        <span class="dim">=</span>
        {{ symbolFor(currencies, currency) }} {{ preview }}
      </p>

      <p v-if="error" class="err">! {{ error }}</p>

      <div class="actions">
        <button type="submit" class="primary" :disabled="submitting">
          {{ submitting ? t('common.loading') : `> ${t('invoices.create')}` }}
        </button>
        <RouterLink to="/invoices" class="ghost">
          {{ t('common.cancel') }}
        </RouterLink>
      </div>
    </form>
  </div>
</template>

<style scoped>
.page {
  max-width: 560px;
  margin: 0 auto;
}
.heading {
  font-size: 1.1rem;
  color: #e2e5e9;
  margin: 0 0 1.5rem;
}
.form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
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
.err {
  color: #ff5c5c;
  font-size: 0.85rem;
  margin: 0;
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
</style>
