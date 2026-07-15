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
</script>

<template>
  <div class="gw-size-picker space-y-3">
    <div class="gw-field-row">
      <label class="gw-field">
        <span class="gw-field-label">{{ t('imageStudio.aspect') }}</span>
        <select
          :value="aspect"
          class="gw-select"
          :disabled="disabled"
          @change="emit('update:aspect', ($event.target as HTMLSelectElement).value)"
        >
          <option
            v-for="item in capabilities?.aspects || []"
            :key="item.id"
            :value="item.id"
            :disabled="isAspectDisabled(item.id)"
          >
            {{ labelFor(item.label) }}
          </option>
        </select>
      </label>
      <label class="gw-field">
        <span class="gw-field-label">{{ t('imageStudio.tier') }}</span>
        <select
          :value="tier"
          class="gw-select"
          :disabled="disabled"
          @change="emit('update:tier', ($event.target as HTMLSelectElement).value)"
        >
          <option
            v-for="item in capabilities?.tiers || []"
            :key="item.id"
            :value="item.id"
            :disabled="isTierDisabled(item.id)"
          >
            {{ labelFor(item.label) }}
          </option>
        </select>
      </label>
    </div>
    <p v-if="resolvedLabel" class="text-xs" style="color: var(--gw-ink-3)">
      {{ t('imageStudio.resolvedSize', { size: resolvedLabel }) }}
    </p>
  </div>
</template>
