<template>
  <section v-if="groups.length" class="space-y-3" data-testid="account-group-model-limits" :aria-busy="disabled">
    <div>
      <h3 class="input-label">{{ t('admin.accounts.groupModelLimits.title') }}</h3>
      <p class="input-hint">{{ t('admin.accounts.groupModelLimits.hint') }}</p>
      <p class="input-hint">{{ t('admin.accounts.groupModelLimits.aliasHint') }}</p>
    </div>
    <div v-for="group in groups" :key="group.id" class="space-y-1">
      <div class="flex items-center justify-between gap-2">
        <label :for="`group-models-${group.id}`" class="input-label mb-0">{{ group.name }}</label>
        <button type="button" :aria-label="`${group.name}: ${t('admin.accounts.groupModelLimits.unrestricted')}`" class="btn btn-secondary" :disabled="disabled || !(modelValue[group.id]?.length)" @click="setModels(group.id, [])">
          {{ t('admin.accounts.groupModelLimits.unrestricted') }}
        </button>
      </div>
      <p :id="`group-models-saved-${group.id}`" class="input-hint break-words">
        {{ t('admin.accounts.groupModelLimits.saved') }}: {{ saved[group.id]?.join(', ') || t('admin.accounts.groupModelLimits.unrestricted') }}
      </p>
      <textarea
        :id="`group-models-${group.id}`"
        :value="(modelValue[group.id] ?? []).join('\n')"
        :disabled="disabled"
        :aria-invalid="!!groupAllowedModelsError(modelValue[group.id] ?? [])"
        :aria-describedby="`group-models-saved-${group.id} group-models-state-${group.id}`"
        class="input w-full font-mono text-sm"
        rows="2"
        :placeholder="t('admin.accounts.groupModelLimits.placeholder')"
        @input="setModels(group.id, ($event.target as HTMLTextAreaElement).value.split(/[\n,]/))"
      />
      <p :id="`group-models-state-${group.id}`" class="input-hint" :class="{ 'text-red-600 dark:text-red-400': groupAllowedModelsError(modelValue[group.id] ?? []) }" aria-live="polite">
        {{ groupAllowedModelsError(modelValue[group.id] ?? [])
          ? t(`admin.accounts.groupModelLimits.${groupAllowedModelsError(modelValue[group.id] ?? [])}`)
          : t('admin.accounts.groupModelLimits.emptyHint') }}
      </p>
    </div>
    <p class="input-hint">{{ t('admin.accounts.groupModelLimits.exportHint') }}</p>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { groupAllowedModelsError, type GroupAllowedModels } from './groupAllowedModels'
const props = defineProps<{
  modelValue: GroupAllowedModels
  saved: GroupAllowedModels
  groups: { id: number; name: string }[]
  disabled?: boolean
}>()
const emit = defineEmits<{ 'update:modelValue': [value: GroupAllowedModels] }>()
const { t } = useI18n()
const setModels = (id: number, models: string[]) => emit('update:modelValue', { ...props.modelValue, [id]: models })
</script>
