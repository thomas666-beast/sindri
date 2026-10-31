<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue';

const canvasRef = ref<HTMLCanvasElement | null>(null);
let animationFrameId: number;

onMounted(() => {
  const canvas = canvasRef.value;
  if (!canvas) return;

  const ctx = canvas.getContext('2d');
  if (!ctx) return;

  // Set canvas to fill the window
  const resizeCanvas = () => {
    canvas.width = window.innerWidth;
    canvas.height = window.innerHeight;
  };
  resizeCanvas();
  window.addEventListener('resize', resizeCanvas);

  // Matrix configuration
  const fontSize = 16;
  const columns = Math.floor(canvas.width / fontSize);
  const drops: number[] = [];

  // Initialize drops at random Y positions
  for (let x = 0; x < columns; x++) {
    drops[x] = Math.random() * canvas.height;
  }

  const chars = '01';

  const draw = () => {
    // Fade previous frame to create the trail
    ctx.fillStyle = 'rgba(5, 6, 8, 0.05)';
    ctx.fillRect(0, 0, canvas.width, canvas.height);

    ctx.font = `${fontSize}px monospace`;

    for (let i = 0; i < drops.length; i++) {
      const text = chars[Math.floor(Math.random() * chars.length)];
      const x = i * fontSize;
      const y = drops[i] * fontSize;

      // The "head" of the drop is white, the rest is green
      if (Math.random() > 0.98) {
        ctx.fillStyle = '#ffffff';
        ctx.shadowBlur = 10;
        ctx.shadowColor = '#00d9a3';
      } else {
        ctx.fillStyle = '#00b589';
        ctx.shadowBlur = 0;
      }

      ctx.fillText(text, x, y);

      // Reset drop to top after it crosses the bottom
      if (y > canvas.height && Math.random() > 0.975) {
        drops[i] = 0;
      }

      drops[i]++;
    }

    animationFrameId = requestAnimationFrame(draw);
  };

  draw();

  onBeforeUnmount(() => {
    window.removeEventListener('resize', resizeCanvas);
    cancelAnimationFrame(animationFrameId);
  });
});
</script>

<template>
  <canvas ref="canvasRef" class="matrix-canvas"></canvas>
</template>

<style scoped>
.matrix-canvas {
  position: fixed;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  background: #050608;
  opacity: 0.85;
}
</style>
