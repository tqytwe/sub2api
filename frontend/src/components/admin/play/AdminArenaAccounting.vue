<template>
  <section class="space-y-4" :aria-busy="loading || saving">
    <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ text('农场 Token 与能量对账', 'Farm token and energy reconciliation') }}</h2>
    <form class="flex flex-wrap items-end gap-3" @submit.prevent="loadUser">
      <label class="space-y-1 text-sm"><span class="block">{{ text('用户 ID', 'User ID') }}</span><input v-model="userId" type="number" min="1" step="1" required class="input" :disabled="loading" /></label>
      <label class="space-y-1 text-sm"><span class="block">{{ text('上海业务日期', 'Shanghai business date') }}</span><input v-model="date" type="date" class="input" :disabled="loading" /></label>
      <button type="submit" class="btn btn-secondary" :disabled="loading">{{ text('查询用量', 'Query usage') }}</button>
    </form>
    <p v-if="error" role="alert" class="text-sm text-red-700 dark:text-red-300">{{ error }}</p>
    <p v-if="loading" role="status">{{ text('读取中…', 'Loading…') }}</p>
    <div v-if="report && !loading" class="space-y-3">
      <p class="text-sm text-gray-500">{{ text('用户', 'User') }} #{{ report.user_id }} · {{ text('数据截至', 'As of') }} {{ formatDate(report.as_of) }}</p>
      <div class="overflow-x-auto" tabindex="0" :aria-label="text('对账数据表，可横向滚动', 'Accounting table, scroll horizontally')"><table class="w-full whitespace-nowrap divide-y divide-gray-200 text-sm dark:divide-dark-700"><thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">{{ text('区间', 'Window') }}</th><th class="px-4 py-3">{{ text('输入', 'Input') }}</th><th class="px-4 py-3">{{ text('输出', 'Output') }}</th><th class="px-4 py-3">{{ text('缓存创建', 'Cache creation') }}</th><th class="px-4 py-3">{{ text('缓存读取', 'Cache read') }}</th><th class="px-4 py-3">{{ text('实际总量', 'Actual total') }}</th></tr></thead><tbody><tr v-for="row in usageRows" :key="row.label"><td class="px-4 py-3">{{ row.label }}<div class="text-xs text-gray-500">{{ row.window }}</div></td><td v-for="key in tokenKeys" :key="key" class="px-4 py-3 tabular-nums">{{ row.tokens[key].toLocaleString(locale) }}</td></tr></tbody></table></div>
      <p>{{ text('本月实际 Token 折算能量', 'Monthly actual token energy') }}: {{ report.monthly_actual_token_energy }} · {{ text('旧口径折算能量', 'Legacy token energy') }}: {{ report.monthly_legacy_token_energy }}</p>
      <div v-if="report.current_farm?.enabled" class="space-y-1" data-testid="current-farm-energy">
        <p>{{ text('今日任务能量', 'Today’s task energy') }}: {{ report.current_farm.task_energy }} · {{ text('本月计分 Token 能量', 'Monthly scored token energy') }}: {{ report.current_farm.monthly_token_energy }} · {{ text('农场当前能量', 'Current farm energy') }}: {{ report.current_farm.energy }}</p>
        <p v-for="task in report.current_farm.tasks" :key="task.key" class="text-sm text-gray-500">{{ taskLabel(task.key) }} · {{ task.completed ? text('已完成', 'Completed') : text('未完成', 'Incomplete') }} · {{ task.energy }}</p>
      </div>
      <p>{{ text('资格成长能量（独立账本累计）', 'Qualification growth energy (separate cumulative ledger)') }}: {{ report.qualification_growth_energy }}</p>
      <p class="text-sm text-gray-500">{{ text('Token 能量每 10000 向下取整。农场当前能量还包含今日任务能量；历史周期按已冻结的计分口径解释，资格成长能量不计入农场合计。', 'Token energy is floor(tokens / 10000). Current farm energy also includes today’s task energy. Historical periods use their frozen accounting version; qualification energy is separate.') }}</p>
    </div>
    <div v-if="periodId" class="space-y-3 border-t border-gray-200 pt-4 dark:border-dark-700">
      <div class="flex flex-wrap items-center gap-3"><h3 class="font-semibold">{{ text('当前周期口径', 'Selected period accounting') }} #{{ periodId }}</h3><button type="button" class="btn btn-secondary" :disabled="loading || saving" @click="loadPreview">{{ text('预览新旧口径', 'Preview accounting change') }}</button></div>
      <template v-if="preview">
        <p>{{ text('口径版本', 'Accounting version') }} {{ preview.version }} · {{ text('参与人数', 'Participants') }} {{ preview.total_users }}</p>
        <p class="text-sm text-gray-500">{{ preview.reward_policy_source === 'frozen' ? text('奖励规则已冻结；修改配置在新周期生效。', 'Reward rules are frozen; configuration changes apply to new periods.') : preview.reward_policy_source === 'legacy_live' ? text('旧日榜未冻结奖励规则，当前按现行配置预估。确认切换时同时冻结本期规则与预算。', 'This legacy daily period has no frozen reward rules. Estimates use current settings; activation also freezes its rules and budget.') : text('缺少历史奖励规则证据，不能将当前配置当作当期规则。', 'Historical reward rules are missing. Current settings cannot establish the original policy.') }}</p>
        <p v-if="preview.truncated" class="text-sm text-gray-500">{{ text('展示新口径前 100 名；预览指纹覆盖全部参与用户。', 'Showing the top 100 under the new version; the preview digest covers all participants.') }}</p>
        <div class="overflow-x-auto" tabindex="0" :aria-label="text('对账数据表，可横向滚动', 'Accounting table, scroll horizontally')"><table class="w-full whitespace-nowrap text-sm"><thead class="text-left"><tr><th class="px-3 py-2">{{ text('用户', 'User') }}</th><th class="px-3 py-2">{{ text('Token 旧 → 新', 'Tokens old → new') }}</th><th class="px-3 py-2">{{ text('排名旧 → 新', 'Rank old → new') }}</th><th class="px-3 py-2">{{ text('预计奖励旧 → 新', 'Estimated reward old → new') }}</th></tr></thead><tbody><tr v-for="row in preview.changes" :key="row.user_id"><td class="px-3 py-2">#{{ row.user_id }}</td><td class="px-3 py-2 tabular-nums">{{ row.old_tokens }} → {{ row.new_tokens }}</td><td class="px-3 py-2">{{ row.old_rank || '—' }} → {{ row.new_rank }}</td><td class="px-3 py-2">{{ row.old_reward }} → {{ row.new_reward }} USD</td></tr></tbody></table></div>
        <form v-if="preview.can_activate" class="space-y-3" @submit.prevent="activate">
          <label class="block text-sm">{{ text('切换原因（10–500 字）', 'Reason (10–500 characters)') }}<textarea v-model="reason" required minlength="10" maxlength="500" class="input mt-1 w-full" :disabled="saving" /></label>
          <label class="flex items-start gap-2 text-sm"><input v-model="confirmed" type="checkbox" :disabled="saving" />{{ text('已核对排名及奖励影响，确认将本周期统一切换为包含缓存读取量的口径。', 'I reviewed ranking and reward changes and confirm including cache-read tokens throughout this period.') }}</label>
          <button type="submit" class="btn btn-primary" :disabled="saving || !confirmed || reason.trim().length < 10">{{ text('确认切换本周期', 'Activate for this period') }}</button>
        </form>
        <p v-else class="text-sm text-gray-500">{{ preview.version === 2 ? text('本周期已使用四项 Token 口径。', 'This period already uses all four token categories.') : text('本周期已结束或存在发奖证据，不能直接切换。请先核对历史结算。', 'This period ended or has payout evidence. Review the historical settlement before changing accounting.') }}</p>
      </template>
      <p v-if="success" role="status" class="text-sm">{{ success }}</p>
    </div>
    <TotpStepUpDialog :controller="stepUp" />
  </section>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import api, { type AdminArenaUserReport, type AdminArenaAccountingPreview } from '@/api/admin/play'
