<template>
  <BaseDialog
    :show="show"
    :title="dialogTitle"
    width="wide"
    :close-on-click-outside="false"
    @close="close"
  >
    <div class="space-y-5" :aria-busy="previewing || executing">
      <div class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900/40">
        <div>
          <div class="font-medium text-gray-900 dark:text-white">
            {{ t('admin.users.bulkActions.selectedCount', { count: snapshotUserIds.length }) }}
          </div>
          <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.users.bulkActions.selectionHint') }}
          </div>
        </div>
        <span :class="['badge', preview ? 'badge-success' : 'badge-warning']">
          {{ preview
            ? t('admin.users.bulkActions.previewReady')
            : t('admin.users.bulkActions.previewRequired') }}
        </span>
      </div>

      <div v-if="!result">
        <label class="input-label" for="bulk-user-action-reason">
          {{ t('admin.users.bulkActions.reason') }}
        </label>
        <textarea
          id="bulk-user-action-reason"
          v-model="reason"
          rows="3"
          maxlength="1000"
          class="input min-h-[96px] resize-y"
          :placeholder="t('admin.users.bulkActions.reasonPlaceholder')"
          :disabled="previewing || executing"
          data-test="reason"
          @input="invalidatePreview"
        ></textarea>
        <div class="mt-1 flex justify-between gap-3 text-xs text-gray-500 dark:text-gray-400">
          <span>{{ t('admin.users.bulkActions.reasonAuditHint') }}</span>
          <span class="tabular-nums">{{ reason.length }}/1000</span>
        </div>
      </div>

      <section v-if="preview && !result" class="space-y-3" aria-live="polite">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('admin.users.bulkActions.impactPreview') }}
          </h4>
          <span class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.users.bulkActions.expiresAt', { time: formatDateTime(preview.expires_at) }) }}
          </span>
        </div>
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
            <div class="text-xs text-gray-500 dark:text-gray-400">
              {{ action === 'delete'
                ? t('admin.users.bulkActions.eligibleDelete')
                : t('admin.users.bulkActions.eligibleDisable') }}
            </div>
            <div class="mt-1 text-xl font-semibold tabular-nums text-gray-900 dark:text-white" data-test="eligible-count">
              {{ preview.eligible_users.length }}
            </div>
          </div>
          <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
            <div class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.users.bulkActions.protectedAdmins') }}
            </div>
            <div class="mt-1 text-xl font-semibold tabular-nums text-gray-900 dark:text-white" data-test="protected-count">
              {{ preview.protected_administrators.length }}
            </div>
          </div>
          <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
            <div class="text-xs text-gray-500 dark:text-gray-400">
              {{ action === 'disable'
                ? t('admin.users.bulkActions.alreadyDisabledOrMissing')
                : t('admin.users.bulkActions.missing') }}
            </div>
            <div class="mt-1 text-xl font-semibold tabular-nums text-gray-900 dark:text-white" data-test="missing-count">
              {{ skippedStateCount }}
            </div>
          </div>
          <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
            <div class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.users.bulkActions.affectedKeys') }}
            </div>
            <div class="mt-1 text-xl font-semibold tabular-nums text-gray-900 dark:text-white" data-test="key-count">
              {{ preview.affected_api_keys }}
            </div>
          </div>
        </div>

        <div
          v-if="previewDisplayRows.length"
          class="divide-y divide-gray-200 overflow-hidden rounded-lg border border-gray-200 text-sm dark:divide-dark-700 dark:border-dark-700"
          data-test="preview-users"
        >
          <div
            v-for="row in previewDisplayRows"
            :key="row.key"
            class="flex items-center justify-between gap-3 px-3 py-2.5"
          >
            <span class="min-w-0 truncate text-gray-900 dark:text-white">{{ row.label }}</span>
            <span
              class="shrink-0 text-right"
              :class="row.protected
                ? 'text-amber-700 dark:text-amber-300'
                : 'text-gray-500 dark:text-gray-400'"
            >
              {{ row.detail }}
            </span>
          </div>
          <div
            v-if="hiddenPreviewUserCount > 0"
            class="flex items-center justify-between gap-3 bg-gray-50 px-3 py-2.5 text-gray-500 dark:bg-dark-900/40 dark:text-gray-400"
          >
            <span>{{ t('admin.users.bulkActions.remainingUsers') }}</span>
            <span>{{ t('admin.users.bulkActions.remainingCount', { count: hiddenPreviewUserCount }) }}</span>
          </div>
        </div>

        <div v-if="preview.protected_administrators.length" class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-800/60 dark:bg-amber-950/30 dark:text-amber-200">
          <div class="flex gap-2">
            <Icon name="exclamationTriangle" size="sm" class="mt-0.5 shrink-0" />
            <span>
              {{ t('admin.users.bulkActions.protectedHint', {
                count: preview.protected_administrators.length
              }) }}
            </span>
          </div>
        </div>
        <div v-if="preview.requires_step_up" class="rounded-lg border border-blue-200 bg-blue-50 p-3 text-sm text-blue-800 dark:border-blue-800/60 dark:bg-blue-950/30 dark:text-blue-200">
          <div class="flex gap-2">
            <Icon name="lock" size="sm" class="mt-0.5 shrink-0" />
            <span>{{ t('admin.users.bulkActions.stepUpRequired') }}</span>
          </div>
        </div>

        <div v-if="action === 'delete'" class="rounded-lg border border-red-200 bg-red-50 p-4 dark:border-red-800/60 dark:bg-red-950/30">
          <label class="input-label text-red-800 dark:text-red-200" for="bulk-user-delete-confirmation">
            {{ t('admin.users.bulkActions.deleteConfirmationLabel', { phrase: deleteConfirmationPhrase }) }}
          </label>
          <input
            id="bulk-user-delete-confirmation"
            v-model="deleteConfirmation"
            class="input font-mono"
            :placeholder="deleteConfirmationPhrase"
            :disabled="executing"
            autocomplete="off"
            data-test="delete-confirmation"
          />
          <p class="input-hint text-red-700 dark:text-red-300">
            {{ t('admin.users.bulkActions.deleteImpactHint') }}
          </p>
        </div>
      </section>

      <section
        v-if="result"
        class="space-y-4 rounded-lg border p-4"
        :class="resultClass"
        aria-live="polite"
      >
        <div class="flex items-start gap-2">
          <Icon
            :name="result.status === 'completed' ? 'checkCircle' : 'exclamationTriangle'"
            size="md"
            class="mt-0.5 shrink-0"
          />
          <div>
            <div class="font-medium">{{ t(`admin.users.bulkActions.result.${result.status}`) }}</div>
            <div class="mt-1 text-sm opacity-80">
              {{ t('admin.users.bulkActions.resultSummary', {
                succeeded: result.succeeded_user_ids.length,
                skipped: result.skipped.length,
                failed: result.failed.length,
                keys: result.affected_api_keys
              }) }}
            </div>
          </div>
        </div>
        <div v-if="result.failed.length" class="divide-y divide-red-200 border-y border-red-200 text-sm dark:divide-red-800/60 dark:border-red-800/60">
          <div v-for="item in result.failed" :key="item.user_id" class="flex justify-between gap-3 py-2">
            <span>{{ item.email || `#${item.user_id}` }}</span>
            <span class="text-right">
              {{ item.message || t('admin.users.bulkActions.itemFailed') }}
            </span>
          </div>
        </div>
      </section>

      <p v-if="selectionTooLarge" class="text-sm text-red-600 dark:text-red-400">
        {{ t('admin.users.bulkActions.selectionLimit', { max: MAX_BATCH_USER_IDS }) }}
      </p>
    </div>

    <template #footer>
      <div class="flex w-full flex-wrap items-center justify-between gap-3">
        <span class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.users.bulkActions.previewRule') }}
        </span>
        <div class="flex gap-2">
          <button type="button" class="btn btn-secondary" :disabled="previewing || executing" @click="close">
            {{ result ? t('common.close') : t('common.cancel') }}
          </button>
          <button
            v-if="!preview && !result"
            type="button"
            class="btn btn-primary"
            :disabled="!canPreview"
            data-test="preview"
            @click="createPreview"
          >
            <Icon name="eye" size="sm" class="mr-2" />
            {{ previewing ? t('admin.users.bulkActions.previewing') : t('admin.users.bulkActions.preview') }}
          </button>
          <button
            v-else-if="preview && !result"
            type="button"
            :class="action === 'delete' ? 'btn btn-danger' : 'btn btn-primary'"
            :disabled="!canExecute"
            data-test="execute"
            @click="execute"
          >
            <Icon :name="action === 'delete' ? 'trash' : 'ban'" size="sm" class="mr-2" />
            {{ executing ? t('admin.users.bulkActions.executing') : executeLabel }}
          </button>
        </div>
      </div>
    </template>
  </BaseDialog>

