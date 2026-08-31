<template>
  <section class="space-y-4" :aria-busy="loading">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('admin.playOps.growthGovernance.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.playOps.growthGovernance.description') }}
        </p>
      </div>
      <button
        type="button"
        class="btn btn-secondary inline-flex items-center justify-center gap-2 self-start"
        data-testid="growth-governance-refresh"
        :disabled="loading || submitting"
        @click="load"
      >
        <Icon name="refresh" size="sm" />
        {{ t('admin.playOps.growthGovernance.refresh') }}
      </button>
    </div>

    <div
      v-if="error"
      class="flex flex-col gap-3 border border-red-200 bg-red-50 p-4 text-sm text-red-800 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-100 sm:flex-row sm:items-center sm:justify-between"
      data-testid="growth-governance-error"
      role="alert"
    >
      <span>{{ error }}</span>
      <button
        type="button"
        class="btn btn-secondary shrink-0"
        data-testid="growth-governance-retry"
        :disabled="loading"
        @click="load"
      >
        {{ t('admin.playOps.growthGovernance.retry') }}
      </button>
    </div>

    <section class="card overflow-hidden" aria-labelledby="growth-governance-cohort-title">
      <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 sm:flex-row sm:items-start sm:justify-between dark:border-dark-700">
        <div>
          <h3 id="growth-governance-cohort-title" class="text-base font-semibold text-gray-900 dark:text-white">
            {{ t('admin.playOps.growthGovernance.cohort.title') }}
          </h3>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ cohortWindowLabel }}
          </p>
        </div>
        <span
          class="inline-flex items-center gap-1.5 self-start rounded-full px-2.5 py-1 text-xs font-medium"
          :class="cohortComplete ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-100' : 'bg-amber-100 text-amber-900 dark:bg-amber-900/30 dark:text-amber-100'"
          data-testid="growth-governance-cohort-status"
        >
          <Icon :name="cohortComplete ? 'checkCircle' : 'exclamationTriangle'" size="xs" />
          {{ cohortComplete ? t('admin.playOps.growthGovernance.cohort.available') : t('admin.playOps.growthGovernance.cohort.incomplete') }}
        </span>
      </div>

      <div v-if="loading && !cohort" class="px-5 py-8 text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.playOps.growthGovernance.loading') }}
      </div>
      <div v-else-if="cohort" class="p-5">
        <div class="grid border-l border-t border-gray-200 sm:grid-cols-2 xl:grid-cols-4 dark:border-dark-700">
          <div v-for="item in cohortSummary" :key="item.key" class="min-h-24 border-b border-r border-gray-200 p-4 dark:border-dark-700">
            <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ item.label }}</p>
            <p class="mt-2 text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ item.value }}</p>
            <p v-if="item.detail" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ item.detail }}</p>
          </div>
        </div>
        <p class="mt-3 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.playOps.growthGovernance.cohort.lagHint') }}
        </p>
      </div>
      <div v-else class="px-5 py-8 text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.playOps.growthGovernance.empty.cohort') }}
      </div>
    </section>

    <div class="grid gap-4 xl:grid-cols-2">
      <section class="card overflow-hidden" aria-labelledby="growth-governance-evidence-title">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h3 id="growth-governance-evidence-title" class="text-base font-semibold text-gray-900 dark:text-white">
            {{ t('admin.playOps.growthGovernance.evidence.title') }}
          </h3>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.playOps.growthGovernance.evidence.description') }}
          </p>
        </div>
        <div v-if="cohort" class="p-5">
          <div class="grid border-l border-t border-gray-200 sm:grid-cols-2 dark:border-dark-700">
            <div v-for="metric in metricRows" :key="metric.key" class="min-h-24 border-b border-r border-gray-200 p-3 dark:border-dark-700">
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ metric.label }}</p>
              <p class="mt-1 text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ metric.value }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ metric.detail }}</p>
            </div>
          </div>
          <div
            v-if="!cohortComplete"
            class="mt-4 border-l-4 border-amber-500 bg-amber-50 px-4 py-3 text-sm text-amber-950 dark:border-amber-400 dark:bg-amber-950/30 dark:text-amber-100"
            data-testid="growth-governance-blocked-notice"
          >
            <p class="font-medium">{{ t('admin.playOps.growthGovernance.cohort.blockedTitle') }}</p>
            <p class="mt-1">{{ t('admin.playOps.growthGovernance.cohort.blockedDescription') }}</p>
            <ul v-if="unavailableMetricLabels.length" class="mt-2 list-disc space-y-1 pl-5 text-xs">
              <li v-for="metric in unavailableMetricLabels" :key="metric">{{ metric }}</li>
            </ul>
          </div>
        </div>
        <div v-else class="px-5 py-8 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.playOps.growthGovernance.empty.evidence') }}
        </div>
      </section>

      <section class="card overflow-hidden" aria-labelledby="growth-governance-status-title">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 sm:flex-row sm:items-start sm:justify-between dark:border-dark-700">
          <div>
            <h3 id="growth-governance-status-title" class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('admin.playOps.growthGovernance.status.title') }}
            </h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ governanceDescription }}</p>
          </div>
          <span
            class="inline-flex items-center gap-1.5 self-start rounded-full px-2.5 py-1 text-xs font-medium"
            :class="governanceStatusClass"
            data-testid="growth-governance-status"
          >
            <Icon :name="governance?.approved ? 'checkCircle' : governance?.decision === 'revoked' ? 'xCircle' : 'exclamationTriangle'" size="xs" />
            {{ governanceLabel }}
          </span>
        </div>
        <div v-if="governance" class="p-5">
          <dl class="divide-y divide-gray-100 text-sm dark:divide-dark-700">
            <div class="flex items-baseline justify-between gap-4 py-3 first:pt-0">
              <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.playOps.growthGovernance.status.budgetAmount') }}</dt>
              <dd class="font-medium tabular-nums text-gray-900 dark:text-white">{{ governanceBudgetValue(governance.budget_amount) }}</dd>
            </div>
            <div class="flex items-baseline justify-between gap-4 py-3">
              <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.playOps.growthGovernance.status.budgetSpent') }}</dt>
              <dd class="font-medium tabular-nums text-gray-900 dark:text-white">{{ governanceBudgetValue(governance.budget_spent) }}</dd>
            </div>
            <div class="flex items-baseline justify-between gap-4 py-3">
              <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.playOps.growthGovernance.status.budgetRemaining') }}</dt>
              <dd class="font-medium tabular-nums text-gray-900 dark:text-white">{{ governanceBudgetValue(governance.budget_remaining) }}</dd>
            </div>
            <div class="py-3">
              <div class="flex items-baseline justify-between gap-4">
                <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.playOps.growthGovernance.status.rollout') }}</dt>
                <dd class="font-medium tabular-nums text-gray-900 dark:text-white">{{ isLatestApproval ? formatPercent(governance.rollout_percent / 100) : '—' }}</dd>
              </div>
              <div v-if="isLatestApproval" class="mt-2 h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700" role="progressbar" :aria-valuemin="0" :aria-valuemax="100" :aria-valuenow="governance.rollout_percent">
                <div class="h-full bg-primary-600 dark:bg-primary-400" :style="{ width: `${Math.max(0, Math.min(100, governance.rollout_percent || 0))}%` }"></div>
              </div>
            </div>
            <div v-if="governance.rule_version" class="flex items-baseline justify-between gap-4 py-3">
              <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.playOps.growthGovernance.status.ruleVersion') }}</dt>
              <dd class="font-medium tabular-nums text-gray-900 dark:text-white">{{ governance.rule_version }}</dd>
            </div>
            <div v-if="governance.created_at" class="flex items-baseline justify-between gap-4 py-3">
              <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.playOps.growthGovernance.status.decidedAt') }}</dt>
              <dd class="text-right font-medium text-gray-900 dark:text-white">{{ formatDateTime(governance.created_at) }}</dd>
            </div>
          </dl>
          <div v-if="governance.reason" class="mt-4 border-l-4 border-gray-300 bg-gray-50 px-4 py-3 text-sm text-gray-700 dark:border-dark-500 dark:bg-dark-800 dark:text-gray-200">
            <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.playOps.growthGovernance.status.reason') }}</p>
            <p class="mt-1 whitespace-pre-wrap">{{ governance.reason }}</p>
          </div>
          <div class="mt-5 flex flex-col gap-2 sm:flex-row sm:justify-end">
            <button
              type="button"
              class="btn btn-secondary"
              data-testid="growth-governance-approval-open"
              :disabled="!canOpenApproval"
              :title="approvalDisabledReason || undefined"
              @click="openApproval"
            >
              {{ t('admin.playOps.growthGovernance.actions.approve') }}
            </button>
            <button
              type="button"
              class="btn btn-danger"
              data-testid="growth-governance-revoke-open"
              :disabled="!canOpenRevoke"
              @click="openRevoke"
            >
              {{ t('admin.playOps.growthGovernance.actions.revoke') }}
            </button>
          </div>
          <p v-if="approvalDisabledReason" class="mt-3 text-xs text-gray-500 dark:text-gray-400">{{ approvalDisabledReason }}</p>
        </div>
        <div v-else class="px-5 py-8 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.playOps.growthGovernance.empty.governance') }}
        </div>
      </section>
    </div>

    <div
      v-if="actionError"
      class="border border-red-200 bg-red-50 p-4 text-sm text-red-800 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-100"
      data-testid="growth-governance-action-error"
      role="alert"
    >
      {{ actionError }}
    </div>

    <BaseDialog
      :show="approvalDialogOpen"
      :title="t('admin.playOps.growthGovernance.approve.title')"
      width="normal"
      @close="closeApproval"
    >
      <form
        id="growth-governance-approval-form"
        class="space-y-4"
        data-testid="growth-governance-approval-dialog"
        @submit.prevent="submitApproval"
      >
        <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('admin.playOps.growthGovernance.approve.description') }}</p>
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="grid gap-1.5 text-sm font-medium text-gray-700 dark:text-gray-200">
            <span>{{ t('admin.playOps.growthGovernance.form.budget') }}</span>
            <input
              v-model="approvalBudget"
              class="input tabular-nums"
              data-testid="growth-governance-budget"
              inputmode="decimal"
              min="0.01"
              required
              step="0.01"
              type="number"
            >
          </label>
          <label class="grid gap-1.5 text-sm font-medium text-gray-700 dark:text-gray-200">
            <span>{{ t('admin.playOps.growthGovernance.form.rollout') }}</span>
            <input
              v-model="approvalRollout"
              class="input tabular-nums"
              data-testid="growth-governance-rollout"
              max="20"
              min="10"
              required
              step="1"
              type="number"
            >
          </label>
        </div>
        <label class="grid gap-1.5 text-sm font-medium text-gray-700 dark:text-gray-200">
          <span>{{ t('admin.playOps.growthGovernance.form.reason') }}</span>
          <textarea
            v-model="approvalReason"
            class="input min-h-28"
            data-testid="growth-governance-approval-reason"
            maxlength="500"
            minlength="10"
            required
            rows="4"
          ></textarea>
          <span class="text-xs font-normal text-gray-500 dark:text-gray-400">{{ t('admin.playOps.growthGovernance.form.reasonHint', { count: approvalReasonLength }) }}</span>
        </label>
        <p class="border-l-4 border-amber-500 bg-amber-50 px-3 py-2 text-xs text-amber-950 dark:border-amber-400 dark:bg-amber-950/30 dark:text-amber-100">
          {{ t('admin.playOps.growthGovernance.approve.budgetUnitHint') }}
        </p>
      </form>
      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="submitting" @click="closeApproval">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          class="btn btn-primary"
          data-testid="growth-governance-approval-submit"
          form="growth-governance-approval-form"
          :disabled="!canSubmitApproval"
        >
          {{ submitting ? t('admin.playOps.growthGovernance.actions.submitting') : t('admin.playOps.growthGovernance.approve.submit') }}
        </button>
      </template>
    </BaseDialog>

    <BaseDialog
      :show="revokeDialogOpen"
      :title="t('admin.playOps.growthGovernance.revoke.title')"
      width="narrow"
      @close="closeRevoke"
    >
      <form
        id="growth-governance-revoke-form"
        class="space-y-4"
        data-testid="growth-governance-revoke-dialog"
        @submit.prevent="submitRevoke"
      >
        <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('admin.playOps.growthGovernance.revoke.description') }}</p>
        <label class="grid gap-1.5 text-sm font-medium text-gray-700 dark:text-gray-200">
          <span>{{ t('admin.playOps.growthGovernance.form.reason') }}</span>
          <textarea
            v-model="revokeReason"
            class="input min-h-28"
            data-testid="growth-governance-revoke-reason"
            maxlength="500"
            minlength="10"
            required
            rows="4"
          ></textarea>
          <span class="text-xs font-normal text-gray-500 dark:text-gray-400">{{ t('admin.playOps.growthGovernance.form.reasonHint', { count: revokeReasonLength }) }}</span>
        </label>
      </form>
      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="submitting" @click="closeRevoke">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          class="btn btn-danger"
          data-testid="growth-governance-revoke-submit"
          form="growth-governance-revoke-form"
          :disabled="!canSubmitRevoke"
        >
          {{ submitting ? t('admin.playOps.growthGovernance.actions.submitting') : t('admin.playOps.growthGovernance.revoke.submit') }}
        </button>
      </template>
    </BaseDialog>

    <TotpStepUpDialog :controller="stepUp" />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import adminPlayAPI, {
  type AdminPlayGrowthCohort,
  type AdminPlayGrowthGovernanceState,
} from '@/api/admin/play'
import { useAppStore } from '@/stores'
import { extractI18nErrorMessage } from '@/utils/apiError'
import {
  isStepUpBlocked,
  isStepUpCancelled,
  stepUpBlockReason,
  useStepUp,
} from '@/composables/useStepUp'

