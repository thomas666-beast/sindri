<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import * as adminApi from '../api/admin';

const { t } = useI18n();

const loading = ref(true);
const error = ref<string | null>(null);
const success = ref<string | null>(null);
const saving = ref<string | null>(null);

const commission = ref('');
const autoRelease = ref('');

async function load() {
  loading.value = true;
  error.value = null;
  try {
    const settings = await adminApi.getSettings();
    commission.value = settings['commission_percent'] || '5';
    autoRelease.value = settings['escrow_auto_release_days'] || '3';
  } catch {
    error.value = t('admin.settings_load_failed');
  } finally {
    loading.value = false;
  }
}

async function save(key: string, val: string) {
  saving.value = key;
  error.value = null;
  success.value = null;
  try {
    // Force the value to be a string before sending
    await adminApi.updateSetting(key, String(val));
    success.value = t('admin.settings_save_ok');
    setTimeout(() => { success.value = null; }, 3000);
  } catch {
    error.value = t('admin.settings_save_failed');
  } finally {
    saving.value = null;
  }
}

onMounted(load);
</script>

<template>
  <div>
    <h1 class="heading">{{ t('admin.settings_heading') }}</h1>

    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="err">! {{ error }}</p>

    <template v-else>
      <p v-if="success" class="ok">{{ success }}</p>

      <div class="settings-list">
        <!-- Commission Percentage -->
        <div class="setting-row">
          <div class="setting-info">
            <span class="setting-name">commission percent</span>
            <span class="setting-desc">The % fee taken from every sale</span>
          </div>
          <div class="setting-action">
            <div class="input-group">
              <input v-model="commission" type="number" min="0" max="100" />
              <span class="suffix">%</span>
            </div>
            <button 
              class="primary" 
              :disabled="saving === 'commission_percent'"
              @click="save('commission_percent', commission)"
            >
              {{ saving === 'commission_percent' ? '...' : 'save' }}
            </button>
          </div>
        </div>

        <!-- Escrow Auto Release Days -->
        <div class="setting-row">
          <div class="setting-info">
            <span class="setting-name">escrow auto-release days</span>
            <span class="setting-desc">Days to wait before auto-releasing funds</span>
          </div>
          <div class="setting-action">
            <div class="input-group">
              <input v-model="autoRelease" type="number" min="1" max="30" />
              <span class="suffix">days</span>
            </div>
            <button 
              class="primary" 
              :disabled="saving === 'escrow_auto_release_days'"
              @click="save('escrow_auto_release_days', autoRelease)"
            >
              {{ saving === 'escrow_auto_release_days' ? '...' : 'save' }}
            </button>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.heading { color: #00d9a3; font-size: 1.25rem; font-weight: normal; margin: 0 0 1.5rem; }
.muted { color: #6b7a90; }
.err { color: #ff5c5c; font-size: 0.85rem; margin-bottom: 1rem; }
.ok { color: #00d9a3; font-size: 0.85rem; margin-bottom: 1rem; }

.settings-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.setting-row {
  border: 1px solid #1e2229;
  background: rgba(15, 17, 21, 0.85);
  border-radius: 4px;
  padding: 1rem;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
  flex-wrap: wrap;
}
.setting-info {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}
.setting-name {
  color: #e2e5e9;
  font-weight: 600;
  font-size: 0.9rem;
}
.setting-desc {
  color: #6b7a90;
  font-size: 0.75rem;
}
.setting-action {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.input-group {
  display: flex;
  align-items: center;
  background: #11151c;
  border: 1px solid #1e2229;
  border-radius: 4px;
  overflow: hidden;
}
.input-group input {
  background: transparent;
  border: none;
  color: #e2e5e9;
  padding: 0.4rem 0.6rem;
  width: 80px;
  font-family: inherit;
  font-size: 0.85rem;
  text-align: right;
}
.input-group input:focus { outline: none; }
.input-group .suffix {
  background: #1e2229;
  color: #6b7a90;
  padding: 0.4rem 0.6rem;
  font-size: 0.75rem;
  border-left: 1px solid #1e2229;
}
.primary {
  background: #00d9a3;
  color: #0a0e14;
  border: none;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.8rem;
  font-weight: bold;
  padding: 0.5rem 1rem;
  cursor: pointer;
}
.primary:hover:not(:disabled) { background: #00f5aa; }
.primary:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
