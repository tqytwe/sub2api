<template>
  <BaseDialog :show="show" :title="t('admin.users.balanceHistoryTitle')" width="wide" :close-on-click-outside="true" :z-index="40" @close="$emit('close')">
    <div v-if="user" class="space-y-4">
      <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-700">
        <div class="flex items-center gap-3">
          <div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full bg-primary-100 dark:bg-primary-900/30">
            <span class="text-lg font-medium text-primary-700 dark:text-primary-300">
              {{ user.email.charAt(0).toUpperCase() }}
            </span>
          </div>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <p class="truncate font-medium text-gray-900 dark:text-white">{{ user.email }}</p>
              <span v-if="user.deleted_at" class="flex-shrink-0 inline-flex items-center rounded px-1 py-px text-[10px] font-medium leading-tight bg-rose-100 text-rose-600 ring-1 ring-inset ring-rose-200 dark:bg-rose-500/20 dark:text-rose-400 dark:ring-rose-500/30">
                {{ t('admin.usage.userDeletedBadge') }}
              </span>
              <span
                v-if="user.username"
                class="flex-shrink-0 rounded bg-primary-50 px-1.5 py-0.5 text-xs text-primary-600 dark:bg-primary-900/20 dark:text-primary-400"
              >
                {{ user.username }}
              </span>
            </div>
            <p class="text-xs text-gray-400 dark:text-dark-500">
              {{ t('admin.users.createdAt') }}: {{ formatDateTime(user.created_at) }}
            </p>
          </div>
          <div class="flex-shrink-0 text-right">
            <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.users.currentBalance') }}</p>
            <p class="text-xl font-bold text-gray-900 dark:text-white">
              {{ formatMoney(summary.current_balance) }}
            </p>
          </div>
        </div>

        <div class="mt-3 grid grid-cols-2 gap-3 border-t border-gray-200/60 pt-3 text-xs dark:border-dark-600/60 sm:grid-cols-3 lg:grid-cols-6">
          <div>
            <p class="text-gray-500 dark:text-dark-400">{{ t('admin.users.frozenBalance') }}</p>
            <p class="mt-0.5 font-semibold text-gray-900 dark:text-white">{{ formatMoney(summary.frozen_balance) }}</p>
          </div>
          <div>
            <p class="text-gray-500 dark:text-dark-400">{{ t('admin.users.flowTotalIn') }}</p>
            <p class="mt-0.5 font-semibold text-emerald-600 dark:text-emerald-400">{{ formatMoney(summary.total_in) }}</p>
          </div>
          <div>
            <p class="text-gray-500 dark:text-dark-400">{{ t('admin.users.flowTotalOut') }}</p>
            <p class="mt-0.5 font-semibold text-red-600 dark:text-red-400">{{ formatMoney(summary.total_out) }}</p>
          </div>
          <div>
            <p class="text-gray-500 dark:text-dark-400">{{ t('admin.users.flowNetDelta') }}</p>
            <p :class="['mt-0.5 font-semibold', amountClass(summary.net_delta)]">{{ formatSignedMoney(summary.net_delta) }}</p>
          </div>
          <div>
            <p class="text-gray-500 dark:text-dark-400">{{ t('admin.users.totalRecharged') }}</p>
            <p class="mt-0.5 font-semibold text-emerald-600 dark:text-emerald-400">{{ formatMoney(summary.recharge_total) }}</p>
          </div>
          <div>
            <p class="text-gray-500 dark:text-dark-400">{{ t('admin.users.flowRows') }}</p>
            <p class="mt-0.5 font-semibold text-gray-900 dark:text-white">{{ total }}</p>
          </div>
        </div>

        <p class="mt-3 min-w-0 truncate border-t border-gray-200/60 pt-2.5 text-xs text-gray-500 dark:border-dark-600/60 dark:text-dark-400" :title="user.notes || ''">
          <template v-if="user.notes">{{ t('admin.users.notes') }}: {{ user.notes }}</template>
          <template v-else>&nbsp;</template>
        </p>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <Select
          v-model="typeFilter"
          :options="typeOptions"
          class="w-60"
          @change="loadHistory(1)"
        />
        <button
          v-if="!hideActions"
          @click="emit('deposit')"
          class="flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700"
        >
          <Icon name="plus" size="sm" class="text-emerald-500" :stroke-width="2" />
          {{ t('admin.users.deposit') }}
        </button>
        <button
          v-if="!hideActions"
          @click="emit('withdraw')"
          class="flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700"
        >
          <Icon name="arrowDown" size="sm" class="text-amber-500" :stroke-width="2" />
          {{ t('admin.users.withdraw') }}
        </button>
        <button
          :disabled="reconciling"
          class="flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700"
          @click="loadReconciliation"
        >
          <Icon name="sync" size="sm" class="text-primary-500" :stroke-width="2" />
          {{ t('admin.users.reconcileBalance') }}
        </button>
      </div>

      <div v-if="reconciliation" class="rounded-lg border border-gray-200 bg-white p-3 dark:border-dark-600 dark:bg-dark-800">
        <div class="grid grid-cols-2 gap-3 text-xs sm:grid-cols-3 lg:grid-cols-6">
          <div>
            <p class="text-gray-500 dark:text-dark-400">{{ t('admin.users.currentBalance') }}</p>
            <p class="mt-0.5 font-semibold text-gray-900 dark:text-white">{{ formatMoney(reconciliation.current_balance) }}</p>
          </div>
          <div>
            <p class="text-gray-500 dark:text-dark-400">{{ t('admin.users.frozenBalance') }}</p>
            <p class="mt-0.5 font-semibold text-gray-900 dark:text-white">{{ formatMoney(reconciliation.current_frozen) }}</p>
          </div>
          <div>
            <p class="text-gray-500 dark:text-dark-400">{{ t('admin.users.ledgerBalanceSum') }}</p>
            <p class="mt-0.5 font-semibold text-gray-900 dark:text-white">{{ formatMoney(reconciliation.ledger_balance_sum) }}</p>
          </div>
          <div>
            <p class="text-gray-500 dark:text-dark-400">{{ t('admin.users.ledgerFrozenSum') }}</p>
            <p class="mt-0.5 font-semibold text-gray-900 dark:text-white">{{ formatMoney(reconciliation.ledger_frozen_sum) }}</p>
          </div>
          <div>
            <p class="text-gray-500 dark:text-dark-400">{{ t('admin.users.balanceDifference') }}</p>
            <p :class="['mt-0.5 font-semibold', amountClass(reconciliation.balance_difference)]">{{ formatSignedMoney(reconciliation.balance_difference) }}</p>
          </div>
          <div>
            <p class="text-gray-500 dark:text-dark-400">{{ t('admin.users.frozenDifference') }}</p>
            <p :class="['mt-0.5 font-semibold', amountClass(reconciliation.frozen_difference)]">{{ formatSignedMoney(reconciliation.frozen_difference) }}</p>
          </div>
        </div>
        <div v-if="reconciliation.warnings?.length" class="mt-3 space-y-1 border-t border-gray-100 pt-2 text-xs text-amber-700 dark:border-dark-700 dark:text-amber-300">
          <p v-for="warning in reconciliation.warnings" :key="warning">{{ warning }}</p>
        </div>
      </div>

      <div v-if="loading" class="flex justify-center py-8">
        <svg class="h-8 w-8 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
        </svg>
      </div>

      <div v-else-if="history.length === 0 && subscriptionHistory.length === 0" class="py-8 text-center">
        <p class="text-sm text-gray-500">{{ t('admin.users.noBalanceHistory') }}</p>
      </div>

      <div v-else class="max-h-[30rem] overflow-y-auto">
        <template v-if="isSubscriptionFilter">
          <div class="space-y-3">
            <div
              v-for="item in subscriptionHistory"
              :key="`subscription-${item.id}`"
              class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800"
            >
              <div class="flex items-start justify-between gap-4">
                <div class="flex min-w-0 items-start gap-3">
                  <div class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-purple-100 dark:bg-purple-900/30">
                    <Icon name="badge" size="sm" class="text-purple-600 dark:text-purple-400" />
                  </div>
                  <div class="min-w-0">
                    <p class="truncate text-sm font-medium text-gray-900 dark:text-white">
                      {{ subscriptionGroupName(item) }}
                    </p>
                    <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
                      {{ t('admin.users.subscriptionValidity') }}:
                      {{ formatDateTime(item.starts_at) }} - {{ formatDateTime(item.expires_at) }}
                    </p>
                    <p
                      v-if="item.purchase_order"
                      class="mt-0.5 text-xs text-gray-500 dark:text-dark-400"
                    >
                      {{ t('admin.users.subscriptionOrder') }} #{{ item.purchase_order.id }}
                      <span v-if="item.purchase_order.out_trade_no">· {{ item.purchase_order.out_trade_no }}</span>
                    </p>
                    <p
                      v-if="item.purchase_order"
                      class="mt-0.5 text-xs text-gray-500 dark:text-dark-400"
                    >
                      {{ t('admin.users.subscriptionPayment') }}:
                      {{ formatPaymentMethod(item.purchase_order.payment_type) }}
                      · {{ formatSubscriptionPayAmount(item) }}
                      <span v-if="item.purchase_order.paid_at">
                        · {{ t('admin.users.subscriptionPaidAt') }} {{ formatDateTime(item.purchase_order.paid_at) }}
                      </span>
                    </p>
                    <p
                      v-if="item.purchase_order?.audit_action"
                      class="mt-0.5 text-xs text-gray-400 dark:text-dark-500"
                    >
                      {{ item.purchase_order.audit_action }}
                      <span v-if="item.purchase_order.audit_at">· {{ formatDateTime(item.purchase_order.audit_at) }}</span>
                    </p>
                    <p
                      v-else-if="item.notes"
                      class="mt-0.5 text-xs text-gray-400 dark:text-dark-500"
                      :title="item.notes"
                    >
                      {{ truncate(item.notes, 60) }}
                    </p>
                  </div>
                </div>
                <div class="flex-shrink-0 text-right">
                  <p class="text-sm font-semibold text-purple-600 dark:text-purple-400">
                    {{ formatSubscriptionDays(item) }}
                  </p>
                  <p class="text-xs text-gray-400 dark:text-dark-500">
                    {{ item.status }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </template>

        <div v-else class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
          <table class="min-w-full table-fixed divide-y divide-gray-200 text-sm dark:divide-dark-600">
            <thead class="bg-gray-50 text-xs uppercase tracking-normal text-gray-500 dark:bg-dark-700 dark:text-dark-400">
              <tr>
                <th class="w-36 px-4 py-3 text-left font-medium">{{ t('admin.users.flowColumnTime') }}</th>
                <th class="w-44 px-4 py-3 text-left font-medium">{{ t('admin.users.flowColumnType') }}</th>
                <th class="w-32 px-4 py-3 text-right font-medium">{{ t('admin.users.flowColumnAmount') }}</th>
                <th class="w-44 px-4 py-3 text-left font-medium">{{ t('admin.users.flowColumnBalance') }}</th>
                <th class="w-44 px-4 py-3 text-left font-medium">{{ t('admin.users.flowColumnSource') }}</th>
                <th class="w-28 px-4 py-3 text-left font-medium">{{ t('admin.users.flowColumnActor') }}</th>
                <th class="px-4 py-3 text-left font-medium">{{ t('admin.users.flowColumnNotes') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-800">
              <template v-for="item in history" :key="item.id">
                <tr class="align-top">
                  <td class="px-4 py-3 text-xs text-gray-500 dark:text-dark-400">
                    {{ formatDateTime(item.occurred_at) }}
                  </td>
                  <td class="px-4 py-3">
                    <div class="flex min-w-0 items-start gap-2">
                      <span :class="['mt-0.5 flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-lg', iconBgClass(item)]">
                        <Icon :name="iconName(item)" size="xs" :class="iconTextClass(item)" />
                      </span>
                      <div class="min-w-0">
                        <p class="truncate font-medium text-gray-900 dark:text-white">{{ flowTitle(item) }}</p>
                        <p class="mt-0.5 truncate text-xs text-gray-500 dark:text-dark-400">{{ item.description || item.source_type }}</p>
                      </div>
                    </div>
                  </td>
                  <td class="px-4 py-3 text-right">
                    <p :class="['font-semibold', amountClass(item.balance_delta)]">
                      {{ formatSignedMoney(item.balance_delta) }}
                    </p>
                    <p v-if="item.frozen_delta" class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                      {{ t('admin.users.frozenBalance') }} {{ formatSignedMoney(item.frozen_delta) }}
                    </p>
                  </td>
                  <td class="px-4 py-3 text-xs text-gray-600 dark:text-dark-300">
                    <p>{{ formatBalanceRange(item.balance_before, item.balance_after) }}</p>
                    <p v-if="item.frozen_delta" class="mt-1 text-gray-500 dark:text-dark-400">
                      {{ formatBalanceRange(item.frozen_before, item.frozen_after) }}
                    </p>
                  </td>
                  <td class="px-4 py-3 text-xs text-gray-600 dark:text-dark-300">
                    <p class="truncate">{{ relatedObject(item) }}</p>
                    <p v-if="item.reference" class="mt-1 truncate font-mono text-gray-400 dark:text-dark-500" :title="item.reference">
                      {{ item.reference }}
                    </p>
                  </td>
                  <td class="px-4 py-3 text-xs text-gray-600 dark:text-dark-300">
                    {{ actorLabel(item) }}
                  </td>
                  <td class="px-4 py-3">
                    <div class="flex items-start justify-between gap-2">
                      <p class="min-w-0 flex-1 truncate text-xs text-gray-500 dark:text-dark-400" :title="item.notes || item.confidence">
                        {{ item.notes || confidenceLabel(item.confidence) }}
                      </p>
                      <button
                        v-if="hasDetails(item)"
                        class="flex-shrink-0 rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-700 dark:hover:text-gray-200"
                        :title="t('admin.users.flowDetails')"
                        @click="toggleDetails(item.id)"
                      >
                        <Icon :name="expandedItemID === item.id ? 'chevronUp' : 'chevronDown'" size="xs" />
                      </button>
                    </div>
                  </td>
                </tr>
                <tr v-if="expandedItemID === item.id">
                  <td colspan="7" class="bg-gray-50 px-4 py-3 dark:bg-dark-700/60">
                    <div class="grid gap-2 text-xs text-gray-600 dark:text-dark-300 sm:grid-cols-2 lg:grid-cols-3">
                      <div v-if="item.related_object_type || item.related_object_id" class="min-w-0">
                        <span class="text-gray-400 dark:text-dark-500">{{ t('admin.users.flowRelatedObject') }}:</span>
                        <span class="ml-1 break-all">{{ relatedObject(item) }}</span>
                      </div>
                      <div v-if="item.source_type || item.source_id" class="min-w-0">
                        <span class="text-gray-400 dark:text-dark-500">{{ t('admin.users.flowSource') }}:</span>
                        <span class="ml-1 break-all">{{ item.source_type }}{{ item.source_id ? ` #${item.source_id}` : '' }}</span>
                      </div>
                      <div v-if="item.reference" class="min-w-0">
                        <span class="text-gray-400 dark:text-dark-500">{{ t('admin.users.flowReference') }}:</span>
                        <span class="ml-1 break-all font-mono">{{ item.reference }}</span>
                      </div>
                      <div
                        v-for="[key, value] in detailEntries(item)"
                        :key="`${item.id}-${key}`"
                        class="min-w-0"
                      >
                        <span class="text-gray-400 dark:text-dark-500">{{ key }}:</span>
                        <span class="ml-1 break-all">{{ formatDetailValue(value) }}</span>
                      </div>
                    </div>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </div>

      <div v-if="totalPages > 1" class="flex items-center justify-center gap-2 pt-2">
        <button
          :disabled="currentPage <= 1"
          class="btn btn-secondary px-3 py-1 text-sm"
          @click="loadHistory(currentPage - 1)"
        >
          {{ t('pagination.previous') }}
        </button>
        <span class="text-sm text-gray-500 dark:text-dark-400">
          {{ currentPage }} / {{ totalPages }}
        </span>
        <button
          :disabled="currentPage >= totalPages"
          class="btn btn-secondary px-3 py-1 text-sm"
          @click="loadHistory(currentPage + 1)"
        >
          {{ t('pagination.next') }}
        </button>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type BalanceFlowSummary, type BalanceHistoryItem, type BalanceReconciliationResponse } from '@/api/admin'
import { formatDateTime } from '@/utils/format'
import { currencySymbol } from '@/components/payment/currency'
import type { AdminUser, UserSubscription } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ show: boolean; user: AdminUser | null; hideActions?: boolean }>()
const emit = defineEmits(['close', 'deposit', 'withdraw'])
const { t } = useI18n()

const defaultSummary = (): BalanceFlowSummary => ({
  current_balance: props.user?.balance || 0,
  frozen_balance: props.user?.frozen_balance || 0,
  total_in: 0,
  total_out: 0,
  net_delta: 0,
  recharge_total: 0,
})

const history = ref<BalanceHistoryItem[]>([])
const subscriptionHistory = ref<UserSubscription[]>([])
const summary = ref<BalanceFlowSummary>(defaultSummary())
const reconciliation = ref<BalanceReconciliationResponse | null>(null)
const loading = ref(false)
const reconciling = ref(false)
const currentPage = ref(1)
const total = ref(0)
const pageSize = 15
const typeFilter = ref('')
const expandedItemID = ref<string | null>(null)

const isSubscriptionFilter = computed(() => typeFilter.value === 'subscription')
const totalPages = computed(() => isSubscriptionFilter.value ? 1 : Math.ceil(total.value / pageSize) || 1)

const typeOptions = computed(() => [
  { value: '', label: t('admin.users.allTypes') },
  { value: 'payment_recharge', label: t('admin.users.typePaymentRecharge') },
  { value: 'balance', label: t('admin.users.typeBalance') },
  { value: 'admin_balance', label: t('admin.users.typeAdminBalance') },
  { value: 'affiliate_balance', label: t('admin.users.typeAffiliateBalance') },
  { value: 'checkin', label: t('admin.users.typeCheckin') },
  { value: 'checkin_makeup', label: t('admin.users.typeCheckinMakeup') },
  { value: 'quiz', label: t('admin.users.typeQuiz') },
  { value: 'blindbox', label: t('admin.users.typeBlindbox') },
  { value: 'team_shared_reward', label: t('admin.users.typeTeamSharedReward') },
  { value: 'arena_settlement', label: t('admin.users.typeArenaSettlement') },
  { value: 'arena_daily_settlement', label: t('admin.users.typeArenaDailySettlement') },
  { value: 'usage_charge', label: t('admin.users.typeUsageCharge') },
  { value: 'refund', label: t('admin.users.typeRefund') },
  { value: 'promo_bonus', label: t('admin.users.typePromoBonus') },
  { value: 'concurrency', label: t('admin.users.typeConcurrency') },
  { value: 'admin_concurrency', label: t('admin.users.typeAdminConcurrency') },
  { value: 'subscription', label: t('admin.users.typeSubscription') },
])

watch(() => props.show, (v) => {
  if (v && props.user) {
    typeFilter.value = ''
    summary.value = defaultSummary()
    reconciliation.value = null
    expandedItemID.value = null
    loadHistory(1)
  }
})

const loadHistory = async (page: number) => {
  if (!props.user) return
  loading.value = true
  currentPage.value = page
  expandedItemID.value = null
  try {
    if (isSubscriptionFilter.value) {
      const res = await adminAPI.subscriptions.listByUser(props.user.id)
      subscriptionHistory.value = res || []
      history.value = []
      total.value = subscriptionHistory.value.length
      return
    }

    subscriptionHistory.value = []
    const res = await adminAPI.users.getUserBalanceHistory(
      props.user.id,
      page,
      pageSize,
      typeFilter.value || undefined
    )
    history.value = res.items || []
    total.value = res.total || 0
    summary.value = res.summary || defaultSummary()
  } catch (error) {
    console.error('Failed to load balance flow:', error)
  } finally {
    loading.value = false
  }
}

const loadReconciliation = async () => {
  if (!props.user) return
  reconciling.value = true
  try {
    reconciliation.value = await adminAPI.users.getUserBalanceReconciliation(props.user.id)
  } catch (error) {
    console.error('Failed to load balance reconciliation:', error)
  } finally {
    reconciling.value = false
  }
}

const subscriptionGroupName = (item: UserSubscription) => item.group?.name || `#${item.group_id}`

const formatPaymentMethod = (paymentType?: string) => {
  if (!paymentType) return '-'
  return t(`payment.methods.${paymentType}`, paymentType)
}

const formatSubscriptionPayAmount = (item: UserSubscription) => {
  const order = item.purchase_order
  if (!order) return '-'
  return `${currencySymbol(order.currency)}${Number(order.pay_amount || 0).toFixed(2)}`
}

const formatSubscriptionDays = (item: UserSubscription) => {
  const days = item.purchase_order?.subscription_days || calculateSubscriptionDays(item.starts_at, item.expires_at)
  return days > 0 ? t('admin.users.subscriptionDays', { days }) : '-'
}

const calculateSubscriptionDays = (startsAt?: string | null, expiresAt?: string | null) => {
  if (!startsAt || !expiresAt) return 0
  const start = new Date(startsAt).getTime()
  const end = new Date(expiresAt).getTime()
  if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) return 0
  return Math.round((end - start) / (24 * 60 * 60 * 1000))
}

