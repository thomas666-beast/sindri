<script setup lang="ts">
import { ref } from 'vue';
import { RouterLink, useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import { useAuthStore } from '../stores/auth';

const { t } = useI18n();
const router = useRouter();
const auth = useAuthStore();

const email = ref('');
const username = ref('');
const password = ref('');

async function submit() {
  try {
    await auth.register(email.value, username.value, password.value);
    router.push({ name: 'welcome' });
  } catch {
    // error already in auth.error
  }
}
</script>

<template>
  <div class="page">
    <h1 class="heading">{{ t('auth.register_heading') }}</h1>

    <form @submit.prevent="submit" class="form">
      <label class="field">
        <span class="label">{{ t('common.email') }}</span>
        <input
          v-model="email"
          type="email"
          required
          autocomplete="email"
        />
      </label>
      <label class="field">
        <span class="label">{{ t('common.username') }}</span>
        <input
          v-model="username"
          type="text"
          required
          pattern="[a-z0-9_]{3,32}"
          autocomplete="username"
        />
        <span class="hint">3–32 lowercase letters, digits, underscores</span>
      </label>
      <label class="field">
        <span class="label">{{ t('common.password') }}</span>
        <input
          v-model="password"
          type="password"
          required
          minlength="8"
          autocomplete="new-password"
        />
        <span class="hint">at least 8 characters</span>
      </label>

      <p v-if="auth.error" class="err">! {{ auth.error }}</p>

      <button type="submit" :disabled="auth.loading" class="submit">
        {{ auth.loading ? t('common.loading') : `> ${t('common.register')}` }}
      </button>
    </form>

    <p class="hint-line">
      {{ t('auth.have_account') }}
      <RouterLink to="/login">{{ t('common.login') }} →</RouterLink>
    </p>
  </div>
</template>

<style scoped>
.page {
  max-width: 380px;
  margin: 2rem auto;
}
.heading {
  color: #00d9a3;
  font-size: 1.25rem;
  margin: 0 0 1.5rem;
  font-weight: normal;
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
  font-size: 0.75rem;
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
input {
  background: #11151c;
  border: 1px solid #1f2633;
  color: #c8d3e0;
  padding: 0.625rem 0.75rem;
  border-radius: 0;
  font-family: inherit;
  font-size: 0.9rem;
}
input:focus {
  outline: none;
  border-color: #00d9a3;
  box-shadow: 0 0 0 1px #00d9a3;
}
.submit {
  background: #00d9a3;
  color: #0a0e14;
  border: none;
  padding: 0.625rem;
  border-radius: 0;
  font-family: inherit;
  font-weight: bold;
  font-size: 0.9rem;
  cursor: pointer;
  margin-top: 0.5rem;
}
.submit:hover:not(:disabled) {
  box-shadow: 0 0 12px rgba(0, 217, 163, 0.4);
}
.submit:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.err {
  color: #ff5c5c;
  font-size: 0.85rem;
  margin: 0;
}
.hint-line {
  color: #6b7a90;
  font-size: 0.85rem;
  margin-top: 1.5rem;
  text-align: center;
}
.hint-line a {
  color: #00d9a3;
  text-decoration: none;
}
.hint-line a:hover {
  text-decoration: underline;
}
</style>
