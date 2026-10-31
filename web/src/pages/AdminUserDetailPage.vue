<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';
import { RouterLink, useRoute } from 'vue-router';
import { useI18n } from 'vue-i18n';
import * as adminApi from '../api/admin';

const { t } = useI18n();
const route = useRoute();

const user = ref<adminApi.AdminUser | null>(null);
const warnings = ref<adminApi.Warning[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);
const actionError = ref<string | null>(null);
const actionSuccess = ref<string | null>(null);
const busy = ref(false);

const disableReason = ref('');
const showDisable = ref(false);

const kycStatus = ref<'not_required' | 'pending' | 'verified' | 'rejected'>('not_required');

const warningReason = ref('');
const warningSeverity = ref<'info' | 'warning' | 'severe'>('warning');

async function load() {
  loading.value = true;
  error.value = null;
  try {
    user.value = await adminApi.getUser(route.params.id as string);
    kycStatus.value = user.value.kyc_status;
    warnings.value = await adminApi.listWarnings(user.value.id);
  } catch (e: any) {
    const body = e.response?.data;
    error.value = body?.error ?? t('admin.load_failed');
  } finally {
    loading.value = false;
  }
}

async function toggleActive() {
  if (!user.value) return;
  actionError.value = null;
  actionSuccess.value = null;
  busy.value = true;
  try {
    if (user.value.is_active) {
      if (!confirm(t('admin.disable_confirm'))) return;
      if (!disableReason.value.trim()) {
        actionError.value = 'reason required';
        return;
      }
      user.value = await adminApi.disableUser(user.value.id, disableReason.value);
      showDisable.value = false;
      disableReason.value = '';
    } else {
      user.value = await adminApi.enableUser(user.value.id);
    }
    actionSuccess.value = t('admin.action_ok');
  } catch (e: any) {
    actionError.value = e.response?.data?.error ?? t('admin.action_failed');
  } finally {
    busy.value = false;
  }
}

async function saveKYC() {
  if (!user.value) return;
  actionError.value = null;
  actionSuccess.value = null;
  busy.value = true;
  try {
    user.value = await adminApi.setKYC(user.value.id, kycStatus.value);
    actionSuccess.value = t('admin.action_ok');
  } catch (e: any) {
    actionError.value = e.response?.data?.error ?? t('admin.action_failed');
  } finally {
    busy.value = false;
  }
}

async function issueWarning() {
  if (!user.value) return;
  actionError.value = null;
  actionSuccess.value = null;
  if (!warningReason.value.trim()) {
    actionError.value = 'reason required';
    return;
  }
  busy.value = true;
  try {
    const w = await adminApi.createWarning(
      user.value.id,
      warningReason.value,
      warningSeverity.value
    );
    warnings.value = [w, ...warnings.value];
    warningReason.value = '';
    actionSuccess.value = t('admin.warning_created');
  } catch (e: any) {
    actionError.value = e.response?.data?.error ?? t('admin.action_failed');
  } finally {
    busy.value = false;
  }
}

function fmtDate(unix: number): string {
  return new Date(unix * 1000).toISOString().replace('T', ' ').slice(0, 19);
}

watch(() => route.params.id, () => load());
onMounted(load);
</script>

