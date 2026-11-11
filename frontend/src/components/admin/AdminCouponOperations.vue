<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type {
  CouponRewardActivity,
  CouponRewardPoolInput,
  CouponRewardPoolVersion,
  CouponScope,
  CouponTemplate,
  CouponTemplateStatus,
  CouponTemplateInput,
  CouponIssueBatch,
  UserCoupon,
  UserCouponStatus,
} from '@/types/coupon'
import type { Column } from '@/components/common/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatCurrency, formatDateTime } from '@/utils/format'
import CouponCampaignPanel from './CouponCampaignPanel.vue'
import { couponErrorKey } from '@/utils/couponErrors'
import type { CouponSummary, CouponEvent, CouponClaimPolicy } from '@/types/couponCampaign'
const policies = ref<Record<number, CouponClaimPolicy>>({})
const claimTemplate = ref<CouponTemplate | null>(null)
const batches = ref<CouponIssueBatch[]>([])
const batchPage = ref(1)
const batchTotal = ref(0)
const batchLoading = ref(false)
const events = ref<CouponEvent[] | null>(null)
const eventCoupon = ref(0)
const summary = ref<CouponSummary>({ issued: 0, available: 0, used: 0, amounts: [] })

async function loadBatches() {
  batchLoading.value = true
  try {
    const { data } = await adminAPI.coupon.listBatches({ page: batchPage.value, page_size: 20 })
    batches.value = data.items
    batchTotal.value = data.total
  } catch (error) {
    appStore.showError(t(couponErrorKey(error)))
  } finally {
    batchLoading.value = false
  }
}

async function showEvents(id: number) {
  eventCoupon.value = id
  events.value = []
  try {
    events.value = (await adminAPI.coupon.events(id)).data
  } catch (error) {
    appStore.showError(t(couponErrorKey(error)))
  }
}

async function issued() {
  await Promise.all([loadUserCoupons(), loadBatches(), loadSelectableTemplates()])
}

export type CouponOperationsTab = 'templates' | 'issue' | 'blindbox' | 'quiz' | 'checkin'

const props = defineProps<{
  activeTab: CouponOperationsTab
}>()

const { t } = useI18n()
const appStore = useAppStore()
const templates = ref<CouponTemplate[]>([])
const templateLoading = ref(false)
const templateSearch = ref('')
const templatePagination = reactive({
  page: 1,
  page_size: 50,
  total: 0,
  pages: 1,
})
let templateLoadRequest = 0
const selectableTemplates = ref<CouponTemplate[]>([])
const selectableTemplatesLoading = ref(false)
let selectableTemplatesLoadRequest = 0
const userCoupons = ref<UserCoupon[]>([])
const userCouponLoading = ref(false)
const userCouponFilters = reactive({
  user: '',
  template_id: 0,
  source: '',
  status: '',
  issued_from: '',
  issued_to: '',
})
const userCouponPagination = reactive({
  page: 1,
  page_size: 50,
  total: 0,
  pages: 1,
})
let userCouponLoadRequest = 0
const pools = ref<CouponRewardPoolVersion[]>([])
const poolLoading = ref(false)
const saving = ref(false)
const showTemplateDialog = ref(false)
const showPoolDialog = ref(false)
const deletingTemplate = ref<CouponTemplate | null>(null)
const deletingPool = ref<CouponRewardPoolVersion | null>(null)
const voidingUserCoupon = ref<UserCoupon | null>(null)
const voidReason = ref('')
const editingTemplateID = ref<number | null>(null)
const editingPoolID = ref<number | null>(null)
const copyingPool = ref(false)

const activeSelectableTemplates = computed(() =>
  selectableTemplates.value.filter((template) => template.status === 'active'),
)

type CouponTemplateForm = CouponTemplateInput & { eligible_plan_ids_text: string }

function emptyTemplateForm(): CouponTemplateForm {
  return {
    key: '',
    name: '',
    description: '',
    status: 'active',
    benefit_type: 'fixed_amount',
    benefit_value: 1,
    max_discount_amount: null,
    currency: 'CNY',
    applicable_scopes: ['balance'],
    minimum_order_amount: 0,
    eligible_plan_ids: [],
    eligible_plan_ids_text: '',
    validity_mode: 'relative_days',
    validity_days: 3,
    fixed_expires_at: null,
    valid_from: null,
    total_issue_limit: null,
    rules: {},
  }
}

const templateForm = reactive<CouponTemplateForm>(emptyTemplateForm())
type EditablePool = CouponRewardPoolInput & { id?: number }
const poolForm = ref<EditablePool>(emptyPool())

function activeActivity(): CouponRewardActivity {
  if (props.activeTab === 'quiz') return 'quiz'
  if (props.activeTab === 'checkin') return 'checkin'
  return 'blindbox'
}

function emptyPool(): EditablePool {
  const activity = activeActivity()
  return {
    activity,
    version: '',
    status: 'draft',
    coupon_weight_bp: activity === 'blindbox' ? 6000 : 8000,
    redeem_code_weight_bp: activity === 'blindbox' ? 3000 : 2000,
    balance_weight_bp: activity === 'blindbox' ? 1000 : 0,
    reward_config: { redeem_entries: [], balance_entries: [] },
    fallback_template_id: 0,
    entries: [],
  }
}

const splitPresets = [
  { key: 'couponOnly', coupon: 10_000, redeem: 0, balance: 0 },
  { key: 'couponFirst', coupon: 8_000, redeem: 2_000, balance: 0 },
  { key: 'redeemFirst', coupon: 3_000, redeem: 7_000, balance: 0 },
  { key: 'lowBalance', coupon: 6_000, redeem: 3_000, balance: 1_000 },
] as const

const templateColumns = computed<Column[]>(() => [
  { key: 'name', label: t('coupon.admin.columns.template') },
  { key: 'benefit', label: t('coupon.admin.columns.benefit') },
  { key: 'scope', label: t('coupon.admin.columns.scope') },
  { key: 'validity', label: t('coupon.admin.columns.validity') },
  { key: 'issued_count', label: t('coupon.admin.columns.issued') },
  { key: 'status', label: t('coupon.admin.columns.status') },
  { key: 'actions', label: t('coupon.admin.columns.actions') },
])

const userCouponColumns = computed<Column[]>(() => [
  { key: 'id', label: t('coupon.admin.columns.userCoupon') },
  { key: 'template_name', label: t('coupon.admin.columns.template') },
  { key: 'user_id', label: t('coupon.admin.columns.user') },
  { key: 'source', label: t('coupon.admin.columns.issuedVia') },
  { key: 'expires_at', label: t('coupon.admin.columns.expiresAt') },
  { key: 'status', label: t('coupon.admin.columns.status') },
  { key: 'used_order', label: t('coupon.admin.columns.usedOrder') },
  { key: 'conversion', label: t('coupon.admin.columns.conversion') },
  { key: 'actions', label: t('coupon.admin.columns.actions') },
])

const poolColumns = computed<Column[]>(() => [
  { key: 'version', label: t('coupon.admin.columns.poolVersion') },
  { key: 'status', label: t('coupon.admin.columns.status') },
  { key: 'split', label: t('coupon.admin.columns.split') },
  { key: 'entries', label: t('coupon.admin.columns.entries') },
  { key: 'updated_at', label: t('coupon.admin.columns.updatedAt') },
  { key: 'actions', label: t('coupon.admin.columns.actions') },
])

const outerSplit = computed(() => t(`coupon.admin.${activeActivity()}Split`))

const outerWeightTotal = computed(() => Number(poolForm.value.coupon_weight_bp || 0) + Number(poolForm.value.redeem_code_weight_bp || 0) + Number(poolForm.value.balance_weight_bp || 0))

