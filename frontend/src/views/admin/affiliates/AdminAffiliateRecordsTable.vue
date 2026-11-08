<template>
  <AppLayout>
    <div v-if="props.type === 'invites'" class="mb-4 flex flex-wrap gap-2" role="group" :aria-label="t('admin.affiliateOperations.inviteView')">
      <button v-for="view in (['relations', 'promoters'] as const)" :key="view" type="button" class="btn" :class="mode === view ? 'btn-primary' : 'btn-secondary'" :aria-pressed="mode === view" @click="setMode(view)">{{ t(`admin.affiliateOperations.${view}`) }}</button>
    </div>
    <AdminAffiliatePromoters v-if="props.type === 'invites' && mode === 'promoters'" :key="promotersVersion" @open="openUserOverview" />
    <TablePageLayout v-else>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="relative w-full md:w-80">
            <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input v-model="filters.search" type="text" class="input pl-10" :placeholder="t('admin.affiliates.records.searchPlaceholder')" @input="debounceLoad" />
          </div>
          <input v-model="filters.start_at" type="date" class="input w-full sm:w-44" :title="t('admin.affiliates.records.startAt')" @change="reloadFromFirstPage" />
          <input v-model="filters.end_at" type="date" class="input w-full sm:w-44" :title="t('admin.affiliates.records.endAt')" @change="reloadFromFirstPage" />
          <label class="text-xs text-gray-500">{{ t('admin.affiliateOperations.exactUserId') }}<input v-model="filters.user_id" type="number" min="1" step="1" class="input w-full sm:w-36" data-test="exact-user-id" @change="reloadFromFirstPage" /></label>
          <label v-if="props.type !== 'transfers'" class="text-xs text-gray-500">{{ t('admin.affiliateOperations.exactInviteeId') }}<input v-model="filters.invitee_id" type="number" min="1" step="1" class="input w-full sm:w-36" @change="reloadFromFirstPage" /></label>
          <label v-if="props.type === 'rebates'" class="text-xs text-gray-500">{{ t('admin.affiliateOperations.exactOrderId') }}<input v-model="filters.source_order_id" type="number" min="1" step="1" class="input w-full sm:w-36" @change="reloadFromFirstPage" /></label>
          <select v-if="props.type === 'transfers'" v-model="filters.action" class="input w-full sm:w-44" :aria-label="t('admin.affiliates.records.outflowType')" @change="reloadFromFirstPage"><option value="">{{ t('admin.affiliateOperations.allTypes') }}</option><option value="transfer">{{ t('admin.affiliates.outflowTypes.transfer') }}</option><option value="withdraw">{{ t('admin.affiliates.outflowTypes.withdraw') }}</option></select>
          <template v-if="props.type === 'rebates'"><label class="text-xs text-gray-500">{{ t('admin.affiliateOperations.source') }}<select v-model="filters.source_type" class="input mt-1" @change="reloadFromFirstPage"><option value="">{{ t('admin.affiliateOperations.allTypes') }}</option><option v-for="source in ['regular','campaign','other']" :key="source" :value="source">{{ t(`admin.affiliateOperations.sources.${source}`) }}</option></select></label><label class="text-xs text-gray-500">{{ t('admin.affiliateOperations.currency') }}<select v-model="filters.currency" class="input mt-1" @change="reloadFromFirstPage"><option value="">{{ t('admin.affiliateOperations.allTypes') }}</option><option value="USD">USD</option><option value="CNY">CNY</option></select></label><label class="text-xs text-gray-500">{{ t('admin.affiliateOperations.status') }}<select v-model="filters.status" class="input mt-1" @change="reloadFromFirstPage"><option value="">{{ t('admin.affiliateOperations.allStatuses') }}</option><option v-for="status in ['frozen','available','revoked']" :key="status" :value="status">{{ t(`admin.affiliateOperations.fundStatuses.${status}`) }}</option></select></label></template>
          <button class="btn btn-secondary px-2 md:px-3" :disabled="loading" :title="t('common.refresh')" @click="loadRecords">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
          <button type="button" class="btn btn-secondary" data-test="affiliate-export" :disabled="loading || exporting || loadError || !summary || summary.record_count > 10000" @click="exportSelection"><Icon name="download" size="sm" />{{ t('admin.affiliateOperations.export') }}</button>
          <button
            v-if="props.type === 'transfers'"
            type="button"
            class="btn btn-primary ml-auto"
            data-test="affiliate-withdraw-open"
            @click="withdrawDialog = true"
          >
            {{ t('admin.affiliates.withdraw.button') }}
          </button>
        </div>
        <p class="mt-2 text-xs text-gray-500">{{ t('admin.affiliateOperations.exportLimit') }}</p>
        <div v-if="loadError" class="mt-3 text-sm text-red-600" role="alert">{{ t(invalidIds ? 'admin.affiliateOperations.invalidId' : 'admin.affiliateOperations.loadFailed') }} <button type="button" class="btn btn-secondary" @click="loadRecords">{{ t('common.retry') }}</button></div>
        <div v-if="summary" class="mt-3 flex flex-wrap gap-x-5 gap-y-2 text-sm" data-test="affiliate-summary"><span>{{ t('admin.affiliateOperations.filteredRecords') }}: {{ summary.record_count }}</span><span v-for="amount in summary.amounts" :key="`${amount.source_type}-${amount.currency}`" class="tabular-nums">{{ t(`admin.affiliateOperations.sources.${amount.source_type}`) }} · {{ amount.currency }} {{ formatPreciseAmount(amount.total_amount) }}<template v-if="amount.frozen_amount != null"> · {{ t('admin.affiliateOperations.frozen') }} {{ formatPreciseAmount(amount.frozen_amount) }}</template></span><span v-if="!summary.amounts?.length && summary.total_amount != null">{{ summary.currency }} {{ formatPreciseAmount(summary.total_amount) }}</span></div>
        <p v-if="props.type === 'invites'" class="mt-2 text-xs text-gray-500">{{ t('admin.affiliateOperations.inviteSummaryScope') }}</p>
        <p v-if="summaryError" class="mt-2 text-sm text-red-600" role="alert">{{ t('admin.affiliateOperations.summaryFailed') }}</p>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="records"
          :loading="loading"
          :server-side-sort="true"
          default-sort-key="created_at"
          default-sort-order="desc"
          :sort-storage-key="sortStorageKey"
          @sort="handleSort"
        >
          <template #cell-inviter="{ row }">
            <UserCell
              :id="row.inviter_id"
              :email="row.inviter_email"
              :username="row.inviter_username"
              :clickable="props.type !== 'transfers'"
              @open="openUserOverview"
            />
          </template>
          <template #cell-invitee="{ row }">
            <span v-if="row.invitee_id == null" class="text-sm text-gray-400 dark:text-dark-500">-</span>
            <UserCell
              v-else
              :id="row.invitee_id"
              :email="row.invitee_email"
              :username="row.invitee_username"
              :clickable="props.type !== 'transfers'"
              @open="openUserOverview"
            />
          </template>
          <template #cell-user="{ row }">
            <UserCell
              :id="row.user_id"
              :email="row.user_email"
              :username="row.username"
              :clickable="true"
              @open="openUserOverview"
            />
          </template>
          <template #cell-aff_code="{ row }">
            <span class="font-mono text-sm text-gray-700 dark:text-gray-300">{{ row.aff_code || '-' }}</span>
          </template>
          <template #cell-order="{ row }">
            <span v-if="row.order_id == null" class="text-sm text-gray-400 dark:text-dark-500">-</span>
            <div v-else class="space-y-0.5">
              <div class="font-mono text-sm text-gray-900 dark:text-white">#{{ row.order_id }}</div>
              <div class="max-w-56 truncate text-sm text-gray-500 dark:text-dark-400">{{ row.out_trade_no }}</div>
            </div>
          </template>
          <template #cell-payment_type="{ row }">
            <template v-if="row.payment_type">{{ t('payment.methods.' + row.payment_type, row.payment_type) }}</template>
            <span v-else class="text-sm text-gray-400 dark:text-dark-500">-</span>
          </template>
          <template #cell-order_status="{ row }">
            <OrderStatusBadge v-if="row.order_status" :status="row.order_status" />
            <span v-else class="text-sm text-gray-400 dark:text-dark-500">-</span>
          </template>
          <template #cell-total_rebate="{ row }">
            <AmountText :value="row.total_rebate" />
          </template>
          <template #cell-order_amount="{ row }">
            <NullableAmountText :value="row.order_amount" />
          </template>
          <template #cell-pay_amount="{ row }">
            <span v-if="row.pay_amount == null" class="text-sm text-gray-400 dark:text-dark-500">-</span>
            <span v-else class="text-sm text-gray-900 dark:text-white">¥{{ formatAmount(row.pay_amount) }}</span>
          </template>
          <template #cell-rebate_amount="{ row }">
            <span class="font-semibold tabular-nums">{{ row.currency || 'USD' }} {{ formatPreciseAmount(row.rebate_amount) }}</span>
          </template>
          <template #cell-source="{ row }"><span>{{ t(`admin.affiliateOperations.sources.${row.source_type || 'regular'}`) }}</span><div v-if="row.campaign_id" class="text-xs text-gray-500">#{{ row.campaign_id }}</div></template>
          <template #cell-status="{ row }"><span>{{ t(`admin.affiliateOperations.fundStatuses.${row.status || 'available'}`) }}</span><div v-if="row.frozen_until" class="text-xs text-gray-500">{{ formatDateTime(row.frozen_until) }}</div></template>
          <template #cell-proof="{ row }"><span>{{ row.external_reference || '-' }}</span><div v-if="row.paid_at" class="text-xs text-gray-500">{{ formatDateTime(row.paid_at) }}</div><div v-if="row.note" class="whitespace-normal text-xs text-gray-500">{{ row.note }}</div></template>
          <template #cell-action="{ row }">
            <span :class="['badge whitespace-nowrap', row.action === 'withdraw' ? 'badge-warning' : 'badge-primary']">
              {{ outflowTypeLabel(row.action) }}
            </span>
          </template>
          <template #cell-amount="{ row }">
            <AmountText :value="row.amount" strong />
          </template>
          <template #cell-balance_after="{ row }">
            <NullableAmountText :value="row.balance_after" />
          </template>
          <template #cell-available_quota_after="{ row }">
            <NullableAmountText :value="row.available_quota_after" />
          </template>
          <template #cell-frozen_quota_after="{ row }">
            <NullableAmountText :value="row.frozen_quota_after" />
          </template>
          <template #cell-history_quota_after="{ row }">
            <NullableAmountText :value="row.history_quota_after" />
          </template>
          <template #cell-created_at="{ row }">
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(row.created_at) }}</span>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <AffiliateOfflineWithdrawDialog
      v-if="props.type === 'transfers'"
      :show="withdrawDialog"
      @close="withdrawDialog = false"
      @success="handleWithdrawSuccess"
    />

    <AdminAffiliateUserDialog v-if="selectedUserId" :show="userDetailDialog" :user-id="selectedUserId" :invitee-id="selectedInviteeId" :initial-tab="String(route.query.detail || 'invites')" @close="userDetailDialog = false" @changed="handleSettingsChanged" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, reactive, ref, watch, type PropType } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { saveAs } from 'file-saver'