<template>
  <div>
    <RouterLink to="/admin" class="back">{{ t('admin.back') }}</RouterLink>

    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="err">! {{ error }}</p>

    <template v-else-if="user">
      <article class="panel">
        <header class="panel-head">
          <h1 class="heading">{{ t('admin.user_heading') }}</h1>
          <div class="badges">
            <span v-if="user.is_admin" class="badge admin">admin</span>
            <span v-if="!user.is_active" class="badge disabled">disabled</span>
            <span class="badge" :class="user.kyc_status">kyc:{{ user.kyc_status }}</span>
          </div>
        </header>

        <dl class="meta">
          <div class="meta-row"><dt>id</dt><dd>{{ user.id }}</dd></div>
          <div class="meta-row"><dt>{{ t('admin.email') }}</dt><dd>{{ user.email }}</dd></div>
          <div class="meta-row"><dt>{{ t('admin.username') }}</dt><dd>{{ user.username }}</dd></div>
          <div class="meta-row"><dt>{{ t('admin.currency') }}</dt><dd>{{ user.preferred_currency }}</dd></div>
          <div class="meta-row"><dt>{{ t('admin.created') }}</dt><dd>{{ fmtDate(user.created_at_unix) }}</dd></div>
          <div v-if="user.disabled_at_unix" class="meta-row">
            <dt>{{ t('admin.disabled') }}</dt>
            <dd>{{ fmtDate(user.disabled_at_unix) }} · {{ user.disable_reason }}</dd>
          </div>
        </dl>

        <div class="actions">
          <button class="btn-danger" :disabled="busy" @click="toggleActive">
            {{ user.is_active ? t('admin.disable') : t('admin.enable') }}
          </button>
          <button v-if="user.is_active" class="btn-ghost" :disabled="busy" @click="showDisable = !showDisable">
            reason…
          </button>
        </div>
        <div v-if="showDisable" class="disable-form">
          <input v-model="disableReason" type="text" :placeholder="t('admin.disable_reason')" />
        </div>

        <p v-if="actionError" class="err">! {{ actionError }}</p>
        <p v-if="actionSuccess" class="ok">{{ actionSuccess }}</p>
      </article>

      <article class="panel">
        <h2 class="heading2">{{ t('admin.set_kyc') }}</h2>
        <div class="inline-form">
          <select v-model="kycStatus">
            <option value="not_required">not_required</option>
            <option value="pending">pending</option>
            <option value="verified">verified</option>
            <option value="rejected">rejected</option>
          </select>
          <button class="btn-primary" :disabled="busy" @click="saveKYC">save</button>
        </div>
      </article>

      <article class="panel">
        <h2 class="heading2">{{ t('admin.warnings') }}</h2>

        <div class="warning-form">
          <label class="field">
            <span class="label">{{ t('admin.warning_reason') }}</span>
            <input v-model="warningReason" type="text" maxlength="2000" />
          </label>
          <label class="field">
            <span class="label">{{ t('admin.warning_severity') }}</span>
            <select v-model="warningSeverity">
              <option value="info">{{ t('admin.warn_severity_info') }}</option>
              <option value="warning">{{ t('admin.warn_severity_warning') }}</option>
              <option value="severe">{{ t('admin.warn_severity_severe') }}</option>
            </select>
          </label>
          <button class="btn-primary" :disabled="busy" @click="issueWarning">
            {{ t('admin.warning_create') }}
          </button>
        </div>

        <div v-if="warnings.length === 0" class="empty">{{ t('admin.no_warnings') }}</div>
        <ul v-else class="warn-list">
          <li v-for="w in warnings" :key="w.id" class="warn-item">
            <span class="badge" :class="'sev-' + w.severity">{{ w.severity }}</span>
            <span class="warn-reason">{{ w.reason }}</span>
            <span class="warn-date">{{ fmtDate(w.created_at_unix) }}</span>
            <span v-if="w.acknowledged_at_unix" class="ack">✓ seen</span>
          </li>
        </ul>
      </article>
    </template>
  </div>
</template>