function formatBp(value: number): string {
  return `${(Number(value || 0) / 100).toFixed(2).replace(/\.00$/, '')}%`
}

function poolSplitLabel(pool: Pick<CouponRewardPoolVersion, 'coupon_weight_bp' | 'redeem_code_weight_bp' | 'balance_weight_bp'>): string {
  return t('coupon.admin.splitLabel3', {
    coupon: formatBp(pool.coupon_weight_bp),
    redeem: formatBp(pool.redeem_code_weight_bp || 0),
    balance: formatBp(pool.balance_weight_bp),
  })
}

function applySplitPreset(preset: typeof splitPresets[number]) {
  poolForm.value.coupon_weight_bp = preset.coupon
  poolForm.value.redeem_code_weight_bp = preset.redeem
  poolForm.value.balance_weight_bp = preset.balance
}

function ensureRewardConfig() {
  if (!poolForm.value.reward_config) poolForm.value.reward_config = { redeem_entries: [], balance_entries: [] }
  if (!poolForm.value.reward_config.redeem_entries) poolForm.value.reward_config.redeem_entries = []
  if (!poolForm.value.reward_config.balance_entries) poolForm.value.reward_config.balance_entries = []
  return poolForm.value.reward_config
}

function addRedeemEntry() {
  ensureRewardConfig().redeem_entries!.push({
    name: '',
    batch_name: '',
    code_type: '',
    weight_bp: 10_000,
    enabled: true,
    delivery_mode: 'issue_code',
  })
}

function removeRedeemEntry(index: number) {
  ensureRewardConfig().redeem_entries!.splice(index, 1)
}

function addBalanceEntry() {
  ensureRewardConfig().balance_entries!.push({
    name: '',
    amount: 0.5,
    weight_bp: 10_000,
    enabled: true,
  })
}

function removeBalanceEntry(index: number) {
  ensureRewardConfig().balance_entries!.splice(index, 1)
}

function parseIDs(input: string): number[] {
  return Array.from(new Set(input.split(/[\s,，]+/).map((value) => Number(value)).filter((value) => Number.isInteger(value) && value > 0)))
}

function toISO(value: string | null | undefined): string | null {
  if (!value) return null
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date.toISOString()
}

function localDate(value: string | null | undefined): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function templateBenefit(template: CouponTemplate): string {
  if (template.benefit_type === 'percentage') return t('couponCampaign.benefit',{off:template.benefit_value,pay:100-template.benefit_value})
  return `${template.currency} ${template.benefit_value.toFixed(2)}`
}

function templateScope(template: CouponTemplate): string {
  if (template.applicable_scopes.length === 2) return t('coupon.admin.scopeBoth')
  return template.applicable_scopes[0] === 'subscription'
    ? t('coupon.admin.scopeSubscription')
    : t('coupon.admin.scopeRecharge')
}

function templateValidity(template: CouponTemplate): string {
  if (template.validity_mode === 'fixed') return formatDateTime(template.fixed_expires_at)
  if (template.validity_mode === 'end_of_day') return t('coupon.admin.endOfDay')
  if (template.validity_mode === 'end_of_month') return t('coupon.admin.endOfMonth')
  return t('coupon.admin.days', { count: template.validity_days || 0 })
}

function statusClass(status: string): string {
  if (status === 'active' || status === 'published' || status === 'available') return 'badge-success'
  if (status === 'draft' || status === 'locked') return 'badge-warning'
  return 'badge-gray'
}

function templateStatusLabel(status: CouponTemplateStatus): string {
  return t(`coupon.admin.templateStatus.${status}`)
}

function userCouponStatusLabel(status: UserCouponStatus): string {
  return t(`coupon.admin.couponStatus.${status}`)
}

function poolStatusLabel(status: string): string {
  return t(`coupon.admin.poolStatus.${status}`)
}

function issueSourceLabel(source: string): string {
 if(source==='code_claim')return t('couponCampaign.codeSource')
  const key = `coupon.admin.issueSource.${source}`
  const translated = t(key)
  return translated === key ? source : translated
}

function orderStatusLabel(status: string | undefined): string {
  if (!status) return ''
  const normalized = status.trim().toUpperCase()
  const key = `coupon.admin.orderStatus.${normalized}`
  const translated = t(key)
  return translated === key ? status : translated
}

function orderTypeLabel(type: string | undefined): string {
  if (!type) return ''
  const key = `coupon.admin.orderType.${type}`
  const translated = t(key)
  return translated === key ? type : translated
}

function userCouponIdentity(coupon: UserCoupon): string {
  return coupon.user_email || coupon.user_name || `#${coupon.user_id}`
}

function userCouponOrderLabel(coupon: UserCoupon): string {
  if (!coupon.used_order_id) return '-'
  const orderNo = coupon.used_order_no || `#${coupon.used_order_id}`
  const type = orderTypeLabel(coupon.used_order_type)
  const status = orderStatusLabel(coupon.used_order_status)
  return [orderNo, type, status].filter(Boolean).join(' · ')
}

function userCouponConversion(coupon: UserCoupon): string {
  if (!coupon.used_order_id) return t('coupon.admin.conversionPending')
  return t('coupon.admin.conversionValue', {
    pay: formatCurrency(coupon.used_order_pay_amount || 0, coupon.used_order_currency || coupon.terms_snapshot?.currency || 'CNY'),
    discount: formatCurrency(coupon.used_order_discount_amount || 0, coupon.used_order_currency || coupon.terms_snapshot?.currency || 'CNY'),
  })
}

async function loadTemplates() {
  const requestID = ++templateLoadRequest
  templateLoading.value = true
  try {
    const response = await adminAPI.coupon.listTemplates({
      page: templatePagination.page,
      page_size: templatePagination.page_size,
      search: templateSearch.value.trim() || undefined,
    })
    if (requestID !== templateLoadRequest) return
    const rows = response.data.items || []
    const claimRows = rows.length ? (await adminAPI.coupon.listClaimPolicies(rows.map(v => v.id))).data : []
    if (requestID !== templateLoadRequest) return
    policies.value = Object.fromEntries(claimRows.map(v => [v.template_id, v]))
    templates.value = rows
    templatePagination.total = response.data.total || 0
    templatePagination.page = response.data.page || templatePagination.page
    templatePagination.page_size = response.data.page_size || templatePagination.page_size
    templatePagination.pages = response.data.pages || Math.max(1, Math.ceil(templatePagination.total / templatePagination.page_size))
    if (templates.value.length === 0 && templatePagination.total > 0 && templatePagination.page > templatePagination.pages) {
      templatePagination.page = templatePagination.pages
      void loadTemplates()
    }
  } catch {
    if (requestID !== templateLoadRequest) return
    templates.value = []
    templatePagination.total = 0
    templatePagination.pages = 1
    appStore.showError(t('coupon.admin.loadFailed'))
  } finally {
    if (requestID === templateLoadRequest) templateLoading.value = false
  }
}

async function loadSelectableTemplates() {
  const requestID = ++selectableTemplatesLoadRequest
  selectableTemplatesLoading.value = true
  const collected = new Map<number, CouponTemplate>()
  let page = 1
  let pages = 1
  try {
    while (page <= pages) {
      const response = await adminAPI.coupon.listTemplates({ page, page_size: 1000 })
      if (requestID !== selectableTemplatesLoadRequest) return
      for (const template of response.data.items || []) collected.set(template.id, template)

      const pageSize = response.data.page_size || 1000
      pages = response.data.pages || Math.max(1, Math.ceil((response.data.total || 0) / pageSize))
      page += 1
    }
    if (requestID !== selectableTemplatesLoadRequest) return
    selectableTemplates.value = Array.from(collected.values())
  } catch {
    if (requestID !== selectableTemplatesLoadRequest) return
    selectableTemplates.value = []
    appStore.showError(t('coupon.admin.loadFailed'))
  } finally {
    if (requestID === selectableTemplatesLoadRequest) selectableTemplatesLoading.value = false
  }
}

