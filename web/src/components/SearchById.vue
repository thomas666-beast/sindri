<script setup lang="ts">
import { ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
const props = defineProps<{
  modelValue: string;
  placeholder?: string;
}>();
const emit = defineEmits<{
  (e: 'update:modelValue', v: string): void;
}>();

const val = ref(props.modelValue);
let timer: number | null = null;

watch(() => props.modelValue, (v) => { if (v !== val.value) val.value = v; });

function onInput() {
  if (timer) window.clearTimeout(timer);
  timer = window.setTimeout(() => {
    emit('update:modelValue', val.value.trim());
  }, 300);
}
</script>

<template>
  <div class="search-id">
    <span class="icon">#</span>
    <input
      v-model="val"
      type="text"
      :placeholder="placeholder || t('common.search_id')"
      @input="onInput"
    />
  </div>
</template>

<style scoped>
.search-id {
  display: flex;
  align-items: center;
  background: #0a0e14;
  border: 1px solid #1e2229;
  border-radius: 4px;
  padding: 0 0.5rem;
  width: 180px;
}
.search-id:focus-within {
  border-color: #00d9a3;
  box-shadow: 0 0 0 1px #00d9a3;
}
.icon {
  color: #6b7a90;
  font-weight: bold;
  font-size: 0.85rem;
}
input {
  background: transparent;
  border: none;
  color: #c8d3e0;
  font-family: inherit;
  font-size: 0.8rem;
  padding: 0.35rem 0.5rem;
  outline: none;
  width: 100%;
}
</style>
