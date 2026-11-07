<template>
  <fieldset class="min-w-0 space-y-2" :disabled="disabled || loading || !!error">
    <legend class="field-title">{{ t('admin.qualityOps.isolation.groups') }}</legend>
    <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.qualityOps.isolation.help') }}</p>
    <p v-if="loading" role="status" class="text-sm text-gray-500">{{ t('common.loading') }}</p>
    <p v-else-if="error" role="alert" class="text-sm text-red-700 dark:text-red-300">{{ error }}</p>
    <p v-else-if="!options.length" class="text-sm text-gray-500">{{ t('common.noGroupsAvailable') }}</p>
    <div class="flex flex-wrap gap-x-4 gap-y-2">
      <label v-for="group in options" :key="group.id" class="flex min-h-11 items-center gap-2 text-sm">
        <input type="checkbox" :value="group.id" :checked="modelValue.includes(group.id)" @change="toggle(group.id, ($event.target as HTMLInputElement).checked)" />
        <span class="break-all">{{ group.name }} #{{ group.id }}</span>
      </label>
    </div>
  </fieldset>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
const props = defineProps<{
  modelValue: number[]
  groups: { id: number; name: string }[]
  loading: boolean
  error: string
  disabled: boolean
}>()
const emit = defineEmits<{ 'update:modelValue': [value: number[]] }>()
const { t } = useI18n()
// A missing/deleted group stays visible and selected until explicitly removed.
const options = computed(() => [...props.groups, ...props.modelValue.filter(id => !props.groups.some(g => g.id === id)).map(id => ({ id, name: t('admin.qualityOps.isolation.unavailableGroup') }))])
function toggle(id: number, checked: boolean) {
  emit('update:modelValue', checked ? [...new Set([...props.modelValue, id])] : props.modelValue.filter(value => value !== id))
}
</script>
