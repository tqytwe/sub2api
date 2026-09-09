<template>
  <AppLayout>
    <div class="min-w-0 space-y-6 overflow-x-hidden">
      <header class="flex min-w-0 flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div class="min-w-0">
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.funds.title') }}</h1>
          <p class="mt-1 break-words text-sm text-gray-500 dark:text-gray-400">{{ t('admin.funds.description') }}</p>
        </div>
        <button type="button" class="btn btn-secondary inline-flex items-center gap-2 self-start" :disabled="loading" @click="refreshActive">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          {{ t('common.refresh') }}
        </button>
      </header>

      <div v-if="message" class="break-words rounded border px-4 py-3 text-sm" :class="message.type === 'error' ? 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900/50 dark:bg-rose-950/30 dark:text-rose-200' : 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/50 dark:bg-emerald-950/30 dark:text-emerald-200'" role="status">
        {{ message.text }}
      </div>

      <nav class="flex flex-wrap gap-2" :aria-label="t('admin.funds.title')">
        <button v-for="tab in tabs" :key="tab" type="button" class="btn btn-secondary btn-sm" :class="{ 'bg-primary-600 text-white hover:bg-primary-700 dark:bg-primary-500': activeTab === tab }" @click="setTab(tab)">
          {{ t(`admin.funds.tabs.${tab}`) }}
        </button>
      </nav>

      <section v-if="activeTab === 'refunds'" class="card min-w-0 overflow-hidden" :aria-busy="loading">
        <div class="flex min-w-0 flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 lg:flex-row lg:items-end lg:justify-between">
          <div class="min-w-0">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.funds.refunds.title') }}</h2>
            <p class="mt-1 break-words text-sm text-gray-500 dark:text-gray-400">{{ t('admin.funds.refunds.description') }}</p>
          </div>
          <div class="grid min-w-0 gap-2 sm:grid-cols-[170px_220px_auto]">
            <select v-model="refundQuery.status" class="input" data-testid="admin-funds-refund-status">
              <option value="all">{{ t('admin.funds.status.all') }}</option>
              <option v-for="status in refundStatuses" :key="status" :value="status">{{ t(`admin.funds.status.${status}`) }}</option>
            </select>
            <input v-model.trim="refundQuery.account" class="input" :placeholder="t('admin.funds.refunds.accountPlaceholder')" @keyup.enter="loadRefunds" />
            <button type="button" class="btn btn-primary" :disabled="loading" @click="loadRefunds">{{ t('common.search') }}</button>
          </div>
        </div>

        <div class="min-w-0 overflow-x-auto">
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
              <tr v-for="item in refundPage.items" :key="item.request_no">
                <td class="px-4 py-3 align-top">
                  <strong class="break-words text-gray-900 dark:text-white">{{ item.request_no }}</strong>
                  <p v-if="item.reason" class="mt-1 max-w-[200px] truncate text-xs text-gray-500 dark:text-gray-400">{{ item.reason }}</p>
                </td>
                <td class="px-4 py-3 align-top break-words">{{ item.user_email || '-' }}</td>
                <td class="px-4 py-3 align-top">{{ refundTypeLabel(item.request_type) }}</td>
                <td class="px-4 py-3 align-top text-right font-medium tabular-nums">{{ money(item.amount, item.currency) }}</td>
                <td class="px-4 py-3 align-top"><span class="inline-flex rounded-full px-2 py-0.5 text-xs font-medium" :class="refundStatusClass(item.status)">{{ t(`admin.funds.status.${item.status}`) }}</span></td>
                <td class="px-4 py-3 align-top">
                  <p class="max-w-[180px] truncate">{{ item.payout_account_mask || '-' }}</p>
                  <button type="button" class="mt-1 text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-300" :disabled="loading" @click="openSensitivePayout(item.request_no)">{{ t('admin.funds.refunds.viewSensitive') }}</button>
                </td>
                <td class="px-4 py-3 align-top whitespace-nowrap">{{ dateTime(item.created_at) }}</td>
                <td class="px-4 py-3 align-top"><div class="flex flex-wrap justify-end gap-2">
                  <button v-if="item.status === 'pending_review'" type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="approve(item.request_no)">{{ t('admin.funds.actions.approve') }}</button>
                  <button v-if="item.status === 'pending_review'" type="button" class="btn btn-danger btn-sm" :disabled="loading" @click="openReject(item.request_no)">{{ t('admin.funds.actions.reject') }}</button>
                  <button v-if="item.status === 'payout_pending'" type="button" class="btn btn-primary btn-sm" :disabled="loading" @click="openMarkPaid(item)">{{ t('admin.funds.actions.markPaid') }}</button>
                </div></td>
              </tr>
              <tr v-if="!refundPage.items.length"><td colspan="8" class="px-4 py-10 text-center text-sm text-gray-500">{{ loading ? t('admin.funds.loading') : t('admin.funds.refunds.empty') }}</td></tr>
            </tbody>
          </table>
        </div>
        <div class="flex items-center justify-end gap-2 border-t border-gray-100 px-5 py-4 text-sm dark:border-dark-700">
          <button type="button" class="btn btn-secondary btn-sm" :disabled="loading || refundPage.page <= 1" @click="changeRefundPage(refundPage.page - 1)">{{ t('common.previous') }}</button>
          <span class="tabular-nums text-gray-500">{{ refundPage.page }} / {{ refundPage.pages }}</span>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="loading || refundPage.page >= refundPage.pages" @click="changeRefundPage(refundPage.page + 1)">{{ t('common.next') }}</button>
        </div>
      </section>

      <section v-if="activeTab === 'credits'" class="grid min-w-0 gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]" :aria-busy="loading">
        <form class="card min-w-0 p-5" @submit.prevent="openCreditConfirmation">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.funds.credits.title') }}</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.funds.credits.description') }}</p>
          <div class="mt-4 grid gap-3">
            <select v-model="creditForm.kind" class="input" data-testid="admin-funds-credit-kind">
              <option value="ops_gift">{{ t('admin.funds.kinds.ops_gift') }}</option>
              <option value="compensation">{{ t('admin.funds.kinds.compensation') }}</option>
              <option value="offline_recharge">{{ t('admin.funds.kinds.offline_recharge') }}</option>
            </select>
            <div class="relative">
              <input v-model.trim="accountSearch" class="input" autocomplete="off" :placeholder="t('admin.funds.credits.accountSearch')" @input="searchAccounts" />
              <div v-if="accountResults.length" class="absolute z-10 mt-2 w-full divide-y overflow-hidden rounded border border-gray-200 bg-white shadow-sm dark:divide-dark-700 dark:border-dark-600 dark:bg-dark-800">
                <button v-for="account in accountResults" :key="account.email" type="button" class="block w-full px-3 py-2 text-left hover:bg-gray-50 dark:hover:bg-dark-700" @click="selectAccount(account)">
                  <strong class="block break-all">{{ account.email }}</strong><span class="text-xs text-gray-500">{{ account.username || '-' }} · {{ account.status }} · {{ money(account.current_balance) }}</span>
                </button>
              </div>
            </div>
            <div v-if="selectedAccount" class="rounded border border-gray-200 bg-gray-50 px-3 py-3 text-sm dark:border-dark-600 dark:bg-dark-800">
              <strong class="block break-all text-gray-900 dark:text-white">{{ selectedAccount.email }}</strong><span class="text-gray-500">{{ selectedAccount.username || '-' }} · {{ selectedAccount.status }} · {{ t('admin.funds.credits.currentBalance') }} {{ money(selectedAccount.current_balance) }}</span>
            </div>
            <input v-model.trim="creditForm.amount" class="input" inputmode="decimal" :placeholder="t('admin.funds.forms.amount')" />
            <input v-if="creditForm.kind === 'offline_recharge'" v-model.trim="creditForm.external_ref" class="input" :placeholder="t('admin.funds.forms.externalRef')" />
            <textarea v-model.trim="creditForm.reason" class="input min-h-[96px]" :placeholder="t('admin.funds.forms.reason')" />
            <button type="submit" class="btn btn-primary" data-testid="admin-funds-review-credit" :disabled="loading || !selectedAccount">{{ t('admin.funds.credits.review') }}</button>
          </div>
        </form>
        <aside class="card min-w-0 p-5">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.funds.credits.safetyTitle') }}</h2>
          <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('admin.funds.credits.safetyDescription') }}</p>
        </aside>
      </section>

      <section v-if="activeTab === 'operations'" class="card min-w-0 overflow-hidden" :aria-busy="loading">
        <div class="flex min-w-0 flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 lg:flex-row lg:items-end lg:justify-between">
          <div class="min-w-0"><h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.funds.operations.title') }}</h2><p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.funds.operations.description') }}</p></div>
          <div class="grid min-w-0 gap-2 sm:grid-cols-[160px_160px_220px_auto]">
            <select v-model="operationQuery.kind" class="input"><option value="all">{{ t('admin.funds.operations.allKinds') }}</option><option v-for="kind in operationKinds" :key="kind" :value="kind">{{ t(`admin.funds.kinds.${kind}`) }}</option></select>
            <select v-model="operationQuery.status" class="input"><option value="all">{{ t('admin.funds.status.all') }}</option><option v-for="status in operationStatuses" :key="status" :value="status">{{ t(`admin.funds.operationStatus.${status}`) }}</option></select>
            <input v-model.trim="operationQuery.q" class="input" :placeholder="t('admin.funds.operations.keyword')" @keyup.enter="loadOperations" />
            <button type="button" class="btn btn-primary" :disabled="loading" @click="loadOperations">{{ t('common.search') }}</button>
          </div>
        </div>
        <div class="min-w-0 overflow-x-auto"><table class="min-w-[1000px] divide-y divide-gray-100 text-sm dark:divide-dark-700"><thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400"><tr><th class="px-4 py-3">{{ t('admin.funds.operations.operationNo') }}</th><th class="px-4 py-3">{{ t('admin.funds.table.user') }}</th><th class="px-4 py-3">{{ t('admin.funds.table.type') }}</th><th class="px-4 py-3 text-right">{{ t('admin.funds.table.amount') }}</th><th class="px-4 py-3">{{ t('admin.funds.table.status') }}</th><th class="px-4 py-3">{{ t('admin.funds.operations.operator') }}</th><th class="px-4 py-3">{{ t('admin.funds.operations.reference') }}</th><th class="px-4 py-3 text-right">{{ t('admin.funds.table.actions') }}</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="item in operationPage.items" :key="item.operation_no"><td class="px-4 py-3"><strong>{{ item.operation_no }}</strong><p class="mt-1 text-xs text-gray-500">{{ dateTime(item.created_at) }}</p></td><td class="px-4 py-3 break-all">{{ item.account_email }}</td><td class="px-4 py-3">{{ t(`admin.funds.kinds.${item.operation_kind}`) }}</td><td class="px-4 py-3 text-right font-medium tabular-nums">{{ money(item.amount, item.currency) }}</td><td class="px-4 py-3"><span class="inline-flex rounded-full px-2 py-0.5 text-xs font-medium" :class="operationStatusClass(item.status)">{{ t(`admin.funds.operationStatus.${item.status}`) }}</span></td><td class="px-4 py-3 break-all">{{ item.actor_account_email || '-' }}</td><td class="px-4 py-3"><span>{{ item.external_ref_masked || '-' }}</span><p v-if="item.reason" class="mt-1 max-w-[180px] truncate text-xs text-gray-500">{{ item.reason }}</p></td><td class="px-4 py-3 text-right"><button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="openOperation(item.operation_no)">{{ t('admin.funds.operations.detail') }}</button></td></tr><tr v-if="!operationPage.items.length"><td colspan="8" class="px-4 py-10 text-center text-sm text-gray-500">{{ loading ? t('admin.funds.loading') : t('admin.funds.operations.empty') }}</td></tr></tbody></table></div>
      </section>

      <BaseDialog :show="showCreditConfirmation" :title="t('admin.funds.confirm.title')" width="normal" @close="showCreditConfirmation = false">
        <dl v-if="selectedAccount" class="grid gap-3 text-sm sm:grid-cols-[140px_minmax(0,1fr)]"><dt class="text-gray-500">{{ t('admin.funds.confirm.account') }}</dt><dd class="break-all">{{ selectedAccount.email }}</dd><dt class="text-gray-500">{{ t('admin.funds.confirm.amount') }}</dt><dd class="tabular-nums">{{ money(creditForm.amount) }}</dd><dt class="text-gray-500">{{ t('admin.funds.confirm.reason') }}</dt><dd class="break-words">{{ creditForm.reason }}</dd></dl>
        <input v-model.trim="confirmationEmail" class="input mt-4" autocomplete="off" :placeholder="t('admin.funds.confirm.typeEmail')" />
        <template #footer><button type="button" class="btn btn-secondary" :disabled="loading" @click="showCreditConfirmation = false">{{ t('common.cancel') }}</button><button type="button" class="btn btn-primary" data-testid="admin-funds-confirm-credit" :disabled="loading || confirmationEmail !== selectedAccount?.email" @click="submitCredit">{{ t('admin.funds.confirm.submit') }}</button></template>
      </BaseDialog>

      <BaseDialog :show="Boolean(rejectRequestNo)" :title="t('admin.funds.actions.reject')" width="normal" @close="rejectRequestNo = ''">
        <textarea v-model.trim="rejectForm.reason" class="input min-h-[96px]" :placeholder="t('admin.funds.actions.reasonPlaceholder')" />
        <input v-model.trim="rejectForm.note" class="input mt-3" :placeholder="t('admin.funds.actions.notePlaceholder')" />
        <template #footer><button type="button" class="btn btn-secondary" :disabled="loading" @click="rejectRequestNo = ''">{{ t('common.cancel') }}</button><button type="button" class="btn btn-danger" data-testid="admin-funds-confirm-reject" :disabled="loading || rejectForm.reason.trim().length < 3" @click="reject">{{ t('admin.funds.actions.reject') }}</button></template>
      </BaseDialog>

      <BaseDialog :show="Boolean(payoutRequest)" :title="t('admin.funds.actions.markPaid')" width="normal" @close="payoutRequest = null">
        <div class="grid gap-3"><input v-model.trim="paidForm.external_txn_id" class="input" :placeholder="t('admin.funds.paid.externalTxn')" /><input v-model.trim="paidForm.paid_amount" class="input" inputmode="decimal" :placeholder="t('admin.funds.paid.amount')" /><select v-model="paidForm.paid_currency" class="input"><option value="USD">USD</option><option value="CNY">CNY</option></select><input v-model.trim="paidForm.payout_fx_rate" class="input" inputmode="decimal" :placeholder="t('admin.funds.paid.fxRate')" /><input v-model.trim="paidForm.note" class="input" :placeholder="t('admin.funds.actions.notePlaceholder')" /></div>
        <template #footer><button type="button" class="btn btn-secondary" :disabled="loading" @click="payoutRequest = null">{{ t('common.cancel') }}</button><button type="button" class="btn btn-primary" data-testid="admin-funds-confirm-paid" :disabled="loading || !paidForm.external_txn_id || !paidForm.paid_amount" @click="submitMarkPaid">{{ t('admin.funds.actions.markPaid') }}</button></template>
      </BaseDialog>

      <BaseDialog :show="Boolean(sensitivePayout)" :title="t('admin.funds.refunds.sensitiveTitle')" width="wide" @close="sensitivePayout = null"><pre class="max-h-80 overflow-auto rounded bg-gray-950 p-4 text-xs text-gray-100">{{ JSON.stringify(sensitivePayout, null, 2) }}</pre></BaseDialog>

      <BaseDialog :show="Boolean(operationDetail)" :title="t('admin.funds.operations.detailTitle')" width="wide" @close="operationDetail = null">
        <dl v-if="operationDetail" class="grid gap-3 text-sm sm:grid-cols-[150px_minmax(0,1fr)]"><dt class="text-gray-500">{{ t('admin.funds.operations.operationNo') }}</dt><dd class="break-all">{{ operationDetail.operation_no }}</dd><dt class="text-gray-500">{{ t('admin.funds.confirm.account') }}</dt><dd class="break-all">{{ operationDetail.account_email }}</dd><dt class="text-gray-500">{{ t('admin.funds.operations.reference') }}</dt><dd>{{ operationDetail.external_ref || operationDetail.external_ref_masked || '-' }} <button v-if="operationDetail.external_ref_masked && !operationDetail.external_ref" type="button" class="btn btn-secondary btn-sm ml-2" :disabled="loading" @click="revealReference">{{ t('admin.funds.operations.revealReference') }}</button></dd><dt class="text-gray-500">{{ t('admin.funds.operations.balanceChange') }}</dt><dd class="tabular-nums">{{ operationDetail.balance_before ? money(operationDetail.balance_before) : '-' }} → {{ operationDetail.balance_after ? money(operationDetail.balance_after) : '-' }}</dd><dt class="text-gray-500">{{ t('admin.funds.operations.membershipEffect') }}</dt><dd>{{ operationDetail.membership_effect || '-' }}</dd></dl>
        <form v-if="operationDetail && canCorrect(operationDetail)" class="mt-5 border-t border-gray-100 pt-5 dark:border-dark-700" @submit.prevent="correctOperation"><h3 class="font-semibold text-gray-900 dark:text-white">{{ t('admin.funds.correction.title') }}</h3><div class="mt-3 grid gap-3 md:grid-cols-2"><input v-model.trim="correctionForm.email" class="input" :placeholder="t('admin.funds.correction.account')" /><input v-model.trim="correctionForm.reason" class="input" :placeholder="t('admin.funds.correction.reason')" /></div><button class="btn btn-danger mt-3" :disabled="loading" type="submit">{{ t('admin.funds.correction.submit') }}</button></form>
        <div v-if="operationDetail && canManagePendingCorrection(operationDetail)" class="mt-5 border-t border-gray-100 pt-5 dark:border-dark-700"><h3 class="font-semibold text-gray-900 dark:text-white">{{ t('admin.funds.correction.pendingTitle') }}</h3><p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.funds.correction.pendingDescription') }}</p><div class="mt-3 flex flex-wrap gap-2"><button type="button" class="btn btn-primary" :disabled="loading" @click="retryPendingCorrection">{{ t('admin.funds.correction.retry') }}</button><button type="button" class="btn btn-danger" :disabled="loading" @click="cancelPendingCorrection">{{ t('admin.funds.correction.cancel') }}</button></div></div>
      </BaseDialog>
      <TotpStepUpDialog :controller="fundStepUp" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { approveRefundRequest, cancelFundOperationCorrection, correctFundOperation, getFundOperation, getFundOperationSensitive, getRefundSensitivePayout, grantCompensation, grantGift, grantOfflineRecharge, listFundOperations, listRefundRequests, markRefundPaid, rejectRefundRequest, retryFundOperationCorrection, searchFundAccounts, type AdminFundAccount, type AdminFundOperation, type AdminFundOperationPage, type AdminFundRefundRequest, type AdminFundRefundRequestPage, type FundOperationKind, type FundOperationStatus } from '@/api/admin/funds'
