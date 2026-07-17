<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import PublicPlayBackLink from '@/components/common/PublicPlayBackLink.vue'
import PlayUserAvatar from '@/components/play/PlayUserAvatar.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import playAPI, { type PlayTeamMe, type PlayTeamSettlementRecord } from '@/api/play'
import { useClipboard } from '@/composables/useClipboard'
import '@/styles/public-pages.css'

type RankTone = 'gold' | 'silver' | 'bronze' | 'standard'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const submitting = ref(false)
const teamMe = ref<PlayTeamMe | null>(null)
const teamName = ref('')
const inviteCode = ref('')
const settlements = ref<PlayTeamSettlementRecord[]>([])

const isCaptain = computed(
  () => teamMe.value?.team && authStore.user?.id === teamMe.value.team.captain_id,
)
const teamSpend = computed(() => Number(teamMe.value?.team?.team_spend ?? 0))
const rewardRatePct = computed(() => Number(teamMe.value?.team?.reward_rate ?? 0) * 100)
const nextThreshold = computed(() => Number(teamMe.value?.team?.next_threshold ?? 0))
const nextTierGap = computed(() => Math.max(0, nextThreshold.value - teamSpend.value))
const sortedMembers = computed(() => {
  const members = teamMe.value?.team?.members ?? []
  return [...members].sort((a, b) => Number(b.spend) - Number(a.spend))
})
const currentRewardTiers = computed(() => teamMe.value?.team?.reward_tiers ?? [])

function formatMoney(value: string | number | undefined) {
  return Number(value ?? 0).toFixed(2)
}

function formatTokens(value?: number) {
  return (value ?? 0).toLocaleString()
}

function toneForIndex(index: number): RankTone {
  if (index === 0) return 'gold'
  if (index === 1) return 'silver'
  if (index === 2) return 'bronze'
  return 'standard'
}

function settlementStatusLabel(status: PlayTeamSettlementRecord['settlement']['status']) {
  return t(`agentTeam.status.${status}`)
}

function payoutStatusLabel(status: PlayTeamSettlementRecord['allocations'][number]['payout_status']) {
  return t(`agentTeam.payout.${status}`)
}

function memberDisplayName(userId: number) {
  return teamMe.value?.team?.members.find(member => member.user_id === userId)?.display_name || `#${userId}`
}

function tierReached(threshold: string) {
  return teamSpend.value >= Number(threshold)
}

async function loadTeam() {
  if (!authStore.isAuthenticated) {
    teamMe.value = { enabled: false }
    loading.value = false
    return
  }
  loading.value = true
  try {
    teamMe.value = await playAPI.getTeamMe()
    settlements.value = teamMe.value?.team ? await playAPI.getTeamSettlements() : []
  } catch {
    teamMe.value = null
  } finally {
    loading.value = false
  }
}

async function handleCreate() {
  if (!teamName.value.trim() || submitting.value) return
  submitting.value = true
  try {
    await playAPI.createTeam(teamName.value.trim())
    appStore.showSuccess(t('agentTeam.created'))
    teamName.value = ''
    await loadTeam()
  } catch (err: unknown) {
    const code = (err as { response?: { data?: { code?: string } } })?.response?.data?.code
    if (code === 'PLAY_TEAM_ALREADY_JOINED') {
      appStore.showInfo(t('agentTeam.alreadyJoined'))
      await loadTeam()
      return
    }
    appStore.showError(t('agentTeam.failed'))
  } finally {
    submitting.value = false
  }
}

async function handleJoin() {
  if (!inviteCode.value.trim() || submitting.value) return
  submitting.value = true
  try {
    await playAPI.joinTeam(inviteCode.value.trim())
    appStore.showSuccess(t('agentTeam.joined'))
    inviteCode.value = ''
    await loadTeam()
  } catch (err: unknown) {
    const code = (err as { response?: { data?: { code?: string } } })?.response?.data?.code
    if (code === 'PLAY_TEAM_NOT_FOUND') {
      appStore.showError(t('agentTeam.notFound'))
      return
    }
    if (code === 'PLAY_TEAM_ALREADY_JOINED') {
      appStore.showInfo(t('agentTeam.alreadyJoined'))
      await loadTeam()
      return
    }
    appStore.showError(t('agentTeam.failed'))
  } finally {
    submitting.value = false
  }
}

async function copyCombinedInviteLink() {
  const code = teamMe.value?.team?.invite_code
  if (!code) return
  await copyToClipboard(code, t('agentTeam.linkCopied'))
}

