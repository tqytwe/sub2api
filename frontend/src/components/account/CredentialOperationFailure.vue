<template>
  <div class="mt-2 max-w-72 text-xs leading-5" data-testid="credential-operation-failure">
    <p class="font-medium text-red-700 dark:text-red-300">{{ reason }}</p>
    <p class="text-gray-500 dark:text-gray-400">{{ action }}</p>
    <details class="mt-1 text-gray-500 dark:text-gray-400">
      <summary class="cursor-pointer">{{ t('tokenGuardV2.operationFailure.details') }}</summary>
      <p>{{ t('tokenGuardV2.operationFailure.code') }}: {{ safe.code }}</p>
      <p>{{ t('tokenGuardV2.operationFailure.stage') }}: {{ safe.stage }}</p>
      <p>{{ t('tokenGuardV2.operationFailure.time') }}: {{ time }}</p>
    </details>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CredentialOperationFailure } from '@/api/admin/accountTokenGuardV2'
import { safeOperationFailure } from '@/utils/credentialOperationFailure'

const props = defineProps<{ failure: CredentialOperationFailure }>()
const { t, tm, getLocaleMessage, locale } = useI18n()
const safe = computed(() => safeOperationFailure(props.failure))
const operationFailureMessages = computed(() => {
  if (typeof tm === 'function') return tm('tokenGuardV2.operationFailure') as Record<string, Record<string, string>>
  const messages = typeof getLocaleMessage === 'function' ? getLocaleMessage(locale.value) as Record<string, unknown> : undefined
  const tokenGuard = messages?.tokenGuardV2 as Record<string, unknown> | undefined
  return tokenGuard?.operationFailure as Record<string, Record<string, string>> | undefined
})
const reason = computed(() => operationFailureMessages.value?.reasons?.[safe.value.category ?? 'unknown'] || t(`tokenGuardV2.operationFailure.reasons.${safe.value.category ?? 'unknown'}`))
const action = computed(() => operationFailureMessages.value?.actions?.[safe.value.action ?? 'review_diagnostics'] || t(`tokenGuardV2.operationFailure.actions.${safe.value.action ?? 'review_diagnostics'}`))
const detail = (key: string) => operationFailureMessages.value?.[key] as unknown as string || t(`tokenGuardV2.operationFailure.${key}`)
const time = computed(() => safe.value.occurred_at
  ? new Date(safe.value.occurred_at).toLocaleString()
  : detail('unknownTime'))
</script>
