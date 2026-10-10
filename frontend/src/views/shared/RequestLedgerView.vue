<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <p class="text-sm text-gray-600 dark:text-gray-400">{{ t('requestLedger.notice') }}</p>
        <RouterLink :to="{ name: admin ? 'AdminUsage' : 'Usage', query: languageQuery() }" class="btn btn-secondary">{{ t('requestLedger.back') }}</RouterLink>
      </div>
      <form class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4" @submit.prevent="applyFilters">
        <label class="input-label">{{ t('requestLedger.privateId') }}<input v-model.trim="filters.private_id" name="private_id" class="input mt-1" maxlength="36" pattern="[0-9a-fA-F-]{36}" /></label>
        <label v-for="axis in axes" :key="axis.field" :for="axis.field" class="input-label">{{ t(`requestLedger.${axis.label}`) }}<Select :id="axis.field" v-model="filters[axis.field]" class="mt-1" :options="options(axis.values)" /></label>
        <label v-if="admin" class="input-label">{{ t('requestLedger.userId') }}<input v-model="filters.user_id" name="user_id" class="input mt-1" type="number" min="1" step="1" /></label>
        <label class="input-label">{{ t('requestLedger.keyId') }}<input v-model="filters.api_key_id" name="api_key_id" class="input mt-1" type="number" min="1" step="1" /></label>
        <label v-if="admin" class="input-label">{{ t('requestLedger.accountId') }}<input v-model="filters.account_id" name="account_id" class="input mt-1" type="number" min="1" step="1" /></label>
        <label class="input-label">{{ t('requestLedger.start') }}<input v-model="filters.start_date" name="start_date" class="input mt-1" type="datetime-local" /></label>
        <label class="input-label">{{ t('requestLedger.end') }}<input v-model="filters.end_date" name="end_date" class="input mt-1" type="datetime-local" /></label>
        <div class="flex flex-wrap items-end gap-2 sm:col-span-2 xl:col-span-4">
          <button class="btn btn-primary" :disabled="loading" type="submit">{{ t('requestLedger.filter') }}</button>
          <button class="btn btn-secondary" type="button" :disabled="loading" data-testid="ledger-refresh" @click="load"> <Icon name="refresh" size="sm" class="mr-2" />{{ t('requestLedger.refresh') }}</button>
          <button class="btn btn-secondary" type="button" :disabled="loading" @click="reset">{{ t('requestLedger.reset') }}</button>
        </div>
      </form>
      <div v-if="error" role="alert" class="flex flex-wrap items-center gap-3 text-sm text-red-600 dark:text-red-400">
        <span>{{ error }}</span><button type="button" class="btn btn-secondary" data-testid="ledger-retry" @click="load">{{ t('requestLedger.retry') }}</button>
      </div>
      <section :aria-busy="loading" :aria-label="t('requestLedger.title')">
        <DataTable v-if="!error" :data="page.items" :columns="columns" :loading="loading" row-key="id">
          <template #cell-request="{ row }"><div class="space-y-1"><span class="block break-all">{{ row.method }} {{ row.route }}</span><span class="block text-xs tabular-nums text-gray-500">{{ date(row.started_at) }}</span><span class="block font-mono text-xs">{{ row.id }}</span><span v-if="row.turn_no" class="block text-xs">WS · {{ row.turn_no }}</span></div></template>
          <template #cell-identity="{ row }"><div class="space-y-1"><span>{{ row.user_id ? `${t('requestLedger.userId')} ${row.user_id}` : t('requestLedger.anonymous') }}</span><span v-if="row.api_key_id" class="block text-xs text-gray-500">{{ t('requestLedger.keyId') }} {{ row.api_key_id }}</span></div></template>
          <template #cell-execution_state="{ row }"><span :class="stateClass(row.execution_state)">{{ state(row.execution_state) }}</span><span v-if="row.http_status" class="ml-2 text-xs tabular-nums text-gray-500">{{ row.http_status }}</span></template>
          <template #cell-usage_state="{ row }"><span :class="stateClass(row.usage_state)">{{ state(row.usage_state) }}</span></template>
          <template #cell-settlement_state="{ row }"><span :class="stateClass(row.settlement_state)">{{ state(row.settlement_state) }}</span></template>
          <template #cell-actions="{ row }"><button type="button" class="btn btn-secondary" @click="open(row.id)">{{ t('requestLedger.detail') }}</button></template>
          <template #empty><p class="text-sm text-gray-500">{{ t('requestLedger.empty') }}</p></template>
        </DataTable>
        <Pagination v-if="!error && page.total > 0" :page="page.page" :page-size="page.page_size" :total="page.total" @update:page="changePage" @update:page-size="changePageSize" />
      </section>
    </div>
    <BaseDialog :show="!!selectedId" :title="t('requestLedger.detail')" width="extra-wide" @close="close">
      <div :aria-busy="detailLoading" class="space-y-6">
        <p v-if="detailLoading" role="status">{{ t('common.loading') }}</p>
        <div v-else-if="detailError" role="alert"><p>{{ t('requestLedger.loadError') }}</p><button class="btn btn-secondary mt-3" @click="open(selectedId)">{{ t('requestLedger.retry') }}</button></div>
        <template v-else-if="detail">
          <dl class="grid gap-4 text-sm sm:grid-cols-2">
            <div><dt class="text-gray-500">{{ t('requestLedger.privateId') }}</dt><dd class="break-all font-mono">{{ detail.request.id }}</dd></div>
            <div v-if="detail.request.parent_id"><dt class="text-gray-500">{{ t('requestLedger.parent') }}</dt><dd><button class="break-all text-primary-600 underline dark:text-primary-400" @click="open(detail.request.parent_id)">{{ detail.request.parent_id }}</button></dd></div>
            <div><dt class="text-gray-500">{{ t('requestLedger.began') }}</dt><dd class="tabular-nums">{{ date(detail.request.started_at) }}</dd></div>
            <div><dt class="text-gray-500">{{ t('requestLedger.ended') }}</dt><dd class="tabular-nums">{{ detail.request.ended_at ? date(detail.request.ended_at) : t('requestLedger.unfinished') }}</dd></div>
            <div v-for="axis in axes" :key="axis.field"><dt class="text-gray-500">{{ t(`requestLedger.${axis.label}`) }}</dt><dd>{{ state(detail.request[axis.field]) }}</dd></div>
            <div><dt class="text-gray-500">{{ t('requestLedger.output') }}</dt><dd>{{ t(detail.request.output_observed ? 'requestLedger.yes' : 'requestLedger.no') }}</dd></div>
            <div v-if="detail.request.error_code"><dt class="text-gray-500">{{ t('requestLedger.errorCode') }}</dt><dd>{{ detail.request.error_code }}</dd></div>
          </dl>
          <section class="space-y-3"><h3 class="text-base font-semibold">{{ t('requestLedger.attempts') }}</h3><DataTable :data="detail.attempts" :columns="attemptColumns" row-key="attempt_no"><template #cell-phase="{ row }">{{ t(`requestLedger.phases.${row.phase}`) }}</template><template #cell-usage_state="{ row }">{{ state(row.usage_state) }}</template><template #cell-output_observed="{ row }">{{ t(row.output_observed ? 'requestLedger.yes' : 'requestLedger.no') }}</template><template #cell-execution_state="{ row }">{{ state(row.execution_state) }}</template></DataTable></section>
          <section v-if="detail.wallet?.length" class="space-y-3"><h3 class="text-base font-semibold">{{ t('requestLedger.walletTransaction') }}</h3>
            <p class="text-sm text-gray-500">{{ t('requestLedger.walletNotice') }}</p>
            <DataTable :data="detail.wallet" :columns="[{ key: 'id', label: '#' }, { key: 'operation', label: t('requestLedger.phase') }]" row-key="id"><template #cell-operation="{ row }">{{ t(`requestLedger.walletOperations.${row.operation}`) }}</template></DataTable>
          </section>
          <section class="space-y-3"><h3 class="text-base font-semibold">{{ t('requestLedger.billing') }}</h3>
            <p v-if="!detail.billing.length" class="text-sm text-gray-500">{{ t('requestLedger.noBilling') }} {{ detail.request.usage_state === 'not_applicable' ? '' : t('requestLedger.pendingCost') }}</p>
            <dl v-for="(ref, index) in detail.billing" :key="index" class="grid gap-3 border-t border-gray-200 pt-3 text-sm dark:border-dark-700 sm:grid-cols-2">
              <div><dt class="text-gray-500">{{ t('requestLedger.usageLog') }}</dt><dd><button v-if="ref.usage_log_id" class="text-primary-600 underline dark:text-primary-400" @click="openUsage(ref.usage_log_id)">{{ t('requestLedger.usageLog') }} #{{ ref.usage_log_id }}</button><span v-else>{{ t('requestLedger.pendingCost') }}</span></dd></div>
              <div><dt class="text-gray-500">{{ t('requestLedger.amount') }}</dt><dd class="tabular-nums">{{ money(ref.billed_cost) }}</dd></div>
              <div><dt class="text-gray-500">{{ t('requestLedger.settlement') }}</dt><dd>{{ ref.settled ? t(ref.applied ? 'requestLedger.applied' : 'requestLedger.deduplicated') : t('requestLedger.pendingCost') }}</dd></div>
              <div v-if="ref.wallet_transaction_id"><dt class="text-gray-500">{{ t('requestLedger.walletTransaction') }}</dt><dd>{{ ref.wallet_transaction_id }}</dd></div>
              <div v-if="ref.subscription_id"><dt class="text-gray-500">{{ t('requestLedger.subscription') }}</dt><dd>{{ ref.subscription_id }}</dd></div>
              <div v-if="ref.package_entitlement_id"><dt class="text-gray-500">{{ t('requestLedger.package') }}</dt><dd>{{ ref.package_entitlement_id }}</dd></div>
            </dl>
            <div v-if="usageLoading" role="status">{{ t('common.loading') }}</div>
            <p v-if="usageError" role="alert">{{ t('requestLedger.loadError') }}</p>
            <dl v-if="linkedUsage" class="grid gap-3 border-t border-gray-200 pt-3 text-sm dark:border-dark-700 sm:grid-cols-2">
              <div><dt>{{ t('requestLedger.usageLog') }} #{{ linkedUsage.id }}</dt><dd>{{ linkedUsage.model }}</dd></div>
              <div><dt>{{ t('requestLedger.amount') }}</dt><dd class="tabular-nums">{{ money(linkedUsage.billed_cost) }}</dd></div>
              <div><dt>{{ t('requestLedger.inputTokens') }}</dt><dd class="tabular-nums">{{ linkedUsage.input_tokens.toLocaleString(locale) }}</dd></div>
              <div><dt>{{ t('requestLedger.outputTokens') }}</dt><dd class="tabular-nums">{{ linkedUsage.output_tokens.toLocaleString(locale) }}</dd></div>
            </dl>
          </section>
        </template>
      </div>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import { requestLedgerAPI, type RequestDetail, type RequestFilters, type RequestPage, type LinkedUsage } from '@/api/requestLedger'

