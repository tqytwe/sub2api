<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import adminPlayAPI, {
  type AdminInviteGrowthOverview,
  type AdminReferralCampaignDetail,
  type AdminReferralCampaignInput,
  type AdminReferralCampaignInvite,
  type AdminReferralCampaignPage,
  type AdminReferralCampaignParticipant,
  type AdminReferralCampaignReward,
  type AdminReferralCampaignStatus,
  type AdminReferralPage,
  type AdminReferralReviewDecision,
  type AdminReferralReviewType,
} from '@/api/admin/play'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

type DetailTab = 'participants' | 'invites' | 'rewards'

interface CampaignDraft extends AdminReferralCampaignInput {
  id?: number
  version?: number
}

const { t, locale } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const campaignsLoading = ref(false)
const detailLoading = ref(false)
const ledgerLoading = ref(false)
const saving = ref(false)
const actionLoading = ref(false)
const error = ref('')
const overview = ref<AdminInviteGrowthOverview | null>(null)
const campaigns = ref<AdminReferralCampaignPage>({ items: [], total: 0, page: 1, page_size: 20 })
const selectedID = ref<number | null>(null)
const detail = ref<AdminReferralCampaignDetail | null>(null)
const campaignSearch = ref('')
const campaignStatus = ref('')
const detailTab = ref<DetailTab>('participants')
const ledgerSearch = ref('')
const ledgerStatus = ref('')
const participants = ref<AdminReferralPage<AdminReferralCampaignParticipant>>(emptyPage())
const invites = ref<AdminReferralPage<AdminReferralCampaignInvite>>(emptyPage())
const rewards = ref<AdminReferralPage<AdminReferralCampaignReward>>(emptyPage())
const statusNote = ref('')
const reviewNote = ref('')
const showCampaignDialog = ref(false)
const campaignDraft = ref<CampaignDraft>(blankCampaignDraft())
const debtReward = ref<AdminReferralCampaignReward | null>(null)
const debtNote = ref('')

const statuses: AdminReferralCampaignStatus[] = [
  'draft', 'review', 'approved', 'scheduled', 'running', 'paused', 'settling', 'closed', 'cancelled',
]
const reviewTypes: AdminReferralReviewType[] = ['ops', 'finance', 'risk', 'ux']
const inviteStatuses = ['pending', 'approved', 'rejected', 'revoked']
const rewardStatuses = ['claimable', 'claimed_frozen', 'available', 'expired', 'revoked', 'debt_review', 'resolved']

const stats = computed(() => [
  { label: t('admin.playOps.inviteGrowth.invited'), value: overview.value?.invited_count ?? 0 },
  { label: t('admin.playOps.inviteGrowth.paidInvitees'), value: overview.value?.paid_invitee_count ?? 0 },
  { label: t('admin.playOps.inviteGrowth.qualified'), value: overview.value?.qualified_count ?? 0 },
  { label: t('admin.playOps.inviteGrowth.unlocked'), value: formatMoney(overview.value?.reward_unlocked ?? 0) },
  { label: t('admin.playOps.inviteGrowth.claimed'), value: formatMoney(overview.value?.reward_claimed ?? 0) },
])

const detailStats = computed(() => {
  const value = detail.value?.stats
  return [
    { key: 'enrolled', value: value?.enrolled ?? 0 },
    { key: 'attributed', value: value?.attributed ?? 0 },
    { key: 'qualified', value: value?.qualified ?? 0 },
    { key: 'riskPending', value: value?.risk_pending ?? 0 },
    { key: 'reserved', value: formatMoney(value?.rewards_reserved ?? 0) },
    { key: 'claimed', value: formatMoney(value?.rewards_claimed ?? 0) },
    { key: 'expired', value: formatMoney(value?.rewards_expired ?? 0) },
    { key: 'revoked', value: formatMoney(value?.rewards_revoked ?? 0) },
  ]
})

const statusActions = computed<AdminReferralCampaignStatus[]>(() => {
  const current = detail.value?.campaign.status
  if (current === 'draft') return ['review', 'cancelled']
  if (current === 'approved') return ['scheduled', 'cancelled']
  if (current === 'scheduled') return ['running', 'paused', 'cancelled']
  if (current === 'running') return ['paused', 'settling']
  if (current === 'paused') return ['running', 'settling', 'cancelled']
  if (current === 'settling') return ['closed']
  return []
})

const perUserLiability = computed(() => {
  const amounts = campaignDraft.value.tiers.map(item => Number(item.reward_amount) || 0)
  return campaignDraft.value.reward_mode === 'replace'
    ? Math.max(0, ...amounts)
    : amounts.reduce((sum, amount) => sum + amount, 0)
})
const maximumLiability = computed(() => perUserLiability.value * Math.max(0, Number(campaignDraft.value.max_enrollments) || 0))
const coveredEnrollments = computed(() => perUserLiability.value > 0
  ? Math.floor(Math.max(0, Number(campaignDraft.value.budget_total) || 0) / perUserLiability.value)
  : 0)
const budgetGap = computed(() => Math.max(0, maximumLiability.value - (Number(campaignDraft.value.budget_total) || 0)))
const currentLedger = computed(() => detailTab.value === 'participants' ? participants.value : detailTab.value === 'invites' ? invites.value : rewards.value)

function emptyPage<T>(): AdminReferralPage<T> {
  return { items: [], total: 0, page: 1, page_size: 20 }
}

function dateTimeLocal(offsetDays: number): string {
  const date = new Date(Date.now() + offsetDays * 24 * 60 * 60 * 1000)
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 16)
}

function blankCampaignDraft(): CampaignDraft {
  return {
    key: '',
    name: '',
    registration_from: dateTimeLocal(0),
    registration_to: dateTimeLocal(7),
    starts_at: dateTimeLocal(0),
    ends_at: dateTimeLocal(30),
    qualification_to: dateTimeLocal(35),
    claim_deadline: dateTimeLocal(40),
    risk_hold_hours: 168,
    pay_threshold: 100,
    usage_threshold: 20,
    max_enrollments: 100,
    budget_total: 10000,
    reward_mode: 'additive',
    tiers: [{ tier: 1, required_invites: 1, reward_amount: 0, currency: 'CNY' }],
  }
}

