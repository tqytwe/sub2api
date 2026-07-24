<template>
  <div class="relative w-full">
    <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
      <Icon
        name="search"
        size="md"
        :class="disabled ? 'text-gray-300 dark:text-dark-500' : 'text-gray-400 dark:text-dark-400'"
      />
    </div>
    <input
      ref="inputRef"
      :value="modelValue"
      type="text"
      class="input pl-10 transition-[background-color,border-color,box-shadow,color] duration-150"
      :class="[
        clearable && hasValue ? 'pr-11' : '',
        disabled ? 'cursor-not-allowed bg-gray-100 opacity-60 dark:bg-dark-900' : ''
      ]"
      :placeholder="placeholder"
      :disabled="disabled"
      :aria-label="ariaLabelText"
      :autocomplete="autocomplete"
      @input="handleInput"
    />
    <button
      v-if="clearable && hasValue && !disabled"
      type="button"
      class="absolute inset-y-1.5 right-1.5 inline-flex w-8 items-center justify-center rounded-lg text-gray-400 transition-[background-color,color,box-shadow] duration-150 hover:bg-gray-100 hover:text-gray-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:text-dark-400 dark:hover:bg-dark-700 dark:hover:text-gray-100"
      :aria-label="clearLabelText"
      @click="clearSearch"
    >
      <Icon name="x" size="sm" aria-hidden="true" />
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  debounceMs?: number
  disabled?: boolean
  clearable?: boolean
  ariaLabel?: string
  clearLabel?: string
  autocomplete?: string
}>(), {
  placeholder: 'Search...',
  debounceMs: 300,
  disabled: false,
  clearable: true,
  autocomplete: 'off'
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'search', value: string): void
  (e: 'clear'): void
}>()

const { t } = useI18n()
const inputRef = ref<HTMLInputElement | null>(null)
const hasValue = computed(() => props.modelValue.length > 0)
const ariaLabelText = computed(() => props.ariaLabel || props.placeholder || t('common.search'))
const clearLabelText = computed(() => props.clearLabel || t('common.clearSearch'))

const debouncedEmitSearch = useDebounceFn((value: string) => {
  emit('search', value)
}, props.debounceMs)

const handleInput = (event: Event) => {
  if (props.disabled) return
  const value = (event.target as HTMLInputElement).value
  emit('update:modelValue', value)
  debouncedEmitSearch(value)
}

const clearSearch = () => {
  if (props.disabled) return
  const cancelDebouncedSearch = (debouncedEmitSearch as typeof debouncedEmitSearch & { cancel?: () => void }).cancel
  cancelDebouncedSearch?.()
  emit('update:modelValue', '')
  emit('search', '')
  emit('clear')
  inputRef.value?.focus()
}

defineExpose({
  focus: () => inputRef.value?.focus()
})
</script>
