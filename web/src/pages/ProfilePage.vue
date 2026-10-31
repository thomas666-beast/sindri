<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import * as profileApi from '../api/profile';
import { api } from '../api';
import Avatar from '../components/Avatar.vue';

const { t } = useI18n();

const profile = ref<profileApi.Profile | null>(null);
const stats = ref<profileApi.GeekStats | null>(null);
const loading = ref(true);
const error = ref<string | null>(null);
const editing = ref(false);
const saving = ref(false);
const actionError = ref<string | null>(null);
const actionSuccess = ref<string | null>(null);
const uploading = ref(false);
const uploadError = ref<string | null>(null);

// form state
const displayName = ref('');
const bio = ref('');
const skillsRaw = ref('');
const interestsRaw = ref('');

async function uploadAvatar(e: Event) {
  uploadError.value = null;
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;

  if (file.size > 2 * 1024 * 1024) {
    uploadError.value = 'avatar must be under 2MB';
    return;
  }
  const allowed = ['image/png', 'image/jpeg', 'image/webp'];
  if (!allowed.includes(file.type)) {
    uploadError.value = 'must be PNG, JPEG, or WEBP';
    return;
  }

  uploading.value = true;
  try {
    const form = new FormData();
    form.append('avatar', file);
    const { data } = await api.post<{ data: { avatar_url: string } }>(
      '/me/avatar',
      form,
      { headers: { 'Content-Type': 'multipart/form-data' } }
    );
    if (profile.value) {
      // Force Vue to detect the change by creating a new object reference
      profile.value = {
        ...profile.value,
        avatar_url: data.data.avatar_url
      };
    }
    input.value = '';
  } catch (e: any) {
    uploadError.value = e.response?.data?.error ?? 'upload failed';
  } finally {
    uploading.value = false;
  }
}

function splitList(raw: string): string[] {
  return raw
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);
}

function enterEdit() {
  if (!profile.value) return;
  displayName.value = profile.value.display_name;
  bio.value = profile.value.bio;
  skillsRaw.value = profile.value.skills.join(', ');
  interestsRaw.value = profile.value.interests.join(', ');
  editing.value = true;
  actionError.value = null;
  actionSuccess.value = null;
}

function cancelEdit() {
  editing.value = false;
  actionError.value = null;
}

async function save() {
  actionError.value = null;
  actionSuccess.value = null;
  if (!displayName.value.trim()) {
    actionError.value = 'display name is required';
    return;
  }
  saving.value = true;
  try {
    profile.value = await profileApi.update({
      display_name: displayName.value.trim(),
      bio: bio.value,
      skills: splitList(skillsRaw.value),
      interests: splitList(interestsRaw.value),
      avatar_url: profile.value?.avatar_url ?? '', // Keep existing avatar
    });
    editing.value = false;
    actionSuccess.value = t('profile.saved_ok');
  } catch (e: any) {
    const body = e.response?.data;
    if (body?.errors) {
      actionError.value = Object.values(body.errors).join(', ');
    } else {
      actionError.value = body?.error ?? t('profile.save_failed');
    }
  } finally {
    saving.value = false;
  }
}

function fmtDate(unix: number): string {
  return new Date(unix * 1000).toISOString().slice(0, 10);
}

async function load() {
  loading.value = true;
  error.value = null;
  try {
    profile.value = await profileApi.getMine();
    stats.value = await profileApi.getStats(profile.value.username);
  } catch {
    error.value = t('profile.load_failed');
  } finally {
    loading.value = false;
  }
}

const hasBio = computed(() => !!profile.value?.bio);

onMounted(load);
</script>