const { t, locale } = useI18n()
const route = useRoute(), router = useRouter()
const admin = computed(() => route.meta.requiresAdmin === true)
const defaults = () => ({ private_id: '', user_id: '', api_key_id: '', account_id: '', execution_state: '', usage_state: '', settlement_state: '', start_date: '', end_date: '' })
const filters = reactive(defaults())
for (const key of Object.keys(filters) as (keyof typeof filters)[]) { const value = route.query[key]; if (typeof value === 'string') filters[key] = value }
const axes = [
  { field: 'execution_state', label: 'execution', values: ['inflight', 'succeeded', 'failed', 'cancelled', 'timeout', 'interrupted'] },
  { field: 'usage_state', label: 'usage', values: ['not_applicable', 'pending', 'known', 'usage_unknown'] },
  { field: 'settlement_state', label: 'settlement', values: ['not_required', 'settlement_pending', 'settled'] },
] as const
const state = (value: string) => t(`requestLedger.states.${value}`)
const options = (values: readonly string[]) => [{ value: '', label: t('requestLedger.all') }, ...values.map(value => ({ value, label: state(value) }))]
const stateClass = (value: string) => ['failed', 'timeout', 'interrupted', 'usage_unknown'].includes(value) ? 'text-red-600 dark:text-red-400' : ['pending', 'inflight', 'settlement_pending'].includes(value) ? 'text-amber-700 dark:text-amber-400' : 'text-gray-700 dark:text-gray-300'
const date = (value: string) => new Date(value).toLocaleString(locale.value)
const money = (value: number | null) => value === null ? t('requestLedger.pendingCost') : new Intl.NumberFormat(locale.value, { style: 'currency', currency: 'USD', minimumFractionDigits: 4 }).format(value)
const columns = computed(() => [{ key: 'request', label: t('requestLedger.request') }, { key: 'identity', label: t('requestLedger.identity') }, ...axes.map(axis => ({ key: axis.field, label: t(`requestLedger.${axis.label}`) })), { key: 'attempt_count', label: t('requestLedger.attempts') }, { key: 'actions', label: t('requestLedger.operations') }])
const attemptColumns = computed(() => [{ key: 'attempt_no', label: '#' }, { key: 'phase', label: t('requestLedger.phase') }, ...(admin.value ? [{ key: 'account_id', label: t('requestLedger.account') }, { key: 'credential_account_id', label: t('requestLedger.credential') }] : []), { key: 'execution_state', label: t('requestLedger.execution') }, { key: 'usage_state', label: t('requestLedger.usage') }, { key: 'output_observed', label: t('requestLedger.output') }, { key: 'http_status', label: t('requestLedger.http') }, { key: 'error_code', label: t('requestLedger.errorCode') }])
const page = ref<RequestPage>({ items: [], total: 0, page: 1, page_size: 20 })
const loading = ref(false), error = ref(''), selectedId = ref(''), detailLoading = ref(false), detailError = ref(false), detail = ref<RequestDetail | null>(null)
const linkedUsage = ref<LinkedUsage | null>(null), usageLoading = ref(false), usageError = ref(false)
let usageController: AbortController | undefined
let listController: AbortController | undefined, detailController: AbortController | undefined
async function load() {
  listController?.abort(); const controller = new AbortController(); listController = controller
  error.value = ''; loading.value = true
  try {
    const params: RequestFilters = { page: page.value.page, page_size: page.value.page_size }
    for (const [key, value] of Object.entries(filters)) if (value && (admin.value || !['user_id', 'account_id'].includes(key))) params[key] = key.endsWith('_date') ? new Date(value).toISOString() : value
    if (params.start_date && params.end_date && params.start_date >= params.end_date) { error.value = t('requestLedger.invalidDates'); return }
    const result = await requestLedgerAPI.list(admin.value, params, controller.signal)
    if (!controller.signal.aborted) page.value = result
  } catch { if (!controller.signal.aborted) error.value = t('requestLedger.loadError') }
  finally { if (!controller.signal.aborted) loading.value = false }
}
const languageQuery = () => Object.fromEntries(Object.entries(route.query).filter(([key]) => ['lang', 'locale'].includes(key)))
function applyFilters() { page.value.page = 1; void router.replace({ query: { ...languageQuery(), ...Object.fromEntries(Object.entries(filters).filter(([, value]) => value)) } }); void load() }
function reset() { Object.assign(filters, defaults()); applyFilters() }
function changePage(value: number) { page.value.page = value; void load() }
function changePageSize(value: number) { page.value.page_size = value; page.value.page = 1; void load() }
async function open(id: string) {
  detailController?.abort(); const controller = new AbortController(); detailController = controller
  usageController?.abort(); linkedUsage.value = null; usageError.value = false; usageLoading.value = false
  selectedId.value = id; detail.value = null; detailLoading.value = true; detailError.value = false
  try { const result = await requestLedgerAPI.detail(admin.value, id, controller.signal); if (!controller.signal.aborted) detail.value = result }
  catch { if (!controller.signal.aborted) detailError.value = true }
  finally { if (!controller.signal.aborted) detailLoading.value = false }
}
function close() { usageController?.abort(); detailController?.abort(); selectedId.value = ''; detail.value = null }
async function openUsage(id: number) {
  usageController?.abort(); const controller = new AbortController(); usageController = controller
  usageLoading.value = true; usageError.value = false; linkedUsage.value = null
  try { const result = await requestLedgerAPI.usage(admin.value, selectedId.value, id, controller.signal); if (!controller.signal.aborted) linkedUsage.value = result }
  catch { if (!controller.signal.aborted) usageError.value = true }
  finally { if (!controller.signal.aborted) usageLoading.value = false }
}
onMounted(load)
onUnmounted(() => { listController?.abort(); detailController?.abort(); usageController?.abort() })
</script>
