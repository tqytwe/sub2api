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
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  getWalletSummary,
  getWalletTransactions,
  type WalletDirection,
  type WalletSource,
  type WalletSummary,
  type WalletTransaction,
  type WalletTransactionPage,
} from '@/api/wallet'

const { t, locale } = useI18n()

const sourceOptions: WalletSource[] = [
  'all',
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
const transactionPage = ref<WalletTransactionPage>({
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
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

async function loadWallet() {
  loading.value = true
  loadError.value = false
  try {
    const [nextSummary, nextTransactions] = await Promise.all([
      getWalletSummary(),
      getWalletTransactions({
        source: sourceFilter.value,
        page: transactionPage.value.page,
        page_size: transactionPage.value.page_size,
      }),
    ])
    summary.value = nextSummary
    transactionPage.value = nextTransactions
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

onMounted(loadWallet)
</script>