function applyTemplateSearch() {
  templatePagination.page = 1
  void loadTemplates()
}

function clearTemplateSearch() {
  if (!templateSearch.value) return
  templateSearch.value = ''
  templatePagination.page = 1
  void loadTemplates()
}

function changeTemplatePage(page: number) {
  if (page === templatePagination.page || page < 1 || page > templatePagination.pages) return
  templatePagination.page = page
  void loadTemplates()
}

function changeTemplatePageSize(pageSize: number) {
  if (!Number.isInteger(pageSize) || pageSize <= 0) return
  templatePagination.page_size = pageSize
  templatePagination.page = 1
  void loadTemplates()
}

async function loadUserCoupons() {
  const requestID = ++userCouponLoadRequest
  userCouponLoading.value = true
  try {
    const templateID = Number(userCouponFilters.template_id)
    const filters = {
      page: userCouponPagination.page,
      page_size: userCouponPagination.page_size,
      user: userCouponFilters.user.trim() || undefined,
      template_id: Number.isInteger(templateID) && templateID > 0 ? templateID : undefined,
      source: userCouponFilters.source || undefined,
      status: userCouponFilters.status || undefined,
      issued_from: toISO(userCouponFilters.issued_from) || undefined,
      issued_to: toISO(userCouponFilters.issued_to) || undefined,
    }
    const [response,stats]=await Promise.all([adminAPI.coupon.listUserCoupons(filters),adminAPI.coupon.summary(filters)])
    if (requestID !== userCouponLoadRequest) return
    summary.value=stats.data
    userCoupons.value = response.data.items || []
    userCouponPagination.total = response.data.total || 0
    userCouponPagination.page = response.data.page || userCouponPagination.page
    userCouponPagination.page_size = response.data.page_size || userCouponPagination.page_size
    userCouponPagination.pages = response.data.pages || Math.max(1, Math.ceil(userCouponPagination.total / userCouponPagination.page_size))
  } catch {
    if (requestID !== userCouponLoadRequest) return
    userCoupons.value = []
    summary.value = { issued: 0, available: 0, used: 0, amounts: [] }
    userCouponPagination.total = 0
    userCouponPagination.pages = 1
    appStore.showError(t('coupon.admin.loadFailed'))
  } finally {
    if (requestID === userCouponLoadRequest) userCouponLoading.value = false
  }
}

function applyUserCouponFilters() {
  userCouponPagination.page = 1
  void loadUserCoupons()
}

function clearUserCouponFilters() {
  userCouponFilters.user = ''
  userCouponFilters.template_id = 0
  userCouponFilters.source = ''
  userCouponFilters.status = ''
  userCouponFilters.issued_from = ''
  userCouponFilters.issued_to = ''
  userCouponPagination.page = 1
  void loadUserCoupons()
}

function changeUserCouponPage(page: number) {
  if (page === userCouponPagination.page || page < 1 || page > userCouponPagination.pages) return
  userCouponPagination.page = page
  void loadUserCoupons()
}

function changeUserCouponPageSize(pageSize: number) {
  if (!Number.isInteger(pageSize) || pageSize <= 0) return
  userCouponPagination.page_size = pageSize
  userCouponPagination.page = 1
  void loadUserCoupons()
}

async function loadPools() {
  poolLoading.value = true
  try {
    const response = await adminAPI.coupon.listPools(activeActivity())
    pools.value = response.data || []
  } catch {
    pools.value = []
    appStore.showError(t('coupon.admin.loadFailed'))
  } finally {
    poolLoading.value = false
  }
}

async function loadActiveTab() {
  if (props.activeTab === 'templates') {
    await loadTemplates()
    return
  }
  if (props.activeTab === 'issue') {
    await Promise.all([loadSelectableTemplates(), loadUserCoupons(),loadBatches()])
    return
  }
  await Promise.all([loadSelectableTemplates(), loadPools()])
}

function resetTemplateForm() {
  Object.assign(templateForm, emptyTemplateForm())
  editingTemplateID.value = null
}

function openCreateTemplate() {
  resetTemplateForm()
  showTemplateDialog.value = true
}

function copyTemplate(template: CouponTemplate) {
  openEditTemplate(template)
  editingTemplateID.value = null
  templateForm.key = ''
  templateForm.name = template.name + ' ' + t('common.copy')
}
function openEditTemplate(template: CouponTemplate) {
  Object.assign(templateForm, {
    ...template,
    eligible_plan_ids_text: (template.eligible_plan_ids || []).join(', '),
    fixed_expires_at: localDate(template.fixed_expires_at),
    valid_from: localDate(template.valid_from),
    max_discount_amount: template.max_discount_amount ?? null,
    total_issue_limit: template.total_issue_limit ?? null,
    rules: template.rules || {},
  })
  editingTemplateID.value = template.id
  showTemplateDialog.value = true
}

function updateScope(scope: CouponScope, checked: boolean) {
  const next = new Set(templateForm.applicable_scopes)
  if (checked) next.add(scope)
  else next.delete(scope)
  templateForm.applicable_scopes = Array.from(next)
}

function updateScopeFromEvent(scope: CouponScope, event: Event) {
  updateScope(scope, (event.target as HTMLInputElement).checked)
}

async function saveTemplate() {
  if (!templateForm.name.trim() || templateForm.applicable_scopes.length === 0) return
  saving.value = true
  try {
    const validityMode = templateForm.validity_mode
    const payload: CouponTemplateInput = {
      ...templateForm,
      key: templateForm.key.trim(),
      name: templateForm.name.trim(),
      description: templateForm.description?.trim() || '',
      eligible_plan_ids: parseIDs(templateForm.eligible_plan_ids_text),
      validity_mode: validityMode,
      validity_days: validityMode === 'relative_days' ? Number(templateForm.validity_days || 0) : 0,
      fixed_expires_at: validityMode === 'fixed' ? toISO(templateForm.fixed_expires_at) : null,
      valid_from: toISO(templateForm.valid_from),
      total_issue_limit: templateForm.total_issue_limit || null,
      max_discount_amount: templateForm.max_discount_amount || null,
    }
    if (editingTemplateID.value) await adminAPI.coupon.updateTemplate(editingTemplateID.value, payload)
    else await adminAPI.coupon.createTemplate(payload)
    showTemplateDialog.value = false
    appStore.showSuccess(t('coupon.admin.saved'))
    await Promise.all([loadTemplates(), loadSelectableTemplates()])
  } catch (e) {
    appStore.showError(t(couponErrorKey(e)))
  } finally {
    saving.value = false
  }
}

async function deleteTemplate() {
  if (!deletingTemplate.value) return
  saving.value = true
  try {
    await adminAPI.coupon.deleteTemplate(deletingTemplate.value.id)
    deletingTemplate.value = null
    appStore.showSuccess(t('coupon.admin.deleted'))
    await Promise.all([loadTemplates(), loadSelectableTemplates()])
  } catch (e) {
    appStore.showError(t(couponErrorKey(e)))
  } finally {
    saving.value = false
  }
}

function openVoidUserCoupon(coupon: UserCoupon) {
  voidingUserCoupon.value = coupon
  voidReason.value = ''
}

function closeVoidUserCoupon() {
  voidingUserCoupon.value = null
  voidReason.value = ''
}

