<template>
  <section
    class="rounded-lg border bg-white p-6 text-center shadow-sm dark:bg-dark-900/80 sm:p-8"
    :class="toneClasses.panel"
    :aria-busy="loading || undefined"
  >
    <div
      class="mx-auto flex h-14 w-14 items-center justify-center rounded-full border"
      :class="toneClasses.iconWrap"
      aria-hidden="true"
    >
      <LoadingSpinner v-if="loading" size="md" :color="loadingColor" />
      <Icon v-else-if="icon" :name="icon" size="xl" :class="toneClasses.icon" />
    </div>

    <p v-if="eyebrow" class="mt-5 text-xs font-semibold uppercase text-gray-500 dark:text-dark-400">
      {{ eyebrow }}
    </p>
    <h1 class="mt-3 text-2xl font-semibold leading-8 text-gray-950 dark:text-white">
      {{ title }}
    </h1>
    <p v-if="description" class="mt-3 text-sm leading-6 text-gray-600 dark:text-dark-300">
      {{ description }}
    </p>

    <div
      v-if="$slots.details"
      class="mt-6 rounded-lg border border-gray-200 bg-gray-50 p-4 text-left dark:border-dark-700 dark:bg-dark-950/50"
    >
      <slot name="details" />
    </div>

    <div v-if="$slots.actions" class="mt-6 flex flex-col gap-3 sm:flex-row sm:justify-center">
      <slot name="actions" />
    </div>

    <div v-if="$slots.support" class="mt-6 border-t border-gray-100 pt-5 dark:border-dark-700">
      <slot name="support" />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'

type StatusTone = 'neutral' | 'primary' | 'success' | 'warning' | 'danger'
type IconName = InstanceType<typeof Icon>['$props']['name']

const props = withDefaults(defineProps<{
  title: string
  description?: string
  eyebrow?: string
  icon?: IconName
  tone?: StatusTone
  loading?: boolean
}>(), {
  description: '',
  eyebrow: '',
  tone: 'neutral',
  loading: false,
})

const toneMap: Record<StatusTone, {
  panel: string
  iconWrap: string
  icon: string
  loading: 'primary' | 'secondary'
}> = {
  neutral: {
    panel: 'border-gray-200 dark:border-dark-700',
    iconWrap: 'border-gray-200 bg-gray-50 text-gray-500 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-300',
    icon: 'text-gray-500 dark:text-dark-300',
    loading: 'secondary',
  },
  primary: {
    panel: 'border-primary-200 dark:border-primary-800/60',
    iconWrap: 'border-primary-200 bg-primary-50 text-primary-600 dark:border-primary-800/60 dark:bg-primary-900/30 dark:text-primary-300',
    icon: 'text-primary-600 dark:text-primary-300',
    loading: 'primary',
  },
  success: {
    panel: 'border-emerald-200 dark:border-emerald-800/60',
    iconWrap: 'border-emerald-200 bg-emerald-50 text-emerald-600 dark:border-emerald-800/60 dark:bg-emerald-900/30 dark:text-emerald-300',
    icon: 'text-emerald-600 dark:text-emerald-300',
    loading: 'primary',
  },
  warning: {
    panel: 'border-amber-200 dark:border-amber-800/60',
    iconWrap: 'border-amber-200 bg-amber-50 text-amber-600 dark:border-amber-800/60 dark:bg-amber-900/30 dark:text-amber-300',
    icon: 'text-amber-600 dark:text-amber-300',
    loading: 'secondary',
  },
  danger: {
    panel: 'border-red-200 dark:border-red-800/60',
    iconWrap: 'border-red-200 bg-red-50 text-red-600 dark:border-red-800/60 dark:bg-red-900/30 dark:text-red-300',
    icon: 'text-red-600 dark:text-red-300',
    loading: 'secondary',
  },
}

const toneClasses = computed(() => toneMap[props.tone])
const loadingColor = computed(() => toneClasses.value.loading)
</script>