async function handleLeave() {
  if (submitting.value || !window.confirm(t('agentTeam.leaveConfirm'))) return
  submitting.value = true
  try {
    await playAPI.leaveTeam()
    appStore.showSuccess(t('agentTeam.left'))
    await loadTeam()
  } catch {
    appStore.showError(t('agentTeam.failed'))
  } finally {
    submitting.value = false
  }
}

async function handleTransfer(userId: number) {
  if (submitting.value || !window.confirm(t('agentTeam.transferConfirm'))) return
  submitting.value = true
  try {
    await playAPI.transferTeam(userId)
    appStore.showSuccess(t('agentTeam.transferred'))
    await loadTeam()
  } catch {
    appStore.showError(t('agentTeam.failed'))
  } finally {
    submitting.value = false
  }
}

async function handleRemove(userId: number) {
  if (submitting.value || !window.confirm(t('agentTeam.removeConfirm'))) return
  submitting.value = true
  try {
    await playAPI.removeTeamMember(userId)
    appStore.showSuccess(t('agentTeam.removed'))
    await loadTeam()
  } catch {
    appStore.showError(t('agentTeam.failed'))
  } finally {
    submitting.value = false
  }
}

onMounted(loadTeam)
</script>

<template>
  <div class="play-page">
    <header class="public-page-header">
      <PublicPlayBackLink />
      <PublicPageToolbar />
    </header>

    <main class="play-main agent-team-main">
      <p class="play-eyebrow">{{ t('play.agentTeam.eyebrow') }}</p>
      <h1 class="play-title">{{ t('play.agentTeam.title') }}</h1>
      <p class="play-subtitle">{{ t('play.agentTeam.subtitle') }}</p>
      <p class="play-intro">{{ t('play.agentTeam.intro') }}</p>

      <div v-if="loading" class="play-note">{{ t('models.loading') }}</div>
      <div v-else-if="!teamMe?.enabled" class="play-note">{{ t('agentTeam.disabled') }}</div>
      <div v-else-if="!authStore.isAuthenticated" class="play-actions">
        <router-link to="/register" class="play-btn play-btn-primary">{{ t('play.agentTeam.ctaGuest') }}</router-link>
      </div>
      <div v-else-if="teamMe.team" class="play-section">
        <div class="agent-team-header">
          <div>
            <p class="agent-team-kicker">Agent Team</p>
            <h2 class="play-section-title">{{ teamMe.team.name }}</h2>
          </div>
          <span class="agent-pill">{{ teamMe.team.member_count }} {{ t('agentTeam.membersUnit') }}</span>
        </div>

        <div class="agent-dashboard">
          <section class="agent-team-score-panel">
            <p class="agent-panel-label">{{ t('agentTeam.teamRecord') }}</p>
            <div class="agent-team-pool">${{ formatMoney(teamMe.team.estimated_pool) }}</div>
            <p>{{ t('agentTeam.rewardRuleDetail') }}</p>
            <div class="agent-tier-track">
              <span
                v-for="tier in currentRewardTiers"
                :key="`${tier.threshold}-${tier.rate}`"
                :class="{ active: tierReached(tier.threshold) }"
              >
                ${{ formatMoney(tier.threshold) }} · {{ (Number(tier.rate) * 100).toFixed(0) }}%
                <small>{{ tierReached(tier.threshold) ? t('agentTeam.tierReached') : t('agentTeam.tierLocked') }}</small>
              </span>
            </div>
          </section>

          <section class="agent-next-tier-panel">
            <p class="agent-panel-label">{{ t('agentTeam.nextTierTitle') }}</p>
            <strong v-if="nextThreshold > 0">{{ t('agentTeam.moreToNextTier', { amount: nextTierGap.toFixed(2) }) }}</strong>
            <strong v-else>{{ t('agentTeam.currentTier', { rate: rewardRatePct.toFixed(0) }) }}</strong>
            <span v-if="nextThreshold > 0">
              {{ t('agentTeam.nextTier', { amount: nextTierGap.toFixed(2), threshold: nextThreshold.toFixed(2) }) }}
            </span>
            <span>{{ t('agentTeam.rewardRule', { cap: formatMoney(teamMe.team.reward_cap) }) }}</span>
          </section>
        </div>

        <section v-if="isCaptain" class="agent-invite-card">
          <p class="agent-panel-label">{{ t('agentTeam.inviteCodeLabel') }}</p>
          <div class="agent-invite-row">
            <code>{{ teamMe.team.invite_code }}</code>
            <button type="button" class="play-btn play-btn-secondary" @click="copyCombinedInviteLink">
              {{ t('agentTeam.copyInviteCode') }}
            </button>
          </div>
        </section>

        <section class="play-section">
          <h3 class="play-section-title">{{ t('agentTeam.contributionsTitle') }}</h3>
          <p v-if="teamSpend <= 0" class="play-intro text-sm">
            {{ t('agentTeam.memberUsageEmpty') }}
          </p>
          <div v-else class="agent-member-board">
            <article
              v-for="(member, index) in sortedMembers"
              :key="member.user_id"
              class="agent-member-card"
              :class="[`tone-${toneForIndex(index)}`, { current: member.user_id === authStore.user?.id }]"
            >
              <div class="agent-member-main">
                <span class="agent-rank-number">#{{ index + 1 }}</span>
                <PlayUserAvatar :name="member.display_name" :avatar-url="member.avatar_url" />
                <span v-if="member.user_id === teamMe.team.captain_id" class="agent-captain-badge">
                  {{ t('agentTeam.captainBadge') }}
                </span>
              </div>
              <div class="agent-member-metrics">
                <strong>${{ formatMoney(member.spend) }}</strong>
                <span>{{ t('agentTeam.memberTokens', { tokens: formatTokens(member.token_sum) }) }} · {{ member.spend_pct ?? 0 }}%</span>
                <div class="agent-member-bar">
                  <span :style="{ width: `${member.spend_pct ?? 0}%` }" />
                </div>
                <div v-if="isCaptain && member.user_id !== authStore.user?.id" class="agent-member-actions">
                  <button type="button" class="play-btn play-btn-secondary" :disabled="submitting" @click="handleTransfer(member.user_id)">
                    {{ t('agentTeam.transfer') }}
                  </button>
                  <button type="button" class="play-btn play-btn-secondary" :disabled="submitting" @click="handleRemove(member.user_id)">
                    {{ t('agentTeam.remove') }}
                  </button>
                </div>
              </div>
            </article>
          </div>
        </section>

        <div class="flex justify-end">
          <button type="button" class="play-btn play-btn-secondary" :disabled="submitting" @click="handleLeave">
            {{ t('agentTeam.leave') }}
          </button>
        </div>

        <section class="play-section">
          <h3 class="play-section-title">{{ t('agentTeam.settlementHistory') }}</h3>
          <p v-if="settlements.length === 0" class="play-intro">{{ t('agentTeam.noSettlements') }}</p>
          <div v-else class="agent-settlement-list">
            <article v-for="record in settlements" :key="record.settlement.id" class="agent-settlement-card">
              <div class="agent-settlement-head">
                <strong>{{ record.settlement.period_start.slice(0, 7) }}</strong>
                <span class="agent-status-pill" :class="`status-${record.settlement.status}`">
                  {{ settlementStatusLabel(record.settlement.status) }}
                </span>
                <span>${{ formatMoney(record.settlement.pool_amount) }}</span>
              </div>
              <div class="agent-allocation-list">
                <span v-for="allocation in record.allocations" :key="allocation.id">
                  {{ memberDisplayName(allocation.user_id) }} ${{ formatMoney(allocation.reward_amount) }} · {{ payoutStatusLabel(allocation.payout_status) }}
                </span>
              </div>
            </article>
          </div>
        </section>
      </div>
      <div v-else class="play-section space-y-6">
        <div>
          <label class="mb-2 block text-sm font-medium">{{ t('agentTeam.createLabel') }}</label>
          <div class="flex gap-2">
            <input v-model="teamName" type="text" class="input flex-1" :placeholder="t('agentTeam.createPlaceholder')" />
            <button type="button" class="play-btn play-btn-primary" :disabled="submitting" @click="handleCreate">
              {{ t('agentTeam.createButton') }}
            </button>
          </div>
        </div>
        <div>
          <label class="mb-2 block text-sm font-medium">{{ t('agentTeam.joinLabel') }}</label>
          <div class="flex gap-2">
            <input v-model="inviteCode" type="text" class="input flex-1" :placeholder="t('agentTeam.joinPlaceholder')" />
            <button type="button" class="play-btn play-btn-secondary" :disabled="submitting" @click="handleJoin">
              {{ t('agentTeam.joinButton') }}
            </button>
          </div>
        </div>
      </div>
    </main>

    <SupportFloatingCard />
  </div>
