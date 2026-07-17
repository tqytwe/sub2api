<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ImageStudioCapabilities, ImageStudioModelOption } from '@/api/imageStudio'

const props = defineProps<{
  capabilities: ImageStudioCapabilities | null
  aspect: string
  tier: string
  selectedModel?: ImageStudioModelOption | null
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:aspect': [value: string]
  'update:tier': [value: string]
}>()

const { t, locale } = useI18n()

function labelFor(obj?: { zh: string; en: string }) {
  if (!obj) return ''
  return locale.value.startsWith('zh') ? obj.zh : obj.en
}

const supportedSizeSet = computed(() => {
  if (!props.selectedModel?.supported_sizes?.length) return null
  return new Set(props.selectedModel.supported_sizes)
})

const sizeOptions = computed(() => props.capabilities?.size_options ?? [])

function isAspectDisabled(aspectId: string) {
  const supported = supportedSizeSet.value
  if (!supported) return false
  return !sizeOptions.value.some((opt) => opt.aspect === aspectId && supported.has(opt.size))
}

function isTierDisabled(tierId: string) {
  const supported = supportedSizeSet.value
  if (!supported) return false
  return !sizeOptions.value.some(
    (opt) => opt.aspect === props.aspect && opt.tier === tierId && supported.has(opt.size),
  )
}

const resolvedLabel = computed(() => {
  const match = sizeOptions.value.find((opt) => opt.aspect === props.aspect && opt.tier === props.tier)
  return match?.size ?? ''
})

function aspectShapeStyle(aspectId: string) {
  const [width, height] = aspectId.split(':').map(Number)
  if (!width || !height) return { width: '14px', height: '14px' }
  const max = 16
  if (width >= height) {
    return { width: `${max}px`, height: `${Math.max(8, Math.round(max * height / width))}px` }
  }
  return { width: `${Math.max(8, Math.round(max * width / height))}px`, height: `${max}px` }
}
</script>

<template>
  <div class="grid gap-4 sm:grid-cols-2">
    <fieldset class="min-w-0">
      <legend class="input-label">{{ t('imageStudio.aspect') }}</legend>
      <div class="grid min-h-11 grid-cols-5 gap-1 rounded-xl border border-gray-200 bg-gray-50 p-1 dark:border-dark-600 dark:bg-dark-900">
        <button
          v-for="item in capabilities?.aspects || []"
          :key="item.id"
          type="button"
          class="flex min-w-0 flex-col items-center justify-center gap-1 rounded-lg px-0.5 py-2 text-[10px] font-medium transition focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40"
          :class="item.id === aspect
            ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-700 dark:text-primary-300'
            : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-100'"
          :disabled="disabled || isAspectDisabled(item.id)"
          :title="isAspectDisabled(item.id) ? t('imageStudio.optionUnsupported') : labelFor(item.label)"
          @click="emit('update:aspect', item.id)"
        >
          <span class="inline-block flex-shrink-0 rounded-[2px] border border-current" :style="aspectShapeStyle(item.id)" />
          <span>{{ item.id }}</span>
        </button>
      </div>
    </fieldset>
    <fieldset class="min-w-0">
      <legend class="input-label">{{ t('imageStudio.tier') }}</legend>
      <div class="grid min-h-11 grid-cols-3 gap-1 rounded-xl border border-gray-200 bg-gray-50 p-1 dark:border-dark-600 dark:bg-dark-900">
        <button
          v-for="item in capabilities?.tiers || []"
          :key="item.id"
          type="button"
          class="rounded-lg px-2 py-2 text-xs font-semibold transition focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 disabled:cursor-not-allowed disabled:opacity-35"
          :class="item.id === tier
            ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-700 dark:text-primary-300'
            : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-100'"
          :disabled="disabled || isTierDisabled(item.id)"
          :title="isTierDisabled(item.id) ? t('imageStudio.optionUnsupported') : labelFor(item.label)"
          @click="emit('update:tier', item.id)"
        >
          {{ labelFor(item.label) || item.id }}
        </button>
      </div>
    </fieldset>
    <div v-if="resolvedLabel" class="sm:col-span-2 flex items-center justify-between rounded-lg bg-gray-50 px-3 py-2 text-xs text-gray-500 dark:bg-dark-900 dark:text-gray-400">
      <span>{{ t('imageStudio.outputSpec') }}</span>
      <span class="font-mono font-medium text-gray-700 dark:text-gray-200">{{ resolvedLabel }}</span>
    </div>
  </div>
</template>