import AdminAffiliatePromoters from './AdminAffiliatePromoters.vue'
import AdminAffiliateUserDialog from './AdminAffiliateUserDialog.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import OrderStatusBadge from '@/components/payment/OrderStatusBadge.vue'
import AffiliateOfflineWithdrawDialog from './AffiliateOfflineWithdrawDialog.vue'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { affiliatesAPI, type AffiliateInviteRecord, type AffiliateRebateRecord, type AffiliateTransferRecord, type ListAffiliateRecordsParams, type AffiliateRecordsSummary } from '@/api/admin/affiliates'
import type { PaginatedResponse } from '@/types'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatDateTime as formatDisplayDateTime } from '@/utils/format'

type RecordType = 'invites' | 'rebates' | 'transfers'
type AffiliateRecord = AffiliateInviteRecord | AffiliateRebateRecord | AffiliateTransferRecord

const props = defineProps<{
  type: RecordType
}>()

const { t } = useI18n()
const appStore = useAppStore()
const route = useRoute()
const router = useRouter()
const mode = computed(() => route.query.mode === 'promoters' ? 'promoters' : 'relations')
function setMode(value: string) { void router.replace({ query: { ...route.query, mode: value } }) }
const loading = ref(false)
const records = ref<AffiliateRecord[]>([])
const filters = reactive({ search: '', start_at: '', end_at: '', user_id: String(route.query.user_id || route.query.inviter_id || ''), invitee_id: String(route.query.invitee_id || ''), source_order_id: String(route.query.source_order_id || ''), action: String(route.query.action || ''), source_type: String(route.query.source_type || ''), status: String(route.query.status || ''), currency: String(route.query.currency || '') })
const summary = ref<AffiliateRecordsSummary | null>(null)
const summaryError = ref(false)
const loadError = ref(false)
const exporting = ref(false)
const promotersVersion = ref(0)
const invalidIds = computed(() => [filters.user_id, ...(props.type !== 'transfers' ? [filters.invitee_id] : []), ...(props.type === 'rebates' ? [filters.source_order_id] : [])].some(value => value !== '' && !positiveId(value)))
let loadSequence = 0
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
const selectedUserId = ref<number | null>(null)
const selectedInviteeId = ref<number | undefined>()
const userDetailDialog = ref(false)
const withdrawDialog = ref(false)
let debounceTimer: ReturnType<typeof setTimeout> | null = null

