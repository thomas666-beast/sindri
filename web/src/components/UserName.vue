<script setup lang="ts">
import { computed } from 'vue';
import { RouterLink } from 'vue-router';
import { useAuthStore } from '../stores/auth';

const props = defineProps<{
  username?: string;
  userId?: string;
  prefix?: 'at' | 'none';
}>();

const auth = useAuthStore();

const display = computed(() => {
  if (props.username) return props.username;
  if (props.userId) return props.userId.slice(0, 8);
  return 'unknown';
});

const isMe = computed(() => auth.user?.id === props.userId);

const to = computed(() => {
  if (props.username) return `/u/${props.username}`;
  return '/wallet';
});
</script>

<template>
  <RouterLink
    v-if="username"
    :to="to"
    class="uname"
    :class="{ me: isMe }"
  >
    <span v-if="prefix === 'at'" class="at">@</span>{{ display }}
    <span v-if="isMe" class="you">(you)</span>
  </RouterLink>
  <span v-else class="uname plain" :title="userId">{{ display }}</span>
</template>

<style scoped>
.uname {
  color: #00d9a3;
  text-decoration: none;
}
.uname:hover {
  text-decoration: underline;
}
.uname.me {
  color: #ffb454;
}
.uname.plain {
  color: #6b7a90;
}
.at {
  opacity: 0.6;
  margin-right: 0.05em;
}
.you {
  color: #6b7a90;
  font-size: 0.85em;
  margin-left: 0.25em;
}
</style>
