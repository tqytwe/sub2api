<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import AnnouncementContent from '@/components/common/AnnouncementContent.vue'
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
const detail = ref<UserAffiliateDetail | null>(null)
const teamInviteCode = ref('')
const campaignLoading = ref(false)
const campaignAction = ref<number | null>(null)
const campaigns = ref<ReferralCampaignProgress[]>([])
const growthCampaigns = ref<PlayCampaignSummary[]>([])
const growthAction = ref<number | null>(null)

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

async function loadAffiliateDetail(silent = false): Promise<void> {
  if (!silent) {
    loading.value = true
  }
  try {
    detail.value = await userAPI.getAffiliateDetail()
  } catch (error) {
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
  try {
    const resp = await userAPI.transferAffiliateQuota()
    appStore.showSuccess(t('affiliate.transfer.success', { amount: formatCurrency(resp.transferred_quota) }))
    await Promise.all([
      loadAffiliateDetail(true),
      authStore.refreshUser().catch(() => undefined),
    ])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.transferFailed')))
  } finally {
    transferring.value = false
  }
}

async function loadCampaigns(): Promise<void> {
  campaignLoading.value = true
  try {
    campaigns.value = await listReferralCampaigns()
    await Promise.all(campaigns.value.filter(item => item.unseen_update).map(item => markReferralCampaignViewed(item.campaign.id).catch(() => undefined)))
    try {
      const growth = await getActiveCampaigns()
      growthCampaigns.value = growth.filter(item => item.new_user_growth)
    } catch {
      growthCampaigns.value = []
    }
  } catch (error) { appStore.showError(extractI18nErrorMessage(error, t, 'affiliate.campaign.errors', t('affiliate.campaign.loadFailed'))) } finally { campaignLoading.value = false }
}

async function claimGrowthReward(campaign: PlayCampaignSummary, rewardId: number): Promise<void> {
  const progress = campaign.new_user_growth
  if (!progress || !progress.referral_campaign_id || !progress.referral_version) return
  growthAction.value = rewardId
  try {
    await claimReferralCampaignReward(progress.referral_campaign_id, rewardId, progress.referral_version)
    const refreshed = await getActiveCampaigns()
    growthCampaigns.value = refreshed.filter(item => item.new_user_growth)
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
  campaignAction.value = campaign.campaign.id
  try { await enrollReferralCampaign(campaign.campaign.id); const progress = await getReferralCampaignProgress(campaign.campaign.id); campaigns.value = campaigns.value.map(item => item.campaign.id === progress.campaign.id ? progress : item); appStore.showSuccess(t('affiliate.campaign.enrolled')) } catch (error) { appStore.showError(extractI18nErrorMessage(error, t, 'affiliate.campaign.errors', t('affiliate.campaign.enrollFailed'))) } finally { campaignAction.value = null }
}

async function copyCampaignLink(campaign: ReferralCampaignProgress): Promise<void> {
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
  campaignAction.value = rewardId
  try { await claimReferralCampaignReward(campaign.campaign.id, rewardId, campaign.campaign.version); const progress = await getReferralCampaignProgress(campaign.campaign.id); campaigns.value = campaigns.value.map(item => item.campaign.id === progress.campaign.id ? progress : item); appStore.showSuccess(t('affiliate.campaign.claimed')) } catch (error) { appStore.showError(extractI18nErrorMessage(error, t, 'affiliate.campaign.errors', t('affiliate.campaign.claimFailed'))) } finally { campaignAction.value = null }
}

function campaignReward(campaign: ReferralCampaignProgress, tier: number) { return campaign.rewards.find(item => item.tier === tier) }
function campaignProgressPercent(campaign: ReferralCampaignProgress) { const max = Math.max(...campaign.tiers.map(item => item.required_invites), 1); return Math.min(100, Math.round(campaign.qualified_count * 100 / max)) }

onMounted(() => {
  void loadAffiliateDetail()
  void loadTeamInviteCode()
  void loadCampaigns()
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

      <div v-if="loading" class="gw-polling py-12 text-center">{{ t('models.loading') }}</div>

      <template v-else-if="detail">
        <section v-if="growthCampaigns.length" class="gw-panel" aria-labelledby="growth-campaign-title">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between"><div><p class="gw-eyebrow">{{ t('affiliate.growth.eyebrow') }}</p><h2 id="growth-campaign-title" class="gw-section-title">{{ t('affiliate.growth.title') }}</h2><p class="gw-subtitle">{{ t('affiliate.growth.description') }}</p></div><span class="agent-pill">{{ t('affiliate.growth.inviteOnly') }}</span></div>
          <div class="mt-5 space-y-6">
            <article v-for="campaign in growthCampaigns" :key="campaign.id" class="border-t pt-5 first:border-t-0 first:pt-0" style="border-color: var(--gw-line)">
              <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between"><div><h3 class="text-lg font-semibold">{{ campaign.name }}</h3><p class="mt-1 gw-subtitle text-sm">{{ campaign.new_user_growth?.qualification_metric === 'actual_consumption' ? t('affiliate.growth.metricConsumption') : t('affiliate.growth.metricRecharge') }}</p><p class="mt-1 gw-subtitle text-xs">{{ t('affiliate.growth.rebatePolicy', { policy: campaign.rules.legacy_rebate_policy === 'stack' ? t('affiliate.campaign.rebateStack') : t('affiliate.campaign.rebateExclude') }) }}</p></div><p class="text-sm gw-subtitle">{{ t('affiliate.growth.deadline', { date: formatDateTime(campaign.end_at) }) }}</p></div>
              <p v-if="campaign.new_user_growth?.funding_conflict" class="mt-3 border-l-2 px-3 py-2 text-sm gw-subtitle" style="border-color: var(--gw-warn); color: var(--gw-warn)" role="status">{{ t('affiliate.growth.fundingConflict') }}</p>
              <div class="mt-4"><div class="flex items-end justify-between gap-3"><p class="gw-field-label">{{ t('affiliate.growth.progress') }}</p><p class="text-lg font-semibold tabular-nums">{{ formatCurrency(campaign.new_user_growth?.qualified_amount || 0) }}</p></div><div class="mt-2 h-2 overflow-hidden rounded bg-gray-200 dark:bg-dark-700" role="progressbar" :aria-valuenow="growthProgressPercent(campaign)" aria-valuemin="0" aria-valuemax="100"><div class="h-full bg-primary-600" :style="{ width: `${growthProgressPercent(campaign)}%` }"></div></div></div>
              <div class="mt-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-4"><div v-for="tier in campaign.rules.reward_tiers" :key="tier.tier" class="rounded border p-4" style="border-color: var(--gw-line)"><div class="flex items-start justify-between gap-2"><div><p class="font-semibold">{{ t('affiliate.growth.tier', { amount: formatCurrency(tier.required_amount) }) }}</p><p class="mt-1 text-lg font-semibold" style="color: var(--gw-ok)">{{ formatCurrency(tier.reward_amount) }}</p></div><span class="agent-pill">{{ (campaign.new_user_growth?.qualified_amount ?? 0) >= tier.required_amount ? t('affiliate.campaign.unlocked') : t('affiliate.campaign.locked') }}</span></div><button v-if="campaign.new_user_growth?.rewards.find(item => item.tier === tier.tier)?.status === 'claimable'" type="button" class="gw-btn gw-btn-primary mt-3 w-full" :disabled="growthAction === campaign.new_user_growth?.rewards.find(item => item.tier === tier.tier)?.reward_id" @click="claimGrowthReward(campaign, campaign.new_user_growth?.rewards.find(item => item.tier === tier.tier)?.reward_id || 0)">{{ t('affiliate.growth.claim') }}</button></div></div>
            </article>
          </div>
        </section>

        <section class="gw-panel" aria-labelledby="referral-campaign-title">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"><div><p class="gw-eyebrow">{{ t('affiliate.campaign.eyebrow') }}</p><h2 id="referral-campaign-title" class="gw-section-title">{{ t('affiliate.campaign.title') }}</h2></div><button type="button" class="gw-btn gw-btn-secondary" :disabled="campaignLoading" @click="loadCampaigns"><Icon name="refresh" size="sm" />{{ t('affiliate.campaign.refresh') }}</button></div>
          <div v-if="campaignLoading" class="gw-polling py-8 text-center">{{ t('affiliate.campaign.loading') }}</div>
          <div v-else-if="!campaigns.length" class="mt-4 border border-dashed p-8 text-center gw-subtitle" style="border-color: var(--gw-line)">{{ t('affiliate.campaign.empty') }}</div>
          <div v-else class="mt-4 space-y-6">
            <article v-for="campaign in campaigns" :key="campaign.campaign.id" class="border-t pt-5 first:border-t-0 first:pt-0" style="border-color: var(--gw-line)">
              <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between"><div><div class="flex flex-wrap items-center gap-2"><h3 class="text-lg font-semibold">{{ campaign.campaign.name }}</h3><span class="agent-pill">{{ t(`affiliate.campaign.statuses.${campaign.campaign.status}`, campaign.campaign.status) }}</span><span v-if="campaign.attention" class="agent-pill" style="color: var(--gw-warn)">{{ t(`affiliate.campaign.attention.${campaign.attention}`) }}</span></div><p class="mt-2 gw-subtitle text-sm">{{ t('affiliate.campaign.rule', { pay: formatCurrency(campaign.campaign.pay_threshold), spend: formatCurrency(campaign.campaign.usage_threshold) }) }}</p><p class="mt-1 gw-subtitle text-xs">{{ t('affiliate.campaign.windows', { registration: formatDateTime(campaign.campaign.registration_to), qualification: formatDateTime(campaign.campaign.qualification_to), claim: formatDateTime(campaign.campaign.claim_deadline) }) }}</p></div><div class="flex flex-col gap-2 sm:flex-row"><button v-if="!campaign.enrollment" type="button" class="gw-btn gw-btn-primary" :disabled="campaignAction === campaign.campaign.id" @click="enrollCampaign(campaign)">{{ t('affiliate.campaign.enroll') }}</button><button v-else type="button" class="gw-btn gw-btn-secondary" :disabled="campaignAction === campaign.campaign.id" @click="copyCampaignLink(campaign)"><Icon name="copy" size="sm" />{{ t('affiliate.campaign.share') }}</button></div></div>
              <div class="mt-4 grid gap-3 border-y py-4 text-sm sm:grid-cols-2" style="border-color: var(--gw-line)"><p><strong>{{ t('affiliate.campaign.rebatePolicy') }}</strong> {{ campaign.campaign.legacy_rebate_policy === 'stack' ? t('affiliate.campaign.rebateStack') : t('affiliate.campaign.rebateExclude') }}</p><p><strong>{{ t('affiliate.campaign.version') }}</strong> {{ campaign.campaign.rules_version }} · {{ formatDateTime(campaign.campaign.rules_updated_at) }}</p><p>{{ t('affiliate.campaign.riskNotice', { hours: campaign.campaign.risk_hold_hours }) }}</p><p>{{ t('affiliate.campaign.refundNotice') }}</p></div>
              <AnnouncementContent class="mt-4" dense :content="campaign.campaign.public_rules_md || ''" />

              <div class="mt-5"><div class="flex items-end justify-between gap-3"><div><p class="gw-field-label">{{ t('affiliate.campaign.progress') }}</p><p class="mt-1 text-2xl font-semibold tabular-nums">{{ campaign.qualified_count }} <span class="text-sm font-normal gw-subtitle">/ {{ Math.max(...campaign.tiers.map(item => item.required_invites), 0) }}</span></p><p class="mt-1 text-xs gw-subtitle">{{ t('affiliate.campaign.invitedBreakdown', { invited: campaign.invited_count, qualified: campaign.qualified_count }) }}</p></div><p v-if="campaign.ranking" class="text-sm gw-subtitle">{{ t('affiliate.campaign.myRank', { rank: campaign.ranking.rank }) }}</p></div><div class="mt-3 h-2 overflow-hidden rounded bg-gray-200 dark:bg-dark-700" role="progressbar" :aria-valuenow="campaignProgressPercent(campaign)" aria-valuemin="0" aria-valuemax="100"><div class="h-full bg-primary-600" :style="{ width: `${campaignProgressPercent(campaign)}%` }"></div></div></div>

              <div class="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-3"><div v-for="tier in campaign.tiers" :key="tier.tier" class="rounded border p-4" style="border-color: var(--gw-line)"><div class="flex items-start justify-between gap-3"><div><p class="font-semibold">{{ t('affiliate.campaign.milestone', { count: tier.required_invites }) }}</p><p class="mt-1 text-lg font-semibold" style="color: var(--gw-ok)">{{ formatCurrency(tier.reward_amount) }}</p></div><span class="agent-pill">{{ campaign.qualified_count >= tier.required_invites ? t('affiliate.campaign.unlocked') : t('affiliate.campaign.locked') }}</span></div><div v-if="campaignReward(campaign, tier.tier)" class="mt-3"><button v-if="campaignReward(campaign, tier.tier)?.status === 'claimable'" type="button" class="gw-btn gw-btn-primary w-full" :disabled="campaignAction === campaignReward(campaign, tier.tier)?.id" @click="claimCampaignReward(campaign, campaignReward(campaign, tier.tier)!.id)">{{ t('affiliate.campaign.claim') }}</button><p v-else class="text-sm gw-subtitle">{{ t(`affiliate.campaign.rewardStatuses.${campaignReward(campaign, tier.tier)?.status}`, campaignReward(campaign, tier.tier)?.status || '') }}</p></div></div></div>
              <div class="gw-table-wrap mt-5 overflow-x-auto"><h4 class="gw-field-label px-4 pt-4">{{ t('affiliate.campaign.leaderboard') }}</h4><table v-if="campaign.leaderboard?.length" class="gw-leaderboard min-w-[560px]"><thead><tr><th>{{ t('affiliate.campaign.rank') }}</th><th>{{ t('affiliate.campaign.email') }}</th><th class="text-right">{{ t('affiliate.campaign.qualified') }}</th><th class="text-right">{{ t('affiliate.campaign.reward') }}</th></tr></thead><tbody><tr v-for="row in campaign.leaderboard" :key="`${campaign.campaign.id}-${row.rank}`"><td>#{{ row.rank }}</td><td>{{ row.email_masked }}<span v-if="row.is_me" class="ml-2 text-primary-600">{{ t('affiliate.campaign.me') }}</span></td><td class="text-right tabular-nums">{{ row.qualified_count }}</td><td class="text-right tabular-nums">{{ formatCurrency(row.reward_amount) }}</td></tr></tbody></table><p v-else class="px-4 py-5 text-sm gw-subtitle">{{ t('affiliate.campaign.leaderboardEmpty') }}</p></div>
            </article>
          </div>
        </section>

        <section class="mt-6" aria-labelledby="standard-affiliate-title">
          <p class="gw-eyebrow">{{ t('affiliate.standard.eyebrow') }}</p><h2 id="standard-affiliate-title" class="gw-section-title">{{ t('affiliate.standard.title') }}</h2>
        </section>
        <div class="gw-stat-grid mt-4">
          <div class="gw-stat-card">
            <p class="gw-balance-label">{{ t('affiliate.stats.rebateRate') }}</p>
            <p class="gw-stat-value">{{ formattedRebateRate }}<span class="text-base">%</span></p>
            <p class="gw-subtitle text-xs mt-1">{{ t('affiliate.stats.rebateRateHint') }}</p>
          </div>
          <div class="gw-stat-card">
            <p class="gw-balance-label">{{ t('affiliate.stats.invitedUsers') }}</p>
            <p class="gw-stat-value">{{ formatCount(detail.aff_count) }}</p>
          </div>
          <div class="gw-stat-card">
            <p class="gw-balance-label">{{ t('affiliate.stats.availableQuota') }}</p>
            <p class="gw-stat-value ok">{{ formatCurrency(detail.aff_quota) }}</p>
          </div>
          <div class="gw-stat-card">
            <p class="gw-balance-label">{{ t('affiliate.stats.totalQuota') }}</p>
            <p class="gw-stat-value">{{ formatCurrency(detail.aff_history_quota) }}</p>
            <p v-if="detail.aff_frozen_quota > 0" class="gw-subtitle text-xs mt-1" style="color: var(--gw-warn)">
              {{ t('affiliate.stats.frozenQuota') }}: {{ formatCurrency(detail.aff_frozen_quota) }}
            </p>
          </div>
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
            <p v-if="detail.aff_quota <= 0" class="mt-3 text-sm" style="color: var(--gw-warn)">
              {{ t('affiliate.transfer.empty') }}
            </p>
          </div>
        </div>

        <div class="gw-panel">
          <h3 class="gw-section-title">{{ t('affiliate.invitees.title') }}</h3>
          <div v-if="detail.invitees.length === 0" class="mt-4 gw-subtitle text-center py-8 border border-dashed rounded-xl" style="border-color: var(--gw-line)">
            {{ t('affiliate.invitees.empty') }}
          </div>
          <div v-else class="gw-table-wrap mt-4 overflow-x-auto">
            <table class="gw-leaderboard min-w-[560px]">
              <thead>
                <tr>
                  <th>{{ t('affiliate.invitees.columns.email') }}</th>
                  <th>{{ t('affiliate.invitees.columns.username') }}</th>
                  <th class="text-right">{{ t('affiliate.invitees.columns.rebate') }}</th>
                  <th>{{ t('affiliate.invitees.columns.joinedAt') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in detail.invitees" :key="item.user_id">
                  <td>{{ item.email || '-' }}</td>
                  <td>{{ item.username || '-' }}</td>
                  <td class="text-right font-medium" style="color: var(--gw-ok)">{{ formatCurrency(item.total_rebate) }}</td>
                  <td>{{ formatDateTime(item.created_at) || '-' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
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
