<template>
  <div class="inline-flex max-w-full items-start gap-3">
    <button
      v-bind="$attrs"
      :id="controlId"
      type="button"
      class="relative inline-flex h-6 w-11 flex-shrink-0 rounded-full border-2 border-transparent transition-[background-color,border-color,box-shadow] duration-150 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 focus-visible:ring-offset-2 disabled:cursor-not-allowed dark:focus-visible:ring-offset-dark-800"
      :class="[
        modelValue ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600',
        disabled ? 'bg-gray-200 opacity-60 dark:bg-dark-700' : 'cursor-pointer hover:border-primary-200 dark:hover:border-primary-700',
        error ? 'border-red-500 focus-visible:ring-red-500/30' : ''
      ]"
      role="switch"
      :disabled="disabled"
      :aria-checked="modelValue"
      :aria-label="ariaLabelText"
      :aria-describedby="describedBy"
      :aria-invalid="error ? 'true' : undefined"
      @click="toggle"
    >
      <span
        class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow-sm transition-transform duration-150"
        :class="[modelValue ? 'translate-x-5' : 'translate-x-0']"
      />
    </button>

    <div v-if="hasSupportText" class="min-w-0">
      <label v-if="label" :for="controlId" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
        {{ label }}
      </label>
      <p v-if="description" :id="descriptionId" class="mt-1 text-xs leading-5 text-gray-500 dark:text-dark-400">
        {{ description }}
      </p>
      <p v-if="error" :id="errorId" class="mt-1 text-xs leading-5 text-red-500">
        {{ error }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

defineOptions({
  inheritAttrs: false
})

const props = withDefaults(defineProps<{
  modelValue: boolean
  id?: string
  label?: string
  description?: string
  error?: string
  disabled?: boolean
  ariaLabel?: string
}>(), {
  disabled: false
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

const { t } = useI18n()
const generatedId = `toggle-${Math.random().toString(36).slice(2, 9)}`

const controlId = computed(() => props.id || generatedId)
const descriptionId = computed(() => `${controlId.value}-description`)
const errorId = computed(() => `${controlId.value}-error`)
const hasSupportText = computed(() => Boolean(props.label || props.description || props.error))
const ariaLabelText = computed(() => props.ariaLabel || props.label || t('common.toggleSetting'))
const describedBy = computed(() => {
  const ids = []
  if (props.description) ids.push(descriptionId.value)
  if (props.error) ids.push(errorId.value)
  return ids.length > 0 ? ids.join(' ') : undefined
})

function toggle() {
  if (props.disabled) return
  emit('update:modelValue', !props.modelValue)
}
</script>