const typeLabelKeys: Record<string, string> = {
  payment_recharge: 'admin.users.typePaymentRecharge',
  balance: 'admin.users.typeBalance',
  admin_balance: 'admin.users.typeAdminBalance',
  affiliate_balance: 'admin.users.typeAffiliateBalance',
  checkin: 'admin.users.typeCheckin',
  checkin_makeup: 'admin.users.typeCheckinMakeup',
  checkin_milestone: 'admin.users.typeCheckinMilestone',
  quiz: 'admin.users.typeQuiz',
  blindbox: 'admin.users.typeBlindbox',
  team_shared_reward: 'admin.users.typeTeamSharedReward',
  arena_settlement: 'admin.users.typeArenaSettlement',
  arena_daily_settlement: 'admin.users.typeArenaDailySettlement',
  usage_charge: 'admin.users.typeUsageCharge',
  refund: 'admin.users.typeRefund',
  reversal: 'admin.users.typeReversal',
  promo_bonus: 'admin.users.typePromoBonus',
  concurrency: 'admin.users.typeConcurrency',
  admin_concurrency: 'admin.users.typeAdminConcurrency',
}

const flowTitle = (item: BalanceHistoryItem) => {
  const key = typeLabelKeys[item.type]
  return key ? t(key) : item.type || t('common.unknown')
}

