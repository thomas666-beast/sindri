<script setup lang="ts">
import { computed } from 'vue';
import MatrixRainCanvas from './MatrixRainCanvas.vue';
import MatrixRainDOM from './MatrixRainDOM.vue';

// Check the environment variable set in .env
const useCanvas = computed(() => {
  // Default to true if not set
  const envVal = import.meta.env.VITE_MATRIX_RAIN_WITH_CANVAS;
  return envVal !== 'false' && envVal !== '0';
});
</script>

<template>
  <MatrixRainCanvas v-if="useCanvas" />
  <MatrixRainDOM v-else />
</template>