</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { isStepUpCancelled, type StepUpController } from '@/composables/useStepUp'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import type {
  UserBatchAction,
  UserBatchActionPreview,
  UserBatchActionResult,
} from '@/api/admin/users'

const props = defineProps<{
  show: boolean
  selectedIds: number[]
  action: UserBatchAction
  stepUp: StepUpController
}>()

const emit = defineEmits<{
  close: []
  completed: [result: UserBatchActionResult]
}>()

const MAX_BATCH_USER_IDS = 500
const { t } = useI18n()
const appStore = useAppStore()
const snapshotUserIds = ref<number[]>([])
const reason = ref('')
const deleteConfirmation = ref('')
const previewing = ref(false)
const executing = ref(false)
const preview = ref<UserBatchActionPreview | null>(null)
const result = ref<UserBatchActionResult | null>(null)
const MAX_PREVIEW_ROWS = 8

const dialogTitle = computed(() =>
  props.action === 'delete'
    ? t('admin.users.bulkActions.deleteTitle')
    : t('admin.users.bulkActions.disableTitle')
)
const selectionTooLarge = computed(() => snapshotUserIds.value.length > MAX_BATCH_USER_IDS)
const canPreview = computed(() =>
  !previewing.value
  && !executing.value
  && snapshotUserIds.value.length > 0
  && !selectionTooLarge.value
  && reason.value.trim().length > 0
)
const skippedStateCount = computed(() =>
  (preview.value?.already_disabled_users.length || 0) + (preview.value?.missing_user_ids.length || 0)
)
const previewDisplayRows = computed(() => {
  if (!preview.value) return []
  const rows: Array<{ key: string; label: string; detail: string; protected: boolean }> = []
  for (const user of preview.value.eligible_users) {
    rows.push({
      key: `eligible-${user.id}`,
      label: user.email || t('admin.users.bulkActions.userId', { id: user.id }),
      detail: props.action === 'delete'
        ? t('admin.users.bulkActions.deleteUserImpact', { count: user.api_key_count })
        : t('admin.users.bulkActions.disableUserImpact'),
      protected: false,
    })
  }
  for (const user of preview.value.protected_administrators) {
    rows.push({
      key: `admin-${user.id}`,
      label: user.email || t('admin.users.bulkActions.userId', { id: user.id }),
      detail: t('admin.users.bulkActions.protectedUserImpact'),
      protected: true,
    })
  }
  for (const user of preview.value.already_disabled_users) {
    rows.push({
      key: `disabled-${user.id}`,
      label: user.email || t('admin.users.bulkActions.userId', { id: user.id }),
      detail: t('admin.users.bulkActions.alreadyDisabledImpact'),
      protected: false,
    })
  }
  for (const userId of preview.value.missing_user_ids) {
    rows.push({
      key: `missing-${userId}`,
      label: t('admin.users.bulkActions.userId', { id: userId }),
      detail: t('admin.users.bulkActions.missingUserImpact'),
      protected: false,
    })
  }
  return rows.slice(0, MAX_PREVIEW_ROWS)
})
const hiddenPreviewUserCount = computed(() =>
  Math.max(0, (preview.value?.requested_count || 0) - previewDisplayRows.value.length)
)
const deleteConfirmationPhrase = computed(() => `DELETE ${preview.value?.eligible_users.length || 0}`)
const canExecute = computed(() =>
  !!preview.value
  && !executing.value
  && preview.value.eligible_users.length > 0
  && (props.action !== 'delete' || deleteConfirmation.value === deleteConfirmationPhrase.value)
)
const executeLabel = computed(() =>
  props.action === 'delete'
    ? t('admin.users.bulkActions.confirmDelete', { count: preview.value?.eligible_users.length || 0 })
    : t('admin.users.bulkActions.confirmDisable', { count: preview.value?.eligible_users.length || 0 })
)
const resultClass = computed(() => {
  if (result.value?.status === 'completed') {
    return 'border-emerald-200 bg-emerald-50 text-emerald-900 dark:border-emerald-800/60 dark:bg-emerald-950/30 dark:text-emerald-200'
  }
  if (result.value?.status === 'partial') {
    return 'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-800/60 dark:bg-amber-950/30 dark:text-amber-200'
  }
  return 'border-red-200 bg-red-50 text-red-900 dark:border-red-800/60 dark:bg-red-950/30 dark:text-red-200'
})

