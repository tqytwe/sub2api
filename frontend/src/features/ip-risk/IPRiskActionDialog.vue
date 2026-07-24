<template>
  <BaseDialog
    :show="show"
    :title="t('admin.ipRisk.actionDialog.title')"
    width="wide"
    :close-on-click-outside="false"
    @close="close"
  >
    <div v-if="detail" class="space-y-5">
      <div class="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900/40">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <div class="text-sm font-medium text-gray-900 dark:text-white">
              {{ detail.case.primary_ip }} · {{ detail.case.score }} {{ t('admin.ipRisk.riskScoreUnit') }}
            </div>
            <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.ipRisk.actionDialog.selectedSummary', { count: selectedUserIds.length }) }}
            </div>
          </div>
          <span :class="['badge', preview ? 'badge-success' : 'badge-warning']">
            {{ preview ? t('admin.ipRisk.actionDialog.previewReady') : t('admin.ipRisk.actionDialog.previewRequired') }}
          </span>
        </div>
      </div>

      <div class="grid gap-4 md:grid-cols-2">
        <div>
          <label class="input-label" for="ip-risk-action-type">{{ t('admin.ipRisk.actionDialog.action') }}</label>
          <Select
            id="ip-risk-action-type"
            v-model="form.action_type"
            :options="actionOptions"
            :disabled="executing"
            @change="resetPreview"
          />
        </div>
        <div v-if="form.action_type === 'temporary_registration_block'">
          <label class="input-label" for="ip-risk-duration">{{ t('admin.ipRisk.actionDialog.duration') }}</label>
          <Select
            id="ip-risk-duration"
            v-model="form.duration_minutes"
            :options="durationOptions"
            :disabled="executing"
            @change="resetPreview"
          />
        </div>
      </div>

      <div>
        <label class="input-label" for="ip-risk-action-reason">{{ t('admin.ipRisk.actionDialog.reason') }}</label>
        <textarea
          id="ip-risk-action-reason"
          v-model="form.reason"
          rows="3"
          maxlength="1000"
          class="input min-h-[96px] resize-y"
          :placeholder="t('admin.ipRisk.actionDialog.reasonPlaceholder')"
          :disabled="executing"
          @input="resetPreview"
        ></textarea>
        <div class="mt-1 text-right text-xs tabular-nums text-gray-500 dark:text-gray-400">
          {{ form.reason.length }}/1000
        </div>
      </div>

      <div v-if="form.action_type === 'permanent_registration_block'" class="rounded-lg border border-red-200 bg-red-50 p-4 dark:border-red-800/60 dark:bg-red-950/30">
        <label class="input-label text-red-800 dark:text-red-200" for="ip-risk-permanent-confirmation">
          {{ t('admin.ipRisk.actionDialog.permanentConfirmationLabel') }}
        </label>
        <p class="mb-2 text-xs text-red-700 dark:text-red-300">
          {{ t('admin.ipRisk.actionDialog.permanentConfirmationHint', { ip: detail.case.primary_ip }) }}
        </p>
        <input
          id="ip-risk-permanent-confirmation"
          v-model.trim="permanentConfirmation"
          class="input font-mono"
          :placeholder="detail.case.primary_ip"
          :disabled="executing"
          autocomplete="off"
          @input="resetPreview"
        />
      </div>

      <div v-if="actionNeedsUsers && selectedUserIds.length === 0" class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-800/60 dark:bg-amber-950/30 dark:text-amber-200">
        <div class="flex gap-2">
          <Icon name="exclamationTriangle" size="sm" class="mt-0.5 shrink-0" />
          <span>{{ t('admin.ipRisk.actionDialog.selectUsersFirst') }}</span>
        </div>
      </div>

      <section v-if="preview" class="space-y-3" aria-live="polite">
        <div class="flex items-center justify-between gap-3">
          <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.ipRisk.actionDialog.impactPreview') }}</h4>
          <span class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.ipRisk.actionDialog.expiresAt', { time: formatDateTime(preview.expires_at) }) }}
          </span>
        </div>
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div v-for="metric in previewMetrics" :key="metric.label" class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ metric.label }}</div>
            <div class="mt-1 text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ metric.value }}</div>
          </div>
        </div>

        <div v-if="preview.protected_users.length" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800/60 dark:bg-red-950/30 dark:text-red-200">
          {{ t('admin.ipRisk.actionDialog.protectedExcluded', { count: preview.protected_users.length }) }}
        </div>
        <div v-if="preview.trusted_users.length" class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-800/60 dark:bg-amber-950/30 dark:text-amber-200">
          {{ t('admin.ipRisk.actionDialog.trustedWarning', { count: preview.trusted_users.length }) }}
        </div>
        <div v-if="preview.inferred_users.length" class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-800/60 dark:bg-amber-950/30 dark:text-amber-200">
          {{ t('admin.ipRisk.actionDialog.inferredWarning', { count: preview.inferred_users.length }) }}
        </div>
        <div v-if="preview.requires_step_up" class="rounded-lg border border-blue-200 bg-blue-50 p-3 text-sm text-blue-800 dark:border-blue-800/60 dark:bg-blue-950/30 dark:text-blue-200">
          <div class="flex gap-2">
            <Icon name="lock" size="sm" class="mt-0.5 shrink-0" />
            <span>{{ t('admin.ipRisk.actionDialog.stepUpRequired') }}</span>
          </div>
        </div>
      </section>

      <section v-if="result" class="rounded-lg border p-4" :class="result.status === 'completed' ? 'border-emerald-200 bg-emerald-50 dark:border-emerald-800/60 dark:bg-emerald-950/30' : result.status === 'partial' ? 'border-amber-200 bg-amber-50 dark:border-amber-800/60 dark:bg-amber-950/30' : 'border-red-200 bg-red-50 dark:border-red-800/60 dark:bg-red-950/30'">
        <div class="flex items-start gap-2">
          <Icon :name="result.status === 'completed' ? 'checkCircle' : 'exclamationTriangle'" size="md" class="mt-0.5 shrink-0" />
          <div>
            <div class="font-medium">{{ t(`admin.ipRisk.actionDialog.result.${result.status}`) }}</div>
            <div class="mt-1 text-sm opacity-80">
              {{ t('admin.ipRisk.actionDialog.resultSummary', {
                completed: result.result.completed_items || 0,
                failed: result.result.failed_items || 0,
              }) }}
            </div>
          </div>
        </div>
      </section>
    </div>

    <template #footer>
      <div class="flex w-full flex-wrap items-center justify-between gap-3">
        <div class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.ipRisk.actionDialog.previewRule') }}
        </div>
        <div class="flex gap-2">
          <button type="button" class="btn btn-secondary" :disabled="previewing || executing" @click="close">
            {{ t('common.cancel') }}
          </button>
          <button
            v-if="!preview"
            type="button"
            class="btn btn-primary"
            :disabled="!canPreview || previewing"
            @click="createPreview"
          >
            <Icon name="eye" size="sm" class="mr-2" />
            {{ previewing ? t('admin.ipRisk.actionDialog.previewing') : t('admin.ipRisk.actionDialog.preview') }}
          </button>
          <button
            v-else
            type="button"
            :class="isDestructive ? 'btn btn-danger' : 'btn btn-primary'"
            :disabled="executing"
            @click="execute"
          >
            <Icon name="shield" size="sm" class="mr-2" />
            {{ executing ? t('admin.ipRisk.actionDialog.executing') : t('admin.ipRisk.actionDialog.confirmExecute') }}
          </button>
        </div>
      </div>
    </template>
  </BaseDialog>

