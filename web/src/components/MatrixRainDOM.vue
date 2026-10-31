<script setup lang="ts">
import { onMounted, ref } from 'vue';

const columns = ref<{ chars: string[]; left: string; duration: string; delay: string }[]>([]);

onMounted(() => {
  const numColumns = Math.floor(window.innerWidth / 20); // roughly every 20px
  const newColumns = [];

  for (let i = 0; i < numColumns; i++) {
    // Generate a random string of 0s and 1s
    const length = 15 + Math.floor(Math.random() * 20);
    const chars = [];
    for (let j = 0; j < length; j++) {
      chars.push(Math.random() > 0.5 ? '1' : '0');
    }

    newColumns.push({
      chars,
      left: `${(i / numColumns) * 100}%`,
      duration: `${4 + Math.random() * 6}s`, // Falling speed between 4s and 10s
      delay: `${Math.random() * 5}s`,        // Random start delay
    });
  }

  columns.value = newColumns;
});
</script>

<template>
  <div class="matrix-container">
    <div
      v-for="(col, index) in columns"
      :key="index"
      class="matrix-column"
      :style="{ 
        left: col.left, 
        animationDuration: col.duration, 
        animationDelay: col.delay 
      }"
    >
      <span v-for="(char, i) in col.chars" :key="i" class="char">{{ char }}</span>
    </div>
  </div>
</template>

<style scoped>
.matrix-container {
  position: fixed;
  inset: 0;
  z-index: 0;
  background: #050608;
  overflow: hidden;
  pointer-events: none;
  opacity: 0.7; /* Blend into the background */
}

.matrix-column {
  position: absolute;
  top: -100%; /* Start above the screen */
  display: flex;
  flex-direction: column;
  align-items: center;
  font-family: 'JetBrains Mono', monospace;
  font-size: 16px;
  line-height: 1.2;
  animation-name: fall;
  animation-timing-function: linear;
  animation-iteration-count: infinite;
}

.char {
  color: #00b589;
  text-shadow: 0 0 4px rgba(0, 181, 137, 0.5);
  opacity: 0.8;
}

/* The first character in the column is the glowing white "head" */
.char:first-child {
  color: #c8ffdd;
  text-shadow: 0 0 10px rgba(200, 255, 221, 1), 0 0 20px rgba(0, 217, 163, 0.8);
  opacity: 1;
  font-weight: bold;
}

@keyframes fall {
  0% {
    transform: translateY(0);
  }
  100% {
    transform: translateY(200vh); /* Fall way past the bottom */
  }
}
</style>
