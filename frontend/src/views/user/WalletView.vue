<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('wallet.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('wallet.description') }}</p>
        </div>
        <button type="button" class="btn btn-secondary inline-flex items-center gap-2 self-start" :disabled="loading" @click="loadWallet">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          {{ t('common.refresh') }}
        </button>
      </div>

      <div v-if="loadError" class="rounded border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-200">
        {{ t('wallet.loadFailed') }}
      </div>

      <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4 2xl:grid-cols-6">
        <section v-for="card in summaryCards" :key="card.label" class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ card.label }}</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ card.value }}</p>
        </section>
      </div>

      <div class="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(360px,420px)]">
        <section class="card">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('wallet.withdrawals.requestTitle') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ withdrawalAvailabilityText }}</p>
          </div>
          <div class="grid gap-4 p-5 md:grid-cols-2">
            <div class="rounded border border-gray-100 p-4 dark:border-dark-700">
              <div class="flex items-center justify-between gap-3">
                <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('wallet.withdrawals.payoutAccount') }}</p>
                <span v-if="payoutAccount" class="rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-200">
                  {{ t('wallet.withdrawals.accountReady') }}
                </span>
                <span v-else class="rounded-full bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700 dark:bg-amber-950/40 dark:text-amber-200">
                  {{ t('wallet.withdrawals.accountMissing') }}
                </span>
              </div>
              <dl v-if="payoutAccount" class="mt-3 grid gap-2 text-sm">
                <div class="flex justify-between gap-3">
                  <dt class="text-gray-500 dark:text-gray-400">{{ t('wallet.withdrawals.method') }}</dt>
                  <dd class="text-gray-900 dark:text-white">{{ payoutMethodLabel(payoutAccount.method) }}</dd>
                </div>
                <div class="flex justify-between gap-3">
                  <dt class="text-gray-500 dark:text-gray-400">{{ t('wallet.withdrawals.accountMask') }}</dt>
                  <dd class="text-right text-gray-900 dark:text-white">{{ payoutAccount.account_mask }}</dd>
                </div>
                <div class="flex justify-between gap-3">
                  <dt class="text-gray-500 dark:text-gray-400">{{ t('wallet.withdrawals.recipientMask') }}</dt>
                  <dd class="text-right text-gray-900 dark:text-white">{{ payoutAccount.recipient_name_mask || '-' }}</dd>
                </div>
              </dl>

              <form class="mt-4 grid gap-3" @submit.prevent="savePayoutAccount">
                <label class="grid gap-1 text-sm">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('wallet.withdrawals.method') }}</span>
                  <select v-model="accountForm.method" class="input">
                    <option value="alipay">{{ t('wallet.withdrawals.methods.alipay') }}</option>
                    <option value="bank_transfer">{{ t('wallet.withdrawals.methods.bank_transfer') }}</option>
                    <option value="other">{{ t('wallet.withdrawals.methods.other') }}</option>
                  </select>
                </label>
                <label class="grid gap-1 text-sm">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('wallet.withdrawals.currency') }}</span>
                  <select v-model="accountForm.currency" class="input">
                    <option value="CNY">CNY</option>
                    <option value="USD">USD</option>
                  </select>
                </label>
                <label class="grid gap-1 text-sm">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('wallet.withdrawals.recipientName') }}</span>
                  <input v-model.trim="accountForm.recipient_name" class="input" :placeholder="t('wallet.withdrawals.recipientNamePlaceholder')" autocomplete="off" />
                </label>
                <label v-if="accountForm.method === 'bank_transfer'" class="grid gap-1 text-sm">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('wallet.withdrawals.bankName') }}</span>
                  <input v-model.trim="accountForm.bank_name" class="input" :placeholder="t('wallet.withdrawals.bankNamePlaceholder')" autocomplete="off" />
                </label>
                <label class="grid gap-1 text-sm">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('wallet.withdrawals.account') }}</span>
                  <input v-model.trim="accountForm.account" class="input" :placeholder="t('wallet.withdrawals.accountPlaceholder')" autocomplete="off" />
                </label>
                <p v-if="accountMessage" class="text-sm" :class="accountMessageType === 'error' ? 'text-rose-600 dark:text-rose-300' : 'text-emerald-600 dark:text-emerald-300'">
                  {{ accountMessage }}
                </p>
                <button type="submit" class="btn btn-primary inline-flex items-center justify-center gap-2" :disabled="accountSaving">
                  <Icon name="check" size="sm" />
                  {{ accountSaving ? t('wallet.withdrawals.savingAccount') : t('wallet.withdrawals.saveAccount') }}
                </button>
              </form>
            </div>

            <div class="rounded border border-gray-100 p-4 dark:border-dark-700">
              <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('wallet.withdrawals.newRequest') }}</p>
              <dl class="mt-3 grid gap-2 text-sm">
                <div class="flex justify-between gap-3">
                  <dt class="text-gray-500 dark:text-gray-400">{{ t('wallet.withdrawals.minimumAmount') }}</dt>
                  <dd class="text-gray-900 dark:text-white">{{ formatMoney(availability?.minimum_amount) }}</dd>
                </div>
                <div class="flex justify-between gap-3">
                  <dt class="text-gray-500 dark:text-gray-400">{{ t('wallet.withdrawals.remainingDaily') }}</dt>
                  <dd class="text-gray-900 dark:text-white">{{ formatMoney(availability?.remaining_daily_amount) }}</dd>
                </div>
              </dl>
              <form class="mt-4 grid gap-3" @submit.prevent="submitWithdrawal">
                <label class="grid gap-1 text-sm">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('wallet.withdrawals.amount') }}</span>
                  <input v-model.trim="withdrawalAmount" class="input" inputmode="numeric" pattern="[0-9]*" placeholder="10" autocomplete="off" />
                </label>
                <p v-if="withdrawalMessage" class="text-sm" :class="withdrawalMessageType === 'error' ? 'text-rose-600 dark:text-rose-300' : 'text-emerald-600 dark:text-emerald-300'">
                  {{ withdrawalMessage }}
                </p>
                <button type="submit" class="btn btn-primary inline-flex items-center justify-center gap-2" :disabled="!canSubmitWithdrawal || withdrawalSubmitting">
                  <Icon name="dollar" size="sm" />
                  {{ withdrawalSubmitting ? t('wallet.withdrawals.submitting') : t('wallet.withdrawals.requestWithdrawal') }}
                </button>
              </form>
            </div>
          </div>
        </section>

        <section class="card">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('wallet.withdrawals.historyTitle') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('wallet.withdrawals.requestCount', { count: withdrawalPage.total }) }}
            </p>
          </div>
          <div class="divide-y divide-gray-100 dark:divide-dark-700">
            <div v-for="item in withdrawalPage.items" :key="item.id" class="p-4">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <p class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ item.request_no }}</p>
                  <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ formatDateTime(item.created_at) }}</p>
                </div>
                <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="withdrawalStatusClass(item.status)">
                  {{ withdrawalStatusLabel(item.status) }}
                </span>
              </div>
              <div class="mt-3 grid grid-cols-2 gap-3 text-sm">
                <div>
                  <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('wallet.withdrawals.amount') }}</p>
                  <p class="font-medium text-gray-900 dark:text-white">{{ formatMoney(item.amount) }}</p>
                </div>
                <div>
                  <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('wallet.withdrawals.paidAt') }}</p>
                  <p class="font-medium text-gray-900 dark:text-white">{{ formatDateTime(item.paid_at) }}</p>
                </div>
              </div>
              <div class="mt-3 flex flex-wrap gap-2">
                <button type="button" class="btn btn-secondary btn-sm" :disabled="detailLoading" @click="loadWithdrawalDetail(item.id)">
                  {{ t('wallet.withdrawals.viewHistory') }}
                </button>
                <button v-if="isCancelable(item.status)" type="button" class="btn btn-danger btn-sm" :disabled="withdrawalActionID === item.id" @click="cancelRequest(item.id)">
                  {{ withdrawalActionID === item.id ? t('wallet.withdrawals.canceling') : t('wallet.withdrawals.cancel') }}
                </button>
              </div>
            </div>
            <div v-if="!withdrawalPage.items.length" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-gray-400">
              {{ loading ? t('wallet.loading') : t('wallet.withdrawals.empty') }}
            </div>
          </div>
          <div class="flex items-center justify-between border-t border-gray-100 px-5 py-4 text-sm dark:border-dark-700">
            <span class="text-gray-500 dark:text-gray-400">{{ t('wallet.pageInfo', { page: withdrawalPage.page, pages: withdrawalPage.pages }) }}</span>
            <div class="flex gap-2">
              <button type="button" class="btn btn-secondary btn-sm" :disabled="withdrawalPage.page <= 1 || loading" @click="changeWithdrawalPage(withdrawalPage.page - 1)">
                {{ t('common.previous') }}
              </button>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="withdrawalPage.page >= withdrawalPage.pages || loading" @click="changeWithdrawalPage(withdrawalPage.page + 1)">
                {{ t('common.next') }}
              </button>
            </div>
          </div>
        </section>
      </div>

      <section v-if="activeWithdrawal" class="card">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('wallet.withdrawals.statusHistory') }}</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ activeWithdrawal.request_no }}</p>
        </div>
        <div class="grid gap-3 p-5 md:grid-cols-2 xl:grid-cols-4">
          <div>
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('wallet.withdrawals.amount') }}</p>
            <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatMoney(activeWithdrawal.amount) }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('wallet.withdrawals.statusLabel') }}</p>
            <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ withdrawalStatusLabel(activeWithdrawal.status) }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('wallet.withdrawals.accountMask') }}</p>
            <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ activeWithdrawal.payout_account_mask }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('wallet.withdrawals.paidAt') }}</p>
            <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatDateTime(activeWithdrawal.paid_at) }}</p>
          </div>
        </div>
        <ol class="space-y-3 px-5 pb-5">
          <li v-for="event in activeWithdrawal.events || []" :key="event.id" class="rounded border border-gray-100 p-3 text-sm dark:border-dark-700">
            <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
              <span class="font-medium text-gray-900 dark:text-white">{{ withdrawalStatusLabel(event.status) }}</span>
              <span class="text-gray-500 dark:text-gray-400">{{ formatDateTime(event.created_at) }}</span>
            </div>
            <p v-if="event.note" class="mt-2 text-gray-600 dark:text-gray-300">{{ event.note }}</p>
          </li>
          <li v-if="!(activeWithdrawal.events || []).length" class="rounded border border-gray-100 p-4 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
            {{ t('wallet.withdrawals.noEvents') }}
          </li>
        </ol>
      </section>

      <section class="card">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('wallet.transactions') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('wallet.transactionCount', { count: transactionPage.total }) }}
            </p>
          </div>
          <label class="flex flex-col gap-1 text-sm sm:w-56">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('wallet.sourceLabel') }}</span>
            <select v-model="sourceFilter" class="input" @change="reloadTransactions">
              <option v-for="source in sourceOptions" :key="source" :value="source">
                {{ t(`wallet.source.${source}`) }}
              </option>
            </select>
          </label>
        </div>

        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-100 text-sm dark:divide-dark-700">
            <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3">{{ t('wallet.table.time') }}</th>
                <th class="px-4 py-3">{{ t('wallet.table.source') }}</th>
                <th class="px-4 py-3">{{ t('wallet.table.direction') }}</th>
                <th class="px-4 py-3 text-right">{{ t('wallet.table.amount') }}</th>
                <th class="px-4 py-3 text-right">{{ t('wallet.table.balanceAfter') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="item in transactionPage.items" :key="item.id">
                <td class="whitespace-nowrap px-4 py-3 text-gray-600 dark:text-gray-300">{{ formatDateTime(item.created_at) }}</td>
                <td class="px-4 py-3 text-gray-900 dark:text-white">{{ t(`wallet.source.${item.source}`) }}</td>
                <td class="px-4 py-3">
                  <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="directionClass(item.direction)">
                    {{ t(`wallet.direction.${item.direction}`) }}
                  </span>
                </td>
                <td class="px-4 py-3 text-right tabular-nums" :class="amountClass(item.direction)">
                  {{ formatSignedAmount(item) }}
                  <p v-if="hasFrozenDelta(item)" class="mt-1 text-xs font-normal text-gray-500 dark:text-gray-400">
                    {{ t('wallet.table.taskReservedChange') }} {{ formatSignedMoneyValue(item.frozen_delta) }}
                  </p>
                  <p v-if="hasWithdrawalFrozenDelta(item)" class="mt-1 text-xs font-normal text-gray-500 dark:text-gray-400">
                    {{ t('wallet.table.withdrawalFrozenChange') }} {{ formatSignedMoneyValue(item.withdrawal_frozen_delta) }}
                  </p>
                  <p v-if="hasWithdrawableDelta(item)" class="mt-1 text-xs font-normal text-gray-500 dark:text-gray-400">
                    {{ t('wallet.table.withdrawableChange') }} {{ formatSignedMoneyValue(item.withdrawable_delta) }}
                  </p>
                </td>
                <td class="px-4 py-3 text-right tabular-nums text-gray-600 dark:text-gray-300">
                  {{ formatMoney(item.balance_after) }}
                </td>
              </tr>
              <tr v-if="!transactionPage.items.length">
                <td colspan="5" class="px-4 py-10 text-center text-sm text-gray-500">
                  {{ loading ? t('wallet.loading') : t('wallet.empty') }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="flex flex-col gap-3 border-t border-gray-100 px-5 py-4 text-sm dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
          <span class="text-gray-500 dark:text-gray-400">
            {{ t('wallet.pageInfo', { page: transactionPage.page, pages: transactionPage.pages }) }}
          </span>
          <div class="flex items-center gap-2">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="transactionPage.page <= 1 || loading" @click="changePage(transactionPage.page - 1)">
              {{ t('common.previous') }}
            </button>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="transactionPage.page >= transactionPage.pages || loading" @click="changePage(transactionPage.page + 1)">
              {{ t('common.next') }}
            </button>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  cancelWithdrawal,
  createWithdrawal,
  getWalletSummary,
  getWalletTransactions,
  getWithdrawal,
  getWithdrawalAccount,
  getWithdrawalAvailability,
  getWithdrawals,
  normalizeWithdrawalWholeAmount,
  updateWithdrawalAccount,
  type WalletDirection,
  type WalletSource,
  type WalletSummary,
  type WalletTransaction,
  type WalletTransactionPage,
  type WithdrawalAvailability,
  type WithdrawalPayoutAccount,
  type WithdrawalPayoutMethod,
  type WithdrawalRequest,
  type WithdrawalRequestPage,
  type WithdrawalStatus,
} from '@/api/wallet'

const { t, locale } = useI18n()

const sourceOptions: WalletSource[] = [
  'all',
  'withdrawal',
  'team_reward',
  'arena_reward',
  'recharge',
  'affiliate',
  'checkin',
  'quiz',
  'blind_box',
  'usage',
  'image_task',
  'refund',
  'admin_adjustment',
  'promotion',
  'subscription',
  'redeem',
  'other',
]

const loading = ref(false)
const loadError = ref(false)
const sourceFilter = ref<WalletSource>('all')
const summary = ref<WalletSummary | null>(null)
const availability = ref<WithdrawalAvailability | null>(null)
const payoutAccount = ref<WithdrawalPayoutAccount | null>(null)
const activeWithdrawal = ref<WithdrawalRequest | null>(null)
const detailLoading = ref(false)
const accountSaving = ref(false)
const withdrawalSubmitting = ref(false)
const withdrawalActionID = ref<number | null>(null)
const withdrawalAmount = ref('')
const accountMessage = ref('')
const accountMessageType = ref<'success' | 'error'>('success')
const withdrawalMessage = ref('')
const withdrawalMessageType = ref<'success' | 'error'>('success')

const accountForm = reactive({
  method: 'alipay' as WithdrawalPayoutMethod,
  currency: 'CNY' as 'CNY' | 'USD',
  recipient_name: '',
  account: '',
  bank_name: '',
})

const transactionPage = ref<WalletTransactionPage>({
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
  pages: 1,
})

const withdrawalPage = ref<WithdrawalRequestPage>({
  items: [],
  total: 0,
  page: 1,
  page_size: 10,
  pages: 1,
})

const summaryCards = computed(() => [
  { label: t('wallet.available'), value: formatMoney(summary.value?.available_balance) },
  { label: t('wallet.withdrawable'), value: formatMoney(summary.value?.withdrawable_balance) },
  { label: t('wallet.pendingWithdrawable'), value: formatMoney(summary.value?.pending_withdrawable_balance) },
  { label: t('wallet.withdrawalFrozen'), value: formatMoney(summary.value?.withdrawal_frozen_balance) },
  { label: t('wallet.taskReserved'), value: formatMoney(summary.value?.task_reserved_balance) },
  { label: t('wallet.totalCredits'), value: formatMoney(summary.value?.total_credits) },
  { label: t('wallet.totalDebits'), value: formatMoney(summary.value?.total_debits) },
])

const withdrawalAvailabilityText = computed(() => {
  if (!availability.value) return t('wallet.withdrawals.loadingAvailability')
  if (availability.value.can_apply) return t('wallet.withdrawals.availableNow')
  return disabledReasonLabel(availability.value.disabled_reason)
})

const canSubmitWithdrawal = computed(() => {
  return Boolean(availability.value?.can_apply && payoutAccount.value && withdrawalAmount.value.trim() && !withdrawalSubmitting.value)
})

function formatMoney(value: string | number | undefined) {
  return new Intl.NumberFormat(locale.value, {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(Number(value ?? 0))
}

function formatDateTime(value: string | undefined) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString(locale.value)
}

function transactionDelta(item: WalletTransaction) {
  return Number(item.balance_delta || 0) + Number(item.frozen_delta || 0)
}

function hasFrozenDelta(item: WalletTransaction) {
  return Math.abs(Number(item.frozen_delta || 0)) > 0.00000001
}

function hasWithdrawalFrozenDelta(item: WalletTransaction) {
  return Math.abs(Number(item.withdrawal_frozen_delta || 0)) > 0.00000001
}

function hasWithdrawableDelta(item: WalletTransaction) {
  return Math.abs(Number(item.withdrawable_delta || 0)) > 0.00000001
}

function formatSignedMoneyValue(value: string | number | undefined) {
  const numeric = Number(value ?? 0)
  const amount = Math.abs(numeric)
  if (numeric > 0) return `+${formatMoney(amount)}`
  if (numeric < 0) return `-${formatMoney(amount)}`
  return formatMoney(0)
}

function formatSignedAmount(item: WalletTransaction) {
  return formatSignedMoneyValue(transactionDelta(item))
}

function directionClass(direction: WalletDirection) {
  if (direction === 'credit') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-200'
  if (direction === 'debit') return 'bg-rose-50 text-rose-700 dark:bg-rose-950/40 dark:text-rose-200'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

function amountClass(direction: WalletDirection) {
  if (direction === 'credit') return 'text-emerald-600 dark:text-emerald-300'
  if (direction === 'debit') return 'text-rose-600 dark:text-rose-300'
  return 'text-gray-600 dark:text-gray-300'
}

function withdrawalStatusClass(status: WithdrawalStatus) {
  if (status === 'paid') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-200'
  if (status === 'rejected' || status === 'canceled') return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  if (status === 'payout_pending') return 'bg-blue-50 text-blue-700 dark:bg-blue-950/40 dark:text-blue-200'
  return 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-200'
}

function withdrawalStatusLabel(status: string) {
  const key = `wallet.withdrawals.status.${status}`
  const translated = t(key)
  return translated === key ? t('wallet.withdrawals.statusLabel') : translated
}

function payoutMethodLabel(method: string) {
  const key = `wallet.withdrawals.methods.${method}`
  const translated = t(key)
  return translated === key ? t('wallet.withdrawals.method') : translated
}

function disabledReasonLabel(reason: string | undefined) {
  const key = `wallet.withdrawals.disabledReasons.${reason || 'unknown'}`
  const translated = t(key)
  return translated === key ? t('wallet.withdrawals.disabledReasons.unknown') : translated
}

function isCancelable(status: WithdrawalStatus) {
  return status === 'pending_review' || status === 'second_review'
}

async function savePayoutAccount() {
  accountMessage.value = ''
  if (!accountForm.recipient_name || !accountForm.account) {
    accountMessageType.value = 'error'
    accountMessage.value = t('wallet.withdrawals.validation.accountRequired')
    return
  }
  accountSaving.value = true
  try {
    const details: Record<string, string> = { account: accountForm.account }
    if (accountForm.bank_name) details.bank_name = accountForm.bank_name
    payoutAccount.value = await updateWithdrawalAccount({
      method: accountForm.method,
      currency: accountForm.currency,
      recipient_name: accountForm.recipient_name,
      details,
    })
    accountForm.account = ''
    accountForm.bank_name = ''
    accountMessageType.value = 'success'
    accountMessage.value = t('wallet.withdrawals.accountSaved')
  } catch {
    accountMessageType.value = 'error'
    accountMessage.value = t('wallet.withdrawals.accountSaveFailed')
  } finally {
    accountSaving.value = false
  }
}

async function submitWithdrawal() {
  withdrawalMessage.value = ''
  if (!canSubmitWithdrawal.value) {
    withdrawalMessageType.value = 'error'
    withdrawalMessage.value = t('wallet.withdrawals.validation.unavailable')
    return
  }
  const normalizedAmount = normalizeWithdrawalWholeAmount(withdrawalAmount.value)
  if (!/^[1-9]\d*$/.test(normalizedAmount)) {
    withdrawalMessageType.value = 'error'
    withdrawalMessage.value = t('wallet.withdrawals.validation.integerAmountRequired')
    return
  }
  withdrawalSubmitting.value = true
  try {
    const result = await createWithdrawal({ amount: normalizedAmount })
    withdrawalAmount.value = ''
    withdrawalMessageType.value = 'success'
    withdrawalMessage.value = t('wallet.withdrawals.submitSuccess')
    await loadWallet()
    await loadWithdrawalDetail(result.id)
  } catch {
    withdrawalMessageType.value = 'error'
    withdrawalMessage.value = t('wallet.withdrawals.submitFailed')
  } finally {
    withdrawalSubmitting.value = false
  }
}

async function cancelRequest(id: number) {
  withdrawalActionID.value = id
  try {
    const result = await cancelWithdrawal(id)
    activeWithdrawal.value = result
    await loadWallet()
  } catch {
    withdrawalMessageType.value = 'error'
    withdrawalMessage.value = t('wallet.withdrawals.cancelFailed')
  } finally {
    withdrawalActionID.value = null
  }
}

async function loadWithdrawalDetail(id: number) {
  detailLoading.value = true
  try {
    activeWithdrawal.value = await getWithdrawal(id)
  } finally {
    detailLoading.value = false
  }
}

async function loadWallet() {
  loading.value = true
  loadError.value = false
  try {
    const [nextSummary, nextTransactions, nextAvailability, nextAccount, nextWithdrawals] = await Promise.all([
      getWalletSummary(),
      getWalletTransactions({
        source: sourceFilter.value,
        page: transactionPage.value.page,
        page_size: transactionPage.value.page_size,
      }),
      getWithdrawalAvailability(),
      getWithdrawalAccount(),
      getWithdrawals({
        page: withdrawalPage.value.page,
        page_size: withdrawalPage.value.page_size,
      }),
    ])
    summary.value = nextSummary
    transactionPage.value = nextTransactions
    availability.value = nextAvailability
    payoutAccount.value = nextAccount
    withdrawalPage.value = nextWithdrawals
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

async function reloadTransactions() {
  transactionPage.value = { ...transactionPage.value, page: 1 }
  await loadWallet()
}

async function changePage(page: number) {
  if (page < 1 || page > transactionPage.value.pages) return
  transactionPage.value = { ...transactionPage.value, page }
  await loadWallet()
}

async function changeWithdrawalPage(page: number) {
  if (page < 1 || page > withdrawalPage.value.pages) return
  withdrawalPage.value = { ...withdrawalPage.value, page }
  await loadWallet()
}

onMounted(loadWallet)
</script>