const { t, locale } = useI18n()
const appStore = useAppStore()
const stepUp = useStepUp()

const loading = ref(false)
const submitting = ref(false)
const error = ref('')
const actionError = ref('')
const cohort = ref<AdminPlayGrowthCohort | null>(null)
const governance = ref<AdminPlayGrowthGovernanceState | null>(null)
const approvalDialogOpen = ref(false)
const revokeDialogOpen = ref(false)
const approvalBudget = ref('')
const approvalRollout = ref('10')
const approvalReason = ref('')
const revokeReason = ref('')
let loadGeneration = 0

const ratioMetricNames: Record<string, string> = {
  abnormal_redemption_ratio: 'abnormalRedemption',
  appeal_false_positive_ratio: 'appealFalsePositive',
}

const cohortComplete = computed(() => {
  const value = cohort.value
  return Boolean(
    value &&
      value.metrics_available &&
      value.abnormal_redemption_ratio !== null &&
      value.appeal_false_positive_ratio !== null &&
      !value.unavailable_metrics?.length,
  )
})

const cohortWindowLabel = computed(() => {
  if (!cohort.value?.window_start || !cohort.value?.window_end) {
    return t('admin.playOps.growthGovernance.cohort.windowUnavailable')
  }
  return t('admin.playOps.growthGovernance.cohort.window', {
    start: formatDateTime(cohort.value.window_start),
    end: formatDateTime(cohort.value.window_end),
  })
})

