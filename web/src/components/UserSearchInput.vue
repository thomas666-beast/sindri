<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { api } from '../api';
import Avatar from './Avatar.vue';

interface UserHit {
  id: string;
  username: string;
  display_name: string;
  avatar_url?: string;
}

const props = defineProps<{
  modelValue: string;
  placeholder?: string;
}>();

const emit = defineEmits<{
  (e: 'update:modelValue', v: string): void;
}>();

const query = ref(props.modelValue);
const results = ref<UserHit[]>([]);
const open = ref(false);
const loading = ref(false);
const error = ref<string | null>(null);
let debounceTimer: number | null = null;

watch(
  () => props.modelValue,
  (v) => {
    if (v !== query.value) query.value = v;
  }
);

async function search(q: string) {
  if (q.trim().length < 2) {
    results.value = [];
    error.value = null;
    return;
  }
  loading.value = true;
  error.value = null;
  try {
    const { data } = await api.get<{
      data: { results: UserHit[] };
    }>('/users/search', { params: { q: q.trim(), limit: 5 } });
    results.value = data.data.results;
  } catch {
    error.value = 'search failed';
    results.value = [];
  } finally {
    loading.value = false;
  }
}

function onInput() {
  emit('update:modelValue', query.value);
  open.value = true;
  if (debounceTimer) window.clearTimeout(debounceTimer);
  debounceTimer = window.setTimeout(() => search(query.value), 200);
}

function select(user: UserHit) {
  query.value = user.username;
  emit('update:modelValue', user.username);
  open.value = false;
  results.value = [];
}

function onBlur() {
  // Delay close so click on item registers
  window.setTimeout(() => {
    open.value = false;
  }, 150);
}

function handleClickOutside(e: MouseEvent) {
  if (!open.value) return;
  const target = e.target as HTMLElement;
  if (!target.closest('.user-search')) open.value = false;
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside);
});

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside);
  if (debounceTimer) window.clearTimeout(debounceTimer);
});
</script>

<template>
  <div class="user-search">
    <input
      v-model="query"
      type="text"
      :placeholder="placeholder ?? 'search username…'"
      autocomplete="off"
      @input="onInput"
      @focus="open = true"
      @blur="onBlur"
    />
    <div v-if="open && query.length >= 2" class="dropdown">
      <p v-if="loading" class="muted">{{ 'searching…' }}</p>
      <p v-else-if="error" class="err">! {{ error }}</p>
      <p v-else-if="results.length === 0" class="muted">no users found</p>
      <ul v-else class="list">
        <li
          v-for="u in results"
          :key="u.id"
          class="item"
          @mousedown.prevent="select(u)"
        >
          <Avatar :username="u.username" :src="u.avatar_url" :size="24" />
          <div class="user-meta">
            <span class="uname">@{{ u.username }}</span>
            <span v-if="u.display_name && u.display_name !== u.username" class="display">
              {{ u.display_name }}
            </span>
          </div>
        </li>
      </ul>
    </div>
  </div>
</template>

<style scoped>
.user-search {
  position: relative;
}
input {
  width: 100%;
  background: #0a0e14;
  border: 1px solid #1e2229;
  color: #c8d3e0;
  padding: 0.5rem 0.75rem;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.9rem;
}
input:focus {
  outline: none;
  border-color: #00d9a3;
  box-shadow: 0 0 0 2px rgba(0, 229, 160, 0.25);
}
.dropdown {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  background: #161a20;
  border: 1px solid #1e2229;
  border-radius: 4px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
  z-index: 50;
  max-height: 260px;
  overflow-y: auto;
}
.muted {
  color: #6b7a90;
  font-size: 0.8rem;
  padding: 0.5rem 0.75rem;
  margin: 0;
}
.err {
  color: #ff5c5c;
  font-size: 0.8rem;
  padding: 0.5rem 0.75rem;
  margin: 0;
}
.list {
  list-style: none;
  padding: 0;
  margin: 0;
}
.item {
  display: flex;
  gap: 0.5rem;
  align-items: center;
  padding: 0.5rem 0.75rem;
  cursor: pointer;
}
.item:hover {
  background: #0e1319;
}
.user-meta {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  min-width: 0;
}
.uname {
  color: #00d9a3;
  font-size: 0.85rem;
}
.display {
  color: #6b7a90;
  font-size: 0.75rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