const formatMoney = (value?: number | null) => {
  const n = Number(value || 0)
  return `$${n.toFixed(2)}`
}

const formatSignedMoney = (value?: number | null) => {
  const n = Number(value || 0)
  const sign = n > 0 ? '+' : ''
  return `${sign}$${n.toFixed(2)}`
}

const amountClass = (value?: number | null) => {
  const n = Number(value || 0)
  if (n > 0) return 'text-emerald-600 dark:text-emerald-400'
  if (n < 0) return 'text-red-600 dark:text-red-400'
  return 'text-gray-600 dark:text-dark-300'
}

const formatBalanceRange = (before?: number | null, after?: number | null) => {
  if (before == null && after == null) return '-'
  return `${before == null ? '-' : formatMoney(before)} -> ${after == null ? '-' : formatMoney(after)}`
}

const relatedObject = (item: BalanceHistoryItem) => {
  const type = item.related_object_type || item.source_type || '-'
  const id = item.related_object_id || item.source_id
  return id ? `${type} #${id}` : type
}

const actorLabel = (item: BalanceHistoryItem) => {
  const actor = item.actor_type || 'system'
  const label = t(`admin.users.actor_${actor}`, actor)
  return item.actor_user_id ? `${label} #${item.actor_user_id}` : label
}