</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { isStepUpCancelled, type StepUpController } from '@/composables/useStepUp'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import type {
  RiskActionInput,
  RiskActionPreview,
  RiskActionRecord,
  RiskActionType,
  RiskCaseDetail,
} from './types'

const props = defineProps<{
  show: boolean
  detail: RiskCaseDetail | null
  selectedUserIds: number[]
  initialAction?: RiskActionType
  stepUp: StepUpController
}>()

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'completed', value: RiskActionRecord): void
  (event: 'stale'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()
const previewing = ref(false)
const executing = ref(false)
const preview = ref<RiskActionPreview | null>(null)
const result = ref<RiskActionRecord | null>(null)
const permanentConfirmation = ref('')
const form = reactive({
  action_type: 'temporary_registration_block' as RiskActionType,
  duration_minutes: 30,
  reason: '',
})

const actionOptions = computed(() => [
  { value: 'observe', label: t('admin.ipRisk.actionTypes.observe') },
  { value: 'mark_shared_network', label: t('admin.ipRisk.actionTypes.mark_shared_network') },
  { value: 'allowlist', label: t('admin.ipRisk.actionTypes.allowlist') },
  { value: 'temporary_registration_block', label: t('admin.ipRisk.actionTypes.temporary_registration_block') },
  { value: 'permanent_registration_block', label: t('admin.ipRisk.actionTypes.permanent_registration_block') },
  { value: 'disable_api_keys', label: t('admin.ipRisk.actionTypes.disable_api_keys') },
  { value: 'disable_users', label: t('admin.ipRisk.actionTypes.disable_users') },
  { value: 'resolve', label: t('admin.ipRisk.actionTypes.resolve') },
  { value: 'ignore', label: t('admin.ipRisk.actionTypes.ignore') },
])

const durationOptions = computed(() => [
  { value: 30, label: t('admin.ipRisk.durations.30') },
  { value: 120, label: t('admin.ipRisk.durations.120') },
  { value: 1440, label: t('admin.ipRisk.durations.1440') },
  { value: 10080, label: t('admin.ipRisk.durations.10080') },
])

const actionNeedsUsers = computed(() =>
  form.action_type === 'disable_users' || form.action_type === 'disable_api_keys',
)

const canPreview = computed(() =>
  Boolean(
    props.detail &&
    form.reason.trim() &&
    (!actionNeedsUsers.value || props.selectedUserIds.length > 0) &&
    (
      form.action_type !== 'permanent_registration_block' ||
      permanentConfirmation.value === props.detail.case.primary_ip
    ),
  ),
)

const isDestructive = computed(() =>
  ['permanent_registration_block', 'disable_users', 'disable_api_keys'].includes(form.action_type),
)

const previewMetrics = computed(() => {
  if (!preview.value) return []
  return [
    { label: t('admin.ipRisk.actionDialog.users'), value: preview.value.user_count },
    { label: t('admin.ipRisk.actionDialog.keys'), value: preview.value.api_key_count },
    { label: t('admin.ipRisk.actionDialog.alreadyDisabled'), value: preview.value.already_disabled },
    { label: t('admin.ipRisk.actionDialog.protected'), value: preview.value.protected_users.length },
  ]
})

watch(
  () => props.show,
  (show) => {
    if (!show) return
    form.action_type = props.initialAction || 'temporary_registration_block'
    form.duration_minutes = 30
    form.reason = ''
    permanentConfirmation.value = ''
    preview.value = null
    result.value = null
  },
)

watch(
  () => props.selectedUserIds.join(','),
  () => resetPreview(),
)

function buildInput(): RiskActionInput {
  return {
    action_type: form.action_type,
    user_ids: [...props.selectedUserIds],
    api_key_ids: [],
    duration_minutes: form.action_type === 'temporary_registration_block' ? form.duration_minutes : 0,
    reason: form.reason.trim(),
  }
}

function resetPreview() {
  preview.value = null
  result.value = null
}

async function createPreview() {
  if (!props.detail || !canPreview.value) return
  previewing.value = true
  try {
    preview.value = await adminAPI.ipRisk.previewAction(props.detail.case.id, buildInput())
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.ipRisk.actionDialog.previewFailed')))
  } finally {
    previewing.value = false
  }
}

async function execute() {
  if (!props.detail || !preview.value) return
  executing.value = true
  try {
    const input = {
      ...buildInput(),
      preview_token: preview.value.confirmation_token,
    }
    const record = await props.stepUp.run(() =>
      adminAPI.ipRisk.executeAction(props.detail!.case.id, input),
    )
    result.value = record
    emit('completed', record)
    appStore.showSuccess(t('admin.ipRisk.actionDialog.executed'))
  } catch (error) {
    if (isStepUpCancelled(error)) return
    if (extractApiErrorCode(error) === 'risk_action_preview_stale') {
      preview.value = null
      emit('stale')
      appStore.showWarning(t('admin.ipRisk.actionDialog.previewStale'))
      return
    }
    appStore.showError(extractApiErrorMessage(error, t('admin.ipRisk.actionDialog.executeFailed')))
  } finally {
    executing.value = false
  }
}

function close() {
  if (previewing.value || executing.value) return
  emit('close')
}
</script>