const cohortSummary = computed(() => {
  const value = cohort.value
  if (!value) return []
  return [
    {
      key: 'participation',
      label: t('admin.playOps.growthGovernance.metrics.participation'),
      value: formatNumber(value.participation_users),
      detail: '',
    },
    {
      key: 'realCall7d',
      label: t('admin.playOps.growthGovernance.metrics.realCall7d'),
      value: formatPercent(value.real_call_7d_ratio),
      detail: formatFraction(value.real_call_7d_users, value.participation_users),
    },
    {
      key: 'firstRecharge',
      label: t('admin.playOps.growthGovernance.metrics.firstRecharge'),
      value: formatPercent(value.first_recharge_ratio),
      detail: formatFraction(value.first_recharge_users, value.participation_users),
    },
    {
      key: 'actualRewardCost',
      label: t('admin.playOps.growthGovernance.metrics.actualRewardCost'),
      value: formatDecimal(value.actual_reward_cost),
      detail: t('admin.playOps.growthGovernance.metrics.actualRewardCostHint'),
    },
  ]
})

const metricRows = computed(() => {
  const value = cohort.value
  if (!value) return []
  return [
    ratioMetric('realCall30d', value.real_call_30d_ratio, value.real_call_30d_users, value.participation_users),
    ratioMetric('couponRedemption', value.coupon_redemption_ratio, value.coupons_redeemed, value.coupons_issued),
    ratioMetric('d7Retention', value.d7_retention_ratio, value.d7_retained_users, value.participation_users),
    ratioMetric('abnormalRedemption', value.abnormal_redemption_ratio, value.abnormal_redemption_users, value.participation_users),
    ratioMetric('appealFalsePositive', value.appeal_false_positive_ratio, value.false_positive_appeals, value.appeal_count),
  ]
})

