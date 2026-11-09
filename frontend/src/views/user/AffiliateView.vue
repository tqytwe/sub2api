<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import AnnouncementContent from '@/components/common/AnnouncementContent.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import { useLeaderboardLiveRefresh } from '@/composables/useLeaderboardLiveRefresh'
import { getAffiliateOperationsSummary, listAffiliateInvitees, listAffiliateEarnings, getAffiliateLeaderboard, listAffiliateCampaignHistory, type AffiliateOperationsSummary, type AffiliateOperationsPage, type AffiliateOperationsInvitee, type AffiliateOperationsEarning } from '@/api/affiliateOperations'
import userAPI from '@/api/user'
import { getActiveCampaigns, getTeamMe, type PlayCampaignSummary } from '@/api/play'
import type { UserAffiliateDetail } from '@/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { extractApiErrorMessage, extractI18nErrorMessage } from '@/utils/apiError'
import { buildRegisterInviteLink } from '@/utils/oauthAffiliate'
import { claimReferralCampaignReward, enrollReferralCampaign, getReferralCampaignInviteToken, getReferralCampaignProgress, listReferralCampaigns, markReferralCampaignViewed, type ReferralCampaignProgress } from '@/api/referralCampaign'
import '@/styles/growth-world.css'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const transferring = ref(false)
const transferredQuota = ref<number | null>(null)
const detail = ref<UserAffiliateDetail | null>(null)
const teamInviteCode = ref('')
const campaignLoading = ref(false)
const campaignLoadFailed = ref(false)
const campaignAction = ref<number | null>(null)
const campaigns = ref<ReferralCampaignProgress[]>([])
const growthCampaigns = ref<PlayCampaignSummary[]>([])
const growthAction = ref<number | null>(null)
type AffiliateTab = 'overview' | 'invitees' | 'earnings' | 'leaderboard' | 'campaigns'
const activeTab = ref<AffiliateTab>('overview')
const tabs: AffiliateTab[] = ['overview', 'invitees', 'earnings', 'leaderboard', 'campaigns']
const detailLoadFailed = ref(false)
const summary = ref<AffiliateOperationsSummary | null>(null)
const summaryLoading = ref(false)
const summaryFailed = ref(false)
const invitees = ref<AffiliateOperationsPage<AffiliateOperationsInvitee> | null>(null)
const earnings = ref<AffiliateOperationsPage<AffiliateOperationsEarning> | null>(null)
const inviteesLoading = ref(false)
const earningsLoading = ref(false)
const inviteesFailed = ref(false)
const earningsFailed = ref(false)
const inviteePage = ref(1)
const earningPage = ref(1)
const inviteeStatus = ref('')
const inviteeSearch = ref('')
const earningAction = ref('')
const inviteeStart = ref('')
const inviteeEnd = ref('')
const earningStart = ref('')
const earningEnd = ref('')
const dateError = ref(false)
const campaignScope = ref<'available' | 'joined' | 'ended'>('available')
const campaignHistory = ref<AffiliateOperationsPage<ReferralCampaignProgress> | null>(null)
const historyPage = ref(1)
const historyLoading = ref(false)
const historyFailed = ref(false)
const growthLoadFailed = ref(false)
const inviteeStatuses = ['pending_evidence', 'unpaid', 'effective', 'refunded', 'risk_excluded', 'disabled']
const earningActions = ['accrue', 'transfer', 'withdraw', 'revoke']
const shanghaiMonth = () => new Intl.DateTimeFormat('sv-SE', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit' }).format(new Date())
const currentMonth = ref(shanghaiMonth())
const selectedMonth = ref(currentMonth.value)
const leaderboardMode = ref<'monthly' | 'campaign'>('monthly')
const selectedCampaignId = ref<number | null>(null)
const visibleCampaigns = computed(() => campaignScope.value === 'available' ? campaigns.value : campaignHistory.value?.items ?? [])
const leaderboardCampaigns = computed(() => Array.from(new Map([...campaigns.value, ...(campaignHistory.value?.items ?? [])].map(item => [item.campaign.id, item])).values()))
const monthlySelectionValid = computed(() => /^\d{4}-(0[1-9]|1[0-2])$/.test(selectedMonth.value) && selectedMonth.value <= currentMonth.value)
const board = useLeaderboardLiveRefresh({
  fetch: signal => {
    const month = shanghaiMonth()
    const followsCurrentMonth = selectedMonth.value === currentMonth.value
    currentMonth.value = month
    if (followsCurrentMonth) selectedMonth.value = month
    return getAffiliateLeaderboard(leaderboardMode.value === 'monthly' ? { month: selectedMonth.value } : { campaign_id: selectedCampaignId.value! }, signal)
  },
  enabled: () => activeTab.value === 'leaderboard' && (leaderboardMode.value === 'monthly' ? monthlySelectionValid.value : selectedCampaignId.value !== null),
  key: () => `${leaderboardMode.value}:${selectedMonth.value === currentMonth.value ? 'current' : selectedMonth.value}:${selectedCampaignId.value}`,
})
const { data: leaderboard, error: leaderboardError, isRefreshing: leaderboardRefreshing } = board
const leaderboardMetric = computed(() => leaderboardMode.value === 'monthly' ? t('affiliateOperations.effectivePaid') : t('affiliate.campaign.qualified'))
let inviteeRequest = 0
let earningRequest = 0
let historyRequest = 0

const inviteeColumns = computed(() => [
  { key: 'display_name', label: t('affiliateOperations.invitee') },
  { key: 'status', label: t('affiliateOperations.status') },
  { key: 'invited_at', label: t('affiliateOperations.invitedAt') },
  { key: 'first_paid_month', label: t('affiliateOperations.firstPaidMonth') },
  { key: 'total_rebate', label: t('affiliateOperations.totalRebate') },
])
const earningColumns = computed(() => [
  { key: 'created_at', label: t('affiliateOperations.time') },
  { key: 'action', label: t('affiliateOperations.action') },
  { key: 'amount', label: t('affiliateOperations.amount') },
  { key: 'status', label: t('affiliateOperations.status') },
  { key: 'source', label: t('affiliateOperations.source') },
])

async function loadSummary() {
  if (summaryLoading.value) return
  summaryLoading.value = true
  summaryFailed.value = false
  try { summary.value = await getAffiliateOperationsSummary() } catch { summaryFailed.value = true } finally { summaryLoading.value = false }
}

async function loadInvitees(page = inviteePage.value) {
  const request = ++inviteeRequest
  inviteePage.value = page
  inviteesLoading.value = true
  inviteesFailed.value = false
  try {
    const result = await listAffiliateInvitees({ page, page_size: 20, status: inviteeStatus.value || undefined, search: inviteeSearch.value.trim() || undefined, start_date: inviteeStart.value || undefined, end_date: inviteeEnd.value || undefined })
    if (request === inviteeRequest) invitees.value = result
  } catch { if (request === inviteeRequest) inviteesFailed.value = true } finally { if (request === inviteeRequest) inviteesLoading.value = false }
}

async function loadEarnings(page = earningPage.value) {
  const request = ++earningRequest
  earningPage.value = page
  earningsLoading.value = true
  earningsFailed.value = false
  try {
    const result = await listAffiliateEarnings({ page, page_size: 20, action: earningAction.value || undefined, start_date: earningStart.value || undefined, end_date: earningEnd.value || undefined })
    if (request === earningRequest) earnings.value = result
  } catch { if (request === earningRequest) earningsFailed.value = true } finally { if (request === earningRequest) earningsLoading.value = false }
}

function applyFilters(kind: 'invitees' | 'earnings') {
  const start = kind === 'invitees' ? inviteeStart.value : earningStart.value
  const end = kind === 'invitees' ? inviteeEnd.value : earningEnd.value
  dateError.value = Boolean(start && end && start > end)
  if (!dateError.value) void (kind === 'invitees' ? loadInvitees(1) : loadEarnings(1))
}

async function loadHistory(page = historyPage.value) {
  if (campaignScope.value === 'available') return
  const request = ++historyRequest
  historyPage.value = page
  historyLoading.value = true
  historyFailed.value = false
  try {
    const result = await listAffiliateCampaignHistory({ page, page_size: 20, scope: campaignScope.value })
    if (request === historyRequest) campaignHistory.value = result
  } catch { if (request === historyRequest) historyFailed.value = true } finally { if (request === historyRequest) historyLoading.value = false }
}

function openCampaignLeaderboard(campaignId: number) {
  selectedCampaignId.value = campaignId
  leaderboardMode.value = 'campaign'
  activeTab.value = 'leaderboard'
}

function updateCampaignProgress(progress: ReferralCampaignProgress) {
  campaigns.value = campaigns.value.map(item => item.campaign.id === progress.campaign.id ? progress : item)
  if (campaignHistory.value) campaignHistory.value.items = campaignHistory.value.items.map(item => item.campaign.id === progress.campaign.id ? progress : item)
}

function canShareCampaign(campaign: ReferralCampaignProgress) {
  const now = Date.now()
  return ['scheduled', 'running'].includes(campaign.campaign.status) && (!campaign.campaign.registration_from || Date.parse(campaign.campaign.registration_from) <= now) && (!campaign.campaign.registration_to || Date.parse(campaign.campaign.registration_to) > now)
}

watch(activeTab, tab => {
  dateError.value = false
  if (tab === 'invitees' && !invitees.value) void loadInvitees()
  if (tab === 'earnings' && !earnings.value) void loadEarnings()
})
watch(campaignScope, () => { campaignHistory.value = null; void loadHistory(1) })
onBeforeUnmount(() => { inviteeRequest++; earningRequest++; historyRequest++ })

const inviteLink = computed(() => {
  if (!detail.value) return ''
  return buildRegisterInviteLink(detail.value.aff_code, teamInviteCode.value)
})

const formattedRebateRate = computed(() => {
  const v = detail.value?.effective_rebate_rate_percent ?? 0
  const rounded = Math.round(v * 100) / 100
  return Number.isInteger(rounded) ? String(rounded) : rounded.toString()
})

function formatCount(value: number): string {
  return value.toLocaleString()
}

function summaryMoney(value: number | null | undefined, currency = 'USD'): string {
  if (value === null || currency === 'mixed') return t('affiliateOperations.byCurrency')
  return value === undefined ? '—' : formatCurrency(value, currency)
}

function moveTab(event: KeyboardEvent, tab: AffiliateTab) {
  const index = tabs.indexOf(tab)
  const next = event.key === 'ArrowRight' ? (index + 1) % tabs.length : event.key === 'ArrowLeft' ? (index + tabs.length - 1) % tabs.length : event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : null
  if (next === null) return
  event.preventDefault()
  activeTab.value = tabs[next]
  document.getElementById(`affiliate-tab-${tabs[next]}`)?.focus()
}

async function loadAffiliateDetail(silent = false): Promise<void> {
  if (!silent) {
    loading.value = true
  }
  try {
    detailLoadFailed.value = false
    detail.value = await userAPI.getAffiliateDetail()
  } catch (error) {
    detailLoadFailed.value = true
    appStore.showError(extractApiErrorMessage(error, t('affiliate.loadFailed')))
  } finally {
    if (!silent) {
      loading.value = false
    }
  }
}

async function loadTeamInviteCode(): Promise<void> {
  try {
    const me = await getTeamMe()
    teamInviteCode.value = me.team?.invite_code || ''
  } catch {
    teamInviteCode.value = ''
  }
}

async function copyCode(): Promise<void> {
  if (!detail.value?.aff_code) return
  await copyToClipboard(detail.value.aff_code, t('affiliate.codeCopied'))
}

async function copyInviteLink(): Promise<void> {
  if (!inviteLink.value) return
  await copyToClipboard(inviteLink.value, t('affiliate.linkCopied'))
}

async function transferQuota(): Promise<void> {
  if (!detail.value || detail.value.aff_quota <= 0 || transferring.value) return
  transferring.value = true
  transferredQuota.value = null
  try {
    const resp = await userAPI.transferAffiliateQuota()
    transferredQuota.value = resp.transferred_quota
    appStore.showSuccess(t('affiliate.transfer.success', { amount: formatCurrency(resp.transferred_quota) }))
    await Promise.all([
      loadAffiliateDetail(true),
      authStore.refreshUser().catch(() => undefined),
      loadSummary(),
    ])
    earnings.value = null
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.transferFailed')))
  } finally {
    transferring.value = false
  }
}

async function loadCampaigns(): Promise<void> {
  if (campaignLoading.value || campaignAction.value !== null) return
  campaignLoading.value = true
  campaignLoadFailed.value = false
  try {
    campaigns.value = await listReferralCampaigns()
    await Promise.all(campaigns.value.filter(item => item.unseen_update).map(item => markReferralCampaignViewed(item.campaign.id).catch(() => undefined)))
    try {
      growthLoadFailed.value = false
      const growth = await getActiveCampaigns()
      growthCampaigns.value = growth.filter(item => item.new_user_growth)
    } catch {
      growthLoadFailed.value = true
    }
  } catch (error) { campaignLoadFailed.value = true; appStore.showError(extractI18nErrorMessage(error, t, 'affiliate.campaign.errors', t('affiliate.campaign.loadFailed'))) } finally { campaignLoading.value = false }
}

async function claimGrowthReward(campaign: PlayCampaignSummary, rewardId: number): Promise<void> {
  if (growthAction.value !== null) return
  const progress = campaign.new_user_growth
  if (!progress || !progress.referral_campaign_id || !progress.referral_version) return
  growthAction.value = rewardId
  try {
    await claimReferralCampaignReward(progress.referral_campaign_id, rewardId, progress.referral_version)
    const refreshed = await getActiveCampaigns()
    growthCampaigns.value = refreshed.filter(item => item.new_user_growth)
    await Promise.all([loadAffiliateDetail(true), loadSummary()])
    earnings.value = null
    appStore.showSuccess(t('affiliate.growth.claimed'))
  } catch (error) {
    appStore.showError(extractI18nErrorMessage(error, t, 'affiliate.campaign.errors', t('affiliate.growth.claimFailed')))
  } finally { growthAction.value = null }
}

function growthProgressPercent(campaign: PlayCampaignSummary): number {
  const progress = campaign.new_user_growth
  if (!progress || !campaign.rules.reward_tiers?.length) return 0
  const max = Math.max(...campaign.rules.reward_tiers.map(item => item.required_amount), 1)
  return Math.min(100, Math.round(progress.qualified_amount * 100 / max))
}

async function enrollCampaign(campaign: ReferralCampaignProgress): Promise<void> {
  if (campaignAction.value !== null || campaignLoading.value) return
  campaignAction.value = campaign.campaign.id
  try { await enrollReferralCampaign(campaign.campaign.id); const progress = await getReferralCampaignProgress(campaign.campaign.id); updateCampaignProgress(progress); appStore.showSuccess(t('affiliate.campaign.enrolled')) } catch (error) { appStore.showError(extractI18nErrorMessage(error, t, 'affiliate.campaign.errors', t('affiliate.campaign.enrollFailed'))) } finally { campaignAction.value = null }
}

async function copyCampaignLink(campaign: ReferralCampaignProgress): Promise<void> {
  if (campaignAction.value !== null || campaignLoading.value) return
  campaignAction.value = campaign.campaign.id
  try {
    const token = await getReferralCampaignInviteToken(campaign.campaign.id)
    const url = new URL('/register', window.location.origin)
    if (campaign.campaign.legacy_rebate_policy === 'stack' && detail.value?.aff_code) url.searchParams.set('ref', detail.value.aff_code)
    if (teamInviteCode.value) url.searchParams.set('team', teamInviteCode.value)
    url.searchParams.set('campaign', String(campaign.campaign.id)); url.searchParams.set('token', token)
    await copyToClipboard(url.toString(), t('affiliate.campaign.linkCopied'))
  } catch (error) { appStore.showError(extractI18nErrorMessage(error, t, 'affiliate.campaign.errors', t('affiliate.campaign.linkFailed'))) } finally { campaignAction.value = null }
}

async function claimCampaignReward(campaign: ReferralCampaignProgress, rewardId: number): Promise<void> {
  if (campaignAction.value !== null || campaignLoading.value) return
  campaignAction.value = rewardId
  try { await claimReferralCampaignReward(campaign.campaign.id, rewardId, campaign.campaign.version); const progress = await getReferralCampaignProgress(campaign.campaign.id); updateCampaignProgress(progress); await Promise.all([loadAffiliateDetail(true), loadSummary()]); earnings.value = null; appStore.showSuccess(t('affiliate.campaign.claimed')) } catch (error) { appStore.showError(extractI18nErrorMessage(error, t, 'affiliate.campaign.errors', t('affiliate.campaign.claimFailed'))) } finally { campaignAction.value = null }
}

function campaignReward(campaign: ReferralCampaignProgress, tier: number) { return campaign.rewards.find(item => item.tier === tier) }
function campaignProgressPercent(campaign: ReferralCampaignProgress) { const max = Math.max(...campaign.tiers.map(item => item.required_invites), 1); return Math.min(100, Math.round(campaign.qualified_count * 100 / max)) }

onMounted(() => {
  void loadAffiliateDetail()
  void loadTeamInviteCode()
  void loadCampaigns()
  void loadSummary()
})
</script>

<template>
  <AppLayout>
    <div class="gw-page gw-page--wide gw-workspace pb-10">
      <section class="gw-hero-panel">
        <p class="gw-eyebrow">{{ t('affiliate.eyebrow') }}</p>
        <h1 class="gw-title">{{ t('affiliate.title') }}</h1>
        <p class="gw-subtitle">{{ t('affiliate.description') }}</p>
      </section>

      <div class="mt-4 flex flex-wrap gap-2 border-b pb-3" style="border-color: var(--gw-line)" role="tablist" :aria-label="t('affiliate.title')">
        <button v-for="tab in tabs" :id="`affiliate-tab-${tab}`" :key="tab" type="button" role="tab" :aria-selected="activeTab === tab" :tabindex="activeTab === tab ? 0 : -1" @keydown="moveTab($event, tab)" :aria-controls="`affiliate-panel-${tab}`" :data-test="`affiliate-tab-${tab}`" :class="['gw-btn', activeTab === tab ? 'gw-btn-primary' : 'gw-btn-secondary']" @click="activeTab = tab">{{ t(`affiliateOperations.tabs.${tab}`) }}</button>
      </div>
      <div v-if="loading" class="gw-polling py-12 text-center" role="status">{{ t('models.loading') }}</div>
      <div v-else-if="detailLoadFailed && !detail" class="gw-panel" role="alert">
        <p>{{ t('affiliate.loadFailed') }}</p><button type="button" class="gw-btn gw-btn-secondary mt-3" @click="loadAffiliateDetail()">{{ t('affiliateOperations.retry') }}</button>
      </div>
      <template v-else-if="detail">
        <section v-if="activeTab === 'overview'" id="affiliate-panel-overview" role="tabpanel" aria-labelledby="affiliate-tab-overview" :aria-busy="summaryLoading">
          <div v-if="summaryFailed" class="mt-4 flex flex-wrap items-center gap-3" role="alert"><span>{{ t('affiliateOperations.loadFailed') }}</span><button type="button" class="gw-btn gw-btn-secondary" @click="loadSummary">{{ t('affiliateOperations.retry') }}</button></div>
          <div class="gw-stat-grid mt-4">
            <div class="gw-stat-card"><p class="gw-balance-label">{{ t('affiliate.stats.availableQuota') }}</p><p class="gw-stat-value ok">{{ summary ? formatCurrency(summary.available_quota) : '—' }}</p></div>
            <div class="gw-stat-card"><p class="gw-balance-label">{{ t('affiliate.stats.frozenQuota') }}</p><p class="gw-stat-value">{{ summary ? formatCurrency(summary.frozen_quota) : '—' }}</p></div>
            <div class="gw-stat-card"><p class="gw-balance-label">{{ t('affiliateOperations.monthNetEarned') }}</p><p class="gw-stat-value">{{ summaryMoney(summary?.month_net_earned, summary?.period_amounts?.[0]?.currency || summary?.currency) }}</p></div>
            <div class="gw-stat-card"><p class="gw-balance-label">{{ t('affiliateOperations.effectivePaidTotal') }}</p><p class="gw-stat-value">{{ summary ? formatCount(summary.effective_paid_count) : '—' }}</p></div>
          </div>
        <div class="gw-detail-grid">
          <div class="gw-panel">
            <div class="grid gap-4 md:grid-cols-2">
              <div class="gw-field">
                <span class="gw-field-label">{{ t('affiliate.yourCode') }}</span>
                <div class="gw-code-row flex-col items-stretch sm:flex-row sm:items-center">
                  <code class="min-w-0 break-all sm:flex-1 sm:truncate">{{ detail.aff_code }}</code>
                  <button type="button" class="gw-btn gw-btn-secondary w-full sm:w-auto sm:shrink-0" @click="copyCode">
                    <Icon name="copy" size="sm" />
                    <span>{{ t('affiliate.copyCode') }}</span>
                  </button>
                </div>
              </div>
              <div class="gw-field">
                <span class="gw-field-label">{{ t('affiliate.inviteLink') }}</span>
                <div class="gw-code-row flex-col items-stretch sm:flex-row sm:items-center">
                  <code class="min-w-0 break-all sm:flex-1 sm:truncate">{{ inviteLink }}</code>
                  <button type="button" class="gw-btn gw-btn-secondary w-full sm:w-auto sm:shrink-0" @click="copyInviteLink">
                    <Icon name="copy" size="sm" />
                    <span>{{ t('affiliate.copyLink') }}</span>
                  </button>
                </div>
              </div>
            </div>

            <div class="gw-tips">
              <p class="gw-section-title text-base">{{ t('affiliate.tips.title') }}</p>
              <ul class="mt-2 space-y-1 gw-subtitle text-sm">
                <li>1. {{ t('affiliate.tips.line1') }}</li>
                <li>2. {{ t('affiliate.tips.line2', { rate: `${formattedRebateRate}%` }) }}</li>
                <li>3. {{ t('affiliate.tips.line3') }}</li>
                <li v-if="detail.aff_frozen_quota > 0">4. {{ t('affiliate.tips.line4') }}</li>
              </ul>
            </div>
          </div>

          <div class="gw-panel">
            <div class="flex flex-col gap-3 sm:items-start">
              <div>
                <h3 class="gw-section-title">{{ t('affiliate.transfer.title') }}</h3>
                <p class="gw-subtitle">{{ t('affiliate.transfer.description') }}</p>
              </div>
              <button
                type="button"
                class="gw-btn gw-btn-primary"
                :disabled="transferring || detail.aff_quota <= 0"
                @click="transferQuota"
              >
                <Icon v-if="transferring" name="refresh" size="sm" class="animate-spin" />
                <Icon v-else name="dollar" size="sm" />
                <span>{{ transferring ? t('affiliate.transfer.transferring') : t('affiliate.transfer.button') }}</span>
              </button>
            </div>
            <p v-if="transferredQuota !== null" class="mt-3 text-sm" role="status">{{ t('affiliate.transfer.success', { amount: formatCurrency(transferredQuota) }) }}</p>
            <p v-if="detail.aff_quota <= 0" class="mt-3 text-sm" style="color: var(--gw-warn)">
              {{ t('affiliate.transfer.empty') }}
            </p>
          </div>
        </div>


          <section class="gw-panel">
            <h2 class="gw-section-title">{{ t('affiliateOperations.accountSummary') }}</h2>
            <p class="gw-subtitle mt-1">{{ t('affiliateOperations.accountHint') }}</p>
            <dl v-if="summary" class="mt-4 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
              <div><dt class="gw-field-label">{{ t('affiliate.stats.invitedUsers') }}</dt><dd class="tabular-nums">{{ formatCount(summary.invited_count) }}</dd></div>
              <div><dt class="gw-field-label">{{ t('affiliateOperations.pendingEvidence') }}</dt><dd class="tabular-nums">{{ formatCount(summary.pending_count) }}</dd></div>
              <div><dt class="gw-field-label">{{ t('affiliateOperations.grossEarned') }}</dt><dd class="tabular-nums">{{ summaryMoney(summary.gross_earned, summary.currency) }}</dd></div>
              <div><dt class="gw-field-label">{{ t('affiliateOperations.revokedAmount') }}</dt><dd class="tabular-nums">{{ summaryMoney(summary.revoked_amount, summary.currency) }}</dd></div>
              <div><dt class="gw-field-label">{{ t('affiliateOperations.netEarned') }}</dt><dd class="tabular-nums">{{ summaryMoney(summary.net_earned, summary.currency) }}</dd></div>
              <div><dt class="gw-field-label">{{ t('affiliateOperations.transferredAmount') }}</dt><dd class="tabular-nums">{{ formatCurrency(summary.transferred_amount) }}</dd></div>
              <div><dt class="gw-field-label">{{ t('affiliateOperations.withdrawnAmount') }}</dt><dd class="tabular-nums">{{ formatCurrency(summary.withdrawn_amount) }}</dd></div>
              <div><dt class="gw-field-label">{{ t('affiliate.stats.rebateRate') }}</dt><dd class="tabular-nums">{{ formattedRebateRate }}%</dd></div>
            </dl>
            <template v-if="summary">
              <section v-for="group in [{ key: 'lifetimeAmounts', rows: summary.amounts }, { key: 'periodAmounts', rows: summary.period_amounts }]" :key="group.key" class="mt-5 border-t pt-4" style="border-color: var(--gw-line)">
                <h3 class="font-medium">{{ t(`affiliateOperations.${group.key}`, { month: summary.month }) }}</h3>
                <div v-if="group.rows?.length" class="mt-3 overflow-x-auto"><table class="gw-leaderboard w-full"><thead><tr><th>{{ t('affiliateOperations.source') }}</th><th>{{ t('affiliateOperations.currency') }}</th><th>{{ t('affiliateOperations.grossEarned') }}</th><th>{{ t('affiliateOperations.revokedAmount') }}</th><th>{{ t('affiliateOperations.netEarned') }}</th></tr></thead><tbody><tr v-for="row in group.rows" :key="`${row.source_type}-${row.currency}`"><td>{{ t(`affiliateOperations.sources.${row.source_type}`, row.source_type) }}</td><td>{{ row.currency }}</td><td class="tabular-nums">{{ formatCurrency(row.gross_earned, row.currency) }}</td><td class="tabular-nums">{{ formatCurrency(row.revoked_amount, row.currency) }}</td><td class="tabular-nums">{{ formatCurrency(row.net_earned, row.currency) }}</td></tr></tbody></table></div>
                <p v-else class="mt-3 gw-subtitle">{{ t('affiliateOperations.earningsEmpty') }}</p>
              </section>
            </template>
          </section>
        </section>

        <section v-if="activeTab === 'invitees'" id="affiliate-panel-invitees" role="tabpanel" aria-labelledby="affiliate-tab-invitees" class="mt-4" :aria-busy="inviteesLoading">
          <p class="gw-subtitle">{{ t('affiliateOperations.inviteesHint') }}</p>
          <form class="my-4 flex flex-wrap items-end gap-3" @submit.prevent="applyFilters('invitees')">
            <label class="gw-field"><span class="gw-field-label">{{ t('affiliateOperations.search') }}</span><input v-model="inviteeSearch" class="input" type="search" :placeholder="t('affiliateOperations.searchHint')" /></label>
            <label class="gw-field"><span class="gw-field-label">{{ t('affiliateOperations.status') }}</span><select v-model="inviteeStatus" class="input"><option value="">{{ t('affiliateOperations.all') }}</option><option v-for="status in inviteeStatuses" :key="status" :value="status">{{ t(`affiliateOperations.statuses.${status}`) }}</option></select></label>
            <label class="gw-field"><span class="gw-field-label">{{ t('affiliateOperations.from') }}</span><input v-model="inviteeStart" type="date" class="input" /></label>
            <label class="gw-field"><span class="gw-field-label">{{ t('affiliateOperations.to') }}</span><input v-model="inviteeEnd" type="date" class="input" /></label>
            <button type="submit" class="gw-btn gw-btn-secondary" :disabled="inviteesLoading">{{ t('affiliateOperations.filter') }}</button>
          </form>
          <p v-if="dateError" role="alert">{{ t('affiliateOperations.dateError') }}</p>
          <div v-if="inviteesFailed" role="alert"><p>{{ t('affiliateOperations.loadFailed') }}</p><button type="button" data-test="invitees-retry" class="gw-btn gw-btn-secondary mt-3" @click="loadInvitees()">{{ t('affiliateOperations.retry') }}</button></div>
          <template v-else>
            <DataTable :columns="inviteeColumns" :data="invitees?.items ?? []" :loading="inviteesLoading" row-key="user_id">
              <template #empty><p>{{ t('affiliateOperations.inviteesEmpty') }}</p></template>
              <template #cell-status="{ row }"><span>{{ t(`affiliateOperations.statuses.${row.status}`) }}</span><p v-if="['evidence_missing', 'evidence_conflict'].includes(row.reason)" class="text-xs gw-subtitle mt-1">{{ t(`affiliateOperations.reasons.${row.reason}`) }}</p></template>
              <template #cell-invited_at="{ row }">{{ formatDateTime(row.invited_at) || t('affiliateOperations.unknown') }}</template>
              <template #cell-first_paid_month="{ row }">{{ row.first_paid_month || '—' }}</template>
              <template #cell-total_rebate="{ row }">{{ formatCurrency(row.total_rebate) }}</template>
            </DataTable>
            <Pagination v-if="invitees && invitees.total > 0 && !inviteesLoading" :total="invitees.total" :page="inviteePage" :page-size="20" :show-page-size-selector="false" @update:page="loadInvitees" />
          </template>
        </section>

        <section v-if="activeTab === 'earnings'" id="affiliate-panel-earnings" role="tabpanel" aria-labelledby="affiliate-tab-earnings" class="mt-4" :aria-busy="earningsLoading">
          <div class="flex flex-wrap items-center justify-between gap-3"><p class="gw-subtitle">{{ t('affiliateOperations.earningsHint') }}</p><RouterLink :to="{ name: 'Wallet', query: { source: 'affiliate' } }" data-test="affiliate-wallet-link" class="gw-btn gw-btn-secondary">{{ t('affiliateOperations.wallet') }}</RouterLink></div>
          <form class="my-4 flex flex-wrap items-end gap-3" @submit.prevent="applyFilters('earnings')">
            <label class="gw-field"><span class="gw-field-label">{{ t('affiliateOperations.action') }}</span><select v-model="earningAction" class="input"><option value="">{{ t('affiliateOperations.all') }}</option><option v-for="action in earningActions" :key="action" :value="action">{{ t(`affiliateOperations.actions.${action}`) }}</option></select></label>
            <label class="gw-field"><span class="gw-field-label">{{ t('affiliateOperations.from') }}</span><input v-model="earningStart" type="date" class="input" /></label>
            <label class="gw-field"><span class="gw-field-label">{{ t('affiliateOperations.to') }}</span><input v-model="earningEnd" type="date" class="input" /></label>
            <button type="submit" class="gw-btn gw-btn-secondary" :disabled="earningsLoading">{{ t('affiliateOperations.filter') }}</button>
          </form>
          <p v-if="dateError" role="alert">{{ t('affiliateOperations.dateError') }}</p>
          <div v-if="earningsFailed" role="alert"><p>{{ t('affiliateOperations.loadFailed') }}</p><button type="button" class="gw-btn gw-btn-secondary mt-3" @click="loadEarnings()">{{ t('affiliateOperations.retry') }}</button></div>
          <template v-else>
            <DataTable :columns="earningColumns" :data="earnings?.items ?? []" :loading="earningsLoading" row-key="id">
              <template #empty><p>{{ t('affiliateOperations.earningsEmpty') }}</p></template>
              <template #cell-created_at="{ row }">{{ formatDateTime(row.created_at) }}</template>
              <template #cell-action="{ row }">{{ t(`affiliateOperations.actions.${row.action}`, row.action) }}</template>
              <template #cell-amount="{ row }">{{ formatCurrency(row.amount, row.currency || 'USD') }}</template>
              <template #cell-status="{ row }"><span>{{ t(`affiliateOperations.statuses.${row.status}`, row.status) }}</span><p v-if="row.frozen_until" class="text-xs gw-subtitle mt-1">{{ t('affiliateOperations.frozenUntil', { date: formatDateTime(row.frozen_until) }) }}</p></template>
              <template #cell-source="{ row }"><span>{{ t(`affiliateOperations.sources.${row.source_type}`, row.source_type) }}</span><p v-if="row.source_user_display_name" class="text-xs gw-subtitle mt-1">{{ row.source_user_display_name }}</p><p v-if="row.source_order_id" class="text-xs gw-subtitle mt-1">{{ t('affiliateOperations.order', { id: row.source_order_id }) }}</p></template>
            </DataTable>
            <Pagination v-if="earnings && earnings.total > 0 && !earningsLoading" :total="earnings.total" :page="earningPage" :page-size="20" :show-page-size-selector="false" @update:page="loadEarnings" />
          </template>
        </section>

        <section v-if="activeTab === 'leaderboard'" id="affiliate-panel-leaderboard" role="tabpanel" aria-labelledby="affiliate-tab-leaderboard" class="gw-panel" :aria-busy="leaderboardRefreshing">
          <div class="flex flex-wrap items-end gap-3">
            <label class="gw-field"><span class="gw-field-label">{{ t('affiliateOperations.rankingScope') }}</span><select v-model="leaderboardMode" class="input"><option value="monthly">{{ t('affiliateOperations.monthly') }}</option><option value="campaign">{{ t('affiliateOperations.activity') }}</option></select></label>
            <label v-if="leaderboardMode === 'monthly'" class="gw-field"><span class="gw-field-label">{{ t('affiliateOperations.month') }}</span><input v-model="selectedMonth" type="month" :max="currentMonth" class="input" /></label>
            <label v-else class="gw-field"><span class="gw-field-label">{{ t('affiliateOperations.activity') }}</span><select v-model="selectedCampaignId" class="input"><option :value="null">{{ t('affiliateOperations.chooseActivity') }}</option><option v-for="item in leaderboardCampaigns" :key="item.campaign.id" :value="item.campaign.id">{{ item.campaign.name }}</option></select></label>
            <button type="button" data-test="leaderboard-refresh" class="gw-btn gw-btn-secondary" :disabled="leaderboardRefreshing || (leaderboardMode === 'campaign' && !selectedCampaignId) || (leaderboardMode === 'monthly' && !monthlySelectionValid)" @click="board.refresh"><Icon name="refresh" size="sm" />{{ t('affiliate.campaign.refresh') }}</button>
          </div>
          <p class="gw-subtitle mt-4">{{ t('affiliateOperations.honorOnly') }}</p>
          <p class="gw-subtitle mt-1 text-xs">{{ leaderboardMode === 'monthly' ? t('affiliateOperations.rankingRules') : t('affiliateOperations.activityRankingRules') }}</p>
          <p v-if="leaderboardMode === 'monthly' && !monthlySelectionValid" class="mt-3" role="alert">{{ t('affiliateOperations.invalidMonth') }}</p>
          <p v-if="leaderboardError" role="alert" class="mt-3">{{ t('affiliateOperations.refreshFailed') }}</p>
          <p v-if="leaderboardRefreshing && !leaderboard" role="status" class="py-8">{{ t('models.loading') }}</p>
          <template v-if="leaderboard && (leaderboardMode !== 'monthly' || monthlySelectionValid)">
            <div class="mt-4 overflow-x-auto"><table class="gw-leaderboard w-full"><thead><tr><th>{{ t('affiliate.campaign.rank') }}</th><th>{{ t('affiliateOperations.invitee') }}</th><th class="text-right">{{ leaderboardMetric }}</th></tr></thead><tbody><tr v-for="row in leaderboard.items.slice(0, 10)" :key="row.rank" data-test="leaderboard-row"><td class="tabular-nums">#{{ row.rank }}</td><td>{{ row.display_name }}<span v-if="row.is_me" class="ml-2">{{ t('affiliate.campaign.me') }}</span></td><td class="text-right tabular-nums">{{ formatCount(row.effective_paid_count) }}</td></tr></tbody></table></div>
            <div v-if="!leaderboard.items.length" class="py-8">
              <p class="gw-subtitle">{{ t('affiliateOperations.rankingEmpty') }}</p>
              <p v-if="leaderboardMode === 'monthly'" class="mt-2 text-sm gw-subtitle">{{ t('affiliateOperations.paidRankingExplanation') }}</p>
              <button type="button" data-test="ranking-invitations-link" class="mt-3 gw-btn gw-btn-secondary" @click="activeTab = 'invitees'">{{ t('affiliateOperations.tabs.invitees') }}</button>
            </div>
            <div data-test="my-rank" class="mt-4 border-t pt-4" style="border-color: var(--gw-line)"><p class="font-medium">{{ t('affiliateOperations.myRank') }}</p><p v-if="leaderboard.my_rank" class="mt-1 tabular-nums">#{{ leaderboard.my_rank.rank }} · {{ formatCount(leaderboard.my_rank.effective_paid_count) }} {{ leaderboardMetric }}</p><p v-else class="mt-1 gw-subtitle">{{ t('affiliateOperations.notRanked') }}</p></div>
            <p class="mt-4 text-xs gw-subtitle">{{ t('affiliateOperations.updatedAt', { date: formatDateTime(leaderboard.updated_at) }) }}</p>
          </template>
          <p v-else-if="leaderboardMode === 'campaign' && !selectedCampaignId" class="py-8 gw-subtitle">{{ t('affiliateOperations.chooseActivityHint') }}</p>
        </section>

        <div v-if="activeTab === 'campaigns'" id="affiliate-panel-campaigns" role="tabpanel" aria-labelledby="affiliate-tab-campaigns">
          <label class="gw-field mt-4"><span class="gw-field-label">{{ t('affiliateOperations.activityScope') }}</span><select v-model="campaignScope" data-test="campaign-scope" class="input"><option value="available">{{ t('affiliateOperations.availableActivities') }}</option><option value="joined">{{ t('affiliateOperations.joinedActivities') }}</option><option value="ended">{{ t('affiliateOperations.endedActivities') }}</option></select></label>
          <p v-if="growthLoadFailed && campaignScope === 'available'" role="alert" class="mt-3">{{ t('affiliateOperations.growthLoadFailed') }}</p>
        <section v-if="campaignScope === 'available' && growthCampaigns.length" class="gw-panel" aria-labelledby="growth-campaign-title">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between"><div><p class="gw-eyebrow">{{ t('affiliate.growth.eyebrow') }}</p><h2 id="growth-campaign-title" class="gw-section-title">{{ t('affiliate.growth.title') }}</h2><p class="gw-subtitle">{{ t('affiliate.growth.description') }}</p></div><span class="agent-pill">{{ t('affiliate.growth.inviteOnly') }}</span></div>
          <div class="mt-5 space-y-6">
            <article v-for="campaign in growthCampaigns" :key="campaign.id" class="border-t pt-5 first:border-t-0 first:pt-0" style="border-color: var(--gw-line)">
              <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between"><div><h3 class="text-lg font-semibold">{{ campaign.name }}</h3><p class="mt-1 gw-subtitle text-sm">{{ campaign.new_user_growth?.qualification_metric === 'actual_consumption' ? t('affiliate.growth.metricConsumption') : t('affiliate.growth.metricRecharge') }}</p><p class="mt-1 gw-subtitle text-xs">{{ t('affiliate.growth.rebatePolicy', { policy: campaign.rules.legacy_rebate_policy === 'stack' ? t('affiliate.campaign.rebateStack') : t('affiliate.campaign.rebateExclude') }) }}</p></div><p class="text-sm gw-subtitle">{{ t('affiliate.growth.deadline', { date: formatDateTime(campaign.end_at) }) }}</p></div>
              <p v-if="campaign.new_user_growth?.funding_conflict" class="mt-3 border-l-2 px-3 py-2 text-sm gw-subtitle" style="border-color: var(--gw-warn); color: var(--gw-warn)" role="status">{{ t('affiliate.growth.fundingConflict') }}</p>
              <div class="mt-4"><div class="flex items-end justify-between gap-3"><p class="gw-field-label">{{ t('affiliate.growth.progress') }}</p><p class="text-lg font-semibold tabular-nums">{{ formatCurrency(campaign.new_user_growth?.qualified_amount || 0) }}</p></div><div class="mt-2 h-2 overflow-hidden rounded bg-gray-200 dark:bg-dark-700" role="progressbar" :aria-valuenow="growthProgressPercent(campaign)" aria-valuemin="0" aria-valuemax="100"><div class="h-full bg-primary-600" :style="{ width: `${growthProgressPercent(campaign)}%` }"></div></div></div>
              <div class="mt-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-4"><div v-for="tier in campaign.rules.reward_tiers" :key="tier.tier" class="rounded border p-4" style="border-color: var(--gw-line)"><div class="flex items-start justify-between gap-2"><div><p class="font-semibold">{{ t('affiliate.growth.tier', { amount: formatCurrency(tier.required_amount) }) }}</p><p class="mt-1 text-lg font-semibold" style="color: var(--gw-ok)">{{ formatCurrency(tier.reward_amount) }}</p></div><span class="agent-pill">{{ (campaign.new_user_growth?.qualified_amount ?? 0) >= tier.required_amount ? t('affiliate.campaign.unlocked') : t('affiliate.campaign.locked') }}</span></div><button v-if="campaign.new_user_growth?.rewards.find(item => item.tier === tier.tier)?.status === 'claimable'" type="button" class="gw-btn gw-btn-primary mt-3 w-full" :disabled="growthAction !== null" @click="claimGrowthReward(campaign, campaign.new_user_growth?.rewards.find(item => item.tier === tier.tier)?.reward_id || 0)">{{ t('affiliate.growth.claim') }}</button></div></div>
            </article>
          </div>
        </section>

        <section class="gw-panel" aria-labelledby="referral-campaign-title">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"><div><p class="gw-eyebrow">{{ t('affiliate.campaign.eyebrow') }}</p><h2 id="referral-campaign-title" class="gw-section-title">{{ t('affiliate.campaign.title') }}</h2></div><button type="button" class="gw-btn gw-btn-secondary" :disabled="campaignLoading || campaignAction !== null" @click="campaignScope === 'available' ? loadCampaigns() : loadHistory()"><Icon name="refresh" size="sm" />{{ t('affiliate.campaign.refresh') }}</button></div>
          <div v-if="campaignScope === 'available' ? campaignLoading : historyLoading" class="gw-polling py-8 text-center">{{ t('affiliate.campaign.loading') }}</div>
          <div v-else-if="campaignScope === 'available' ? campaignLoadFailed : historyFailed" class="mt-4 gw-subtitle" role="alert">{{ t('affiliate.campaign.loadFailed') }}</div>
          <div v-else-if="!visibleCampaigns.length" class="mt-4 border border-dashed p-8 text-center gw-subtitle" style="border-color: var(--gw-line)">{{ t('affiliate.campaign.empty') }}</div>
          <div v-else class="mt-4 space-y-6">
            <article v-for="campaign in visibleCampaigns" :key="campaign.campaign.id" class="border-t pt-5 first:border-t-0 first:pt-0" style="border-color: var(--gw-line)">
              <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between"><div><div class="flex flex-wrap items-center gap-2"><h3 class="text-lg font-semibold">{{ campaign.campaign.name }}</h3><span class="agent-pill">{{ t(`affiliate.campaign.statuses.${campaign.campaign.status}`, campaign.campaign.status) }}</span><span v-if="campaign.attention" class="agent-pill" style="color: var(--gw-warn)">{{ t(`affiliate.campaign.attention.${campaign.attention}`) }}</span></div><p class="mt-2 gw-subtitle text-sm">{{ t('affiliate.campaign.rule', { pay: formatCurrency(campaign.campaign.pay_threshold), spend: formatCurrency(campaign.campaign.usage_threshold) }) }}</p><p class="mt-1 gw-subtitle text-xs">{{ t('affiliate.campaign.windows', { registration: formatDateTime(campaign.campaign.registration_to), qualification: formatDateTime(campaign.campaign.qualification_to), claim: formatDateTime(campaign.campaign.claim_deadline) }) }}</p></div><div class="flex flex-col gap-2 sm:flex-row"><button v-if="!campaign.enrollment && canShareCampaign(campaign)" type="button" class="gw-btn gw-btn-primary" :disabled="campaignAction !== null" @click="enrollCampaign(campaign)">{{ t('affiliate.campaign.enroll') }}</button><button v-else-if="campaign.enrollment && canShareCampaign(campaign)" type="button" class="gw-btn gw-btn-secondary" :disabled="campaignAction !== null" @click="copyCampaignLink(campaign)"><Icon name="copy" size="sm" />{{ t('affiliate.campaign.share') }}</button></div></div>
              <div class="mt-4 grid gap-3 border-y py-4 text-sm sm:grid-cols-2" style="border-color: var(--gw-line)"><p><strong>{{ t('affiliate.campaign.rebatePolicy') }}</strong> {{ campaign.campaign.legacy_rebate_policy === 'stack' ? t('affiliate.campaign.rebateStack') : t('affiliate.campaign.rebateExclude') }}</p><p><strong>{{ t('affiliate.campaign.version') }}</strong> {{ campaign.campaign.rules_version }} · {{ formatDateTime(campaign.campaign.rules_updated_at) }}</p><p>{{ t('affiliate.campaign.riskNotice', { hours: campaign.campaign.risk_hold_hours }) }}</p><p>{{ t('affiliate.campaign.refundNotice') }}</p></div>
              <details class="mt-4"><summary class="cursor-pointer text-sm font-medium focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500">{{ t('affiliateOperations.fullRules') }}</summary><AnnouncementContent class="mt-3" dense :content="campaign.campaign.public_rules_md || ''" /></details>

              <section class="mt-4 space-y-3 text-sm">
                <h4 class="font-semibold">{{ t('affiliate.campaign.details.title') }}</h4>
                <dl class="grid gap-3 sm:grid-cols-2">
                  <div>
                    <dt class="gw-field-label">{{ t('affiliate.campaign.details.qualificationTitle') }}</dt>
                    <dd>{{ t('affiliate.campaign.details.payThreshold', { amount: formatCurrency(campaign.campaign.pay_threshold) }) }} · {{ t('affiliate.campaign.details.usageThreshold', { amount: formatCurrency(campaign.campaign.usage_threshold) }) }}</dd>
                  </div>
                  <div v-if="['additive', 'replace'].includes(campaign.campaign.reward_mode)">
                    <dt class="gw-field-label">{{ t('affiliate.campaign.details.rewardMode') }}</dt>
                    <dd>{{ t(`affiliate.campaign.details.rewardModes.${campaign.campaign.reward_mode}`) }}</dd>
                  </div>
                  <div>
                    <dt class="gw-field-label">{{ t('affiliate.campaign.details.holdTitle') }}</dt>
                    <dd>{{ t('affiliate.campaign.details.holdDesc', { hours: campaign.campaign.risk_hold_hours }) }}</dd>
                  </div>
                  <div>
                    <dt class="gw-field-label">{{ t('affiliate.campaign.details.deadlineTitle') }}</dt>
                    <dd>{{ formatDateTime(campaign.campaign.claim_deadline) }}</dd>
                  </div>
                </dl>
              </section>

              <div class="mt-5"><div class="flex items-end justify-between gap-3"><div><p class="gw-field-label">{{ t('affiliate.campaign.progress') }}</p><p class="mt-1 text-2xl font-semibold tabular-nums">{{ campaign.qualified_count }} <span class="text-sm font-normal gw-subtitle">/ {{ Math.max(...campaign.tiers.map(item => item.required_invites), 0) }}</span></p><p class="mt-1 text-xs gw-subtitle">{{ t('affiliate.campaign.invitedBreakdown', { invited: campaign.invited_count, qualified: campaign.qualified_count }) }}</p></div><p v-if="campaign.ranking" class="text-sm gw-subtitle">{{ t('affiliate.campaign.myRank', { rank: campaign.ranking.rank }) }}</p></div><div class="mt-3 h-2 overflow-hidden rounded bg-gray-200 dark:bg-dark-700" role="progressbar" :aria-valuenow="campaignProgressPercent(campaign)" aria-valuemin="0" aria-valuemax="100"><div class="h-full bg-primary-600" :style="{ width: `${campaignProgressPercent(campaign)}%` }"></div></div></div>

              <div class="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-3"><div v-for="tier in campaign.tiers" :key="tier.tier" class="rounded border p-4" style="border-color: var(--gw-line)"><div class="flex items-start justify-between gap-3"><div><p class="font-semibold">{{ t('affiliate.campaign.milestone', { count: tier.required_invites }) }}</p><p class="mt-1 text-lg font-semibold" style="color: var(--gw-ok)">{{ formatCurrency(tier.reward_amount, tier.currency) }}</p></div><span class="agent-pill">{{ campaign.qualified_count >= tier.required_invites ? t('affiliate.campaign.unlocked') : t('affiliate.campaign.locked') }}</span></div><div v-if="campaignReward(campaign, tier.tier)" class="mt-3"><button v-if="campaignReward(campaign, tier.tier)?.status === 'claimable'" type="button" class="gw-btn gw-btn-primary w-full" :disabled="campaignAction !== null" @click="claimCampaignReward(campaign, campaignReward(campaign, tier.tier)!.id)">{{ t('affiliate.campaign.claim') }}</button><p v-else class="text-sm gw-subtitle">{{ t(`affiliate.campaign.rewardStatuses.${campaignReward(campaign, tier.tier)?.status}`, campaignReward(campaign, tier.tier)?.status || '') }}</p></div></div></div>
              <button type="button" class="gw-btn gw-btn-secondary mt-4" @click="openCampaignLeaderboard(campaign.campaign.id)">{{ t('affiliateOperations.viewActivityLeaderboard') }}</button>
            </article>
          </div>
        </section>


          <Pagination v-if="campaignScope !== 'available' && campaignHistory && campaignHistory.total > 0 && !historyLoading && !historyFailed" :total="campaignHistory.total" :page="historyPage" :page-size="20" :show-page-size-selector="false" @update:page="loadHistory" />
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<style scoped>
.agent-pill {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  padding: 4px 10px;
  border: 1px solid var(--gw-line);
  border-radius: 8px;
  color: var(--gw-muted);
  font-size: 12px;
}

@media (width < 640px) {
  .gw-code-row code {
    overflow: visible;
    overflow-wrap: anywhere;
    text-overflow: clip;
    white-space: normal;
  }
}
</style>
