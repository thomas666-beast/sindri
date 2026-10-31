<script setup lang="ts">
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';
import * as reviewsApi from '../api/reviews';

const { t } = useI18n();

const props = defineProps<{ orderId: string }>();
const emit = defineEmits<{
  (e: 'submitted', review: reviewsApi.Review): void;
  (e: 'cancel'): void;
}>();

const rating = ref(5);
const comment = ref('');
const submitting = ref(false);
const error = ref<string | null>(null);

async function submit() {
  error.value = null;
  if (rating.value < 1 || rating.value > 5) {
    error.value = 'rating must be 1-5';
    return;
  }
  submitting.value = true;
  try {
    const review = await reviewsApi.create({
      order_id: props.orderId,
      rating: rating.value,
      comment: comment.value.trim(),
    });
    emit('submitted', review);
  } catch (e: any) {
    const body = e.response?.data;
    if (body?.errors) {
      error.value = Object.values(body.errors).join(', ');
    } else {
      error.value = body?.error ?? 'failed to submit review';
    }
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <div class="form">
    <h3 class="heading">{{ t('reviews.leave_review') }}</h3>

    <div class="rating">
      <span class="label">{{ t('reviews.rating') }}</span>
      <button
        v-for="n in 5"
        :key="n"
        type="button"
        class="star"
        :class="{ active: n <= rating }"
        @click="rating = n"
      >
        ★
      </button>
    </div>

    <label class="field">
      <span class="label">{{ t('reviews.comment') }}</span>
      <textarea
        v-model="comment"
        rows="4"
        maxlength="2000"
        :placeholder="t('reviews.comment_placeholder')"
      />
    </label>

    <p v-if="error" class="err">! {{ error }}</p>

    <div class="actions">
      <button type="button" class="primary" :disabled="submitting" @click="submit">
        {{ submitting ? t('common.loading') : `> ${t('reviews.submit')}` }}
      </button>
      <button type="button" class="ghost" @click="emit('cancel')">
        {{ t('common.cancel') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.form {
  border: 1px solid #1f2633;
  background: #0e1319;
  padding: 1rem;
  margin-top: 1rem;
}
.heading {
  color: #00d9a3;
  font-size: 0.95rem;
  font-weight: normal;
  margin: 0 0 1rem;
}
.rating {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  margin-bottom: 1rem;
}
.label {
  color: #6b7a90;
  font-size: 0.75rem;
  margin-right: 0.5rem;
}
.star {
  background: transparent;
  border: none;
  color: #1f2633;
  font-size: 1.5rem;
  cursor: pointer;
  padding: 0;
  line-height: 1;
}
.star.active {
  color: #00d9a3;
}
.star:hover {
  color: #7dffd0;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
  margin-bottom: 1rem;
}
.field .label {
  color: #6b7a90;
  font-size: 0.7rem;
}
textarea {
  background: #0a0e14;
  border: 1px solid #1f2633;
  color: #c8d3e0;
  padding: 0.5rem 0.75rem;
  font-family: inherit;
  font-size: 0.9rem;
  resize: vertical;
}
textarea:focus {
  outline: none;
  border-color: #00d9a3;
}
.err {
  color: #ff5c5c;
  font-size: 0.85rem;
  margin: 0 0 1rem;
}
.actions {
  display: flex;
  gap: 0.5rem;
}
.primary {
  background: #00d9a3;
  color: #0a0e14;
  border: none;
  font-family: inherit;
  font-size: 0.85rem;
  font-weight: bold;
  padding: 0.5rem 1rem;
  cursor: pointer;
}
.primary:hover:not(:disabled) {
  box-shadow: 0 0 12px rgba(0, 217, 163, 0.4);
}
.primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.ghost {
  background: transparent;
  border: 1px solid #1f2633;
  color: #6b7a90;
  font-family: inherit;
  font-size: 0.85rem;
  padding: 0.5rem 1rem;
  cursor: pointer;
}
.ghost:hover {
  border-color: #00d9a3;
  color: #00d9a3;
}
</style>