const unavailableMetricLabels = computed(() => {
  const unavailable = cohort.value?.unavailable_metrics || []
  return unavailable.map((metric) => {
    const key = ratioMetricNames[metric]
    return key
      ? t(`admin.playOps.growthGovernance.metrics.${key}`)
      : metric
  })
})

const isLatestApproval = computed(() => governance.value?.decision === 'approved')

const governanceLabel = computed(() => {
  const decision = governance.value?.decision
  if (decision === 'approved' && !governance.value?.approved) {
    return t('admin.playOps.growthGovernance.status.approvalInactive')
  }
  return t(`admin.playOps.growthGovernance.status.${decision === 'approved' || decision === 'revoked' ? decision : 'none'}`)
})

const governanceDescription = computed(() => {
  if (isLatestApproval.value && !governance.value?.approved) {
    return t('admin.playOps.growthGovernance.status.approvalInactiveDescription')
  }
  if (!isLatestApproval.value) {
    return t('admin.playOps.growthGovernance.status.closedDescription')
  }
  return t('admin.playOps.growthGovernance.status.approvedDescription')
})

const governanceStatusClass = computed(() => {
  if (governance.value?.approved) {
    return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-100'
  }
  if (isLatestApproval.value) {
    return 'bg-amber-100 text-amber-900 dark:bg-amber-900/30 dark:text-amber-100'
  }
  if (governance.value?.decision === 'revoked') {
    return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-100'
  }
  return 'bg-amber-100 text-amber-900 dark:bg-amber-900/30 dark:text-amber-100'
})