const columns = computed<Column[]>(() => {
  if (props.type === 'invites') {
    return [
      { key: 'inviter', label: t('admin.affiliates.records.inviter'), sortable: true },
      { key: 'invitee', label: t('admin.affiliates.records.invitee'), sortable: true },
      { key: 'aff_code', label: t('admin.affiliates.records.affCode'), sortable: true },
      { key: 'total_rebate', label: t('admin.affiliateOperations.regularRebateTotal'), sortable: true },
      { key: 'created_at', label: t('admin.affiliates.records.invitedAt'), sortable: true },
    ]
  }
  if (props.type === 'rebates') {
    return [
      { key: 'order', label: t('admin.affiliates.records.order'), sortable: true },
      { key: 'inviter', label: t('admin.affiliates.records.inviter'), sortable: true },
      { key: 'invitee', label: t('admin.affiliates.records.invitee'), sortable: true },
      { key: 'order_amount', label: t('admin.affiliates.records.orderAmount'), sortable: true },
      { key: 'pay_amount', label: t('admin.affiliates.records.payAmount'), sortable: true },
      { key: 'rebate_amount', label: t('admin.affiliates.records.rebateAmount') },
      { key: 'source', label: t('admin.affiliateOperations.source') },
      { key: 'status', label: t('admin.affiliateOperations.status') },
      { key: 'payment_type', label: t('admin.affiliates.records.paymentType'), sortable: true },
      { key: 'order_status', label: t('admin.affiliates.records.orderStatus'), sortable: true },
      { key: 'created_at', label: t('admin.affiliates.records.rebatedAt'), sortable: true },
    ]
  }
  return [
    { key: 'user', label: t('admin.affiliates.records.user'), sortable: true },
    { key: 'action', label: t('admin.affiliates.records.outflowType'), sortable: true },
    { key: 'amount', label: t('admin.affiliates.records.transferAmount'), sortable: true },
    { key: 'proof', label: t('admin.affiliateOperations.paymentProof') },
    { key: 'balance_after', label: t('admin.affiliates.records.balanceAfter'), sortable: true },
    { key: 'available_quota_after', label: t('admin.affiliates.records.availableQuotaAfter'), sortable: true },
    { key: 'frozen_quota_after', label: t('admin.affiliates.records.frozenQuotaAfter'), sortable: true },
    { key: 'history_quota_after', label: t('admin.affiliates.records.historyQuotaAfter'), sortable: true },
    { key: 'created_at', label: t('admin.affiliates.records.transferredAt'), sortable: true },
  ]
})

