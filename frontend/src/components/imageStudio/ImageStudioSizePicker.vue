<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ImageStudioCapabilities, ImageStudioModelOption } from '@/api/imageStudio'

const props = defineProps<{
  capabilities: ImageStudioCapabilities | null
  aspect: string
  tier: string
  size?: string
  selectedModel?: ImageStudioModelOption | null
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:aspect': [value: string]
  'update:tier': [value: string]
  'update:size': [value: string]
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

const usesCustomDimensions = computed(() => (
  props.selectedModel?.sizing_kind === 'custom_dimensions'
))

const usesDedicatedSizeList = computed(() => {
  if (usesCustomDimensions.value) return false
  const model = props.selectedModel
  if (model?.sizing_kind !== 'fixed' || !model.supported_sizes?.length) return false
  const matrixSizes = new Set(sizeOptions.value.map((option) => option.size))
  return model.supported_sizes.some((size) => !matrixSizes.has(size))
})

const dedicatedSizeOptions = computed(() => props.selectedModel?.supported_sizes ?? [])
const customWidth = ref('')
const customHeight = ref('')

function parseDimensions(value?: string) {
  const match = /^\s*(\d+)x(\d+)\s*$/i.exec(value ?? '')
  if (!match) return null
  return { width: match[1], height: match[2] }
}

function syncCustomDimensions(value?: string) {
  const dimensions = parseDimensions(value)
  if (!dimensions) return
  customWidth.value = dimensions.width
  customHeight.value = dimensions.height
}

watch(
  () => props.size,
  syncCustomDimensions,
  { immediate: true },
)

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
  if (usesCustomDimensions.value || usesDedicatedSizeList.value) return props.size ?? ''
  const match = sizeOptions.value.find((opt) => opt.aspect === props.aspect && opt.tier === props.tier)
  return match?.size ?? ''
})

function updateCustomDimension(dimension: 'width' | 'height', event: Event) {
  const value = (event.target as HTMLInputElement).value
  if (dimension === 'width') customWidth.value = value
  else customHeight.value = value

  if (!/^\d+$/.test(customWidth.value) || !/^\d+$/.test(customHeight.value)) return
  emit('update:size', `${customWidth.value}x${customHeight.value}`)
}

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
    <fieldset v-if="usesCustomDimensions" class="min-w-0 sm:col-span-2">
      <legend class="input-label">{{ t('imageStudio.customDimensions') }}</legend>
      <div class="grid grid-cols-2 gap-3">
        <label class="min-w-0">
          <span class="sr-only">{{ t('imageStudio.width') }}</span>
          <input
            data-testid="image-size-width"
            type="number"
            inputmode="numeric"
            class="input tabular-nums"
            :value="customWidth"
            :min="selectedModel?.min_dimension"
            :max="selectedModel?.max_dimension"
            :step="selectedModel?.dimension_step"
            :disabled="disabled"
            :aria-label="t('imageStudio.width')"
            @input="updateCustomDimension('width', $event)"
          >
        </label>
        <label class="min-w-0">
          <span class="sr-only">{{ t('imageStudio.height') }}</span>
          <input
            data-testid="image-size-height"
            type="number"
            inputmode="numeric"
            class="input tabular-nums"
            :value="customHeight"
            :min="selectedModel?.min_dimension"
            :max="selectedModel?.max_dimension"
            :step="selectedModel?.dimension_step"
            :disabled="disabled"
            :aria-label="t('imageStudio.height')"
            @input="updateCustomDimension('height', $event)"
          >
        </label>
      </div>
      <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
        {{ t('imageStudio.sizeConstraint', {
          min: selectedModel?.min_dimension,
          max: selectedModel?.max_dimension,
          step: selectedModel?.dimension_step,
          ratio: selectedModel?.max_aspect_ratio,
        }) }}
      </p>
    </fieldset>

    <fieldset v-else-if="usesDedicatedSizeList" class="min-w-0 sm:col-span-2">
      <legend class="input-label">{{ t('imageStudio.size') }}</legend>
      <select
        data-testid="image-size-select"
        class="input font-mono"
        :value="size"
        :disabled="disabled"
        @change="emit('update:size', ($event.target as HTMLSelectElement).value)"
      >
        <option v-for="option in dedicatedSizeOptions" :key="option" :value="option">
          {{ option }}
        </option>
      </select>
    </fieldset>

    <fieldset v-else class="min-w-0">
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
      <div class="grid min-h-11 grid-cols-4 gap-1 rounded-xl border border-gray-200 bg-gray-50 p-1 dark:border-dark-600 dark:bg-dark-900">
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