const approvalReasonLength = computed(() => Array.from(approvalReason.value.trim()).length)
const revokeReasonLength = computed(() => Array.from(revokeReason.value.trim()).length)
const approvalBudgetValue = computed(() => Number(approvalBudget.value))
const approvalRolloutValue = computed(() => Number(approvalRollout.value))

const canOpenApproval = computed(() => Boolean(
  cohortComplete.value &&
    governance.value &&
    !isLatestApproval.value &&
    !loading.value &&
    !submitting.value,
))

const canOpenRevoke = computed(() => Boolean(
  isLatestApproval.value && !loading.value && !submitting.value,
))

const approvalDisabledReason = computed(() => {
  if (loading.value) return t('admin.playOps.growthGovernance.status.loading')
  if (!cohort.value) return t('admin.playOps.growthGovernance.status.noCohort')
  if (!cohortComplete.value) return t('admin.playOps.growthGovernance.status.incompleteCohort')
  if (isLatestApproval.value) return t('admin.playOps.growthGovernance.status.alreadyApproved')
  return ''
})

const canSubmitApproval = computed(() => (
  canOpenApproval.value &&
  Number.isFinite(approvalBudgetValue.value) && approvalBudgetValue.value > 0 &&
  Number.isInteger(approvalRolloutValue.value) && approvalRolloutValue.value >= 10 && approvalRolloutValue.value <= 20 &&
  approvalReasonLength.value >= 10 && approvalReasonLength.value <= 500
))

const canSubmitRevoke = computed(() => (
  canOpenRevoke.value && revokeReasonLength.value >= 10 && revokeReasonLength.value <= 500
))

function ratioMetric(
  key: string,
  ratio: number | null,
  numerator: number,
  denominator: number,
) {
  const unavailable = ratio === null
  return {
    key,
    label: t(`admin.playOps.growthGovernance.metrics.${key}`),
    value: unavailable ? t('admin.playOps.growthGovernance.metrics.unavailable') : formatPercent(ratio),
    detail: unavailable
      ? t('admin.playOps.growthGovernance.metrics.unavailableHint')
      : formatFraction(numerator, denominator),
  }
}