</template>

<style scoped>
.agent-team-main {
  max-width: 1120px;
}

.agent-team-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  margin-bottom: 18px;
}

.agent-team-kicker,
.agent-panel-label {
  margin: 0 0 8px;
  font-family: 'IBM Plex Mono', monospace;
  font-size: 11px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--ink-3);
}

.agent-pill,
.agent-status-pill,
.agent-captain-badge {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  border: 1px solid var(--line);
  border-radius: 999px;
  background: var(--card);
  padding: 4px 10px;
  color: var(--ink-2);
  font-size: 12px;
  white-space: nowrap;
}

.agent-dashboard {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(280px, 0.9fr);
  gap: 14px;
  margin-bottom: 18px;
}

.agent-team-score-panel,
.agent-next-tier-panel,
.agent-invite-card,
.agent-member-card,
.agent-settlement-card {
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--card);
}

.agent-team-score-panel,
.agent-next-tier-panel,
.agent-invite-card {
  padding: 20px;
}

.agent-team-pool {
  font-family: 'JetBrains Mono', monospace;
  font-size: clamp(34px, 6vw, 54px);
  font-weight: 800;
  line-height: 1;
  color: var(--ink);
}

.agent-team-score-panel p,
.agent-next-tier-panel span {
  color: var(--ink-2);
  font-size: 14px;
  line-height: 1.7;
}