const sortStorageKey = computed(() => `admin-affiliate-${props.type}-table-sort`)

function loadInitialSortState(): { sort_by: string; sort_order: 'asc' | 'desc' } {
  const fallback = { sort_by: 'created_at', sort_order: 'desc' as 'asc' | 'desc' }
  try {
    const raw = localStorage.getItem(sortStorageKey.value)
    if (!raw) return fallback
    const parsed = JSON.parse(raw) as { key?: string; order?: string }
    const key = typeof parsed.key === 'string' ? parsed.key : ''
    if (!columns.value.some((column) => column.key === key && column.sortable)) return fallback
    return {
      sort_by: key,
      sort_order: parsed.order === 'asc' ? 'asc' : 'desc',
    }
  } catch {
    return fallback
  }
}

const sortState = reactive(loadInitialSortState())

function userTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone
  } catch {
    return 'UTC'
  }
}

function buildParams(): ListAffiliateRecordsParams {
  return {
    page: pagination.page,
    page_size: pagination.page_size,
    search: filters.search.trim() || undefined,
    start_at: filters.start_at || undefined,
    end_at: filters.end_at || undefined,
    sort_by: sortState.sort_by,
    sort_order: sortState.sort_order,
    timezone: userTimezone(),
    ...(props.type === 'transfers' ? { user_id: positiveId(filters.user_id) } : { inviter_id: positiveId(filters.user_id), invitee_id: positiveId(filters.invitee_id) }),
    source_order_id: props.type === 'rebates' ? positiveId(filters.source_order_id) : undefined,
    action: props.type === 'transfers' && ['transfer', 'withdraw'].includes(filters.action) ? filters.action as 'transfer' | 'withdraw' : undefined,
    source_type: props.type === 'rebates' ? filters.source_type || undefined : undefined,
    currency: props.type === 'rebates' ? filters.currency || undefined : undefined,
    status: props.type === 'rebates' ? filters.status || undefined : undefined,
  }
}
function positiveId(value: string) { const id = Number(value); return Number.isSafeInteger(id) && id > 0 ? id : undefined }

