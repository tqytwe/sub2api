<template>
  <AppLayout>
    <div class="min-w-0 space-y-6 overflow-x-hidden">
      <div class="flex min-w-0 flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div class="min-w-0">
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.funds.title') }}</h1>
          <p class="mt-1 break-words text-sm text-gray-500 dark:text-gray-400">{{ t('admin.funds.description') }}</p>
        </div>
        <button type="button" class="btn btn-secondary inline-flex items-center gap-2 self-start" :disabled="loading" @click="refreshAll">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          {{ t('common.refresh') }}
        </button>
      </div>

      <div v-if="message" class="break-words rounded border px-4 py-3 text-sm" :class="messageType === 'error' ? 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900/50 dark:bg-rose-950/30 dark:text-rose-200' : 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/50 dark:bg-emerald-950/30 dark:text-emerald-200'">
        {{ message }}
      </div>

      <div class="flex flex-wrap gap-2">
        <button v-for="tab in tabs" :key="tab" type="button" class="btn btn-secondary btn-sm" :class="{ 'bg-primary-600 text-white hover:bg-primary-700 dark:bg-primary-500': activeTab === tab }" @click="setActiveTab(tab)">
          {{ t(`admin.funds.tabs.${tab}`) }}
        </button>
      </div>

      <section v-if="activeTab === 'refunds'" class="card min-w-0 overflow-hidden">
        <div class="flex min-w-0 flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 lg:flex-row lg:items-end lg:justify-between">
          <div class="min-w-0">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.funds.refunds.title') }}</h2>
            <p class="mt-1 break-words text-sm text-gray-500 dark:text-gray-400">{{ t('admin.funds.refunds.description') }}</p>
          </div>
          <div class="grid min-w-0 gap-2 sm:grid-cols-[160px_160px_auto]">
            <select v-model="refundQuery.status" class="input">
              <option value="all">{{ t('admin.funds.status.all') }}</option>
              <option value="pending_review">{{ t('admin.funds.status.pending_review') }}</option>
              <option value="payout_pending">{{ t('admin.funds.status.payout_pending') }}</option>
              <option value="paid">{{ t('admin.funds.status.paid') }}</option>
              <option value="rejected">{{ t('admin.funds.status.rejected') }}</option>
              <option value="canceled">{{ t('admin.funds.status.canceled') }}</option>
            </select>
            <input v-model.number="refundQuery.user_id" class="input" type="number" min="1" :placeholder="t('admin.funds.refunds.userId')" />
            <button type="button" class="btn btn-primary" :disabled="loading" @click="loadRefunds">{{ t('common.search') }}</button>
          </div>
        </div>

        <div class="max-w-full min-w-0 overflow-x-auto">
          <table class="min-w-[980px] divide-y divide-gray-100 text-sm dark:divide-dark-700">
            <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3">{{ t('admin.funds.table.request') }}</th>
                <th class="px-4 py-3">{{ t('admin.funds.table.user') }}</th>
                <th class="px-4 py-3">{{ t('admin.funds.table.type') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.funds.table.amount') }}</th>
                <th class="px-4 py-3">{{ t('admin.funds.table.status') }}</th>
                <th class="px-4 py-3">{{ t('admin.funds.table.account') }}</th>
                <th class="px-4 py-3">{{ t('admin.funds.table.createdAt') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.funds.table.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="item in refundPage.items" :key="item.id">
                <td class="px-4 py-3 align-top">
                  <p class="break-words font-medium text-gray-900 dark:text-white">{{ item.request_no }}</p>
                  <p v-if="item.reason" class="mt-1 max-w-[220px] truncate text-xs text-gray-500 dark:text-gray-400">{{ item.reason }}</p>
                </td>
                <td class="px-4 py-3 align-top text-gray-600 dark:text-gray-300">
                  <p class="max-w-[220px] break-words">{{ item.user_email || '-' }}</p>
                  <p class="text-xs text-gray-500">ID {{ item.user_id }}</p>
                </td>
                <td class="px-4 py-3 align-top text-gray-600 dark:text-gray-300">{{ refundTypeLabel(item.request_type) }}</td>
                <td class="px-4 py-3 align-top text-right font-medium tabular-nums text-gray-900 dark:text-white">{{ formatMoney(item.amount) }}</td>
                <td class="px-4 py-3 align-top">
                  <span class="inline-flex rounded-full px-2 py-0.5 text-xs font-medium" :class="statusClass(item.status)">{{ statusLabel(item.status) }}</span>
                </td>
                <td class="px-4 py-3 align-top text-gray-600 dark:text-gray-300">
                  <p class="max-w-[180px] truncate">{{ item.payout_account_mask || '-' }}</p>
                  <button type="button" class="mt-1 text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-300" @click="loadSensitive(item.id)">
                    {{ t('admin.funds.refunds.viewSensitive') }}
                  </button>
                </td>
                <td class="whitespace-nowrap px-4 py-3 align-top text-gray-600 dark:text-gray-300">{{ formatDateTime(item.created_at) }}</td>
                <td class="px-4 py-3 align-top">
                  <div class="flex flex-wrap justify-end gap-2">
                    <button v-if="item.status === 'pending_review'" type="button" class="btn btn-secondary btn-sm" :disabled="actionID === item.id" @click="approve(item.id)">
                      {{ t('admin.funds.actions.approve') }}
                    </button>
                    <button v-if="item.status === 'pending_review'" type="button" class="btn btn-danger btn-sm" :disabled="actionID === item.id || !actionReason.trim()" @click="reject(item.id)">
                      {{ t('admin.funds.actions.reject') }}
                    </button>
                    <button v-if="item.status === 'payout_pending'" type="button" class="btn btn-primary btn-sm" :disabled="actionID === item.id || !paidForm.external_txn_id.trim()" @click="markPaid(item)">
                      {{ t('admin.funds.actions.markPaid') }}
                    </button>
                  </div>
                </td>
              </tr>
              <tr v-if="!refundPage.items.length">
                <td colspan="8" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ loading ? t('admin.funds.loading') : t('admin.funds.refunds.empty') }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="grid min-w-0 gap-3 border-t border-gray-100 px-5 py-4 dark:border-dark-700 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
          <input v-model.trim="actionNote" class="input" :placeholder="t('admin.funds.actions.notePlaceholder')" />
          <input v-model.trim="actionReason" class="input" :placeholder="t('admin.funds.actions.reasonPlaceholder')" />
          <div class="flex flex-wrap items-center justify-end gap-2">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="refundPage.page <= 1 || loading" @click="changeRefundPage(refundPage.page - 1)">{{ t('common.previous') }}</button>
            <span class="text-sm text-gray-500 dark:text-gray-400">{{ refundPage.page }} / {{ refundPage.pages }}</span>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="refundPage.page >= refundPage.pages || loading" @click="changeRefundPage(refundPage.page + 1)">{{ t('common.next') }}</button>
          </div>
        </div>

        <div class="grid min-w-0 gap-3 border-t border-gray-100 px-5 py-4 dark:border-dark-700 md:grid-cols-4">
          <input v-model.trim="paidForm.external_txn_id" class="input" :placeholder="t('admin.funds.paid.externalTxn')" />
          <input v-model.trim="paidForm.paid_amount" class="input" inputmode="numeric" :placeholder="t('admin.funds.paid.amount')" />
          <select v-model="paidForm.paid_currency" class="input">
            <option value="USD">USD</option>
            <option value="CNY">CNY</option>
          </select>
          <input v-model.trim="paidForm.payout_fx_rate" class="input" :placeholder="t('admin.funds.paid.fxRate')" />
        </div>
      </section>

      <section v-if="activeTab === 'grants'" class="grid min-w-0 gap-4 xl:grid-cols-2">
        <form class="card min-w-0 overflow-hidden p-5" @submit.prevent="submitGift">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.funds.grants.giftTitle') }}</h2>
          <div class="mt-4 grid min-w-0 gap-3">
            <input v-model.number="giftForm.user_id" class="input" type="number" min="1" :placeholder="t('admin.funds.forms.userId')" />
            <input v-model.trim="giftForm.amount" class="input" inputmode="numeric" :placeholder="t('admin.funds.forms.amount')" />
            <textarea v-model.trim="giftForm.reason" class="input min-h-[96px]" :placeholder="t('admin.funds.forms.reason')" />
            <button type="submit" class="btn btn-primary" data-testid="admin-funds-submit-gift" :disabled="loading">{{ t('admin.funds.grants.submitGift') }}</button>
          </div>
        </form>

        <form class="card min-w-0 overflow-hidden p-5" @submit.prevent="submitOfflineRecharge">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.funds.grants.offlineTitle') }}</h2>
          <div class="mt-4 grid min-w-0 gap-3">
            <input v-model.number="offlineForm.user_id" class="input" type="number" min="1" :placeholder="t('admin.funds.forms.userId')" />
            <input v-model.trim="offlineForm.amount" class="input" inputmode="numeric" :placeholder="t('admin.funds.forms.amount')" />
            <input v-model.trim="offlineForm.external_ref" class="input" :placeholder="t('admin.funds.forms.externalRef')" />
            <textarea v-model.trim="offlineForm.reason" class="input min-h-[96px]" :placeholder="t('admin.funds.forms.reason')" />
            <button type="submit" class="btn btn-primary" :disabled="loading">{{ t('admin.funds.grants.submitOffline') }}</button>
          </div>
        </form>
      </section>

      <section v-if="activeTab === 'classification'" class="card min-w-0 overflow-hidden">
        <div class="flex min-w-0 flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.funds.classification.title') }}</h2>
            <p class="mt-1 break-words text-sm text-gray-500 dark:text-gray-400">{{ t('admin.funds.classification.description') }}</p>
          </div>
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadClassification">{{ t('admin.funds.classification.preview') }}</button>
        </div>
        <div class="max-w-full min-w-0 overflow-x-auto">
          <table class="min-w-[780px] divide-y divide-gray-100 text-sm dark:divide-dark-700">
            <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3">{{ t('admin.funds.table.select') }}</th>
                <th class="px-4 py-3">{{ t('admin.funds.table.user') }}</th>
                <th class="px-4 py-3">{{ t('admin.funds.table.transaction') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.funds.table.amount') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.funds.classification.remaining') }}</th>
                <th class="px-4 py-3">{{ t('admin.funds.table.createdAt') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="item in classification.candidates" :key="item.transaction_id">
                <td class="px-4 py-3"><input v-model="selectedTransactions" type="checkbox" :value="item.transaction_id" /></td>
                <td class="px-4 py-3"><span class="break-words">{{ item.user_email }}</span> <span class="text-xs text-gray-500">ID {{ item.user_id }}</span></td>
                <td class="px-4 py-3">#{{ item.transaction_id }}</td>
                <td class="px-4 py-3 text-right">{{ formatMoney(item.amount) }}</td>
                <td class="px-4 py-3 text-right">{{ formatMoney(item.estimated_remaining) }}</td>
                <td class="px-4 py-3">{{ formatDateTime(item.created_at) }}</td>
              </tr>
              <tr v-if="!classification.candidates.length">
                <td colspan="6" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ loading ? t('admin.funds.loading') : t('admin.funds.classification.empty') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="grid min-w-0 gap-3 border-t border-gray-100 px-5 py-4 dark:border-dark-700 lg:grid-cols-[minmax(0,1fr)_auto]">
          <input v-model.trim="classificationReason" class="input" :placeholder="t('admin.funds.classification.reasonPlaceholder')" />
          <button type="button" class="btn btn-primary" data-testid="admin-funds-execute-classification" :disabled="loading" @click="executeClassification">
            {{ t('admin.funds.classification.execute', { count: selectedTransactions.length }) }}
          </button>
        </div>
      </section>

      <section v-if="sensitivePayout" class="card min-w-0 overflow-hidden p-5">
        <div class="flex min-w-0 flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <h2 class="break-words text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.funds.refunds.sensitiveTitle') }}</h2>
          <button type="button" class="btn btn-secondary btn-sm" @click="sensitivePayout = null">{{ t('common.close') }}</button>
        </div>
        <pre class="mt-4 max-h-80 overflow-auto rounded bg-gray-950 p-4 text-xs text-gray-100">{{ JSON.stringify(sensitivePayout, null, 2) }}</pre>
      </section>

      <TotpStepUpDialog :controller="fundStepUp" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import {
  approveRefundRequest,
  executeSignupGift30,
  getRefundSensitivePayout,
  grantGift,
  grantOfflineRecharge,
  listRefundRequests,
  markRefundPaid,
  previewSignupGift30,
  rejectRefundRequest,
  type AdminFundClassificationPreview,
} from '@/api/admin/funds'
import type { FundRefundRequest, FundRefundRequestPage, FundRefundStatus } from '@/api/wallet'
import { extractApiErrorCode } from '@/utils/apiError'
import { isStepUpCancelled, useStepUp } from '@/composables/useStepUp'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()

type TabKey = 'refunds' | 'grants' | 'classification'

const tabs: TabKey[] = ['refunds', 'grants', 'classification']
const tabPaths: Record<TabKey, string> = {
  refunds: '/admin/funds/refunds',
  grants: '/admin/funds/grants',
  classification: '/admin/funds/classification',
}
const activeTab = ref<TabKey>('refunds')
const loading = ref(false)
const message = ref('')
const messageType = ref<'success' | 'error'>('success')
const actionID = ref<number | null>(null)
const actionNote = ref('')
const actionReason = ref('')
const selectedTransactions = ref<number[]>([])
const classificationReason = ref('')
const sensitivePayout = ref<Record<string, unknown> | null>(null)
const fundStepUp = useStepUp()

const refundQuery = reactive({
  status: 'all' as FundRefundStatus | 'all',
  user_id: undefined as number | undefined,
})

const paidForm = reactive({
  external_txn_id: '',
  paid_amount: '',
  paid_currency: 'USD' as 'USD' | 'CNY',
  payout_fx_rate: '1',
})

const giftForm = reactive({ user_id: undefined as number | undefined, amount: '', reason: '' })
const offlineForm = reactive({ user_id: undefined as number | undefined, amount: '', external_ref: '', reason: '' })

const refundPage = ref<FundRefundRequestPage>({ items: [], total: 0, page: 1, page_size: 20, pages: 1 })
const classification = ref<AdminFundClassificationPreview>({ mode: 'preview', generated_at: '', candidate_count: 0, candidates: [] })

function showMessage(type: 'success' | 'error', text: string) {
  messageType.value = type
  message.value = text
}

function textLength(value: string) {
  return Array.from(value.trim()).length
}

function isPositiveWholeAmount(value: string) {
  return /^[1-9]\d*$/.test(value.trim())
}

function isPositiveUserID(value: unknown) {
  return typeof value === 'number' && Number.isInteger(value) && value > 0
}

function validateReason(reason: string, min: number, max: number) {
  const length = textLength(reason)
  if (length < min) {
    showMessage('error', t('admin.funds.validation.reasonTooShort', { min }))
    return false
  }
  if (length > max) {
    showMessage('error', t('admin.funds.validation.reasonTooLong', { max }))
    return false
  }
  return true
}

function validateCreditForm(userID: unknown, amount: string, reason: string, reasonMin = 3) {
  if (!isPositiveUserID(userID)) {
    showMessage('error', t('admin.funds.validation.userRequired'))
    return false
  }
  if (!isPositiveWholeAmount(amount)) {
    showMessage('error', t('admin.funds.validation.wholeAmountRequired'))
    return false
  }
  return validateReason(reason, reasonMin, 500)
}

function localizedFundError(error: unknown, fallback: string) {
  const code = extractApiErrorCode(error)
  if (!code) return fallback
  const key = `admin.funds.errors.${code}`
  const translated = t(key)
  return translated === key ? fallback : translated
}

function handleFundActionError(error: unknown, fallback: string) {
  if (isStepUpCancelled(error)) return
  showMessage('error', localizedFundError(error, fallback))
}

function formatMoney(value: string | number | undefined) {
  return new Intl.NumberFormat(locale.value, { style: 'currency', currency: 'USD', minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(Number(value ?? 0))
}

function formatDateTime(value: string | undefined) {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString(locale.value)
}

function statusLabel(status: string) {
  const key = `admin.funds.status.${status}`
  const translated = t(key)
  return translated === key ? t('admin.funds.status.unknown') : translated
}

function statusClass(status: FundRefundStatus) {
  if (status === 'paid') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-200'
  if (status === 'rejected' || status === 'canceled') return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  if (status === 'payout_pending') return 'bg-blue-50 text-blue-700 dark:bg-blue-950/40 dark:text-blue-200'
  return 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-200'
}

function refundTypeLabel(type: string) {
  const key = `admin.funds.refundTypes.${type}`
  const translated = t(key)
  return translated === key ? t('admin.funds.table.type') : translated
}

function normalizeTab(raw: unknown): TabKey {
  return tabs.includes(raw as TabKey) ? raw as TabKey : 'refunds'
}

function syncTabFromRoute() {
  activeTab.value = normalizeTab(route.params.tab)
}

async function loadActiveTab() {
  if (activeTab.value === 'classification') {
    await loadClassification()
    return
  }
  if (activeTab.value === 'refunds') {
    await loadRefunds()
  }
}

async function setActiveTab(tab: TabKey) {
  activeTab.value = tab
  if (route.path !== tabPaths[tab]) {
    await router.push(tabPaths[tab])
    return
  }
  await loadActiveTab()
}

async function loadRefunds() {
  loading.value = true
  try {
    refundPage.value = await listRefundRequests({
      status: refundQuery.status,
      user_id: refundQuery.user_id || undefined,
      page: refundPage.value.page,
      page_size: refundPage.value.page_size,
    })
  } catch {
    showMessage('error', t('admin.funds.messages.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function refreshAll() {
  await Promise.all([loadRefunds(), activeTab.value === 'classification' ? loadClassification() : Promise.resolve()])
}

async function approve(id: number) {
  actionID.value = id
  try {
    await fundStepUp.run(() => approveRefundRequest(id, { note: actionNote.value }))
    showMessage('success', t('admin.funds.messages.approved'))
    await loadRefunds()
  } catch (error) {
    handleFundActionError(error, t('admin.funds.messages.actionFailed'))
  } finally {
    actionID.value = null
  }
}

async function reject(id: number) {
  actionID.value = id
  try {
    await fundStepUp.run(() => rejectRefundRequest(id, { reason: actionReason.value, note: actionNote.value }))
    actionReason.value = ''
    showMessage('success', t('admin.funds.messages.rejected'))
    await loadRefunds()
  } catch (error) {
    handleFundActionError(error, t('admin.funds.messages.actionFailed'))
  } finally {
    actionID.value = null
  }
}

async function markPaid(item: FundRefundRequest) {
  actionID.value = item.id
  try {
    await fundStepUp.run(() => markRefundPaid(item.id, {
      paid_amount: paidForm.paid_amount || item.amount,
      paid_currency: paidForm.paid_currency,
      payout_fx_rate: paidForm.payout_fx_rate || '1',
      external_txn_id: paidForm.external_txn_id,
      note: actionNote.value,
    }))
    paidForm.external_txn_id = ''
    paidForm.paid_amount = ''
    showMessage('success', t('admin.funds.messages.paid'))
    await loadRefunds()
  } catch (error) {
    handleFundActionError(error, t('admin.funds.messages.actionFailed'))
  } finally {
    actionID.value = null
  }
}

async function loadSensitive(id: number) {
  try {
    sensitivePayout.value = await fundStepUp.run(() => getRefundSensitivePayout(id))
  } catch (error) {
    handleFundActionError(error, t('admin.funds.messages.sensitiveFailed'))
  }
}

async function submitGift() {
  if (!validateCreditForm(giftForm.user_id, giftForm.amount, giftForm.reason)) return
  loading.value = true
  try {
    await fundStepUp.run(() => grantGift({ user_id: Number(giftForm.user_id), amount: giftForm.amount, reason: giftForm.reason }))
    giftForm.amount = ''
    giftForm.reason = ''
    showMessage('success', t('admin.funds.messages.giftGranted'))
  } catch (error) {
    handleFundActionError(error, t('admin.funds.messages.grantFailed'))
  } finally {
    loading.value = false
  }
}

async function submitOfflineRecharge() {
  if (!validateCreditForm(offlineForm.user_id, offlineForm.amount, offlineForm.reason)) return
  loading.value = true
  try {
    await fundStepUp.run(() => grantOfflineRecharge({ user_id: Number(offlineForm.user_id), amount: offlineForm.amount, external_ref: offlineForm.external_ref, reason: offlineForm.reason }))
    offlineForm.amount = ''
    offlineForm.external_ref = ''
    offlineForm.reason = ''
    showMessage('success', t('admin.funds.messages.offlineGranted'))
  } catch (error) {
    handleFundActionError(error, t('admin.funds.messages.grantFailed'))
  } finally {
    loading.value = false
  }
}

async function loadClassification() {
  loading.value = true
  try {
    classification.value = await previewSignupGift30()
    selectedTransactions.value = classification.value.candidates.map((item) => item.transaction_id)
  } catch {
    showMessage('error', t('admin.funds.messages.classificationFailed'))
  } finally {
    loading.value = false
  }
}

async function executeClassification() {
  if (!selectedTransactions.value.length) {
    showMessage('error', t('admin.funds.validation.classificationSelectionRequired'))
    return
  }
  if (!validateReason(classificationReason.value, 10, 500)) return
  loading.value = true
  try {
    const result = await fundStepUp.run(() => executeSignupGift30({ transaction_ids: selectedTransactions.value, reason: classificationReason.value }))
    showMessage('success', t('admin.funds.messages.classified', { count: result.affected_count }))
    selectedTransactions.value = []
    classificationReason.value = ''
    await loadClassification()
  } catch (error) {
    handleFundActionError(error, t('admin.funds.messages.classificationFailed'))
  } finally {
    loading.value = false
  }
}

async function changeRefundPage(page: number) {
  if (page < 1 || page > refundPage.value.pages) return
  refundPage.value = { ...refundPage.value, page }
  await loadRefunds()
}

watch(
  () => route.params.tab,
  () => {
    syncTabFromRoute()
    void loadActiveTab()
  }
)

onMounted(() => {
  syncTabFromRoute()
  void loadActiveTab()
})
</script>