<template>
  <div class="profile-page">
    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="err">! {{ error }}</p>

    <template v-else-if="profile">
      <!-- Hero Panel -->
      <header class="hero-panel">
        <div class="hero-left">
          <div class="avatar-wrap">
            <Avatar :username="profile.username" :src="profile.avatar_url" :size="96" />
            <label v-if="!editing" class="avatar-upload" :class="{ busy: uploading }" title="change avatar">
              {{ uploading ? '...' : '✎' }}
              <input
                type="file"
                accept="image/png,image/jpeg,image/webp"
                class="avatar-input"
                @change="uploadAvatar"
              />
            </label>
          </div>
          
          <div class="identity">
            <h1 class="display-name">{{ profile.display_name }}</h1>
            <div class="username">@{{ profile.username }}</div>
            
            <!-- Geek Stats -->
            <div v-if="stats" class="stats-row">
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
            
            <div class="meta">{{ t('profile.member_since') }} {{ fmtDate(profile.member_since_unix) }}</div>
          </div>
        </div>

        <div class="hero-actions">
          <button v-if="!editing" class="primary" @click="enterEdit">
            {{ t('profile.edit') }}
          </button>
        </div>
      </header>

      <p v-if="actionError" class="err">! {{ actionError }}</p>
      <p v-if="actionSuccess" class="ok">{{ actionSuccess }}</p>
      <p v-if="uploadError" class="err">! {{ uploadError }}</p>

      <!-- Display Mode -->
      <template v-if="!editing">
        <section class="section card">
          <h2 class="section-title">About</h2>
          <p v-if="hasBio" class="bio">{{ profile.bio }}</p>
          <p v-else class="muted">No bio provided.</p>

          <div v-if="profile.skills.length > 0" class="chips-group">
            <span class="label-caps">{{ t('profile.skills_label') }}</span>
            <div class="chips">
              <span v-for="s in profile.skills" :key="s" class="chip primary">{{ s }}</span>
            </div>
          </div>
          
          <div v-if="profile.interests.length > 0" class="chips-group">
            <span class="label-caps">{{ t('profile.interests_label') }}</span>
            <div class="chips">
              <span v-for="s in profile.interests" :key="s" class="chip secondary">{{ s }}</span>
            </div>
          </div>
        </section>
      </template>

      <!-- Edit Mode -->
      <form v-else class="section card" @submit.prevent="save">
        <label class="field">
          <span class="label">{{ t('profile.display_name') }}</span>
          <input v-model="displayName" type="text" required maxlength="100" />
        </label>

        <label class="field">
          <span class="label">{{ t('profile.bio') }}</span>
          <textarea
            v-model="bio"
            rows="5"
            maxlength="2000"
            :placeholder="t('profile.bio_placeholder')"
          />
        </label>

        <label class="field">
          <span class="label">{{ t('profile.skills') }}</span>
          <input v-model="skillsRaw" type="text" :placeholder="t('profile.skills_hint')" />
          <span class="hint">{{ t('profile.skills_hint') }}</span>
        </label>

        <label class="field">
          <span class="label">{{ t('profile.interests') }}</span>
          <input v-model="interestsRaw" type="text" :placeholder="t('profile.interests_hint')" />
          <span class="hint">{{ t('profile.interests_hint') }}</span>
        </label>

        <div class="actions">
          <button type="submit" class="primary" :disabled="saving">
            {{ saving ? t('profile.saving') : `> ${t('profile.save')}` }}
          </button>
          <button type="button" class="ghost" @click="cancelEdit">
            {{ t('profile.cancel') }}
          </button>
        </div>
      </form>
    </template>
  </div>
</template>

<style scoped>
.profile-page {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.muted { color: #6b7a90; font-size: 0.9rem; }
.err { color: #ff5c5c; font-size: 0.85rem; }
.ok { color: #00d9a3; font-size: 0.85rem; }

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

/* Form Fields */
.field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
  margin-bottom: 1rem;
}
.label {
  color: #6b7a90;
  font-size: 0.75rem;
  text-transform: lowercase;
}
.hint {
  color: #6b7a90;
  font-size: 0.7rem;
}
input,
textarea {
  background: #0a0e14;
  border: 1px solid #1e2229;
  color: #c8d3e0;
  padding: 0.6rem 0.75rem;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.9rem;
  resize: vertical;
}
input:focus,
textarea:focus {
  outline: none;
  border-color: #00d9a3;
  box-shadow: 0 0 0 2px rgba(0, 229, 160, 0.25);
}
.actions { display: flex; gap: 0.75rem; margin-top: 1rem; }

/* Avatar Upload */
.avatar-wrap { position: relative; display: inline-block; }
.avatar-upload {
  position: absolute;
  bottom: 0;
  right: 0;
  background: #00d9a3;
  color: #0a0e14;
  border-radius: 50%;
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 0.9rem;
  font-weight: 600;
  cursor: pointer;
  border: 2px solid #0e1319;
  transition: transform 0.1s;
}
.avatar-upload:hover:not(.busy) { transform: scale(1.1); }
.avatar-upload.busy { opacity: 0.6; cursor: wait; }
.avatar-input { display: none; }

@media (max-width: 700px) {
  .hero-panel { flex-direction: column; padding: 1.5rem; }
  .hero-left { flex-direction: column; align-items: center; text-align: center; }
  .stats-row { justify-content: center; }
  .hero-actions { width: 100%; justify-content: center; }
  .card { padding: 1.5rem; }
}
</style>
