<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import type { Review, Summary } from '../api/reviews';

const { t } = useI18n();

const props = defineProps<{
  reviews: Review[];
  summary: Summary | null;
  loading: boolean;
}>();

const stars = computed(() => {
  if (!props.summary || props.summary.review_count === 0) return '';
  const avg = props.summary.avg_rating;
  const rounded = Math.round(avg * 10) / 10;
  return `${rounded} / 5`;
});

function fmtDate(unix: number): string {
  return new Date(unix * 1000).toISOString().slice(0, 10);
}
</script>

<template>
  <div class="reviews">
    <div v-if="summary && summary.review_count > 0" class="summary">
      <span class="avg">{{ stars }}</span>
      <span class="count">
        {{ summary.review_count }} {{ t('reviews.review_count') }}
      </span>
    </div>
    <p v-else-if="!loading" class="muted">{{ t('reviews.no_reviews') }}</p>

    <p v-if="loading" class="muted">{{ t('common.loading') }}</p>

    <div v-else class="list">
      <article v-for="r in reviews" :key="r.id" class="review">
        <header class="review-head">
          <span class="buyer">{{ r.buyer_username ? `@${r.buyer_username}` : 'buyer' }}</span>
          <span class="rating">{{ '★'.repeat(r.rating) }}{{ '☆'.repeat(5 - r.rating) }}</span>
          <span class="date">{{ fmtDate(r.created_at_unix) }}</span>
        </header>
        <p v-if="r.comment" class="comment">{{ r.comment }}</p>
      </article>
    </div>
  </div>
</template>

<style scoped>
.reviews {
  border-top: 1px solid #1f2633;
  padding-top: 1rem;
  margin-top: 1rem;
}
.summary {
  display: flex;
  align-items: baseline;
  gap: 0.75rem;
  margin-bottom: 1rem;
}
.avg {
  color: #00d9a3;
  font-size: 1.1rem;
  font-weight: bold;
}
.count {
  color: #6b7a90;
  font-size: 0.8rem;
}
.muted {
  color: #6b7a90;
  font-size: 0.85rem;
}
.list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.review {
  background: #0a0e14;
  border: 1px solid #1f2633;
  padding: 0.75rem 1rem;
}
.review-head {
  display: flex;
  align-items: baseline;
  gap: 0.75rem;
  margin-bottom: 0.375rem;
}
.buyer {
  color: #4ea3ff;
  font-size: 0.85rem;
}
.rating {
  color: #00d9a3;
  font-size: 0.85rem;
  letter-spacing: 0.05em;
}
.date {
  margin-left: auto;
  color: #6b7a90;
  font-size: 0.7rem;
}
.comment {
  color: #c8d3e0;
  font-size: 0.9rem;
  margin: 0;
  white-space: pre-wrap;
}
</style>
