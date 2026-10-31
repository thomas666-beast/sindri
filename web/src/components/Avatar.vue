<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps<{
  username: string;
  src?: string;
  size?: number;
}>();

const initials = computed(() => props.username.slice(0, 2).toUpperCase());

// Deterministic hue from username — same user always same color
const hue = computed(() => {
  let h = 0;
  for (let i = 0; i < props.username.length; i++) {
    h = (h * 31 + props.username.charCodeAt(i)) % 360;
  }
  return h;
});

const size = computed(() => props.size ?? 40);
const fontSize = computed(() => Math.floor(size.value * 0.4));
</script>

<template>
  <div
    class="avatar"
    :style="{
      width: size + 'px',
      height: size + 'px',
      fontSize: fontSize + 'px',
      background: src ? 'transparent' : `hsl(${hue}, 60%, 35%)`,
    }"
  >
    <img v-if="src" :src="src" :alt="username" />
    <span v-else>{{ initials }}</span>
  </div>
</template>

<style scoped>
.avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: #e2e5e9;
  font-weight: 600;
  overflow: hidden;
  flex-shrink: 0;
}
img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
</style>