import type { FundRefundStatus, WithdrawalCurrency } from '@/api/wallet'
import { extractApiErrorCode } from '@/utils/apiError'
import { isStepUpCancelled, useStepUp } from '@/composables/useStepUp'

const { t, te, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const fundStepUp = useStepUp()

type TabKey = 'refunds' | 'credits' | 'operations'
const tabs: TabKey[] = ['refunds', 'credits', 'operations']
const tabPaths: Record<TabKey, string> = { refunds: '/admin/funds/refunds', credits: '/admin/funds/credits', operations: '/admin/funds/operations' }
const refundStatuses: FundRefundStatus[] = ['pending_review', 'payout_pending', 'paid', 'rejected', 'canceled']
const operationKinds: FundOperationKind[] = ['offline_recharge', 'ops_gift', 'compensation', 'refund', 'reversal', 'account_correction']
const operationStatuses: FundOperationStatus[] = ['completed', 'pending', 'canceled', 'pending_insufficient_balance']
const activeTab = ref<TabKey>('refunds')
const loading = ref(false)
const message = ref<{ type: 'success' | 'error'; text: string } | null>(null)
const selectedAccount = ref<AdminFundAccount | null>(null)
const accountSearch = ref('')
const accountResults = ref<AdminFundAccount[]>([])
const showCreditConfirmation = ref(false)
const confirmationEmail = ref('')
const rejectRequestNo = ref('')
const payoutRequest = ref<AdminFundRefundRequest | null>(null)
const sensitivePayout = ref<Record<string, unknown> | null>(null)
const operationDetail = ref<AdminFundOperation | null>(null)
let searchVersion = 0

const refundQuery = reactive({ status: 'all' as FundRefundStatus | 'all', account: '' })
const operationQuery = reactive({ kind: 'all' as FundOperationKind | 'all', status: 'all' as FundOperationStatus | 'all', q: '' })
const creditForm = reactive({ kind: 'ops_gift' as 'ops_gift' | 'compensation' | 'offline_recharge', amount: '', external_ref: '', reason: '' })
const correctionForm = reactive({ email: '', reason: '' })
const rejectForm = reactive({ reason: '', note: '' })
const paidForm = reactive({ external_txn_id: '', paid_amount: '', paid_currency: 'USD' as WithdrawalCurrency, payout_fx_rate: '1', note: '' })
const refundPage = ref<AdminFundRefundRequestPage>({ items: [], total: 0, page: 1, page_size: 20, pages: 1 })
const operationPage = ref<AdminFundOperationPage>({ items: [], total: 0, page: 1, page_size: 20, pages: 1 })

function show(type: 'success' | 'error', text: string) { message.value = { type, text } }
function money(value: string | number | undefined, currency = 'USD') { return new Intl.NumberFormat(locale.value, { style: 'currency', currency, minimumFractionDigits: 2, maximumFractionDigits: 8 }).format(Number(value || 0)) }
function dateTime(value?: string) { return value ? new Date(value).toLocaleString(locale.value) : '-' }
function refundStatusClass(status: FundRefundStatus) { return status === 'paid' ? 'bg-emerald-50 text-emerald-700' : status === 'rejected' || status === 'canceled' ? 'bg-gray-100 text-gray-600' : status === 'payout_pending' ? 'bg-blue-50 text-blue-700' : 'bg-amber-50 text-amber-700' }
function operationStatusClass(status: FundOperationStatus) { return status === 'completed' ? 'bg-emerald-50 text-emerald-700' : status === 'pending_insufficient_balance' ? 'bg-amber-50 text-amber-700' : 'bg-gray-100 text-gray-600' }
function refundTypeLabel(value: string) { const key = `admin.funds.refundTypes.${value}`; return te(key) ? t(key) : t('admin.funds.table.type') }
function localError(error: unknown, fallback: string) { const code = extractApiErrorCode(error); const key = code ? `admin.funds.errors.${code}` : ''; return key && te(key) ? t(key) : fallback }
function handleError(error: unknown, fallback: string) { if (!isStepUpCancelled(error)) show('error', localError(error, fallback)) }
function validCredit() { return Boolean(selectedAccount.value) && /^(?:0|[1-9]\d*)(?:\.\d{1,8})?$/.test(creditForm.amount) && Number(creditForm.amount) > 0 && creditForm.reason.trim().length >= 3 && (creditForm.kind !== 'offline_recharge' || Boolean(creditForm.external_ref.trim())) }

async function loadRefunds() { loading.value = true; try { refundPage.value = await listRefundRequests({ ...refundQuery, page: refundPage.value.page, page_size: 20 }) } catch (error) { handleError(error, t('admin.funds.messages.loadFailed')) } finally { loading.value = false } }
async function loadOperations() { loading.value = true; try { operationPage.value = await listFundOperations({ ...operationQuery, page: operationPage.value.page, page_size: 20 }) } catch (error) { handleError(error, t('admin.funds.messages.loadFailed')) } finally { loading.value = false } }
async function refreshActive() { if (activeTab.value === 'refunds') await loadRefunds(); if (activeTab.value === 'operations') await loadOperations() }
async function setTab(tab: TabKey) { if (route.path !== tabPaths[tab]) await router.push(tabPaths[tab]); else { activeTab.value = tab; await refreshActive() } }
async function searchAccounts() { const version = ++searchVersion; if (accountSearch.value.trim().length < 2) { accountResults.value = []; return }; try { const found = await searchFundAccounts(accountSearch.value.trim()); if (version === searchVersion) accountResults.value = found } catch { if (version === searchVersion) accountResults.value = [] } }
function selectAccount(account: AdminFundAccount) { selectedAccount.value = account; accountSearch.value = account.email; accountResults.value = [] }
function openCreditConfirmation() { if (!validCredit()) { show('error', t('admin.funds.validation.creditInvalid')); return }; confirmationEmail.value = ''; showCreditConfirmation.value = true }
async function submitCredit() { if (!selectedAccount.value || !validCredit() || confirmationEmail.value !== selectedAccount.value.email) return; loading.value = true; try { const input = { account_email: selectedAccount.value.email, amount: creditForm.amount, reason: creditForm.reason }; const record = await fundStepUp.run(() => creditForm.kind === 'offline_recharge' ? grantOfflineRecharge({ ...input, external_ref: creditForm.external_ref }) : creditForm.kind === 'compensation' ? grantCompensation(input) : grantGift(input)); showCreditConfirmation.value = false; creditForm.amount = ''; creditForm.reason = ''; creditForm.external_ref = ''; show('success', t('admin.funds.messages.creditCreated', { operation: record.operation_no })); await loadOperations() } catch (error) { handleError(error, t('admin.funds.messages.grantFailed')) } finally { loading.value = false } }
async function approve(requestNo: string) { loading.value = true; try { await fundStepUp.run(() => approveRefundRequest(requestNo)); show('success', t('admin.funds.messages.approved')); await loadRefunds() } catch (error) { handleError(error, t('admin.funds.messages.actionFailed')) } finally { loading.value = false } }
function openReject(requestNo: string) { rejectRequestNo.value = requestNo; rejectForm.reason = ''; rejectForm.note = '' }
async function reject() { if (!rejectRequestNo.value || rejectForm.reason.trim().length < 3) return; loading.value = true; try { await fundStepUp.run(() => rejectRefundRequest(rejectRequestNo.value, { reason: rejectForm.reason, note: rejectForm.note })); rejectRequestNo.value = ''; show('success', t('admin.funds.messages.rejected')); await loadRefunds() } catch (error) { handleError(error, t('admin.funds.messages.actionFailed')) } finally { loading.value = false } }
function openMarkPaid(item: AdminFundRefundRequest) { payoutRequest.value = item; paidForm.external_txn_id = ''; paidForm.paid_amount = item.amount; paidForm.paid_currency = (item.currency === 'CNY' ? 'CNY' : 'USD') as WithdrawalCurrency; paidForm.payout_fx_rate = '1'; paidForm.note = '' }
async function submitMarkPaid() { if (!payoutRequest.value || !paidForm.external_txn_id || !paidForm.paid_amount) return; loading.value = true; try { await fundStepUp.run(() => markRefundPaid(payoutRequest.value!.request_no, paidForm)); payoutRequest.value = null; show('success', t('admin.funds.messages.paid')); await loadRefunds() } catch (error) { handleError(error, t('admin.funds.messages.actionFailed')) } finally { loading.value = false } }
async function openSensitivePayout(requestNo: string) { loading.value = true; try { sensitivePayout.value = await fundStepUp.run(() => getRefundSensitivePayout(requestNo)) } catch (error) { handleError(error, t('admin.funds.messages.sensitiveFailed')) } finally { loading.value = false } }
async function openOperation(operationNo: string) { loading.value = true; try { operationDetail.value = await getFundOperation(operationNo); correctionForm.email = ''; correctionForm.reason = '' } catch (error) { handleError(error, t('admin.funds.messages.loadFailed')) } finally { loading.value = false } }
async function revealReference() { if (!operationDetail.value) return; loading.value = true; try { operationDetail.value = await fundStepUp.run(() => getFundOperationSensitive(operationDetail.value!.operation_no)) } catch (error) { handleError(error, t('admin.funds.messages.actionFailed')) } finally { loading.value = false } }
function canCorrect(operation: AdminFundOperation) { return operation.status === 'completed' && ['ops_gift', 'compensation'].includes(operation.operation_kind) }
function canManagePendingCorrection(operation: AdminFundOperation) { return operation.operation_kind === 'account_correction' && operation.status === 'pending_insufficient_balance' }
async function correctOperation() { if (!operationDetail.value || !correctionForm.email || correctionForm.reason.trim().length < 3) { show('error', t('admin.funds.validation.correctionInvalid')); return }; loading.value = true; try { const result = await fundStepUp.run(() => correctFundOperation(operationDetail.value!.operation_no, { correct_account_email: correctionForm.email, reason: correctionForm.reason })); show('success', result.status === 'pending_insufficient_balance' ? t('admin.funds.messages.correctionPending') : t('admin.funds.messages.correctionCompleted')); operationDetail.value = null; await loadOperations() } catch (error) { handleError(error, t('admin.funds.messages.correctionFailed')) } finally { loading.value = false } }
async function retryPendingCorrection() { if (!operationDetail.value) return; loading.value = true; try { operationDetail.value = await fundStepUp.run(() => retryFundOperationCorrection(operationDetail.value!.operation_no)); show('success', t('admin.funds.messages.correctionCompleted')); await loadOperations() } catch (error) { handleError(error, t('admin.funds.messages.correctionFailed')) } finally { loading.value = false } }
async function cancelPendingCorrection() { if (!operationDetail.value) return; loading.value = true; try { operationDetail.value = await fundStepUp.run(() => cancelFundOperationCorrection(operationDetail.value!.operation_no)); show('success', t('admin.funds.messages.correctionCanceled')); await loadOperations() } catch (error) { handleError(error, t('admin.funds.messages.correctionFailed')) } finally { loading.value = false } }
async function changeRefundPage(page: number) { if (page < 1 || page > refundPage.value.pages) return; refundPage.value = { ...refundPage.value, page }; await loadRefunds() }

watch(() => route.params.tab, (tab) => { const value = tab === 'classification' ? 'operations' : tab; if (value === 'refunds' || value === 'credits' || value === 'operations') { activeTab.value = value; void refreshActive() }; if (tab === 'classification') void router.replace('/admin/funds/operations') }, { immediate: true })
onMounted(() => { void refreshActive() })
</script>
