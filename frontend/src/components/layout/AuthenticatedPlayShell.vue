<script setup lang="ts">
import { computed } from 'vue'
import { useAuthStore } from '@/stores/auth'
import AppLayout from './AppLayout.vue'

const authStore = useAuthStore()
const isAuthenticated = computed(() => authStore.isAuthenticated)
</script>

<template>
  <AppLayout v-if="isAuthenticated">
    <div class="authenticated-play-shell">
      <slot />
    </div>
  </AppLayout>
  <slot v-else />
</template>

<style scoped>
.authenticated-play-shell :deep(.play-page) {
  min-height: auto;
  background: transparent;
}

.authenticated-play-shell :deep(.play-main) {
  padding: 0 0 40px;
}
</style>
