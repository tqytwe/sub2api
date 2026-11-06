<template>
  <section class="space-y-4" aria-labelledby="checkin-records-title" :aria-busy="loading">
    <div>
      <h2 id="checkin-records-title" class="text-lg font-semibold text-gray-900 dark:text-white">{{ text('签到记录与奖励核对', 'Check-in records and reward evidence') }}</h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ text('按业务签到日期查询。现金奖励和成长能量分别展示；历史证据缺失时需核实，不能据此推断漏发金额。', 'Search by check-in date. Cash rewards and growth energy are separate. Missing historical evidence requires review; it does not prove an owed reward.') }}</p>
    </div>
    <form class="flex flex-wrap items-end gap-3" @submit.prevent="search">
      <label class="space-y-1 text-sm"><span class="block">{{ text('用户 ID', 'User ID') }}</span><input id="checkin-user-id" v-model="userId" type="number" min="1" step="1" class="input" :placeholder="text('所有用户', 'All users')" :disabled="loading || repairBusy" /></label>
      <label class="space-y-1 text-sm"><span class="block">{{ text('开始日期', 'Start date') }}</span><input v-model="startDate" type="date" class="input" :disabled="loading || repairBusy" /></label>
      <label class="space-y-1 text-sm"><span class="block">{{ text('结束日期', 'End date') }}</span><input v-model="endDate" type="date" class="input" :disabled="loading || repairBusy" /></label>
      <label class="space-y-1 text-sm"><span class="block">{{ text('奖励模式', 'Reward mode') }}</span><select v-model="rewardMode" class="input" :disabled="loading || repairBusy"><option value="">{{ text('全部', 'All') }}</option><option value="energy">{{ text('成长能量', 'Growth energy') }}</option><option value="redeemable">{{ text('可领奖', 'Redeemable') }}</option><option value="unknown">{{ text('历史资格未知', 'Historical eligibility unknown') }}</option></select></label>
      <button type="submit" class="btn btn-secondary" :disabled="loading || repairBusy">{{ loading ? text('查询中…', 'Loading…') : text('查询', 'Search') }}</button>
    </form>
    <div v-if="error" role="alert" class="text-sm text-red-700 dark:text-red-300"><p>{{ error }}</p><button type="button" data-testid="checkin-retry" class="btn btn-secondary mt-2" :disabled="loading || repairBusy" @click="load(requestedPage)">{{ text('重试', 'Retry') }}</button></div>
    <p v-else-if="loading" role="status" class="text-sm text-gray-500">{{ text('正在读取签到记录…', 'Loading check-in records…') }}</p>
    <template v-else-if="report">
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ report.start_date }} — {{ report.end_date }} · {{ text('数据截至', 'Data as of') }} {{ formatDate(report.as_of) }}</p>
      <div class="overflow-x-auto" tabindex="0" :aria-label="text('签到记录表，可横向滚动', 'Check-in table, scroll horizontally')">
        <table class="w-full whitespace-nowrap divide-y divide-gray-200 text-sm dark:divide-dark-700">
          <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">{{ text('选择', 'Select') }}</th><th class="px-4 py-3">{{ text('日期 / 用户', 'Date / user') }}</th><th class="px-4 py-3">{{ text('连签天数', 'Streak') }}</th><th class="px-4 py-3">{{ text('现金奖励 / 流水', 'Cash reward / ledger') }}</th><th class="px-4 py-3">{{ text('成长能量', 'Growth energy') }}</th><th class="px-4 py-3">{{ text('资格 / 证据', 'Eligibility / evidence') }}</th></tr></thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="row in report.items" :key="row.id"><td class="px-4 py-3"><input v-model="selected" type="checkbox" :value="row.id" :disabled="repairBusy" :data-testid="'checkin-select-' + row.id" :aria-label="text('选择用户', 'Select user') + ' ' + row.user_id + ' / ' + row.date" @change="clearRepairPreview" /></td><td class="px-4 py-3"><div>{{ row.date }} · #{{ row.user_id }}</div><div class="text-xs text-gray-500">{{ row.email }}</div></td><td class="px-4 py-3 tabular-nums">{{ row.observed_streak }}<div v-if="row.stored_streak !== row.observed_streak" class="text-xs text-gray-500">{{ text('原记录', 'Stored') }} {{ row.stored_streak }}</div></td><td class="px-4 py-3 tabular-nums">{{ row.reward_amount }} / {{ row.ledger_amount }} USD<div v-if="row.reconciliation_id" class="text-xs text-gray-500">{{ text('独立补发', 'Separate adjustment') }} {{ row.adjustment_amount }} USD · #{{ row.reconciliation_id }}</div></td><td class="px-4 py-3 tabular-nums">{{ row.growth_energy }}</td><td class="px-4 py-3"><div>{{ modeLabel(row.reward_mode) }}</div><div class="text-xs text-gray-500">{{ evidenceLabel(row.evidence_status) }}</div><div v-if="row.snapshot_id" class="text-xs text-gray-500">{{ text('资格快照', 'Eligibility snapshot') }} #{{ row.snapshot_id }}</div></td></tr><tr v-if="!report.items.length"><td colspan="6" class="px-4 py-6 text-center text-gray-500">{{ text('没有符合筛选条件的签到记录', 'No check-ins match these filters') }}</td></tr></tbody>
        </table>
      </div>
      <div class="flex flex-wrap items-center justify-between gap-3 text-sm"><span>{{ text('记录数', 'Records') }} {{ report.total.toLocaleString(locale) }}</span><div class="flex items-center gap-3"><button type="button" class="btn btn-secondary" :disabled="report.page <= 1 || loading || repairBusy" @click="load(report.page - 1)">{{ text('上一页', 'Previous') }}</button><span>{{ report.page }}</span><button type="button" data-testid="checkin-next" class="btn btn-secondary" :disabled="report.page * report.page_size >= report.total || loading || repairBusy" @click="load(report.page + 1)">{{ text('下一页', 'Next') }}</button></div></div>
    </template>
    <div v-if="report?.items.length" class="space-y-3 border-t border-gray-200 pt-4 dark:border-dark-700">
      <button type="button" data-testid="checkin-preview-repair" class="btn btn-secondary" :disabled="!selected.length || repairBusy || loading" @click="previewRepairs">{{ text('预览所选记录修复', 'Preview selected repairs') }} ({{ selected.length }})</button>
      <p class="text-sm text-gray-500">{{ text('最多 50 条。只按原签到资格与冻结规则核实；证据不足不推算补发。', 'Up to 50 records. Original eligibility and frozen rules are required; missing evidence cannot justify a payout.') }}</p>
      <p v-if="repairBusy" role="status">{{ text('正在核对或提交，请稍候…', 'Reviewing or submitting, please wait…') }}</p>
      <p v-if="repairError" role="alert" class="text-sm text-red-700 dark:text-red-300">{{ repairError }}</p>
      <template v-if="repairPreviews.length">
        <div class="overflow-x-auto" tabindex="0" :aria-label="text('修复预览表', 'Repair preview table')"><table class="w-full whitespace-nowrap text-sm"><thead class="text-left"><tr><th class="px-3 py-2">{{ text('用户 / 日期', 'User / date') }}</th><th class="px-3 py-2">{{ text('有效连签 → 实际', 'Effective → observed streak') }}</th><th class="px-3 py-2">{{ text('独立补发', 'Separate adjustment') }}</th><th class="px-3 py-2">{{ text('判定', 'Decision') }}</th></tr></thead><tbody><tr v-for="item in repairPreviews" :key="item.target.user_id + item.target.date"><td class="px-3 py-2">#{{ item.target.user_id }} / {{ item.target.date }}</td><td class="px-3 py-2">{{ item.effective_streak ?? '—' }} → {{ item.observed_streak || '—' }}</td><td class="px-3 py-2 tabular-nums">{{ item.bonus || 0 }} USD</td><td class="px-3 py-2">{{ repairStatus(item.status) }}</td></tr></tbody></table></div>
        <form v-if="executableRepairs.length" data-testid="checkin-repair-form" class="space-y-3" @submit.prevent="repair">
          <label class="block text-sm">{{ text('核实依据与修复原因（10–500 字）', 'Evidence and reason (10–500 characters)') }}<textarea v-model="repairReason" required minlength="10" maxlength="500" class="input mt-1 w-full" :disabled="repairBusy" /></label>
          <label class="flex items-start gap-2 text-sm"><input v-model="repairConfirmed" type="checkbox" data-testid="checkin-repair-confirmed" :disabled="repairBusy" />{{ text('已核对可执行记录及补发金额；保留原始记录，任一预览过期则整批不执行。', 'I reviewed the actionable records and amounts. Original evidence is retained; a stale preview prevents the whole batch.') }}</label>
          <button type="submit" data-testid="checkin-confirm-repair" class="btn btn-primary" :disabled="repairBusy || !repairConfirmed || repairReason.trim().length < 10">{{ text('确认修复可执行记录', 'Confirm actionable repairs') }} ({{ executableRepairs.length }})</button>
        </form>
      </template>
    </div>
    <p v-if="repairSuccess" role="status" class="text-sm">{{ repairSuccess }}</p>
    <TotpStepUpDialog :controller="stepUp" />
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import adminPlayAPI, { type AdminCheckinPage, type AdminCheckinQuery, type AdminCheckinRepairPreview } from '@/api/admin/play'
import { useStepUp, isStepUpCancelled, isStepUpBlocked, stepUpBlockReason } from '@/composables/useStepUp'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
const { locale, t } = useI18n()
const stepUp = useStepUp()
const selected = ref<number[]>([])
const repairPreviews = ref<AdminCheckinRepairPreview[]>([])
const repairBusy = ref(false)
const repairError = ref('')
const repairSuccess = ref('')
const repairReason = ref('')
const repairConfirmed = ref(false)
const executableRepairs = computed(() => repairPreviews.value.filter(item => item.can_repair))
let repairKey = ''
let repairSequence = 0
function clearRepairPreview() { repairSequence++; repairPreviews.value = []; repairConfirmed.value = false; repairError.value = ''; repairSuccess.value = ''; stepUp.onCancel() }
function repairStatus(status: string) { return status === 'ready' ? text('证据完整，可修复', 'Evidence complete; actionable') : status === 'no_change' ? text('无需调整或已处理', 'No change needed or already processed') : text('历史证据缺失，待核实', 'Historical evidence missing; review required') }
async function previewRepairs() {
  if (repairBusy.value || !report.value) return
  const targets = report.value.items.filter(row => selected.value.includes(row.id)).map(row => ({ user_id: row.user_id, date: row.date }))
  if (!targets.length || targets.length > 50) return
  clearRepairPreview()
  const seq = repairSequence
  repairBusy.value = true
  try {
    const data = await Promise.all(targets.map(target => adminPlayAPI.previewCheckinRepair(target)))
    if (seq === repairSequence) { repairPreviews.value = data; repairKey = crypto.randomUUID() }
  } catch { if (seq === repairSequence) repairError.value = text('修复预览失败，请重新核对。', 'Repair preview failed. Please review again.') }
  finally { repairBusy.value = false }
}
async function repair() {
  if (repairBusy.value || !repairConfirmed.value || !executableRepairs.value.length) return
  const reason = repairReason.value.trim()
  if (reason.length < 10 || reason.length > 500) return
  const items = executableRepairs.value.map(item => ({ target: { ...item.target }, digest: item.digest }))
  const key = repairKey
  const seq = repairSequence
  repairBusy.value = true; repairError.value = ''
  try {
    await stepUp.run(() => adminPlayAPI.repairCheckins(items, reason, key))
    if (seq !== repairSequence) return
    selected.value = []; repairPreviews.value = []; repairConfirmed.value = false; repairReason.value = ''
    report.value = null
    await load(1)
    if (seq !== repairSequence) return
    repairSuccess.value = text('修复已完成，独立调整与操作者证据已保存。', 'Repair complete. Separate adjustments and actor evidence were recorded.')
  } catch (err) {
    if (seq !== repairSequence || isStepUpCancelled(err)) return
    repairError.value = isStepUpBlocked(err) ? t(stepUpBlockReason(err) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN' ? 'stepUp.adminApiKeyForbidden' : 'stepUp.notEnabled') : text('修复未确认成功，请刷新记录并重新预览；重试不会重复发奖。', 'Repair was not confirmed. Refresh records and preview again; retries cannot duplicate rewards.')
  } finally { repairBusy.value = false }
}
const text = (zh: string, en: string) => locale.value.startsWith('zh') ? zh : en
const userId = ref('')
const startDate = ref('')
const endDate = ref('')
const rewardMode = ref('')
const report = ref<AdminCheckinPage | null>(null)
const loading = ref(false)
const error = ref('')
const requestedPage = ref(1)
let sequence = 0
let appliedFilters: AdminCheckinQuery = {}
function search() {
  clearRepairPreview(); selected.value = []
  appliedFilters = { user_id: userId.value ? Number(userId.value) : undefined, start_date: startDate.value || undefined, end_date: endDate.value || undefined, reward_mode: rewardMode.value || undefined }
  report.value = null
  void load(1)
}
async function load(page = 1) {
  selected.value = []; repairPreviews.value = []; repairConfirmed.value = false
  const request = ++sequence
  requestedPage.value = page
  loading.value = true
  error.value = ''
  try {
    const result = await adminPlayAPI.listCheckinRecords({ ...appliedFilters, page, page_size: 20, as_of: report.value?.as_of })
    if (request !== sequence) return
    report.value = result
    startDate.value = result.start_date
    endDate.value = result.end_date
  } catch {
    if (request === sequence) error.value = text('签到记录读取失败，请重试。', 'Unable to load check-ins. Please retry.')
  } finally {
    if (request === sequence) loading.value = false
  }
}
function modeLabel(mode: string) { return mode === 'energy' ? text('成长能量', 'Growth energy') : mode === 'redeemable' ? text('可领奖', 'Redeemable') : text('历史资格未知', 'Historical eligibility unknown') }
function evidenceLabel(status: string) { return status === 'reconciled' ? text('已修正，保留原记录', 'Reconciled; original evidence retained') : status === 'recorded' ? text('已记录', 'Recorded') : status === 'streak_mismatch' ? text('连续天数待核对', 'Review streak count') : text('历史证据缺失，待核实', 'Historical evidence missing; review required') }
function formatDate(value: string) { return new Date(value).toLocaleString(locale.value, { timeZone: 'Asia/Shanghai' }) }
onMounted(() => load())
onBeforeUnmount(() => { sequence++; repairSequence++; stepUp.onCancel() })
</script>
