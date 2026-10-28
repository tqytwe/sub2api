<template>
  <div v-if="safe" class="mt-1 max-w-xs text-xs leading-5" data-testid="credential-operation-failure">
    <p class="font-medium text-red-700 dark:text-red-300">{{ t(`admin.accounts.credentialFailure.reasons.${safe.code}`) }}</p>
    <p class="text-gray-500 dark:text-gray-400">{{ t(`admin.accounts.credentialFailure.actions.${safe.action}`) }}</p>
    <details class="mt-1 text-gray-500 dark:text-gray-400">
      <summary class="cursor-pointer">{{ t('admin.accounts.credentialFailure.details') }}</summary>
      <p>{{ t('admin.accounts.credentialFailure.code') }}: {{ safe.code }}</p>
      <p>{{ t('admin.accounts.credentialFailure.time') }}: {{ time }}</p>
    </details>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CredentialOperationFailure } from '@/types'
import { safeCredentialOperationFailure } from '@/utils/credentialOperationFailure'

const props = defineProps<{ failure?: CredentialOperationFailure | null }>()
const { t } = useI18n()
const safe = computed(() => safeCredentialOperationFailure(props.failure))
const time = computed(() => safe.value?.occurred_at ? new Date(safe.value.occurred_at).toLocaleString() : t('admin.accounts.credentialFailure.unknownTime'))
</script>