watch(
  () => [props.show, props.action] as const,
  ([show]) => {
    if (!show) return
    snapshotUserIds.value = [...new Set(props.selectedIds)]
    reason.value = ''
    deleteConfirmation.value = ''
    preview.value = null
    result.value = null
    previewing.value = false
    executing.value = false
  },
  { immediate: true }
)

const invalidatePreview = () => {
  preview.value = null
  deleteConfirmation.value = ''
}

const createPreview = async () => {
  if (!canPreview.value) return
  previewing.value = true
  try {
    preview.value = await adminAPI.users.previewBatchAction({
      action: props.action,
      user_ids: snapshotUserIds.value,
      reason: reason.value.trim(),
    })
  } catch (error) {
    appStore.showError(
      extractApiErrorMessage(error) || t('admin.users.bulkActions.previewFailed')
    )
  } finally {
    previewing.value = false
  }
}

const execute = async () => {
  if (!canExecute.value || !preview.value) return
  executing.value = true
  try {
    const execution = () => adminAPI.users.executeBatchAction({
      action: props.action,
      user_ids: snapshotUserIds.value,
      reason: reason.value.trim(),
      confirmation_token: preview.value!.confirmation_token,
    })
    result.value = await props.stepUp.run(execution)
    emit('completed', result.value)
  } catch (error) {
    if (isStepUpCancelled(error)) return
    const code = extractApiErrorCode(error)
    if (code === 'USER_BATCH_ACTION_PREVIEW_STALE' || code === 'USER_BATCH_ACTION_PREVIEW_EXPIRED') {
      invalidatePreview()
      appStore.showError(t('admin.users.bulkActions.previewStale'))
      return
    }
    appStore.showError(
      extractApiErrorMessage(error) || t('admin.users.bulkActions.executeFailed')
    )
  } finally {
    executing.value = false
  }
}

const close = () => {
  if (previewing.value || executing.value) return
  emit('close')
}
</script>