async function fetchRecords(params: ListAffiliateRecordsParams): Promise<PaginatedResponse<AffiliateRecord>> {
  if (props.type === 'invites') {
    return affiliatesAPI.listInviteRecords(params)
  }
  if (props.type === 'rebates') {
    return affiliatesAPI.listRebateRecords(params)
  }
  return affiliatesAPI.listTransferRecords(params)
}

async function loadRecords() {
  const seq = ++loadSequence
  loading.value = true
  loadError.value = false
  summaryError.value = false
  summary.value = null
  if (invalidIds.value) { loadError.value = true; records.value = []; pagination.total = 0; loading.value = false; return }
  const params = buildParams()
  try {
    const [res] = await Promise.all([fetchRecords(params), affiliatesAPI.getRecordsSummary(props.type, params).then(value => { if (seq === loadSequence) summary.value = value }).catch(() => { if (seq === loadSequence) summaryError.value = true })])
    if (seq !== loadSequence) return
    records.value = res.items || []
    pagination.total = res.total || 0
  } catch (error) {
    if (seq !== loadSequence) return
    records.value = []
    pagination.total = 0
    loadError.value = true
    appStore.showError(extractI18nErrorMessage(error, t, 'admin.affiliates.errors', t('common.error')))
  } finally {
    if (seq === loadSequence) loading.value = false
  }
}
async function exportSelection() {
  if (exporting.value || !summary.value || summary.value.record_count > 10000) return
  exporting.value = true
  try {
    const blob = await affiliatesAPI.exportRecords(props.type, buildParams())
    saveAs(blob, `affiliate-${props.type}-${new Date().toISOString().slice(0, 10)}.csv`)
  } catch (error) {
    const status = (error as { response?: { status?: number } }).response?.status
    appStore.showError(t(status === 422 ? 'admin.affiliateOperations.exportLimit' : 'admin.affiliateOperations.exportFailed'))
  } finally { exporting.value = false }
}
function formatPreciseAmount(value: number) { return new Intl.NumberFormat(undefined, { maximumFractionDigits: 8 }).format(value) }