async function voidUserCoupon() {
  const coupon = voidingUserCoupon.value
  const reason = voidReason.value.trim()
  if (!coupon || !reason) return
  saving.value = true
  try {
    await adminAPI.coupon.voidUserCoupon(coupon.id, reason)
    closeVoidUserCoupon()
    appStore.showSuccess(t('coupon.admin.voided'))
    if (userCoupons.value.length === 1 && userCouponPagination.page > 1) {
      userCouponPagination.page -= 1
    }
    await loadUserCoupons()
  } catch {
    appStore.showError(t('coupon.admin.voidFailed'))
  } finally {
    saving.value = false
  }
}

function openCreatePool() {
  editingPoolID.value = null
  copyingPool.value = false
  poolForm.value = emptyPool()
  showPoolDialog.value = true
}

function openEditPool(pool: CouponRewardPoolVersion) {
  editingPoolID.value = pool.id
  copyingPool.value = false
  poolForm.value = {
    id: pool.id,
    activity: pool.activity,
    version: pool.version,
    status: pool.status,
    coupon_weight_bp: pool.coupon_weight_bp,
    redeem_code_weight_bp: pool.redeem_code_weight_bp || 0,
    balance_weight_bp: pool.balance_weight_bp,
    reward_config: pool.reward_config || { redeem_entries: [], balance_entries: [] },
    fallback_template_id: pool.fallback_template_id,
    entries: (pool.entries || []).map((entry) => ({
      ...entry,
      starts_at: localDate(entry.starts_at),
      ends_at: localDate(entry.ends_at),
    })),
  }
  showPoolDialog.value = true
}

function copyPoolVersion(pool: CouponRewardPoolVersion) {
  const version = pool.version.trim() || `${pool.activity}-pool`
  const existing = new Set(pools.value.map((item) => item.version))
  let copyNumber = 1
  while (copyNumber < 10_000) {
    const suffix = `-draft-${copyNumber}`
    const candidate = `${version.slice(0, 80 - suffix.length)}${suffix}`
    if (!existing.has(candidate)) return candidate
    copyNumber += 1
  }
  return `${Date.now()}-draft`.slice(0, 80)
}

function copyPoolAsDraft(pool: CouponRewardPoolVersion) {
  editingPoolID.value = null
  copyingPool.value = true
  poolForm.value = {
    activity: pool.activity,
    version: copyPoolVersion(pool),
    status: 'draft',
    coupon_weight_bp: pool.coupon_weight_bp,
    redeem_code_weight_bp: pool.redeem_code_weight_bp || 0,
    balance_weight_bp: pool.balance_weight_bp,
    reward_config: pool.reward_config || { redeem_entries: [], balance_entries: [] },
    fallback_template_id: pool.fallback_template_id,
    entries: (pool.entries || []).map((entry, index) => ({
      template_id: entry.template_id,
      weight_bp: entry.weight_bp,
      enabled: entry.enabled,
      starts_at: localDate(entry.starts_at),
      ends_at: localDate(entry.ends_at),
      stock_cap: entry.stock_cap ?? null,
      per_user_issue_limit: entry.per_user_issue_limit ?? null,
      sort_order: index + 1,
    })),
  }
  showPoolDialog.value = true
}

function addPoolEntry() {
  const ordinaryTemplate = activeSelectableTemplates.value.find(
    (template) => template.id !== poolForm.value.fallback_template_id,
  ) ?? activeSelectableTemplates.value[0]
  poolForm.value.entries.push({
    template_id: ordinaryTemplate?.id || 0,
    weight_bp: 0,
    enabled: true,
    starts_at: null,
    ends_at: null,
    stock_cap: null,
    per_user_issue_limit: null,
    sort_order: poolForm.value.entries.length + 1,
  })
}

function removePoolEntry(index: number) {
  if (isFallbackEntry(poolForm.value.entries[index])) return
  poolForm.value.entries.splice(index, 1)
  poolForm.value.entries.forEach((entry, position) => { entry.sort_order = position + 1 })
}

function isFallbackEntry(entry: EditablePool['entries'][number]): boolean {
  return entry.template_id > 0 && entry.template_id === poolForm.value.fallback_template_id
}

function normalizeFallbackEntryConstraints() {
  const fallbackTemplateID = poolForm.value.fallback_template_id
  if (fallbackTemplateID <= 0) return
  if (!poolForm.value.entries.some((entry) => entry.template_id === fallbackTemplateID)) {
    poolForm.value.entries.push({
      template_id: fallbackTemplateID,
      weight_bp: 1,
      enabled: true,
      starts_at: null,
      ends_at: null,
      stock_cap: null,
      per_user_issue_limit: null,
      sort_order: poolForm.value.entries.length + 1,
    })
  }
  for (const entry of poolForm.value.entries) {
    if (!isFallbackEntry(entry)) continue
    entry.weight_bp = 1
    entry.enabled = true
    entry.starts_at = null
    entry.ends_at = null
    entry.stock_cap = null
    entry.per_user_issue_limit = null
  }
}

const ordinaryCouponWeightTotal = computed(() => poolForm.value.entries
  .filter((entry) => !isFallbackEntry(entry))
  .reduce((total, entry) => total + Number(entry.weight_bp || 0), 0))

async function savePool() {
  const form = poolForm.value
  normalizeFallbackEntryConstraints()
  if (!form.version.trim() || !form.fallback_template_id || form.entries.length === 0 || ordinaryCouponWeightTotal.value !== 10_000 || outerWeightTotal.value !== 10_000) return
  saving.value = true
  try {
    const payload: CouponRewardPoolInput = {
      activity: activeActivity(),
      version: form.version.trim(),
      status: form.status,
      coupon_weight_bp: Number(form.coupon_weight_bp || 0),
      redeem_code_weight_bp: Number(form.redeem_code_weight_bp || 0),
      balance_weight_bp: Number(form.balance_weight_bp || 0),
      reward_config: form.reward_config || { redeem_entries: [], balance_entries: [] },
      fallback_template_id: form.fallback_template_id,
      entries: form.entries.map((entry, index) => ({
        ...entry,
        weight_bp: isFallbackEntry(entry) ? 1 : entry.weight_bp,
        enabled: isFallbackEntry(entry) ? true : entry.enabled,
        starts_at: isFallbackEntry(entry) ? null : toISO(entry.starts_at),
        ends_at: isFallbackEntry(entry) ? null : toISO(entry.ends_at),
        stock_cap: isFallbackEntry(entry) ? null : entry.stock_cap ?? null,
        per_user_issue_limit: isFallbackEntry(entry) ? null : entry.per_user_issue_limit ?? null,
        sort_order: index + 1,
      })),
    }
    if (editingPoolID.value) await adminAPI.coupon.updatePool(editingPoolID.value, payload)
    else await adminAPI.coupon.createPool(payload)
    showPoolDialog.value = false
    appStore.showSuccess(t('coupon.admin.saved'))
    await loadPools()
  } catch (e) {
    appStore.showError(t(couponErrorKey(e)))
  } finally {
    saving.value = false
  }
}

async function publishPool(pool: CouponRewardPoolVersion) {
  saving.value = true
  try {
    await adminAPI.coupon.publishPool(pool.id)
    appStore.showSuccess(t('coupon.admin.published'))
    await loadPools()
  } catch {
    appStore.showError(t('coupon.admin.publishFailed'))
  } finally {
    saving.value = false
  }
}

