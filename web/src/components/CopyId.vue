<script setup lang="ts">
import { ref } from 'vue';

const props = defineProps<{
  id: string;
  label?: string;
}>();

const copied = ref(false);

async function copy() {
  try {
    await navigator.clipboard.writeText(props.id);
    copied.value = true;
    setTimeout(() => { copied.value = false; }, 1500);
  } catch {
    // fallback for old browsers
    const el = document.createElement('textarea');
    el.value = props.id;
    document.body.appendChild(el);
    el.select();
    document.execCommand('copy');
    document.body.removeChild(el);
    copied.value = true;
    setTimeout(() => { copied.value = false; }, 1500);
  }
}
</script>

<template>
  <div class="copy-id" @click="copy" :title="`Copy: ${id}`">
    <span v-if="label" class="label">{{ label }}</span>
    <span class="value">{{ id.slice(0, 8) }}</span>
    <span class="icon">{{ copied ? '✓' : '⧉' }}</span>
  </div>
</template>

<style scoped>
.copy-id {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  background: #0a0e14;
  border: 1px solid #1e2229;
  border-radius: 4px;
  padding: 0.15rem 0.4rem;
  cursor: pointer;
  transition: border-color 0.1s;
  font-family: 'JetBrains Mono', monospace;
}
.copy-id:hover {
  border-color: #4a9eff;
}
.label {
  color: #6b7a90;
  font-size: 0.65rem;
  text-transform: uppercase;
}
.value {
  color: #4a9eff;
  font-size: 0.75rem;
}
.icon {
  color: #6b7a90;
  font-size: 0.75rem;
}
.copy-id:hover .icon {
  color: #4a9eff;
}
</style>