import { useStepUp, isStepUpCancelled, isStepUpBlocked, stepUpBlockReason } from '@/composables/useStepUp'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
const props = defineProps<{ periodId?: number }>()
const emit = defineEmits<{ activated: [] }>()
const { locale, t } = useI18n()
const text = (zh: string, en: string) => locale.value.startsWith('zh') ? zh : en
const stepUp = useStepUp()
const userId = ref(''); const date = ref(''); const loading = ref(false); const saving = ref(false); const error = ref(''); const success = ref('')
const report = ref<AdminArenaUserReport | null>(null); const preview = ref<AdminArenaAccountingPreview | null>(null)
const reason = ref(''); const confirmed = ref(false)
let sequence = 0
let mounted = true
let activationKey = ''
const tokenKeys = ['input', 'output', 'cache_creation', 'cache_read', 'total'] as const
const usageRows = computed(() => report.value ? [
  { label: text('所选日', 'Selected day'), window: formatDate(report.value.day_start) + ' → ' + formatDate(report.value.day_end), tokens: report.value.daily },
  { label: text('所选月', 'Selected month'), window: formatDate(report.value.month_start) + ' → ' + formatDate(report.value.month_end), tokens: report.value.monthly }
] : [])
function taskLabel(key: string) {
  const labels: Record<string, string> = { checkin: text('签到', 'Check-in'), api_call: text('API 调用', 'API usage'), image_generate: text('图片生成', 'Image generation') }
  return labels[key] || key
}
function formatDate(value: string) { return new Date(value).toLocaleString(locale.value, { timeZone: 'Asia/Shanghai' }) }
async function loadUser() {
  const seq = ++sequence; loading.value = true; error.value = ''; report.value = null
  try { const data = await api.getArenaUserReport(Number(userId.value), date.value || undefined); if (seq === sequence) report.value = data }
  catch { if (seq === sequence) error.value = text('用量读取失败，请检查用户 ID 和日期后重新查询。', 'Unable to load usage. Check the user ID and date, then retry.') }
  finally { if (seq === sequence) loading.value = false }
}
async function loadPreview() {
  if (!props.periodId) return
  const seq = ++sequence; loading.value = true; error.value = ''; preview.value = null; confirmed.value = false; success.value = ''
  try { const data = await api.previewArenaAccounting(props.periodId); if (seq === sequence) { preview.value = data; activationKey = crypto.randomUUID() } }
  catch { if (seq === sequence) error.value = text('口径预览失败，请重试。', 'Accounting preview failed. Please retry.') }
  finally { if (seq === sequence) loading.value = false }
}
async function activate() {
  const current = preview.value
  const savedReason = reason.value.trim()
  const savedKey = activationKey
  if (!current?.can_activate || !confirmed.value || saving.value || savedReason.length < 10 || savedReason.length > 500) return
  const seq = sequence
  saving.value = true; error.value = ''
  try {
    const updated = await stepUp.run(() => api.activateArenaAccounting(current.period_id, current.digest, savedReason, savedKey))
    if (!mounted || seq !== sequence || props.periodId !== current.period_id) return
    preview.value = updated
    confirmed.value = false; success.value = text('口径已切换，操作记录已保存。', 'Accounting updated and the operation recorded.'); emit('activated')
  } catch (err) {
    if (!mounted || seq !== sequence) return
    if (isStepUpCancelled(err)) return
    error.value = isStepUpBlocked(err) ? t(stepUpBlockReason(err) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN' ? 'stepUp.adminApiKeyForbidden' : 'stepUp.notEnabled') : text('切换未确认成功。请重新预览核对状态后重试。', 'Activation was not confirmed. Refresh the preview and check the state before retrying.')
  } finally { saving.value = false }
}
watch(() => props.periodId, () => { stepUp.onCancel(); sequence++; loading.value = false; preview.value = null; confirmed.value = false; success.value = '' })
onBeforeUnmount(() => { mounted = false; sequence++; stepUp.onCancel() })
</script>