.agent-tier-track {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  margin-top: 16px;
}

.agent-tier-track span {
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 10px;
  color: var(--ink-2);
  font-size: 12px;
}

.agent-tier-track span.active {
  border-color: rgba(31, 122, 91, 0.45);
  background: #edf7f1;
  color: #1f6d52;
}

.dark .agent-tier-track span.active {
  background: rgba(31, 122, 91, 0.18);
  color: #9be0c2;
}

.agent-tier-track small {
  display: block;
  margin-top: 4px;
}

.agent-next-tier-panel strong {
  display: block;
  margin-bottom: 10px;
  font-size: 24px;
}

.agent-invite-card {
  margin-bottom: 26px;
}

.agent-invite-row {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.agent-invite-row code {
  flex: 1;
  min-width: 12rem;
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 11px 12px;
  color: var(--ink);
}

.agent-member-board,
.agent-settlement-list {
  display: grid;
  gap: 10px;
}

.agent-member-card {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(180px, 0.45fr);
  align-items: center;
  gap: 14px;
  padding: 12px 14px;
}

.agent-member-card.current {
  box-shadow: inset 3px 0 0 #1f7a5b;
}

.agent-member-main {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10px;
}

.agent-rank-number {
  font-family: 'JetBrains Mono', monospace;
  font-weight: 800;
}

.agent-member-metrics {
  display: grid;
  gap: 5px;
  text-align: right;
}

.agent-member-metrics strong {
  font-family: 'JetBrains Mono', monospace;
}

.agent-member-metrics span,
.agent-allocation-list {
  color: var(--ink-2);
  font-size: 13px;
}

.agent-member-bar {
  height: 8px;
  overflow: hidden;
  border-radius: 999px;
  background: rgba(120, 113, 108, 0.16);
}

.agent-member-bar span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: #1f7a5b;
}

.agent-member-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}

.tone-gold {
  border-color: rgba(184, 135, 25, 0.55);
  background: #fff4c7;
}

.tone-silver {
  border-color: rgba(109, 119, 136, 0.55);
  background: #eef2f6;
}

.tone-bronze {
  border-color: rgba(154, 91, 36, 0.55);
  background: #f8e1cc;
}

.dark .tone-gold {
  background: rgba(184, 135, 25, 0.18);
}

.dark .tone-silver {
  background: rgba(109, 119, 136, 0.22);
}

.dark .tone-bronze {
  background: rgba(154, 91, 36, 0.22);
}

.agent-settlement-card {
  padding: 14px;
}

.agent-settlement-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.status-processing,
.status-pending {
  border-color: rgba(184, 135, 25, 0.45);
  color: #9a6a10;
  background: #fff4c7;
}

.status-completed {
  border-color: rgba(31, 122, 91, 0.45);
  color: #1f7a5b;
  background: #edf7f1;
}

.status-partial,
.status-failed {
  border-color: rgba(154, 91, 36, 0.45);
  color: #9a5b24;
  background: #f8e1cc;
}

.agent-allocation-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
}

.agent-allocation-list span {
  border-radius: 999px;
  background: rgba(120, 113, 108, 0.12);
  padding: 6px 9px;
}

@media (max-width: 820px) {
  .agent-team-header {
    flex-direction: column;
  }

  .agent-dashboard,
  .agent-tier-track {
    grid-template-columns: 1fr;
  }

  .agent-member-card {
    grid-template-columns: 1fr;
    align-items: start;
  }

  .agent-member-metrics {
    text-align: left;
  }

  .agent-member-actions {
    justify-content: flex-start;
    flex-wrap: wrap;
  }
}
</style>