function debounceLoad() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => reloadFromFirstPage(), 300)
}

function reloadFromFirstPage() {
  pagination.page = 1
  void loadRecords()
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadRecords()
}

function handlePageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  void loadRecords()
}

function handleSort(key: string, order: 'asc' | 'desc') {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  void loadRecords()
}

function handleWithdrawSuccess() {
  withdrawDialog.value = false
  reloadFromFirstPage()
}
function handleSettingsChanged() {
  promotersVersion.value++
  if (props.type !== 'invites' || mode.value !== 'promoters') void loadRecords()
}

function outflowTypeLabel(action: string | null | undefined): string {
  return action === 'withdraw'
    ? t('admin.affiliates.outflowTypes.withdraw')
    : t('admin.affiliates.outflowTypes.transfer')
}

function formatAmount(value: number | null | undefined): string {
  return Number(value || 0).toFixed(2)
}


function formatDateTime(value: string | null | undefined): string {
  return value ? formatDisplayDateTime(value) : '-'
}

async function openUserOverview(userId: number, inviteeId?: number) {
  if (!userId) return
  selectedUserId.value = userId
  selectedInviteeId.value = inviteeId
  userDetailDialog.value = true
}

const UserCell = defineComponent({
  props: {
    id: { type: Number, required: true },
    email: { type: String, default: '' },
    username: { type: String, default: '' },
    clickable: { type: Boolean, default: false },
  },
  emits: ['open'],
  setup(cellProps, { emit }) {
    return () => h('div', { class: 'space-y-0.5' }, [
      h('div', { class: 'font-mono text-sm text-gray-900 dark:text-white' }, `#${cellProps.id}`),
      h(cellProps.clickable ? 'button' : 'div', {
        class: cellProps.clickable
          ? 'max-w-56 truncate text-left text-sm font-medium text-primary-600 hover:text-primary-700 hover:underline dark:text-primary-400 dark:hover:text-primary-300'
          : 'max-w-56 truncate text-sm text-gray-700 dark:text-gray-300',
        type: cellProps.clickable ? 'button' : undefined,
        onClick: cellProps.clickable ? () => emit('open', cellProps.id) : undefined,
      }, cellProps.email || '-'),
      h('div', { class: 'max-w-56 truncate text-sm text-gray-500 dark:text-dark-400' }, cellProps.username || '-'),
    ])
  },
})

const AmountText = defineComponent({
  props: {
    value: { type: Number, default: 0 },
    strong: { type: Boolean, default: false },
  },
  setup(amountProps) {
    return () => h('span', {
      class: amountProps.strong
        ? 'text-sm font-semibold text-emerald-600 dark:text-emerald-400'
        : 'text-sm text-gray-900 dark:text-white',
    }, `$${formatAmount(amountProps.value)}`)
  },
})

const NullableAmountText = defineComponent({
  props: {
    value: { type: Number as PropType<number | null | undefined>, default: null },
  },
  setup(amountProps) {
    return () => {
      const value = amountProps.value
      if (value === null || value === undefined) {
        return h('span', { class: 'text-sm text-gray-400 dark:text-dark-500' }, '-')
      }
      return h(AmountText, { value })
    }
  },
})

onMounted(() => {
  if (props.type !== 'invites' || mode.value !== 'promoters') void loadRecords()
  if (route.query.mode === 'promoters' && positiveId(String(route.query.user_id || ''))) void openUserOverview(Number(route.query.user_id), positiveId(String(route.query.invitee_id || '')))
})
watch(() => [route.query.user_id, route.query.inviter_id, route.query.invitee_id, route.query.source_order_id, route.query.action, route.query.source_type, route.query.status, route.query.currency], () => {
  filters.user_id = String(route.query.user_id || route.query.inviter_id || '')
  filters.invitee_id = String(route.query.invitee_id || '')
  filters.source_order_id = String(route.query.source_order_id || '')
  filters.action = String(route.query.action || '')
  filters.source_type = String(route.query.source_type || '')
  filters.status = String(route.query.status || '')
  filters.currency = String(route.query.currency || '')
  reloadFromFirstPage()
})
</script>
