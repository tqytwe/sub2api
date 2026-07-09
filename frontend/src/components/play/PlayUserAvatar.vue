<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    name: string
    avatarUrl?: string | null
    sizeClass?: string
  }>(),
  {
    avatarUrl: '',
    sizeClass: 'h-8 w-8',
  },
)

const initials = computed(() => {
  const trimmed = props.name.trim()
  if (!trimmed) return '?'
  const parts = trimmed.split(/\s+/).filter(Boolean)
  if (parts.length >= 2) {
    return `${parts[0]![0] ?? ''}${parts[1]![0] ?? ''}`.toUpperCase()
  }
  return trimmed.slice(0, 2).toUpperCase()
})
</script>

<template>
  <span class="inline-flex min-w-0 items-center gap-2">
    <img
      v-if="avatarUrl"
      :src="avatarUrl"
      :alt="name"
      class="shrink-0 rounded-full object-cover"
      :class="sizeClass"
    />
    <span
      v-else
      class="inline-flex shrink-0 items-center justify-center rounded-full bg-gray-200 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-dark-200"
      :class="sizeClass"
      aria-hidden="true"
    >
      {{ initials }}
    </span>
    <span class="truncate">{{ name }}</span>
  </span>
</template>
