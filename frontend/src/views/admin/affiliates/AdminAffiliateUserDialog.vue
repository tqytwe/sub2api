<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { useStepUp, isStepUpBlocked, isStepUpCancelled } from '@/composables/useStepUp'
import { useAppStore } from '@/stores/app'
import operations, { type AffiliatePromoter, type AffiliateOperationsInvite, type AffiliateOperationEarning, type AffiliateOperationAudit } from '@/api/admin/affiliateOperations'
import { affiliatesAPI, type AffiliateUserOverview, type UpdateAffiliateUserRequest } from '@/api/admin/affiliates'
import { formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = withDefaults(defineProps<{ show: boolean; userId: number | null; inviteeId?: number; initialTab?: string }>(), { initialTab: 'invites' })
const emit = defineEmits<{ close: []; changed: [] }>()
const { t } = useI18n()
const store = useAppStore()
const stepUp = useStepUp()
type DetailTab = 'invites' | 'earnings' | 'audits' | 'settings'
const tabs: DetailTab[] = ['invites', 'earnings', 'audits', 'settings']
const tab = ref<DetailTab>('invites')
const overview = ref<AffiliateUserOverview | null>(null)
const promoter = ref<AffiliatePromoter | null>(null)
const loading = ref(false)
const rowsLoading = ref(false)
const error = ref('')
const rowsError = ref('')
const saving = ref(false)
const code = ref('')
const rate = ref<string | number>('')
const clearConfirm = ref(false)
const status = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const rows = ref<Array<AffiliateOperationsInvite | AffiliateOperationEarning | AffiliateOperationAudit>>([])
let sequence = 0
let rowSequence = 0
const qualificationStatuses = ['pending_evidence', 'unpaid', 'effective', 'refunded', 'risk_excluded', 'disabled']
const title = computed(() => `${t('admin.affiliateOperations.userDetail')} #${props.userId || ''}`)
const columns = computed(() => {
  const label = (key: string) => t(`admin.affiliateOperations.${key}`)
  if (tab.value === 'invites') return [
    { key: 'invitee_email', label: label('invitee') }, { key: 'invited_at', label: label('invitedAt') },
    { key: 'first_paid_at', label: label('firstPaidAt') }, { key: 'status', label: label('status') },
    { key: 'reason', label: label('reason') }, { key: 'actions', label: label('actions') },
  ]
  if (tab.value === 'earnings') return [
    { key: 'id', label: label('ledgerId') }, { key: 'source_type', label: label('source') },
    { key: 'amount', label: label('amount') }, { key: 'status', label: label('status') },
    { key: 'created_at', label: label('date') }, { key: 'actions', label: label('actions') },
  ]
  return [{ key: 'actor_id', label: label('actor') }, { key: 'action', label: label('action') }, { key: 'detail', label: label('detail') }, { key: 'created_at', label: label('date') }]
})
const number = (value: number | undefined) => new Intl.NumberFormat(undefined, { maximumFractionDigits: 8 }).format(value || 0)
const recordLink = (type: string, extra: Record<string, string | number> = {}) => ({ path: `/admin/affiliates/${type}`, query: { user_id: props.userId!, ...extra } })

async function load() {
  if (!props.show || !props.userId) return
  const seq = ++sequence
  loading.value = true
  error.value = ''
  overview.value = null
  promoter.value = null
  try {
    const [account, result] = await Promise.all([affiliatesAPI.getUserOverview(props.userId), operations.listPromoters({ user_id: props.userId, page: 1, page_size: 1 })])
    if (seq !== sequence) return
    overview.value = account
    promoter.value = result.items[0] || null
    code.value = account.aff_code
    rate.value = account.rebate_rate_custom ? account.rebate_rate_percent : ''
  } catch (cause) { if (seq === sequence) error.value = extractApiErrorMessage(cause, t('admin.affiliateOperations.loadFailed')) }
  finally { if (seq === sequence) loading.value = false }
}
async function loadRows() {
  if (!props.show || !props.userId || tab.value === 'settings') return
  const seq = ++rowSequence
  rowsLoading.value = true
  rowsError.value = ''
  rows.value = []
  const params = { user_id: props.userId, page: page.value, page_size: pageSize.value }
  try {
    const result = tab.value === 'invites'
      ? await operations.listInvitees({ ...params, invitee_id: props.inviteeId, status: status.value || undefined })
      : tab.value === 'earnings' ? await operations.listEarnings(params) : await operations.listAudits(params)
    if (seq !== rowSequence) return
    rows.value = result.items
    total.value = result.total
  } catch (cause) { if (seq === rowSequence) rowsError.value = extractApiErrorMessage(cause, t('admin.affiliateOperations.loadFailed')) }
  finally { if (seq === rowSequence) rowsLoading.value = false }
}
function selectTab(value: DetailTab) { tab.value = value; page.value = 1; status.value = ''; void loadRows() }
async function saveSettings() {
  if (!props.userId || !overview.value || saving.value) return
  const rawRate = String(rate.value).trim()
  const parsedRate = Number(rawRate)
  if (rawRate && (!Number.isFinite(parsedRate) || parsedRate < 0 || parsedRate > 100)) { error.value = t('admin.affiliateOperations.invalidRate'); return }
  const payload: UpdateAffiliateUserRequest = rawRate ? { aff_rebate_rate_percent: parsedRate } : { clear_rebate_rate: true }
  if (code.value.trim() && code.value.trim().toUpperCase() !== overview.value.aff_code) payload.aff_code = code.value.trim().toUpperCase()
  const userID = props.userId
  saving.value = true
  error.value = ''
  try {
    await stepUp.run(() => affiliatesAPI.updateUserSettings(userID, payload))
    store.showSuccess(t('common.saved'))
    emit('changed')
    await load()
  } catch (cause) {
    if (isStepUpCancelled(cause)) return
    error.value = isStepUpBlocked(cause) ? t('stepUp.notEnabled') : extractApiErrorMessage(cause, t('admin.affiliateOperations.saveFailed'))
  } finally { saving.value = false }
}
async function resetSettings() {
  if (!props.userId || saving.value) return
  const userID = props.userId
  clearConfirm.value = false
  saving.value = true
  try {
    await stepUp.run(() => affiliatesAPI.clearUserSettings(userID))
    store.showSuccess(t('common.saved')); emit('changed'); await load()
  } catch (cause) { if (!isStepUpCancelled(cause)) error.value = extractApiErrorMessage(cause, t('admin.affiliateOperations.saveFailed')) }
  finally { saving.value = false }
}
watch(() => [props.show, props.userId, props.inviteeId], () => {
  ++sequence; ++rowSequence
  if (!props.show) return
  tab.value = tabs.includes(props.initialTab as DetailTab) ? props.initialTab as DetailTab : 'invites'
  page.value = 1; total.value = 0; status.value = ''
  void load(); void loadRows()
}, { immediate: true })
</script>

<template>
  <BaseDialog :show="show" :title="title" width="extra-wide" @close="!saving && emit('close')">
    <div class="space-y-4" :aria-busy="loading || rowsLoading">
      <p v-if="loading" role="status">{{ t('common.loading') }}</p>
      <div v-if="error" role="alert" class="text-sm text-red-600">{{ error }} <button type="button" class="btn btn-secondary" @click="load">{{ t('common.retry') }}</button></div>
      <template v-if="overview">
        <p class="break-words text-sm font-medium">{{ promoter?.email || overview.email }} · {{ overview.aff_code }} · {{ overview.rebate_rate_percent }}%</p>
        <dl class="grid grid-cols-2 gap-3 text-sm sm:grid-cols-4"><div><dt class="text-gray-500">{{ t('admin.affiliateOperations.invited') }}</dt><dd class="font-semibold tabular-nums">{{ overview.invited_count }}</dd></div><div><dt class="text-gray-500">{{ t('admin.affiliateOperations.effectivePaidTotal') }}</dt><dd class="font-semibold tabular-nums">{{ promoter?.effective_paid_count ?? '—' }}</dd></div><div><dt class="text-gray-500">{{ t('admin.affiliateOperations.available') }}</dt><dd class="font-semibold tabular-nums">USD {{ number(promoter?.available_quota ?? overview.available_quota) }}</dd></div><div><dt class="text-gray-500">{{ t('admin.affiliateOperations.frozen') }}</dt><dd class="font-semibold tabular-nums">USD {{ number(promoter?.frozen_quota) }}</dd></div></dl>
      </template>
      <div class="flex flex-wrap gap-3 text-sm"><RouterLink data-test="detail-rebates-link" :to="recordLink('rebates')" class="text-primary-600 hover:underline dark:text-primary-400" @click="emit('close')">{{ t('admin.affiliateOperations.rebateRecords') }}</RouterLink><RouterLink :to="recordLink('transfers')" class="text-primary-600 hover:underline dark:text-primary-400" @click="emit('close')">{{ t('admin.affiliateOperations.transferRecords') }}</RouterLink><RouterLink :to="{ path: '/admin/play-ops', query: { tab: 'invite-growth' } }" class="text-primary-600 hover:underline dark:text-primary-400" @click="emit('close')">{{ t('admin.affiliateOperations.campaignManagement') }}</RouterLink><RouterLink :to="{ name: 'AdminIPRisk' }" class="text-primary-600 hover:underline dark:text-primary-400" @click="emit('close')">{{ t('admin.affiliateOperations.riskCases') }}</RouterLink></div>
      <div class="flex flex-wrap gap-2" role="group" :aria-label="t('admin.affiliateOperations.userDetail')"><button v-for="item in tabs" :key="item" type="button" class="btn" :class="tab === item ? 'btn-primary' : 'btn-secondary'" :data-test="`detail-tab-${item}`" :aria-pressed="tab === item" @click="selectTab(item)">{{ t(`admin.affiliateOperations.detailTabs.${item}`) }}</button></div>
      <form v-if="tab === 'settings'" class="space-y-4" @submit.prevent="saveSettings">
        <p class="text-xs text-gray-500">{{ t('admin.affiliateOperations.settingsScope') }}</p>
        <label class="block text-sm">{{ t('admin.affiliateOperations.inviteCode') }}<input v-model="code" class="input mt-1" :disabled="saving || loading" maxlength="64" /></label>
        <label class="block text-sm">{{ t('admin.affiliateOperations.exclusiveRate') }}<input v-model="rate" data-test="detail-rate" class="input mt-1" type="number" min="0" max="100" step="any" :disabled="saving || loading" /><span class="mt-1 block text-xs text-gray-500">{{ t('admin.affiliateOperations.rateHint') }}</span></label>
        <div class="flex flex-wrap gap-3"><button data-test="detail-save" class="btn btn-primary" type="submit" :disabled="saving || loading || !overview">{{ t('common.save') }}</button><button type="button" class="btn btn-danger" :disabled="saving || loading || !overview" @click="clearConfirm = true">{{ t('admin.affiliateOperations.resetSettings') }}</button></div>
      </form>
      <template v-else>
        <label v-if="tab === 'invites'" class="block text-sm">{{ t('admin.affiliateOperations.qualification') }}<select v-model="status" class="input mt-1 w-full sm:w-64" @change="page = 1; loadRows()"><option value="">{{ t('admin.affiliateOperations.allStatuses') }}</option><option v-for="item in qualificationStatuses" :key="item" :value="item">{{ t(`admin.affiliateOperations.reasons.${item}`) }}</option></select></label>
        <p v-if="inviteeId && tab === 'invites'" class="text-xs text-gray-500">{{ t('admin.affiliateOperations.exactInviteeId') }} #{{ inviteeId }}</p>
        <div v-if="rowsError" role="alert" class="text-sm text-red-600">{{ rowsError }} <button type="button" class="btn btn-secondary" @click="loadRows">{{ t('common.retry') }}</button></div>
        <DataTable :columns="columns" :data="rows" :loading="rowsLoading">
          <template #cell-invited_at="{ row }">{{ row.invited_at ? formatDateTime(row.invited_at) : '—' }}</template><template #cell-first_paid_at="{ row }">{{ row.first_paid_at ? formatDateTime(row.first_paid_at) : '—' }}<div v-if="row.first_paid_month" class="text-xs text-gray-500">{{ row.first_paid_month }}</div></template>
          <template #cell-reason="{ row }">{{ t(`admin.affiliateOperations.reasons.${row.reason}`) }}</template><template #cell-status="{ row }"><span class="badge badge-primary">{{ t(`admin.affiliateOperations.${tab === 'invites' ? 'reasons' : 'fundStatuses'}.${row.status}`) }}</span></template>
          <template #cell-source_type="{ row }">{{ t(`admin.affiliateOperations.sources.${row.source_type}`) }}</template><template #cell-amount="{ row }"><span class="tabular-nums">{{ row.currency || 'USD' }} {{ number(row.amount) }}</span></template>
          <template #cell-created_at="{ row }">{{ formatDateTime(row.created_at) }}</template><template #cell-detail="{ row }"><pre class="whitespace-pre-wrap break-words font-sans text-xs">{{ JSON.stringify(row.detail, null, 2) }}</pre></template>
          <template #cell-actions="{ row }"><RouterLink v-if="tab === 'invites'" :to="recordLink('rebates', { invitee_id: row.invitee_id })" class="text-primary-600 hover:underline dark:text-primary-400" @click="emit('close')">{{ t('admin.affiliateOperations.rebateRecords') }}</RouterLink><RouterLink v-if="tab === 'invites' && row.risk_case_id" :to="{ name: 'AdminIPRisk', query: { case_id: row.risk_case_id } }" class="ml-3 text-primary-600 hover:underline dark:text-primary-400" @click="emit('close')">{{ t('admin.affiliateOperations.riskCases') }} #{{ row.risk_case_id }}</RouterLink><RouterLink v-if="tab === 'earnings' && row.source_order_id" :to="{ name: 'AdminOrders', query: { order_id: row.source_order_id } }" class="text-primary-600 hover:underline dark:text-primary-400" @click="emit('close')">#{{ row.source_order_id }}</RouterLink><span v-if="tab === 'earnings' && !row.source_order_id">—</span></template>
        </DataTable>
        <Pagination v-if="total > 0" :page="page" :page-size="pageSize" :total="total" @update:page="page = $event; loadRows()" @update:pageSize="pageSize = $event; page = 1; loadRows()" />
      </template>
    </div>
  </BaseDialog>
  <ConfirmDialog :show="clearConfirm" :title="t('admin.affiliateOperations.resetSettings')" :message="t('admin.affiliateOperations.resetWarning')" danger @cancel="clearConfirm = false" @confirm="resetSettings" />
  <TotpStepUpDialog :controller="stepUp" />
</template>
