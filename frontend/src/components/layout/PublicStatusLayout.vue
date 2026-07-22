<script setup lang="ts">
import PageFrame from './PageFrame.vue'

const props = withDefaults(defineProps<{
  frame?: 'compact' | 'reading' | 'content'
}>(), {
  frame: 'compact',
})
</script>

<template>
  <main class="relative flex min-h-screen items-center bg-primary-50 px-4 py-12 dark:bg-dark-950 sm:px-6 lg:px-8">
    <PageFrame :frame="props.frame">
      <slot />
    </PageFrame>
    <slot name="floating" />
  </main>
</template>
