<template>
  <div class="space-y-1.5" data-testid="pelican-prompt-mode">
    <label class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('pelicanTests.questionBank.mode') }}</label>
    <Select v-model="selected" :options="options" :disabled="disabled" :aria-label="t('pelicanTests.questionBank.mode')" data-testid="pelican-mode-select" />
    <p v-if="selected === 'random'" class="text-xs text-gray-500 dark:text-gray-400">{{ t('pelicanTests.questionBank.hint') }}</p>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'

const props = defineProps<{ modelValue?: string; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: 'custom' | 'random'] }>()
const { t } = useI18n()
const options = computed(() => [
  { value: 'custom', label: t('pelicanTests.questionBank.custom') },
  { value: 'random', label: t('pelicanTests.questionBank.random') },
])
const selected = computed({
  get: () => props.modelValue || 'custom',
  set: (value: string) => emit('update:modelValue', value === 'random' ? 'random' : 'custom'),
})
</script>