function formatNumber(value?: number) {
  return new Intl.NumberFormat(locale.value).format(value || 0)
}

function formatDecimal(value?: number) {
  return new Intl.NumberFormat(locale.value, { maximumFractionDigits: 2 }).format(value || 0)
}

function governanceBudgetValue(value: number) {
  return isLatestApproval.value ? formatDecimal(value) : '—'
}

function formatPercent(value: number | null) {
  if (value === null || !Number.isFinite(value)) {
    return t('admin.playOps.growthGovernance.metrics.unavailable')
  }
  return new Intl.NumberFormat(locale.value, {
    style: 'percent',
    maximumFractionDigits: 1,
  }).format(value)
}

function formatFraction(numerator: number, denominator: number) {
  return `${formatNumber(numerator)} / ${formatNumber(denominator)}`
}

function formatDateTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString(locale.value, {
    dateStyle: 'medium',
    timeStyle: 'short',
    timeZone: 'Asia/Shanghai',
  })
}

async function load() {
  const generation = ++loadGeneration
  loading.value = true
  error.value = ''
  try {
    const [nextCohort, nextGovernance] = await Promise.all([
      adminPlayAPI.getGrowthCohort(),
      adminPlayAPI.getGrowthGovernance(),
    ])
    if (generation !== loadGeneration) return
    cohort.value = nextCohort
    governance.value = nextGovernance
  } catch (cause) {
    if (generation !== loadGeneration) return
    error.value = extractI18nErrorMessage(
      cause,
      t,
      'admin.playOps.growthGovernance.errors',
      t('admin.playOps.growthGovernance.loadFailed'),
    )
  } finally {
    if (generation === loadGeneration) loading.value = false
  }
}

function openApproval() {
  if (!canOpenApproval.value) return
  actionError.value = ''
  approvalBudget.value = ''
  approvalRollout.value = '10'
  approvalReason.value = ''
  approvalDialogOpen.value = true
}

function closeApproval() {
  if (submitting.value) return
  approvalDialogOpen.value = false
}

function openRevoke() {
  if (!canOpenRevoke.value) return
  actionError.value = ''
  revokeReason.value = ''
  revokeDialogOpen.value = true
}

function closeRevoke() {
  if (submitting.value) return
  revokeDialogOpen.value = false
}

async function submitApproval() {
  if (!canSubmitApproval.value || !cohort.value) return
  submitting.value = true
  actionError.value = ''
  try {
    const next = await stepUp.run(() => adminPlayAPI.approveGrowthGovernance({
      budget_amount: approvalBudgetValue.value,
      rollout_percent: approvalRolloutValue.value,
      cohort: cohort.value!,
      reason: approvalReason.value.trim(),
    }))
    governance.value = next
    approvalDialogOpen.value = false
    appStore.showSuccess(t('admin.playOps.growthGovernance.success.approved'))
    await load()
  } catch (cause) {
    handleActionError(cause, 'admin.playOps.growthGovernance.approve.failed')
  } finally {
    submitting.value = false
  }
}

async function submitRevoke() {
  if (!canSubmitRevoke.value) return
  submitting.value = true
  actionError.value = ''
  try {
    const next = await stepUp.run(() => adminPlayAPI.revokeGrowthGovernance({
      reason: revokeReason.value.trim(),
    }))
    governance.value = next
    revokeDialogOpen.value = false
    appStore.showSuccess(t('admin.playOps.growthGovernance.success.revoked'))
    await load()
  } catch (cause) {
    handleActionError(cause, 'admin.playOps.growthGovernance.revoke.failed')
  } finally {
    submitting.value = false
  }
}

function handleActionError(cause: unknown, fallbackKey: string) {
  if (isStepUpCancelled(cause)) return
  if (isStepUpBlocked(cause)) {
    actionError.value = stepUpBlockReason(cause) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN'
      ? t('stepUp.adminApiKeyForbidden')
      : t('stepUp.notEnabled')
    return
  }
  actionError.value = extractI18nErrorMessage(
    cause,
    t,
    'admin.playOps.growthGovernance.errors',
    t(fallbackKey),
  )
}

onMounted(() => {
  void load()
})
</script>