function toLocalInput(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Date(date.getTime() - date.getTimezoneOffset() * 60_000).toISOString().slice(0, 16)
}

function formatMoney(value: string | number): string {
  const amount = Number(value) || 0
  return new Intl.NumberFormat(locale.value, { style: 'currency', currency: 'CNY', maximumFractionDigits: 2 }).format(amount)
}

function formatDate(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString(locale.value, { timeZone: 'Asia/Shanghai' })
}

function localizedLabel(prefix: string, value: string): string {
  const key = `${prefix}.${value}`
  const translated = t(key)
  return translated === key ? value : translated
}

function statusClass(status: string): string {
  if (['running', 'approved', 'available', 'qualified'].includes(status)) return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-200'
  if (['review', 'scheduled', 'pending', 'claimed_frozen', 'debt_review', 'settling'].includes(status)) return 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-200'
  if (['cancelled', 'rejected', 'revoked', 'expired'].includes(status)) return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-200'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200'
}

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    await Promise.all([loadOverview(), loadCampaigns(1)])
  } catch (cause) {
    error.value = extractApiErrorMessage(cause, t('admin.playOps.inviteGrowth.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function loadOverview(): Promise<void> {
  overview.value = await adminPlayAPI.getInviteGrowthOverview()
}

async function loadCampaigns(page = campaigns.value.page): Promise<void> {
  campaignsLoading.value = true
  try {
    campaigns.value = await adminPlayAPI.listReferralCampaigns({
      page,
      page_size: campaigns.value.page_size,
      search: campaignSearch.value.trim() || undefined,
      status: campaignStatus.value || undefined,
    })
    if (campaigns.value.items.length) {
      const nextID = campaigns.value.items.some(item => item.id === selectedID.value)
        ? selectedID.value!
        : campaigns.value.items[0].id
      await selectCampaign(nextID)
    } else {
      selectedID.value = null
      detail.value = null
    }
  } finally {
    campaignsLoading.value = false
  }
}

async function selectCampaign(id: number): Promise<void> {
  selectedID.value = id
  detailLoading.value = true
  error.value = ''
  try {
    detail.value = await adminPlayAPI.getReferralCampaign(id)
    await loadLedger(1)
  } catch (cause) {
    error.value = extractApiErrorMessage(cause, t('admin.playOps.inviteGrowth.detailLoadFailed'))
  } finally {
    detailLoading.value = false
  }
}

async function setDetailTab(tab: DetailTab): Promise<void> {
  detailTab.value = tab
  ledgerSearch.value = ''
  ledgerStatus.value = ''
  await loadLedger(1)
}

async function loadLedger(page = 1): Promise<void> {
  if (!selectedID.value) return
  ledgerLoading.value = true
  try {
    const params = { page, page_size: 20, search: ledgerSearch.value.trim() || undefined }
    if (detailTab.value === 'participants') {
      participants.value = await adminPlayAPI.listReferralCampaignParticipants(selectedID.value, params)
    } else if (detailTab.value === 'invites') {
      invites.value = await adminPlayAPI.listReferralCampaignInvites(selectedID.value, { ...params, status: ledgerStatus.value || undefined })
    } else {
      rewards.value = await adminPlayAPI.listReferralCampaignRewards(selectedID.value, { ...params, status: ledgerStatus.value || undefined })
    }
  } catch (cause) {
    appStore.showError(extractApiErrorMessage(cause, t('admin.playOps.inviteGrowth.ledgerLoadFailed')))
  } finally {
    ledgerLoading.value = false
  }
}

function openNewCampaign(): void {
  campaignDraft.value = blankCampaignDraft()
  showCampaignDialog.value = true
}

function openEditCampaign(): void {
  if (!detail.value || detail.value.campaign.status !== 'draft') return
  const campaign = detail.value.campaign
  campaignDraft.value = {
    id: campaign.id,
    version: campaign.version,
    key: campaign.key,
    name: campaign.name,
    registration_from: toLocalInput(campaign.registration_from),
    registration_to: toLocalInput(campaign.registration_to),
    starts_at: toLocalInput(campaign.starts_at),
    ends_at: toLocalInput(campaign.ends_at),
    qualification_to: toLocalInput(campaign.qualification_to),
    claim_deadline: toLocalInput(campaign.claim_deadline),
    risk_hold_hours: campaign.risk_hold_hours,
    pay_threshold: campaign.pay_threshold,
    usage_threshold: campaign.usage_threshold,
    max_enrollments: campaign.max_enrollments,
    budget_total: campaign.budget_total,
    reward_mode: campaign.reward_mode,
    tiers: detail.value.tiers.map(item => ({ ...item })),
  }
  showCampaignDialog.value = true
}

function addTier(): void {
  const last = campaignDraft.value.tiers.at(-1)
  campaignDraft.value.tiers.push({
    tier: (last?.tier || 0) + 1,
    required_invites: (last?.required_invites || 0) + 1,
    reward_amount: campaignDraft.value.reward_mode === 'replace' ? (last?.reward_amount || 0) + 100 : 0,
    currency: 'CNY',
  })
}

function removeTier(index: number): void {
  if (campaignDraft.value.tiers.length <= 1) return
  campaignDraft.value.tiers.splice(index, 1)
}

function buildCampaignInput(): AdminReferralCampaignInput {
  const draft = campaignDraft.value
  if (!draft.key.trim() || !draft.name.trim()) throw new Error(t('admin.playOps.inviteGrowth.errors.required'))
  const registrationFrom = new Date(draft.registration_from)
  const registrationTo = new Date(draft.registration_to)
  const startsAt = new Date(draft.starts_at)
  const endsAt = new Date(draft.ends_at)
  const qualificationTo = new Date(draft.qualification_to)
  const claimDeadline = new Date(draft.claim_deadline)
  const dates = [registrationFrom, registrationTo, startsAt, endsAt, qualificationTo, claimDeadline]
  if (dates.some(value => Number.isNaN(value.getTime()))) {
    throw new Error(t('admin.playOps.inviteGrowth.errors.invalidTime'))
  }
  if (registrationTo <= registrationFrom || endsAt <= startsAt || qualificationTo < endsAt || claimDeadline <= endsAt) {
    throw new Error(t('admin.playOps.inviteGrowth.errors.invalidSchedule'))
  }

  const riskHoldHours = Number(draft.risk_hold_hours)
  const payThreshold = Number(draft.pay_threshold)
  const usageThreshold = Number(draft.usage_threshold)
  const maxEnrollments = Number(draft.max_enrollments)
  const budgetTotal = Number(draft.budget_total)
  if (
    !Number.isFinite(riskHoldHours) || riskHoldHours < 168 || riskHoldHours > 720
    || !Number.isFinite(payThreshold) || payThreshold < 0
    || !Number.isFinite(usageThreshold) || usageThreshold < 0
    || !Number.isInteger(maxEnrollments) || maxEnrollments <= 0
    || !Number.isFinite(budgetTotal) || budgetTotal <= 0
  ) {
    throw new Error(t('admin.playOps.inviteGrowth.errors.invalidSettings'))
  }

  const normalizedTiers = draft.tiers.map(item => ({
    tier: Number(item.tier),
    required_invites: Number(item.required_invites),
    reward_amount: Number(item.reward_amount),
    currency: 'CNY' as const,
  }))
  if (!normalizedTiers.length || normalizedTiers.some(item => !Number.isInteger(item.tier) || item.tier <= 0 || !Number.isInteger(item.required_invites) || item.required_invites <= 0 || !Number.isFinite(item.reward_amount) || item.reward_amount <= 0)) {
    throw new Error(t('admin.playOps.inviteGrowth.errors.invalidTier'))
  }
  if (new Set(normalizedTiers.map(item => item.tier)).size !== normalizedTiers.length || new Set(normalizedTiers.map(item => item.required_invites)).size !== normalizedTiers.length) {
    throw new Error(t('admin.playOps.inviteGrowth.errors.duplicateTier'))
  }
  if (draft.reward_mode === 'replace') {
    const ordered = [...normalizedTiers].sort((a, b) => a.required_invites - b.required_invites)
    if (ordered.some((item, index) => index > 0 && item.reward_amount <= ordered[index - 1].reward_amount)) {
      throw new Error(t('admin.playOps.inviteGrowth.errors.invalidReplace'))
    }
  }
  if (budgetTotal + 1e-9 < maximumLiability.value) {
    throw new Error(t('admin.playOps.inviteGrowth.errors.budgetInsufficient', {
      budget: formatMoney(budgetTotal),
      liability: formatMoney(maximumLiability.value),
    }))
  }
  return {
    key: draft.key.trim(),
    name: draft.name.trim(),
    registration_from: registrationFrom.toISOString(),
    registration_to: registrationTo.toISOString(),
    starts_at: startsAt.toISOString(),
    ends_at: endsAt.toISOString(),
    qualification_to: qualificationTo.toISOString(),
    claim_deadline: claimDeadline.toISOString(),
    risk_hold_hours: riskHoldHours,
    pay_threshold: payThreshold,
    usage_threshold: usageThreshold,
    max_enrollments: maxEnrollments,
    budget_total: budgetTotal,
    reward_mode: draft.reward_mode,
    tiers: normalizedTiers,
  }
}

async function saveCampaign(): Promise<void> {
  let input: AdminReferralCampaignInput
  try {
    input = buildCampaignInput()
  } catch (cause) {
    appStore.showError((cause as Error).message)
    return
  }
  saving.value = true
  try {
    if (campaignDraft.value.id && campaignDraft.value.version) {
      await adminPlayAPI.updateReferralCampaign(campaignDraft.value.id, { ...input, expected_version: campaignDraft.value.version })
      appStore.showSuccess(t('admin.playOps.inviteGrowth.updated'))
    } else {
      await adminPlayAPI.createReferralCampaign(input)
      appStore.showSuccess(t('admin.playOps.inviteGrowth.created'))
    }
    showCampaignDialog.value = false
    await loadCampaigns(1)
  } catch (cause) {
    appStore.showError(extractApiErrorMessage(cause, t('admin.playOps.inviteGrowth.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function changeStatus(status: AdminReferralCampaignStatus): Promise<void> {
  if (!detail.value || actionLoading.value) return
  actionLoading.value = true
  try {
    await adminPlayAPI.setReferralCampaignStatus(detail.value.campaign.id, {
      expected_version: detail.value.campaign.version,
      status,
      note: statusNote.value.trim(),
    })
    statusNote.value = ''
    appStore.showSuccess(t('admin.playOps.inviteGrowth.statusUpdated'))
    await loadCampaigns(campaigns.value.page)
  } catch (cause) {
    appStore.showError(extractApiErrorMessage(cause, t('admin.playOps.inviteGrowth.actionFailed')))
  } finally {
    actionLoading.value = false
  }
}

async function submitReview(reviewType: AdminReferralReviewType, decision: AdminReferralReviewDecision): Promise<void> {
  if (!detail.value || actionLoading.value) return
  actionLoading.value = true
  try {
    await adminPlayAPI.reviewReferralCampaign(detail.value.campaign.id, {
      expected_version: detail.value.campaign.version,
      review_type: reviewType,
      decision,
      note: reviewNote.value.trim(),
    })
    reviewNote.value = ''
    appStore.showSuccess(t('admin.playOps.inviteGrowth.reviewSaved'))
    await loadCampaigns(campaigns.value.page)
  } catch (cause) {
    appStore.showError(extractApiErrorMessage(cause, t('admin.playOps.inviteGrowth.actionFailed')))
  } finally {
    actionLoading.value = false
  }
}

function approvalFor(type: AdminReferralReviewType) {
  if (!detail.value) return undefined
  const campaign = detail.value.campaign
  if (campaign.status === 'draft' || campaign.status === 'cancelled') return undefined
  const targetVersion = campaign.status === 'review'
    ? campaign.version
    : Math.max(...detail.value.approvals.map(item => item.version), 0)
  return detail.value.approvals.find(item => item.review_type === type && item.version === targetVersion)
}

function openDebtResolution(reward: AdminReferralCampaignReward): void {
  debtReward.value = reward
  debtNote.value = ''
}

async function resolveDebt(decision: 'recovered' | 'waived'): Promise<void> {
  if (!detail.value || !debtReward.value || debtNote.value.trim().length < 10 || actionLoading.value) return
  actionLoading.value = true
  try {
    await adminPlayAPI.resolveReferralRewardDebt(detail.value.campaign.id, debtReward.value.id, {
      expected_version: debtReward.value.version,
      decision,
      note: debtNote.value.trim(),
    })
    debtReward.value = null
    debtNote.value = ''
    appStore.showSuccess(t('admin.playOps.inviteGrowth.debtResolved'))
    await Promise.all([selectCampaign(detail.value.campaign.id), loadOverview()])
  } catch (cause) {
    appStore.showError(extractApiErrorMessage(cause, t('admin.playOps.inviteGrowth.actionFailed')))
  } finally {
    actionLoading.value = false
  }
}

onMounted(load)

defineExpose({ selectCampaign })
</script>

<template>
  <section class="space-y-5" aria-labelledby="invite-growth-title" :aria-busy="loading">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h2 id="invite-growth-title" class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.playOps.inviteGrowth.title') }}</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.playOps.inviteGrowth.hint') }}</p>
      </div>
      <div class="flex flex-col gap-2 sm:flex-row">
        <button data-testid="new-referral-campaign" type="button" class="btn btn-primary inline-flex items-center justify-center gap-2" @click="openNewCampaign">
          <Icon name="plus" size="sm" />{{ t('admin.playOps.inviteGrowth.newCampaign') }}
        </button>
        <button type="button" class="btn btn-secondary inline-flex items-center justify-center gap-2" :disabled="loading" @click="load">
          <Icon name="refresh" size="sm" />{{ t('admin.playOps.refresh') }}
        </button>
      </div>
    </div>

    <div v-if="error" class="card border-red-200 p-4 text-sm text-red-700 dark:border-red-900 dark:text-red-300" role="alert">
      <p>{{ error }}</p>
      <button type="button" class="btn btn-secondary mt-3" @click="load">{{ t('common.retry') }}</button>
    </div>

    <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
      <div v-for="stat in stats" :key="stat.label" class="card p-4">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ stat.label }}</p>
        <p class="mt-2 text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ loading ? '—' : stat.value }}</p>
      </div>
    </div>

    <section class="card overflow-hidden" aria-labelledby="invite-growth-ranking-title">
      <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
        <h3 id="invite-growth-ranking-title" class="font-semibold text-gray-900 dark:text-white">{{ t('admin.playOps.inviteGrowth.ranking') }}</h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.playOps.inviteGrowth.maskedHint') }}</p>
      </div>
      <div v-if="!overview?.ranking?.length" class="p-6 text-center text-sm text-gray-500">{{ t('admin.playOps.inviteGrowth.noRanking') }}</div>
      <div v-else class="overflow-x-auto">
        <table class="min-w-[620px] divide-y divide-gray-100 text-sm dark:divide-dark-700">
          <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
            <tr><th class="px-4 py-3">{{ t('admin.playOps.inviteGrowth.rank') }}</th><th class="px-4 py-3">{{ t('admin.playOps.inviteGrowth.inviter') }}</th><th class="px-4 py-3 text-right">{{ t('admin.playOps.inviteGrowth.qualified') }}</th><th class="px-4 py-3 text-right">{{ t('admin.playOps.inviteGrowth.reward') }}</th></tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="item in overview.ranking" :key="`${item.rank}-${item.email_masked}`">
              <td class="px-4 py-3 font-semibold tabular-nums">{{ item.rank }}</td>
              <td class="px-4 py-3"><span>{{ item.email_masked }}</span><span v-if="item.is_me" class="ml-2 rounded bg-primary-50 px-2 py-0.5 text-xs text-primary-700 dark:bg-primary-900/30 dark:text-primary-200">{{ t('admin.playOps.inviteGrowth.me') }}</span></td>
              <td class="px-4 py-3 text-right tabular-nums">{{ item.qualified_count }}</td>
              <td class="px-4 py-3 text-right font-medium tabular-nums">{{ formatMoney(item.reward_amount) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <div class="grid gap-5 xl:grid-cols-[minmax(0,1.4fr)_minmax(340px,0.6fr)]">
      <section class="card overflow-hidden" aria-labelledby="referral-campaign-list-title">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
          <h3 id="referral-campaign-list-title" class="font-semibold text-gray-900 dark:text-white">{{ t('admin.playOps.inviteGrowth.campaigns') }}</h3>
          <div class="flex flex-col gap-2 sm:flex-row">
            <input v-model="campaignSearch" type="search" class="input sm:w-64" :placeholder="t('admin.playOps.inviteGrowth.searchCampaigns')" @keyup.enter="loadCampaigns(1)" />
            <select v-model="campaignStatus" class="input sm:w-40" @change="loadCampaigns(1)">
              <option value="">{{ t('admin.playOps.inviteGrowth.allStatuses') }}</option>
              <option v-for="status in statuses" :key="status" :value="status">{{ localizedLabel('admin.playOps.inviteGrowth.statuses', status) }}</option>
            </select>
          </div>
        </div>
        <div v-if="campaignsLoading" class="p-8 text-center text-sm text-gray-500">{{ t('admin.playOps.loading') }}</div>
        <div v-else-if="!campaigns.items.length" class="p-8 text-center text-sm text-gray-500">{{ t('admin.playOps.inviteGrowth.noCampaigns') }}</div>
        <div v-else class="overflow-x-auto">
          <table class="min-w-[760px] divide-y divide-gray-100 text-sm dark:divide-dark-700">
            <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr><th class="px-4 py-3">{{ t('admin.playOps.inviteGrowth.campaign') }}</th><th class="px-4 py-3">{{ t('admin.playOps.inviteGrowth.window') }}</th><th class="px-4 py-3 text-right">{{ t('admin.playOps.inviteGrowth.budget') }}</th><th class="px-4 py-3">{{ t('admin.playOps.inviteGrowth.status') }}</th></tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="item in campaigns.items" :key="item.id" class="cursor-pointer hover:bg-gray-50 dark:hover:bg-dark-800" :class="{ 'bg-primary-50/70 dark:bg-primary-900/20': item.id === selectedID }" tabindex="0" @click="selectCampaign(item.id)" @keydown.enter="selectCampaign(item.id)">
                <td class="px-4 py-3"><div class="font-medium text-gray-900 dark:text-white">{{ item.name }}</div><div class="mt-1 text-xs text-gray-500">{{ item.key }} · v{{ item.version }}</div></td>
                <td class="px-4 py-3 text-xs text-gray-500"><div>{{ formatDate(item.starts_at) }}</div><div>{{ formatDate(item.ends_at) }}</div></td>
                <td class="px-4 py-3 text-right tabular-nums"><div>{{ formatMoney(item.budget_total) }}</div><div class="mt-1 text-xs text-gray-500">{{ t('admin.playOps.inviteGrowth.usedBudget', { amount: formatMoney(item.budget_reserved + item.budget_paid) }) }}</div></td>
                <td class="px-4 py-3"><span class="rounded px-2 py-1 text-xs font-medium" :class="statusClass(item.status)">{{ localizedLabel('admin.playOps.inviteGrowth.statuses', item.status) }}</span></td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="flex items-center justify-between border-t border-gray-100 px-4 py-3 text-sm dark:border-dark-700">
          <span class="text-gray-500">{{ t('admin.playOps.inviteGrowth.totalCampaigns', { count: campaigns.total }) }}</span>
          <div class="flex gap-2"><button type="button" class="btn btn-secondary" :disabled="campaigns.page <= 1" @click="loadCampaigns(campaigns.page - 1)">{{ t('admin.playOps.membership.previous') }}</button><button type="button" class="btn btn-secondary" :disabled="campaigns.page * campaigns.page_size >= campaigns.total" @click="loadCampaigns(campaigns.page + 1)">{{ t('admin.playOps.membership.next') }}</button></div>
        </div>
      </section>

      <aside class="card p-5" aria-live="polite">
        <div v-if="detailLoading" class="py-8 text-center text-sm text-gray-500">{{ t('admin.playOps.loading') }}</div>
        <div v-else-if="detail" class="space-y-5">
          <div class="flex items-start justify-between gap-3"><div><h3 class="font-semibold text-gray-900 dark:text-white">{{ detail.campaign.name }}</h3><p class="mt-1 text-xs text-gray-500">{{ detail.campaign.key }} · v{{ detail.campaign.version }}</p></div><button v-if="detail.campaign.status === 'draft'" type="button" class="btn btn-secondary" @click="openEditCampaign"><Icon name="edit" size="sm" />{{ t('admin.playOps.inviteGrowth.editDraft') }}</button></div>
          <dl class="space-y-2 text-sm"><div class="flex justify-between gap-3"><dt class="text-gray-500">{{ t('admin.playOps.inviteGrowth.registrationWindow') }}</dt><dd class="text-right">{{ formatDate(detail.campaign.registration_from) }}<br />{{ formatDate(detail.campaign.registration_to) }}</dd></div><div class="flex justify-between gap-3"><dt class="text-gray-500">{{ t('admin.playOps.inviteGrowth.thresholds') }}</dt><dd class="text-right">{{ formatMoney(detail.campaign.pay_threshold) }} / {{ formatMoney(detail.campaign.usage_threshold) }}</dd></div><div class="flex justify-between gap-3"><dt class="text-gray-500">{{ t('admin.playOps.inviteGrowth.capacity') }}</dt><dd class="tabular-nums">{{ detail.campaign.max_enrollments }}</dd></div><div class="flex justify-between gap-3"><dt class="text-gray-500">{{ t('admin.playOps.inviteGrowth.claimDeadline') }}</dt><dd class="text-right">{{ formatDate(detail.campaign.claim_deadline) }}</dd></div></dl>
          <div><h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.playOps.inviteGrowth.tiers') }}</h4><div class="mt-2 divide-y divide-gray-100 text-sm dark:divide-dark-700"><div v-for="tier in detail.tiers" :key="tier.tier" class="flex justify-between gap-3 py-2"><span>{{ t('admin.playOps.inviteGrowth.tierLine', { tier: tier.tier, count: tier.required_invites }) }}</span><strong>{{ formatMoney(tier.reward_amount) }}</strong></div></div></div>
        </div>
        <div v-else class="py-8 text-center text-sm text-gray-500">{{ t('admin.playOps.inviteGrowth.selectCampaign') }}</div>
      </aside>
    </div>

    <template v-if="detail">
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-8">
        <div v-for="item in detailStats" :key="item.key" class="border-b border-gray-200 px-1 py-3 dark:border-dark-700"><p class="text-xs text-gray-500">{{ t(`admin.playOps.inviteGrowth.detailStats.${item.key}`) }}</p><p class="mt-1 font-semibold tabular-nums text-gray-900 dark:text-white">{{ item.value }}</p></div>
      </div>

      <section class="card p-5" aria-labelledby="campaign-governance-title">
        <h3 id="campaign-governance-title" class="font-semibold text-gray-900 dark:text-white">{{ t('admin.playOps.inviteGrowth.governance') }}</h3>
        <div class="mt-4 grid gap-5 xl:grid-cols-2">
          <div><label class="grid gap-1 text-sm"><span class="text-gray-600 dark:text-gray-300">{{ t('admin.playOps.inviteGrowth.operationNote') }}</span><input v-model="statusNote" data-testid="status-note" class="input" maxlength="500" :placeholder="t('admin.playOps.inviteGrowth.operationNotePlaceholder')" /></label><div class="mt-3 flex flex-wrap gap-2"><button v-for="status in statusActions" :key="status" type="button" class="btn" :class="status === 'cancelled' ? 'btn-danger' : 'btn-secondary'" :data-testid="`status-${status}`" :disabled="actionLoading" @click="changeStatus(status)">{{ t('admin.playOps.inviteGrowth.moveTo', { status: localizedLabel('admin.playOps.inviteGrowth.statuses', status) }) }}</button><span v-if="!statusActions.length" class="text-sm text-gray-500">{{ t('admin.playOps.inviteGrowth.noStatusActions') }}</span></div></div>
          <div>
            <label class="grid gap-1 text-sm"><span class="text-gray-600 dark:text-gray-300">{{ t('admin.playOps.inviteGrowth.reviewNote') }}</span><input v-model="reviewNote" data-testid="review-note" class="input" maxlength="500" :placeholder="t('admin.playOps.inviteGrowth.reviewNotePlaceholder')" /></label>
            <div class="mt-3 grid gap-2 sm:grid-cols-2">
              <div v-for="type in reviewTypes" :key="type" class="flex items-start justify-between gap-2 border-b border-gray-100 py-2 dark:border-dark-700">
                <div class="min-w-0">
                  <span class="text-sm font-medium">{{ localizedLabel('admin.playOps.inviteGrowth.reviewTypes', type) }}</span>
                  <p class="text-xs text-gray-500">{{ approvalFor(type) ? localizedLabel('admin.playOps.inviteGrowth.reviewDecisions', approvalFor(type)!.decision) : t('admin.playOps.inviteGrowth.pendingReview') }}</p>
                  <template v-if="approvalFor(type)">
                    <p class="mt-1 text-xs text-gray-500">#{{ approvalFor(type)!.reviewer_id ?? '—' }} · {{ formatDate(approvalFor(type)!.created_at) }}</p>
                    <p v-if="approvalFor(type)!.note" class="mt-1 break-words text-xs text-gray-600 dark:text-gray-300">{{ approvalFor(type)!.note }}</p>
                  </template>
                </div>
                <div v-if="detail.campaign.status === 'review' && !approvalFor(type)" class="flex shrink-0 gap-1"><button type="button" class="btn btn-secondary" :data-testid="`review-approve-${type}`" :disabled="actionLoading" @click="submitReview(type, 'approved')">{{ t('admin.playOps.inviteGrowth.approve') }}</button><button type="button" class="btn btn-secondary text-red-600" :data-testid="`review-reject-${type}`" :disabled="actionLoading" @click="submitReview(type, 'rejected')">{{ t('admin.playOps.inviteGrowth.reject') }}</button></div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section class="card overflow-hidden" aria-labelledby="campaign-ledger-title">
        <div class="border-b border-gray-100 px-5 pt-4 dark:border-dark-700">
          <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between"><h3 id="campaign-ledger-title" class="font-semibold text-gray-900 dark:text-white">{{ t('admin.playOps.inviteGrowth.linkedLedger') }}</h3><div class="flex flex-col gap-2 sm:flex-row"><input v-model="ledgerSearch" type="search" class="input sm:w-64" :placeholder="t('admin.playOps.inviteGrowth.searchLedger')" @keyup.enter="loadLedger(1)" /><select v-if="detailTab !== 'participants'" v-model="ledgerStatus" class="input sm:w-44" @change="loadLedger(1)"><option value="">{{ t('admin.playOps.inviteGrowth.allStatuses') }}</option><option v-for="status in detailTab === 'invites' ? inviteStatuses : rewardStatuses" :key="status" :value="status">{{ localizedLabel(`admin.playOps.inviteGrowth.${detailTab === 'invites' ? 'inviteStatuses' : 'rewardStatuses'}`, status) }}</option></select><button type="button" class="btn btn-secondary" :disabled="ledgerLoading" @click="loadLedger(1)"><Icon name="refresh" size="sm" />{{ t('admin.playOps.refresh') }}</button></div></div>
          <div class="mt-3 flex gap-1 overflow-x-auto" role="tablist"><button v-for="tab in (['participants','invites','rewards'] as DetailTab[])" :key="tab" type="button" role="tab" class="shrink-0 border-b-2 px-3 py-3 text-sm font-medium" :class="detailTab === tab ? 'border-primary-600 text-primary-700 dark:text-primary-300' : 'border-transparent text-gray-500'" :aria-selected="detailTab === tab" :data-testid="`detail-tab-${tab}`" @click="setDetailTab(tab)">{{ t(`admin.playOps.inviteGrowth.tabs.${tab}`) }}</button></div>
        </div>
        <div v-if="ledgerLoading" class="p-8 text-center text-sm text-gray-500">{{ t('admin.playOps.loading') }}</div>
        <div v-else-if="!currentLedger.items.length" class="p-8 text-center text-sm text-gray-500">{{ t('admin.playOps.inviteGrowth.noLedgerRecords') }}</div>
        <div v-else class="overflow-x-auto">
          <table v-if="detailTab === 'participants'" class="min-w-[760px] divide-y divide-gray-100 text-sm dark:divide-dark-700"><thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">{{ t('admin.playOps.inviteGrowth.user') }}</th><th class="px-4 py-3">{{ t('admin.playOps.inviteGrowth.enrolledAt') }}</th><th class="px-4 py-3 text-right">{{ t('admin.playOps.inviteGrowth.invited') }}</th><th class="px-4 py-3 text-right">{{ t('admin.playOps.inviteGrowth.qualified') }}</th><th class="px-4 py-3 text-right">{{ t('admin.playOps.inviteGrowth.unlocked') }}</th><th class="px-4 py-3 text-right">{{ t('admin.playOps.inviteGrowth.claimed') }}</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="item in participants.items" :key="item.user_id"><td class="px-4 py-3"><div>{{ item.email }}</div><div class="text-xs text-gray-500">{{ item.username || `#${item.user_id}` }}</div></td><td class="px-4 py-3">{{ formatDate(item.enrolled_at) }}</td><td class="px-4 py-3 text-right">{{ item.invited_count }}</td><td class="px-4 py-3 text-right">{{ item.qualified_count }}</td><td class="px-4 py-3 text-right">{{ formatMoney(item.reward_unlocked) }}</td><td class="px-4 py-3 text-right">{{ formatMoney(item.reward_claimed) }}</td></tr></tbody></table>
          <table v-else-if="detailTab === 'invites'" class="min-w-[980px] divide-y divide-gray-100 text-sm dark:divide-dark-700"><thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">{{ t('admin.playOps.inviteGrowth.inviter') }}</th><th class="px-4 py-3">{{ t('admin.playOps.inviteGrowth.invitee') }}</th><th class="px-4 py-3">{{ t('admin.playOps.inviteGrowth.registeredAt') }}</th><th class="px-4 py-3 text-right">{{ t('admin.playOps.inviteGrowth.netPaid') }}</th><th class="px-4 py-3 text-right">{{ t('admin.playOps.inviteGrowth.actualCost') }}</th><th class="px-4 py-3">{{ t('admin.playOps.inviteGrowth.risk') }}</th><th class="px-4 py-3">{{ t('admin.playOps.inviteGrowth.qualification') }}</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="item in invites.items" :key="item.attribution_id"><td class="px-4 py-3">{{ item.inviter_email }}<div class="text-xs text-gray-500">#{{ item.inviter_id }}</div></td><td class="px-4 py-3">{{ item.invitee_email }}<div class="text-xs text-gray-500">#{{ item.invitee_id }}</div></td><td class="px-4 py-3">{{ formatDate(item.registered_at) }}</td><td class="px-4 py-3 text-right">{{ formatMoney(item.net_paid) }}</td><td class="px-4 py-3 text-right">{{ formatMoney(item.actual_cost) }}</td><td class="px-4 py-3"><span class="rounded px-2 py-1 text-xs" :class="statusClass(item.risk_status)">{{ localizedLabel('admin.playOps.inviteGrowth.inviteStatuses', item.risk_status) }}</span></td><td class="px-4 py-3"><span class="rounded px-2 py-1 text-xs" :class="statusClass(item.qualification_status)">{{ localizedLabel('admin.playOps.inviteGrowth.qualificationStatuses', item.qualification_status) }}</span></td></tr></tbody></table>
          <table v-else class="min-w-[900px] divide-y divide-gray-100 text-sm dark:divide-dark-700"><thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">{{ t('admin.playOps.inviteGrowth.user') }}</th><th class="px-4 py-3">{{ t('admin.playOps.inviteGrowth.tier') }}</th><th class="px-4 py-3 text-right">{{ t('admin.playOps.inviteGrowth.amount') }}</th><th class="px-4 py-3">{{ t('admin.playOps.inviteGrowth.rewardStatus') }}</th><th class="px-4 py-3">{{ t('admin.playOps.inviteGrowth.claimDeadline') }}</th><th class="px-4 py-3 text-right">{{ t('admin.playOps.inviteGrowth.action') }}</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="item in rewards.items" :key="item.id"><td class="px-4 py-3">{{ item.email }}<div class="text-xs text-gray-500">{{ item.username || `#${item.user_id}` }}</div></td><td class="px-4 py-3">V{{ item.tier }}</td><td class="px-4 py-3 text-right font-medium">{{ formatMoney(item.amount) }}</td><td class="px-4 py-3"><span class="rounded px-2 py-1 text-xs" :class="statusClass(item.status)">{{ localizedLabel('admin.playOps.inviteGrowth.rewardStatuses', item.status) }}</span></td><td class="px-4 py-3">{{ formatDate(item.claim_deadline) }}</td><td class="px-4 py-3 text-right"><button v-if="item.status === 'debt_review'" type="button" class="btn btn-secondary" :data-testid="`resolve-debt-${item.id}`" @click="openDebtResolution(item)">{{ t('admin.playOps.inviteGrowth.resolveDebt') }}</button><span v-else>—</span></td></tr></tbody></table>
        </div>
        <div class="flex items-center justify-between border-t border-gray-100 px-4 py-3 text-sm dark:border-dark-700"><span class="text-gray-500">{{ t('admin.playOps.inviteGrowth.totalRecords', { count: currentLedger.total }) }}</span><div class="flex gap-2"><button type="button" class="btn btn-secondary" :disabled="currentLedger.page <= 1 || ledgerLoading" @click="loadLedger(currentLedger.page - 1)">{{ t('admin.playOps.membership.previous') }}</button><button type="button" class="btn btn-secondary" :disabled="currentLedger.page * currentLedger.page_size >= currentLedger.total || ledgerLoading" @click="loadLedger(currentLedger.page + 1)">{{ t('admin.playOps.membership.next') }}</button></div></div>
      </section>
    </template>

    <BaseDialog :show="showCampaignDialog" :title="campaignDraft.id ? t('admin.playOps.inviteGrowth.editCampaign') : t('admin.playOps.inviteGrowth.newCampaign')" width="extra-wide" @close="showCampaignDialog = false">
      <form class="space-y-5" @submit.prevent="saveCampaign">
        <div class="grid gap-4 md:grid-cols-2"><label class="grid gap-1 text-sm"><span>{{ t('admin.playOps.inviteGrowth.campaignKey') }}</span><input v-model="campaignDraft.key" data-testid="referral-key" class="input" maxlength="64" :disabled="Boolean(campaignDraft.id)" required /></label><label class="grid gap-1 text-sm"><span>{{ t('admin.playOps.inviteGrowth.campaignName') }}</span><input v-model="campaignDraft.name" data-testid="referral-name" class="input" maxlength="128" required /></label></div>
        <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3"><label v-for="field in (['registration_from','registration_to','starts_at','ends_at','qualification_to','claim_deadline'] as const)" :key="field" class="grid gap-1 text-sm"><span>{{ t(`admin.playOps.inviteGrowth.fields.${field}`) }}</span><input v-model="campaignDraft[field]" type="datetime-local" class="input" required /></label></div>
        <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-5"><label class="grid gap-1 text-sm"><span>{{ t('admin.playOps.inviteGrowth.payThreshold') }}</span><input v-model.number="campaignDraft.pay_threshold" type="number" min="0" step="0.01" class="input" /></label><label class="grid gap-1 text-sm"><span>{{ t('admin.playOps.inviteGrowth.usageThreshold') }}</span><input v-model.number="campaignDraft.usage_threshold" type="number" min="0" step="0.01" class="input" /></label><label class="grid gap-1 text-sm"><span>{{ t('admin.playOps.inviteGrowth.riskHold') }}</span><input v-model.number="campaignDraft.risk_hold_hours" type="number" min="168" max="720" class="input" /></label><label class="grid gap-1 text-sm"><span>{{ t('admin.playOps.inviteGrowth.capacity') }}</span><input v-model.number="campaignDraft.max_enrollments" data-testid="referral-capacity" type="number" min="1" class="input" /></label><label class="grid gap-1 text-sm"><span>{{ t('admin.playOps.inviteGrowth.budget') }}</span><input v-model.number="campaignDraft.budget_total" data-testid="referral-budget" type="number" min="0.01" step="0.01" class="input" /></label></div>
        <fieldset class="space-y-2"><legend class="text-sm font-medium">{{ t('admin.playOps.inviteGrowth.rewardMode') }}</legend><div class="flex flex-wrap gap-3"><label v-for="mode in (['additive','replace'] as const)" :key="mode" class="inline-flex items-center gap-2"><input v-model="campaignDraft.reward_mode" type="radio" :value="mode" />{{ t(`admin.playOps.inviteGrowth.rewardModes.${mode}`) }}</label></div></fieldset>
        <section class="border-t border-gray-100 pt-4 dark:border-dark-700"><div class="flex items-center justify-between"><div><h3 class="font-semibold">{{ t('admin.playOps.inviteGrowth.tiers') }}</h3><p class="mt-1 text-xs text-gray-500">{{ t('admin.playOps.inviteGrowth.tiersHint') }}</p></div><button data-testid="referral-add-tier" type="button" class="btn btn-secondary" @click="addTier"><Icon name="plus" size="sm" />{{ t('admin.playOps.inviteGrowth.addTier') }}</button></div><div class="mt-3 space-y-3"><div v-for="(tier,index) in campaignDraft.tiers" :key="index" class="grid gap-3 border-b border-gray-100 pb-3 dark:border-dark-700 sm:grid-cols-[100px_minmax(0,1fr)_minmax(0,1fr)_44px]"><label class="grid gap-1 text-sm"><span>{{ t('admin.playOps.inviteGrowth.tier') }}</span><input v-model.number="tier.tier" type="number" min="1" class="input" /></label><label class="grid gap-1 text-sm"><span>{{ t('admin.playOps.inviteGrowth.requiredInvites') }}</span><input v-model.number="tier.required_invites" type="number" min="1" class="input" /></label><label class="grid gap-1 text-sm"><span>{{ t('admin.playOps.inviteGrowth.rewardAmount') }}</span><input v-model.number="tier.reward_amount" :data-testid="`referral-tier-reward-${index}`" type="number" min="0.01" step="0.01" class="input" /></label><button type="button" class="btn-icon self-end text-red-600" :title="t('admin.playOps.inviteGrowth.removeTier')" :disabled="campaignDraft.tiers.length <= 1" @click="removeTier(index)"><Icon name="trash" size="sm" /></button></div></div></section>
        <div data-testid="referral-liability" class="border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-100"><h3 class="font-semibold">{{ t('admin.playOps.inviteGrowth.maxLiability') }}</h3><div class="mt-2 grid gap-2 sm:grid-cols-2 xl:grid-cols-4"><span>{{ t('admin.playOps.inviteGrowth.perUserLiability') }} <strong>{{ formatMoney(perUserLiability) }}</strong></span><span>{{ t('admin.playOps.inviteGrowth.totalLiability') }} <strong>{{ formatMoney(maximumLiability) }}</strong></span><span>{{ t('admin.playOps.inviteGrowth.coveredEnrollments') }} <strong>{{ coveredEnrollments }}</strong></span><span>{{ t('admin.playOps.inviteGrowth.budgetGap') }} <strong>{{ formatMoney(budgetGap) }}</strong></span></div><p class="mt-2 text-xs">{{ t('admin.playOps.inviteGrowth.liabilityHint') }}</p></div>
      </form>
      <template #footer><button type="button" class="btn btn-secondary" :disabled="saving" @click="showCampaignDialog = false">{{ t('admin.playOps.cancel') }}</button><button data-testid="save-referral-campaign" type="button" class="btn btn-primary" :disabled="saving" @click="saveCampaign">{{ saving ? t('admin.playOps.saving') : t('admin.playOps.inviteGrowth.saveDraft') }}</button></template>
    </BaseDialog>

    <BaseDialog :show="Boolean(debtReward)" :title="t('admin.playOps.inviteGrowth.resolveDebt')" width="narrow" @close="debtReward = null">
      <div v-if="debtReward" class="space-y-4"><p class="text-sm text-gray-600 dark:text-gray-300">{{ t('admin.playOps.inviteGrowth.debtSummary', { email: debtReward.email, amount: formatMoney(debtReward.amount) }) }}</p><label class="grid gap-1 text-sm"><span>{{ t('admin.playOps.inviteGrowth.debtNote') }}</span><textarea v-model="debtNote" data-testid="debt-note" rows="4" maxlength="500" class="input" :placeholder="t('admin.playOps.inviteGrowth.debtNotePlaceholder')" /></label><p class="text-xs text-gray-500">{{ t('admin.playOps.inviteGrowth.debtWarning') }}</p></div>
      <template #footer><button type="button" class="btn btn-secondary" @click="debtReward = null">{{ t('admin.playOps.cancel') }}</button><button data-testid="debt-recovered" type="button" class="btn btn-secondary" :disabled="debtNote.trim().length < 10 || actionLoading" @click="resolveDebt('recovered')">{{ t('admin.playOps.inviteGrowth.recovered') }}</button><button data-testid="debt-waive" type="button" class="btn btn-primary" :disabled="debtNote.trim().length < 10 || actionLoading" @click="resolveDebt('waived')">{{ t('admin.playOps.inviteGrowth.waived') }}</button></template>
    </BaseDialog>
  </section>
</template>