async function deletePool() {
  const pool = deletingPool.value
  if (!pool || pool.status !== 'draft') return
  saving.value = true
  try {
    await adminAPI.coupon.deletePool(pool.id)
    deletingPool.value = null
    appStore.showSuccess(t('coupon.admin.poolDeleted'))
    await loadPools()
  } catch {
    appStore.showError(t('coupon.admin.deletePoolFailed'))
  } finally {
    saving.value = false
  }
}

watch(() => props.activeTab, () => { void loadActiveTab() }, { immediate: true })
watch(
  () => [poolForm.value.fallback_template_id, ...poolForm.value.entries.map((entry) => entry.template_id)],
  normalizeFallbackEntryConstraints,
)
</script>

<template>
  <section v-if="activeTab === 'templates'" class="space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('coupon.admin.templatesTitle') }}</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('coupon.admin.templatesDescription') }}</p>
      </div>
      <div class="flex w-full flex-wrap items-center gap-2 sm:w-auto">
        <form class="flex min-w-0 flex-1 gap-2 sm:w-72" data-test="template-search-form" @submit.prevent="applyTemplateSearch">
          <div class="relative min-w-0 flex-1">
            <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input v-model.trim="templateSearch" data-test="template-search" type="search" class="input pl-9" :placeholder="t('coupon.admin.searchTemplates')" :aria-label="t('coupon.admin.searchTemplates')" />
          </div>
          <button type="submit" class="btn btn-secondary" :disabled="templateLoading" :title="t('coupon.admin.searchTemplates')" :aria-label="t('coupon.admin.searchTemplates')"><Icon name="search" size="sm" /></button>
          <button v-if="templateSearch" type="button" class="btn btn-ghost" :disabled="templateLoading" :title="t('coupon.admin.clearFilters')" :aria-label="t('coupon.admin.clearFilters')" @click="clearTemplateSearch"><Icon name="x" size="sm" /></button>
        </form>
        <button type="button" class="btn btn-secondary" :disabled="templateLoading" :title="t('common.refresh')" :aria-label="t('common.refresh')" @click="loadTemplates"><Icon name="refresh" size="sm" /></button>
        <button type="button" class="btn btn-primary" @click="openCreateTemplate"><Icon name="plus" size="sm" class="mr-1" />{{ t('coupon.admin.newTemplate') }}</button>
      </div>
    </div>
    <DataTable :columns="templateColumns" :data="templates" :loading="templateLoading">
      <template #cell-name="{ row }"><div><p class="font-medium text-gray-900 dark:text-white">{{ row.name }}</p><code class="text-xs text-gray-500 dark:text-gray-400">{{ policies[row.id]?.code || row.key }}</code><p v-if="policies[row.id]" class="text-xs text-gray-500 dark:text-gray-400">{{ t(policies[row.id].enabled?'couponCampaign.open':'couponCampaign.closed') }} · {{ t('couponCampaign.policyCounts',{issued:policies[row.id].issued_count,manual:policies[row.id].manual_count,claimed:policies[row.id].claimed_count,used:policies[row.id].used_count}) }}</p></div></template>
      <template #cell-benefit="{ row }"><span class="tabular-nums">{{ templateBenefit(row) }}</span></template>
      <template #cell-scope="{ row }"><span>{{ templateScope(row) }}</span></template>
      <template #cell-validity="{ row }"><span class="text-sm text-gray-600 dark:text-gray-300">{{ templateValidity(row) }}</span></template>
      <template #cell-issued_count="{ row }"><span class="tabular-nums">{{ row.issued_count }} / {{ row.total_issue_limit ?? '∞' }}</span></template>
      <template #cell-status="{ row }"><span class="badge" :class="statusClass(row.status)">{{ templateStatusLabel(row.status) }}</span></template>
      <template #cell-actions="{ row }"><div class="flex flex-wrap gap-1"><button type="button" class="btn btn-secondary btn-sm" @click="claimTemplate=row">{{ t('couponCampaign.codeSettings') }}</button><button type="button" class="btn btn-secondary btn-sm" @click="openEditTemplate(row)">{{ t('common.edit') }}</button><button type="button" class="btn btn-secondary btn-sm" @click="copyTemplate(row)">{{ t('common.copy') }}</button><button type="button" class="btn btn-danger btn-sm" @click="deletingTemplate = row">{{ t('common.delete') }}</button></div></template>
    </DataTable>
    <Pagination
      v-if="templatePagination.total > 0"
      :page="templatePagination.page"
      :total="templatePagination.total"
      :page-size="templatePagination.page_size"
      @update:page="changeTemplatePage"
      @update:pageSize="changeTemplatePageSize"
    />
  </section>

  <section v-else-if="activeTab === 'issue'" class="space-y-4">
    <div>
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('coupon.admin.issueTitle') }}</h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('coupon.admin.issueDescription') }}</p>
    </div>
    <CouponCampaignPanel :templates="selectableTemplates" @issued="issued" />
    <form class="grid gap-3 rounded-lg border border-gray-200 p-4 sm:grid-cols-2 xl:grid-cols-4 xl:items-end dark:border-dark-700" @submit.prevent="applyUserCouponFilters">
      <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.filterUserID') }}</span><input v-model.trim="userCouponFilters.user" data-test="coupon-user-filter" class="input" /></label>
      <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.filterTemplate') }}</span><select v-model.number="userCouponFilters.template_id" data-test="coupon-template-filter" class="input" :disabled="selectableTemplatesLoading"><option :value="0">{{ t('coupon.admin.allTemplates') }}</option><option v-for="template in selectableTemplates" :key="template.id" :value="template.id">{{ template.name }}</option></select></label>
      <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.filterSource') }}</span><select v-model="userCouponFilters.source" data-test="coupon-source-filter" class="input"><option value="">{{ t('coupon.admin.allSources') }}</option><option value="blindbox">{{ t('coupon.admin.issueSource.blindbox') }}</option><option value="quiz">{{ t('coupon.admin.issueSource.quiz') }}</option><option value="checkin">{{ t('coupon.admin.issueSource.checkin') }}</option><option value="admin_batch">{{ t('coupon.admin.issueSource.admin_batch') }}</option><option value="manual">{{ t('coupon.admin.issueSource.manual') }}</option><option value="code_claim">{{ t('couponCampaign.codeSource') }}</option><option value="compensation">{{ t('coupon.admin.issueSource.compensation') }}</option></select></label>
      <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.filterStatus') }}</span><select v-model="userCouponFilters.status" data-test="coupon-status-filter" class="input"><option value="">{{ t('coupon.admin.allStatuses') }}</option><option value="available">{{ t('coupon.wallet.status.available') }}</option><option value="locked">{{ t('coupon.wallet.status.locked') }}</option><option value="used">{{ t('coupon.wallet.status.used') }}</option><option value="expired">{{ t('coupon.wallet.status.expired') }}</option><option value="voided">{{ t('coupon.wallet.status.voided') }}</option></select></label>
      <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.issuedFrom') }}</span><input v-model="userCouponFilters.issued_from" data-test="coupon-issued-from-filter" type="datetime-local" class="input" /></label>
      <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.issuedTo') }}</span><input v-model="userCouponFilters.issued_to" data-test="coupon-issued-to-filter" type="datetime-local" class="input" /></label>
      <button type="submit" class="btn btn-secondary" data-test="apply-coupon-user-filters" :disabled="userCouponLoading"><Icon name="filter" size="sm" class="mr-1" />{{ t('coupon.admin.applyFilters') }}</button>
      <button type="button" class="btn btn-ghost" data-test="clear-coupon-user-filters" :disabled="userCouponLoading" @click="clearUserCouponFilters"><Icon name="x" size="sm" class="mr-1" />{{ t('coupon.admin.clearFilters') }}</button>
    </form>
    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <div class="card p-4"><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('couponCampaign.totalIssued') }}</p><p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ summary.issued }}</p></div>
      <div class="card p-4"><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('coupon.admin.dashboardAvailable') }}</p><p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ summary.available }}</p></div>
      <div class="card p-4"><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('coupon.admin.dashboardUsed') }}</p><p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ summary.used }}</p></div>
      <div class="card p-4"><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('couponCampaign.netPaid') }}</p><p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white"><span v-if="!summary.amounts.length">—</span><span v-for="a in summary.amounts" :key="a.currency" class="block">{{ formatCurrency(a.net_paid,a.currency) }}<span class="block text-xs font-normal">{{ t('couponCampaign.amounts',{paid:formatCurrency(a.paid,a.currency),discount:formatCurrency(a.discount,a.currency),refund:formatCurrency(a.refunded,a.currency)}) }}</span></span></p></div>
    </div>
    <DataTable :columns="userCouponColumns" :data="userCoupons" :loading="userCouponLoading">
      <template #cell-id="{ row }"><code>#{{ row.id }}</code></template>
      <template #cell-template_name="{ row }">{{ row.template_name || row.terms_snapshot?.name }}</template>
      <template #cell-user_id="{ row }"><div><p class="font-medium text-gray-900 dark:text-white">{{ userCouponIdentity(row) }}</p><p class="text-xs text-gray-500 dark:text-gray-400">UID {{ row.user_id }}</p></div></template>
      <template #cell-source="{ row }"><div><p>{{ issueSourceLabel(row.source) }}</p><p v-if="row.source_ref" class="text-xs text-gray-500 dark:text-gray-400">{{ row.source_ref }}</p></div></template>
      <template #cell-expires_at="{ row }">{{ formatDateTime(row.expires_at) }}</template>
      <template #cell-status="{ row }"><span class="badge" :class="statusClass(row.status)">{{ userCouponStatusLabel(row.status) }}</span></template>
      <template #cell-used_order="{ row }"><span>{{ userCouponOrderLabel(row) }}</span></template>
      <template #cell-conversion="{ row }"><span>{{ userCouponConversion(row) }}</span></template>
      <template #cell-actions="{ row }"><button type="button" class="btn btn-secondary btn-sm" @click="showEvents(row.id)">{{ t('couponCampaign.history') }}</button><button v-if="row.status === 'available'" type="button" class="btn btn-danger btn-sm" data-test="void-user-coupon" @click="openVoidUserCoupon(row)">{{ t('coupon.admin.voidCoupon') }}</button></template>
    </DataTable>
    <Pagination
      v-if="userCouponPagination.total > 0"
      :page="userCouponPagination.page"
      :total="userCouponPagination.total"
      :page-size="userCouponPagination.page_size"
      @update:page="changeUserCouponPage"
      @update:pageSize="changeUserCouponPageSize"
    />
    <div class="space-y-3 border-t border-gray-200 pt-4 dark:border-dark-700"><h3 class="text-base font-semibold">{{ t('couponCampaign.batches') }}</h3><button type="button" class="btn btn-secondary btn-sm" :disabled="batchLoading" @click="loadBatches">{{ t('common.refresh') }}</button><ul class="divide-y divide-gray-100 dark:divide-dark-700"><li v-for="b in batches" :key="b.id" class="py-3 text-sm"><button type="button" class="btn btn-secondary btn-sm" @click="userCouponFilters.template_id=b.template_id;applyUserCouponFilters()">#{{ b.id }} · {{ b.template_name }}</button> {{ t('couponCampaign.batchCounts',{requested:b.requested_count,issued:b.issued_count,skipped:b.requested_count-b.issued_count}) }} · {{ formatDateTime(b.created_at) }}<details v-if="b.input_snapshot"><summary>{{ t('couponCampaign.details') }}</summary><p class="break-all">{{ t('couponCampaign.ids') }}: {{ b.input_snapshot.user_ids?.join(', ') }}</p><ul><li v-for="v in b.input_snapshot.skipped" :key="v.user_id">#{{ v.user_id }} · {{ t('couponCampaign.reasons.'+v.reason) }}</li></ul></details></li></ul><Pagination v-if="batchTotal>20" :page="batchPage" :total="batchTotal" :page-size="20" @update:page="batchPage=$event;loadBatches()" /></div>
  </section>

  <section v-else class="space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t(`coupon.admin.${activeActivity()}PoolTitle`) }}</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ outerSplit }}</p>
      </div>
      <div class="flex gap-2"><button type="button" class="btn btn-secondary" :disabled="poolLoading" @click="loadPools"><Icon name="refresh" size="sm" /></button><button type="button" class="btn btn-primary" :disabled="selectableTemplatesLoading" @click="openCreatePool"><Icon name="plus" size="sm" class="mr-1" />{{ t('coupon.admin.newPool') }}</button></div>
    </div>
    <DataTable :columns="poolColumns" :data="pools" :loading="poolLoading">
      <template #cell-status="{ row }"><span class="badge" :class="statusClass(row.status)">{{ poolStatusLabel(row.status) }}</span></template>
      <template #cell-split="{ row }"><span class="tabular-nums">{{ poolSplitLabel(row) }}</span></template>
      <template #cell-entries="{ row }"><span>{{ row.entries?.length || 0 }}</span></template>
      <template #cell-updated_at="{ row }">{{ formatDateTime(row.updated_at) }}</template>
      <template #cell-actions="{ row }"><div class="flex flex-wrap gap-1"><button v-if="row.status === 'draft'" type="button" class="btn btn-secondary btn-sm" data-test="edit-draft-pool" @click="openEditPool(row)">{{ t('common.edit') }}</button><button v-else type="button" class="btn btn-secondary btn-sm" data-test="copy-immutable-pool" @click="copyPoolAsDraft(row)"><Icon name="copy" size="sm" class="mr-1" />{{ t('coupon.admin.copyPool') }}</button><button v-if="row.status === 'draft'" type="button" class="btn btn-primary btn-sm" data-test="publish-draft-pool" :disabled="saving" @click="publishPool(row)">{{ t('coupon.admin.publish') }}</button><button v-if="row.status === 'draft'" type="button" class="btn btn-danger btn-sm" data-test="delete-draft-pool" :disabled="saving" @click="deletingPool = row">{{ t('coupon.admin.deletePool') }}</button></div></template>
    </DataTable>
  </section>

  <BaseDialog :show="showTemplateDialog" :title="editingTemplateID ? t('coupon.admin.editTemplate') : t('coupon.admin.newTemplate')" width="wide" @close="showTemplateDialog = false">
    <form id="coupon-template-form" class="grid gap-4 md:grid-cols-2" @submit.prevent="saveTemplate">
      <p class="text-sm text-gray-500 dark:text-gray-400 md:col-span-2">{{ t('couponCampaign.autoCode') }}</p>
      <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.name') }}</span><input v-model.trim="templateForm.name" class="input" required /></label>
      <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.benefitType') }}</span><select v-model="templateForm.benefit_type" class="input"><option value="fixed_amount">{{ t('coupon.admin.fixedAmount') }}</option><option value="percentage">{{ t('coupon.admin.percentage') }}</option></select></label>
      <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.benefitValue') }}</span><input v-model.number="templateForm.benefit_value" type="number" min="0.01" step="0.01" class="input" required /></label>
      <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.maxDiscount') }}</span><input v-model.number="templateForm.max_discount_amount" type="number" min="0" step="0.01" class="input" /></label>
      <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.currency') }}</span><input v-model.trim="templateForm.currency" data-test="coupon-template-currency" class="input uppercase" maxlength="3" required /></label>
      <fieldset class="md:col-span-2"><legend class="input-label">{{ t('coupon.admin.scope') }}</legend><div class="mt-2 flex flex-wrap gap-4"><label class="inline-flex items-center gap-2 text-sm"><input type="checkbox" :checked="templateForm.applicable_scopes.includes('balance')" @change="updateScopeFromEvent('balance', $event)" />{{ t('coupon.admin.scopeRecharge') }}</label><label class="inline-flex items-center gap-2 text-sm"><input type="checkbox" :checked="templateForm.applicable_scopes.includes('subscription')" @change="updateScopeFromEvent('subscription', $event)" />{{ t('coupon.admin.scopeSubscription') }}</label></div></fieldset>
      <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.minimumOrder') }}</span><input v-model.number="templateForm.minimum_order_amount" type="number" min="0" step="0.01" class="input" /></label>
      <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.eligiblePlans') }}</span><input v-model.trim="templateForm.eligible_plan_ids_text" class="input" :placeholder="t('coupon.admin.eligiblePlansPlaceholder')" /></label>
      <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.validity') }}</span><select v-model="templateForm.validity_mode" data-test="coupon-template-validity-mode" class="input"><option value="relative_days">{{ t('coupon.admin.relativeDays') }}</option><option value="end_of_day">{{ t('coupon.admin.endOfDay') }}</option><option value="end_of_month">{{ t('coupon.admin.endOfMonth') }}</option><option value="fixed">{{ t('coupon.admin.fixedDate') }}</option></select></label>
      <label v-if="templateForm.validity_mode === 'relative_days'" class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.validityDays') }}</span><input v-model.number="templateForm.validity_days" type="number" min="1" class="input" /></label>
      <label v-if="templateForm.validity_mode === 'fixed'" class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.fixedExpiry') }}</span><input v-model="templateForm.fixed_expires_at" type="datetime-local" class="input" /></label>
      <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.validFrom') }}</span><input v-model="templateForm.valid_from" type="datetime-local" class="input" /></label>
      <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.totalIssueLimit') }}</span><input v-model.number="templateForm.total_issue_limit" type="number" min="0" class="input" /></label>
      <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.status') }}</span><select v-model="templateForm.status" class="input"><option value="draft">{{ t('coupon.admin.templateStatus.draft') }}</option><option value="active">{{ t('coupon.admin.templateStatus.active') }}</option><option value="paused">{{ t('coupon.admin.templateStatus.paused') }}</option><option value="archived">{{ t('coupon.admin.templateStatus.archived') }}</option></select></label>
      <label class="grid gap-1 text-sm md:col-span-2"><span class="input-label">{{ t('coupon.admin.description') }}</span><textarea v-model.trim="templateForm.description" rows="3" class="input" /></label>
    </form>
    <template #footer><div class="flex justify-end gap-3"><button type="button" class="btn btn-secondary" @click="showTemplateDialog = false">{{ t('common.cancel') }}</button><button type="submit" form="coupon-template-form" class="btn btn-primary" :disabled="saving">{{ saving ? t('common.saving') : t('common.save') }}</button></div></template>
  </BaseDialog>

  <BaseDialog :show="!!claimTemplate" :title="t('couponCampaign.codeSettings')" width="wide" @close="claimTemplate=null;loadTemplates()"><CouponCampaignPanel v-if="claimTemplate" :templates="selectableTemplates" :template-id="claimTemplate.id" policy-mode /></BaseDialog>
  <BaseDialog :show="events!==null" :title="t('couponCampaign.history')+' #'+eventCoupon" width="wide" @close="events=null"><ol class="space-y-3"><li v-for="e in events" :key="e.id" class="text-sm">{{ formatDateTime(e.created_at) }} · {{ t('couponCampaign.events.'+e.event_type) }} · {{ e.order_id?'#'+e.order_id:'' }}</li></ol><p v-if="!events?.length">{{ t('common.noData') }}</p><p class="mt-4 text-xs text-gray-500 dark:text-gray-400">{{ t('couponCampaign.refundHint') }}</p></BaseDialog>
  <BaseDialog :show="showPoolDialog" :title="editingPoolID ? t('coupon.admin.editPool') : copyingPool ? t('coupon.admin.copyPool') : t('coupon.admin.newPool')" width="wide" @close="showPoolDialog = false">
    <form id="coupon-pool-form" class="space-y-4" @submit.prevent="savePool">
      <p class="rounded-lg border border-primary-200 bg-primary-50 px-3 py-2 text-sm text-primary-800 dark:border-primary-500/30 dark:bg-primary-500/10 dark:text-primary-200">{{ outerSplit }}</p>
      <div class="grid gap-4 md:grid-cols-2"><label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.poolVersion') }}</span><input v-model.trim="poolForm.version" class="input" required /></label><label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.fallbackTemplate') }}</span><select v-model.number="poolForm.fallback_template_id" data-test="pool-fallback-template" class="input" :disabled="selectableTemplatesLoading" required><option :value="0" disabled>{{ t('coupon.admin.selectTemplate') }}</option><option v-for="template in activeSelectableTemplates" :key="template.id" :value="template.id">{{ template.name }}</option></select><span class="text-xs text-gray-500 dark:text-gray-400">{{ t('coupon.admin.fallbackHint') }}</span></label></div>
      <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-600">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('coupon.admin.splitConfigTitle') }}</h3>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('coupon.admin.splitConfigHint') }}</p>
          </div>
          <div class="flex flex-wrap gap-2">
            <button v-for="preset in splitPresets" :key="preset.key" type="button" class="btn btn-secondary btn-sm" @click="applySplitPreset(preset)">{{ t(`coupon.admin.splitPresets.${preset.key}`) }}</button>
          </div>
        </div>
        <div class="mt-3 grid gap-3 md:grid-cols-3">
          <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.couponWeight') }}</span><input v-model.number="poolForm.coupon_weight_bp" data-test="pool-coupon-weight" type="number" min="0" max="10000" step="100" class="input" /><span class="text-xs text-gray-500 dark:text-gray-400">{{ formatBp(poolForm.coupon_weight_bp) }}</span></label>
          <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.redeemCodeWeight') }}</span><input v-model.number="poolForm.redeem_code_weight_bp" data-test="pool-redeem-weight" type="number" min="0" max="10000" step="100" class="input" /><span class="text-xs text-gray-500 dark:text-gray-400">{{ formatBp(poolForm.redeem_code_weight_bp || 0) }}</span></label>
          <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.balanceWeight') }}</span><input v-model.number="poolForm.balance_weight_bp" data-test="pool-balance-weight" type="number" min="0" max="10000" step="100" class="input" /><span class="text-xs text-gray-500 dark:text-gray-400">{{ formatBp(poolForm.balance_weight_bp) }}</span></label>
        </div>
        <p class="mt-2 text-sm" :class="outerWeightTotal === 10000 ? 'text-emerald-700 dark:text-emerald-300' : 'text-rose-600 dark:text-rose-300'">{{ t('coupon.admin.splitTotal', { total: formatBp(outerWeightTotal) }) }}</p>
      </section>
      <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-600">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('coupon.admin.redeemPrizeTitle') }}</h3>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('coupon.admin.redeemPrizeHint') }}</p>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" @click="addRedeemEntry"><Icon name="plus" size="sm" class="mr-1" />{{ t('coupon.admin.addRedeemPrize') }}</button>
        </div>
        <div class="mt-3 space-y-2">
          <div v-for="(entry, index) in poolForm.reward_config?.redeem_entries || []" :key="`redeem-${index}`" class="grid gap-2 rounded-lg border border-gray-100 p-3 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,0.8fr)_120px_90px_auto] md:items-end dark:border-dark-700">
            <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.prizeName') }}</span><input v-model.trim="entry.name" class="input" /></label>
            <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.redeemBatchName') }}</span><input v-model.trim="entry.batch_name" class="input" :placeholder="t('coupon.admin.redeemBatchPlaceholder')" /></label>
            <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.redeemCodeType') }}</span><input v-model.trim="entry.code_type" class="input" placeholder="balance / subscription" /></label>
            <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.weight') }}</span><input v-model.number="entry.weight_bp" type="number" min="1" max="10000" class="input" /></label>
            <label class="inline-flex items-center gap-2 pb-2 text-sm"><input v-model="entry.enabled" type="checkbox" />{{ t('coupon.admin.enabled') }}</label>
            <button type="button" class="btn btn-danger btn-sm" @click="removeRedeemEntry(index)">{{ t('coupon.admin.remove') }}</button>
          </div>
        </div>
      </section>
      <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-600">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('coupon.admin.balancePrizeConfigTitle') }}</h3>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('coupon.admin.balancePrizeConfigHint') }}</p>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" @click="addBalanceEntry"><Icon name="plus" size="sm" class="mr-1" />{{ t('coupon.admin.addBalancePrize') }}</button>
        </div>
        <div class="mt-3 space-y-2">
          <div v-for="(entry, index) in poolForm.reward_config?.balance_entries || []" :key="`balance-${index}`" class="grid gap-2 rounded-lg border border-gray-100 p-3 md:grid-cols-[minmax(0,1fr)_120px_120px_90px_auto] md:items-end dark:border-dark-700">
            <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.prizeName') }}</span><input v-model.trim="entry.name" class="input" /></label>
            <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.rewardAmount') }}</span><input v-model.number="entry.amount" type="number" min="0.01" step="0.01" class="input" /></label>
            <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.weight') }}</span><input v-model.number="entry.weight_bp" type="number" min="1" max="10000" class="input" /></label>
            <label class="inline-flex items-center gap-2 pb-2 text-sm"><input v-model="entry.enabled" type="checkbox" />{{ t('coupon.admin.enabled') }}</label>
            <button type="button" class="btn btn-danger btn-sm" @click="removeBalanceEntry(index)">{{ t('coupon.admin.remove') }}</button>
          </div>
        </div>
      </section>
      <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-600">
        <table class="min-w-full text-left text-sm">
          <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-700/50 dark:text-gray-400">
            <tr>
              <th class="px-3 py-2">{{ t('coupon.admin.template') }}</th>
              <th class="px-3 py-2">{{ t('coupon.admin.weight') }}</th>
              <th class="px-3 py-2">{{ t('coupon.admin.startsAt') }}</th>
              <th class="px-3 py-2">{{ t('coupon.admin.endsAt') }}</th>
              <th class="px-3 py-2">{{ t('coupon.admin.stockCap') }}</th>
              <th class="px-3 py-2">{{ t('coupon.admin.perUserLimit') }}</th>
              <th class="px-3 py-2">{{ t('coupon.admin.enabled') }}</th>
              <th class="px-3 py-2"><span class="sr-only">{{ t('coupon.admin.remove') }}</span></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="(entry, index) in poolForm.entries" :key="index">
              <td class="px-3 py-2"><select v-model.number="entry.template_id" class="input min-w-44" :disabled="selectableTemplatesLoading"><option :value="0" disabled>{{ t('coupon.admin.selectTemplate') }}</option><option v-for="template in activeSelectableTemplates" :key="template.id" :value="template.id">{{ template.name }}</option></select></td>
              <td class="px-3 py-2"><input v-model.number="entry.weight_bp" data-test="pool-entry-weight" type="number" min="1" max="10000" class="input w-28" :disabled="isFallbackEntry(entry)" /></td>
              <td class="px-3 py-2"><input v-model="entry.starts_at" data-test="pool-entry-start" type="datetime-local" class="input min-w-44" :disabled="isFallbackEntry(entry)" /></td>
              <td class="px-3 py-2"><input v-model="entry.ends_at" data-test="pool-entry-end" type="datetime-local" class="input min-w-44" :disabled="isFallbackEntry(entry)" /></td>
              <td class="px-3 py-2"><input v-model.number="entry.stock_cap" data-test="pool-entry-stock-cap" type="number" min="0" class="input w-28" :disabled="isFallbackEntry(entry)" /></td>
              <td class="px-3 py-2"><input v-model.number="entry.per_user_issue_limit" data-test="pool-entry-per-user-limit" type="number" min="0" class="input w-28" :disabled="isFallbackEntry(entry)" /></td>
              <td class="px-3 py-2"><input v-model="entry.enabled" type="checkbox" :disabled="isFallbackEntry(entry)" /></td>
              <td class="px-3 py-2"><button type="button" class="btn btn-danger btn-sm" :disabled="isFallbackEntry(entry)" @click="removePoolEntry(index)">{{ t('coupon.admin.remove') }}</button></td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="flex flex-wrap items-center justify-between gap-3"><button type="button" class="btn btn-secondary" :disabled="selectableTemplatesLoading" @click="addPoolEntry"><Icon name="plus" size="sm" class="mr-1" />{{ t('coupon.admin.addEntry') }}</button><p class="text-sm" :class="ordinaryCouponWeightTotal === 10000 ? 'text-emerald-700 dark:text-emerald-300' : 'text-rose-600 dark:text-rose-300'">{{ t('coupon.admin.weightTotal', { total: ordinaryCouponWeightTotal }) }}</p></div>
    </form>
    <template #footer><div class="flex justify-end gap-3"><button type="button" class="btn btn-secondary" @click="showPoolDialog = false">{{ t('common.cancel') }}</button><button type="submit" form="coupon-pool-form" class="btn btn-primary" :disabled="saving || selectableTemplatesLoading || ordinaryCouponWeightTotal !== 10000 || outerWeightTotal !== 10000">{{ saving ? t('common.saving') : t('common.save') }}</button></div></template>
  </BaseDialog>

  <BaseDialog :show="!!voidingUserCoupon" :title="t('coupon.admin.voidCoupon')" @close="closeVoidUserCoupon">
    <form id="void-user-coupon-form" class="space-y-3" @submit.prevent="voidUserCoupon">
      <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('coupon.admin.voidCouponConfirm') }}</p>
      <label class="grid gap-1 text-sm"><span class="input-label">{{ t('coupon.admin.voidReason') }}</span><textarea v-model.trim="voidReason" data-test="void-coupon-reason" class="input" rows="3" :placeholder="t('coupon.admin.voidReasonPlaceholder')" maxlength="500" required /></label>
    </form>
    <template #footer><div class="flex justify-end gap-3"><button type="button" class="btn btn-secondary" @click="closeVoidUserCoupon">{{ t('common.cancel') }}</button><button type="submit" form="void-user-coupon-form" class="btn btn-danger" :disabled="saving || !voidReason.trim()">{{ saving ? t('common.processing') : t('coupon.admin.voidCoupon') }}</button></div></template>
  </BaseDialog>

  <ConfirmDialog :show="!!deletingTemplate" :title="t('coupon.admin.deleteTemplate')" :message="t('coupon.admin.deleteTemplateConfirm')" :confirm-text="t('common.delete')" :cancel-text="t('common.cancel')" danger @confirm="deleteTemplate" @cancel="deletingTemplate = null" />
  <ConfirmDialog :show="!!deletingPool" :title="t('coupon.admin.deletePool')" :message="t('coupon.admin.deletePoolConfirm')" :confirm-text="t('common.delete')" :cancel-text="t('common.cancel')" danger @confirm="deletePool" @cancel="deletingPool = null" />
</template>