const confidenceLabel = (confidence?: string) => {
  if (!confidence || confidence === 'high') return ''
  return t(`admin.users.confidence_${confidence}`, confidence)
}

const iconName = (item: BalanceHistoryItem) => {
  if (item.type === 'payment_recharge' || item.type === 'balance' || item.type === 'admin_balance' || item.type === 'affiliate_balance') return 'dollar'
  if (item.type === 'usage_charge' || item.type === 'refund') return 'arrowDown'
  if (item.type === 'blindbox' || item.type === 'promo_bonus') return 'gift'
  if (item.type === 'team_shared_reward') return 'users'
  if (item.type.includes('arena')) return 'badge'
  if (item.type === 'quiz') return 'questionCircle'
  return 'calendar'
}

const iconBgClass = (item: BalanceHistoryItem) => {
  const n = Number(item.balance_delta || 0)
  if (item.type === 'blindbox') return 'bg-amber-100 dark:bg-amber-900/30'
  if (n > 0) return 'bg-emerald-100 dark:bg-emerald-900/30'
  if (n < 0) return 'bg-red-100 dark:bg-red-900/30'
  return 'bg-gray-100 dark:bg-dark-600'
}

const iconTextClass = (item: BalanceHistoryItem) => {
  const n = Number(item.balance_delta || 0)
  if (item.type === 'blindbox') return 'text-amber-600 dark:text-amber-400'
  if (n > 0) return 'text-emerald-600 dark:text-emerald-400'
  if (n < 0) return 'text-red-600 dark:text-red-400'
  return 'text-gray-500 dark:text-dark-300'
}

const hasDetails = (item: BalanceHistoryItem) => {
  return !!(item.reference || item.source_id || item.related_object_id || Object.keys(item.metadata || {}).length)
}

const detailEntries = (item: BalanceHistoryItem) => {
  return Object.entries(item.metadata || {}).filter(([, value]) => value !== null && value !== undefined && value !== '')
}

const formatDetailValue = (value: unknown) => {
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  return JSON.stringify(value)
}

const toggleDetails = (id: string) => {
  expandedItemID.value = expandedItemID.value === id ? null : id
}

const truncate = (value: string, max: number) => {
  return value.length > max ? `${value.substring(0, max - 5)}...` : value
}
</script>