<style scoped>
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
.muted {
  color: #6b7a90;
}
.err {
  color: #ff5c5c;
  font-size: 0.85rem;
}
.ok {
  color: #00d9a3;
  font-size: 0.85rem;
}
.heading {
  color: #00d9a3;
  font-size: 1.1rem;
  font-weight: normal;
  margin: 0;
}
.heading2 {
  color: #c8d3e0;
  font-size: 0.95rem;
  font-weight: normal;
  margin: 0 0 0.75rem;
}
.panel {
  border: 1px solid #1f2633;
  background: #11151c;
  padding: 1.25rem;
  margin-bottom: 1rem;
}
.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 1rem;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.badges {
  display: flex;
  gap: 0.375rem;
}
.badge {
  font-size: 0.65rem;
  padding: 0.05rem 0.4rem;
  border: 1px solid #1f2633;
  color: #6b7a90;
  text-transform: uppercase;
}
.badge.admin {
  color: #00d9a3;
  border-color: #00d9a3;
}
.badge.disabled {
  color: #ff5c5c;
  border-color: #ff5c5c;
}
.badge.verified {
  color: #00d9a3;
  border-color: #00d9a3;
}
.badge.pending {
  color: #ffb454;
  border-color: #ffb454;
}
.badge.rejected {
  color: #ff5c5c;
  border-color: #ff5c5c;
}
.badge.sev-info {
  color: #4ea3ff;
  border-color: #4ea3ff;
}
.badge.sev-warning {
  color: #ffb454;
  border-color: #ffb454;
}
.badge.sev-severe {
  color: #ff5c5c;
  border-color: #ff5c5c;
}
.meta {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
  margin: 0 0 1rem;
}
.meta-row {
  display: flex;
  justify-content: space-between;
  font-size: 0.85rem;
  gap: 1rem;
}
.meta-row dt {
  color: #6b7a90;
}
.meta-row dd {
  color: #c8d3e0;
  margin: 0;
  text-align: right;
  word-break: break-all;
}
.actions {
  display: flex;
  gap: 0.5rem;
}
.disable-form {
  margin-top: 0.5rem;
}
.disable-form input {
  width: 100%;
  background: #0a0e14;
  border: 1px solid #1f2633;
  color: #c8d3e0;
  padding: 0.5rem 0.75rem;
  font-family: inherit;
  font-size: 0.85rem;
}
.btn-primary {
  background: #00d9a3;
  color: #0a0e14;
  border: none;
  font-family: inherit;
  font-size: 0.85rem;
  font-weight: bold;
  padding: 0.4rem 0.9rem;
  cursor: pointer;
}
.btn-primary:hover:not(:disabled) {
  box-shadow: 0 0 12px rgba(0, 217, 163, 0.4);
}
.btn-primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.btn-danger {
  background: transparent;
  border: 1px solid #ff5c5c;
  color: #ff5c5c;
  font-family: inherit;
  font-size: 0.85rem;
  padding: 0.4rem 0.9rem;
  cursor: pointer;
}
.btn-danger:hover:not(:disabled) {
  background: rgba(255, 92, 92, 0.1);
}
.btn-ghost {
  background: transparent;
  border: 1px solid #1f2633;
  color: #6b7a90;
  font-family: inherit;
  font-size: 0.85rem;
  padding: 0.4rem 0.9rem;
  cursor: pointer;
}
.btn-ghost:hover {
  border-color: #00d9a3;
  color: #00d9a3;
}
.inline-form {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}
.inline-form select {
  background: #0a0e14;
  border: 1px solid #1f2633;
  color: #c8d3e0;
  padding: 0.4rem 0.75rem;
  font-family: inherit;
  font-size: 0.85rem;
}
.warning-form {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-bottom: 1rem;
}
.warning-form .field {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
.warning-form .label {
  color: #6b7a90;
  font-size: 0.7rem;
}
.warning-form input,
.warning-form select {
  background: #0a0e14;
  border: 1px solid #1f2633;
  color: #c8d3e0;
  padding: 0.4rem 0.75rem;
  font-family: inherit;
  font-size: 0.85rem;
}
.empty {
  color: #6b7a90;
  font-size: 0.85rem;
  padding: 1rem 0;
}
.warn-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}
.warn-item {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  background: #0a0e14;
  border: 1px solid #1f2633;
  font-size: 0.85rem;
}
.warn-reason {
  flex: 1;
  color: #c8d3e0;
}
.warn-date {
  color: #6b7a90;
  font-size: 0.7rem;
}
.ack {
  color: #00d9a3;
  font-size: 0.7rem;
}
</style>
