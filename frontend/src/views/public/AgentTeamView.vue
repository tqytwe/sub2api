<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import AuthenticatedPlayShell from '@/components/layout/AuthenticatedPlayShell.vue'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import PublicPlayBackLink from '@/components/common/PublicPlayBackLink.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import PlayUserAvatar from '@/components/play/PlayUserAvatar.vue'
import Icon from '@/components/icons/Icon.vue'
import playAPI, {
  type PlayTeamDirectory,
  type PlayTeamDirectoryEntry,
  type PlayTeamJoinApplication,
  type PlayTeamLeaderboard,
  type PlayTeamLeaderboardEntry,
  type PlayTeamMe,
  type PlayTeamPublicLeaderboard,
  type PlayTeamPublicLeaderboardEntry,
  type PlayTeamRewardShowcase,
  type PlayTeamSeason,
  type PlayTeamSeasonDetail,
  type PlayTeamSettlementHistoryRecord,
  type PlayUserTeamSettlementRecord,
} from '@/api/play'
import { useClipboard } from '@/composables/useClipboard'
import { localizedEnumOrUnknown } from '@/utils/localizedEnum'
import '@/styles/public-pages.css'

type CompetitionRow = PlayTeamPublicLeaderboardEntry | PlayTeamLeaderboardEntry

const { t, locale } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const loadFailed = ref(false)
const historyLoading = ref(false)
const historyLoaded = ref(false)
const historyFailed = ref(false)
const actionBusy = ref<string | null>(null)
const directory = ref<PlayTeamDirectory | null>(null)
const publicLeaderboard = ref<PlayTeamPublicLeaderboard | null>(null)
const authenticatedLeaderboard = ref<PlayTeamLeaderboard | null>(null)
const teamMe = ref<PlayTeamMe | null>(null)
const settlements = ref<PlayTeamSettlementHistoryRecord[]>([])
const myApplications = ref<PlayTeamJoinApplication[]>([])
const captainApplications = ref<PlayTeamJoinApplication[]>([])
const selectedApplicationTeam = ref<PlayTeamDirectoryEntry | null>(null)
const applicationMessage = ref('')
const inviteCode = ref('')
const teamName = ref('')
const captainInviteCode = ref('')
const recruiting = ref(true)
const recruitingKnown = ref(false)
const seasons = ref<PlayTeamSeason[]>([])
const selectedHistoryMonth = ref('')
const historyDetail = ref<PlayTeamSeasonDetail | null>(null)
const rewardShowcase = ref<PlayTeamRewardShowcase>({ winners: [] })

const isAuthenticated = computed(() => authStore.isAuthenticated)
const ownTeam = computed(() => teamMe.value?.team ?? null)
const isCaptain = computed(() => Boolean(ownTeam.value && authStore.user?.id === ownTeam.value.captain_id))
const competitionEnabled = computed(() => teamMe.value?.enabled !== false)
const leaderboard = computed<PlayTeamPublicLeaderboard | PlayTeamLeaderboard | null>(
  () => authenticatedLeaderboard.value ?? publicLeaderboard.value,
)
const leaderboardRows = computed<CompetitionRow[]>(() => leaderboard.value?.rows ?? [])
const ownLeaderboardRow = computed(() => leaderboardRows.value.find(isOwnTeamRow) ?? null)
const ownSettlementRows = computed(() => settlements.value
  .filter((record): record is PlayUserTeamSettlementRecord => 'settlement_id' in record)
  .slice(0, 3))
const pendingMyApplications = computed(() => myApplications.value.filter(application => application.status === 'pending'))
const historyRows = computed(() => historyDetail.value?.rows ?? [])
const selectedTeamName = computed(() => selectedApplicationTeam.value?.team_name ?? '')
const activeInviteCode = computed(() => captainInviteCode.value || (isCaptain.value ? ownTeam.value?.invite_code ?? '' : ''))

function formatMoney(value: string | number | undefined) {
  return new Intl.NumberFormat(locale.value, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(Number(value ?? 0))
}

function formatDateTime(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat(locale.value, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

function isOwnTeamRow(row: CompetitionRow) {
  return ('is_mine' in row && row.is_mine) || row.team_id === ownTeam.value?.id
}

function applicationStatusLabel(status: string) {
  return localizedEnumOrUnknown(t, `agentTeam.applicationStatus.${status}`)
}

function payoutStatusLabel(status: string) {
  return localizedEnumOrUnknown(t, `agentTeam.payout.${status}`)
}

function publicRecipientName(value?: string) {
  const email = value?.trim()
  if (!email || !email.includes('@')) return t('agentTeam.anonymous')
  const [local = '', domain = ''] = email.split('@')
  if (!local || !domain) return t('agentTeam.anonymous')
  return `${local.slice(0, 1)}***@${domain}`
}

function applicationFor(teamID: number) {
  return pendingMyApplications.value.find(application => application.team_id === teamID)
}

function setActionBusy(key: string) {
  if (actionBusy.value) return false
  actionBusy.value = key
  return true
}

function clearActionBusy() {
  actionBusy.value = null
}

async function loadCompetition() {
  loading.value = true
  loadFailed.value = false
  try {
    const [directoryData, publicBoardData, teamData] = await Promise.all([
      playAPI.getTeamDirectory(20),
      playAPI.getTeamPublicLeaderboard(50),
      isAuthenticated.value ? playAPI.getTeamMe() : Promise.resolve(null),
    ])
    directory.value = directoryData
    publicLeaderboard.value = publicBoardData
    teamMe.value = teamData
    authenticatedLeaderboard.value = null
    settlements.value = []
    myApplications.value = []
    captainApplications.value = []
    captainInviteCode.value = ''
    recruitingKnown.value = typeof teamData?.team?.is_recruiting === 'boolean'
    recruiting.value = teamData?.team?.is_recruiting ?? false

    if (!isAuthenticated.value || !teamData?.enabled) return

    if (teamData.team) {
      void loadMemberSupplementalData(teamData.team.id, authStore.user?.id === teamData.team.captain_id)
      return
    }

    void loadMyApplications()
  } catch {
    directory.value = null
    publicLeaderboard.value = null
    authenticatedLeaderboard.value = null
    teamMe.value = null
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

async function loadMyApplications() {
  try {
    myApplications.value = await playAPI.getTeamMyApplications()
  } catch {
    // Public competition remains usable if the private application history is unavailable.
  }
}

async function loadMemberSupplementalData(teamID: number, captain: boolean) {
  const loadPrivateLeaderboard = playAPI.getTeamLeaderboard()
    .then((privateBoard) => {
      if (ownTeam.value?.id === teamID) {
        authenticatedLeaderboard.value = privateBoard
      }
    })
    .catch(() => undefined)

  const loadPrivateSettlements = playAPI.getTeamSettlements()
    .then((settlementData) => {
      if (ownTeam.value?.id === teamID) {
        settlements.value = settlementData
      }
    })
    .catch(() => undefined)

  const loadCaptainQueue = captain
    ? playAPI.getTeamCaptainApplications()
      .then((applications) => {
        if (ownTeam.value?.id === teamID && isCaptain.value) {
          captainApplications.value = applications
        }
      })
      .catch(() => undefined)
    : Promise.resolve()

  await Promise.all([loadPrivateLeaderboard, loadPrivateSettlements, loadCaptainQueue])
}

function selectApplicationTeam(team: PlayTeamDirectoryEntry) {
  if (!team.accepting_applications || actionBusy.value) return
  selectedApplicationTeam.value = team
  applicationMessage.value = ''
}

async function submitApplication() {
  const team = selectedApplicationTeam.value
  if (!team || !setActionBusy('application')) return
  try {
    const application = await playAPI.applyToTeam(team.team_id, applicationMessage.value.trim())
    myApplications.value = [application, ...myApplications.value.filter(item => item.team_id !== team.team_id)]
    selectedApplicationTeam.value = null
    applicationMessage.value = ''
    appStore.showSuccess(t('agentTeam.applicationSubmitted'))
  } catch {
    appStore.showError(t('agentTeam.failed'))
  } finally {
    clearActionBusy()
  }
}

async function handleCreateTeam() {
  const name = teamName.value.trim()
  if (!name || !setActionBusy('create')) return
  try {
    await playAPI.createTeam(name)
    teamName.value = ''
    appStore.showSuccess(t('agentTeam.created'))
    await loadCompetition()
  } catch {
    appStore.showError(t('agentTeam.failed'))
  } finally {
    clearActionBusy()
  }
}

async function handleInviteJoin() {
  const code = inviteCode.value.trim()
  if (!code || !setActionBusy('join')) return
  try {
    await playAPI.joinTeam(code)
    inviteCode.value = ''
    appStore.showSuccess(t('agentTeam.joined'))
    await loadCompetition()
  } catch {
    appStore.showError(t('agentTeam.failed'))
  } finally {
    clearActionBusy()
  }
}

async function handleLeave() {
  if (!setActionBusy('leave')) return
  if (!window.confirm(t('agentTeam.leaveConfirm'))) {
    clearActionBusy()
    return
  }
  try {
    await playAPI.leaveTeam()
    appStore.showSuccess(t('agentTeam.left'))
    await loadCompetition()
  } catch {
    appStore.showError(t('agentTeam.failed'))
  } finally {
    clearActionBusy()
  }
}

async function decideApplication(applicationID: number, decision: 'approve' | 'reject') {
  if (!setActionBusy(`application-${applicationID}`)) return
  try {
    const updated = await playAPI.decideTeamApplication(applicationID, decision)
    captainApplications.value = captainApplications.value.map(application => application.id === applicationID ? updated : application)
    appStore.showSuccess(t(decision === 'approve' ? 'agentTeam.applicationApproved' : 'agentTeam.applicationRejected'))
  } catch {
    appStore.showError(t('agentTeam.failed'))
  } finally {
    clearActionBusy()
  }
}

async function rotateInvite() {
  if (!setActionBusy('rotate-invite')) return
  try {
    const invite = await playAPI.rotateTeamInvite()
    captainInviteCode.value = invite.invite_code
    appStore.showSuccess(t('agentTeam.inviteRotated'))
  } catch {
    appStore.showError(t('agentTeam.failed'))
  } finally {
    clearActionBusy()
  }
}

async function copyInvite() {
  if (!activeInviteCode.value) return
  await copyToClipboard(activeInviteCode.value, t('agentTeam.inviteCopied'))
}

async function toggleRecruiting() {
  if (!setActionBusy('recruiting')) return
  try {
    const result = await playAPI.setTeamRecruiting(!recruiting.value)
    recruiting.value = result.recruiting
    recruitingKnown.value = true
    appStore.showSuccess(t('agentTeam.recruitingUpdated'))
  } catch {
    appStore.showError(t('agentTeam.failed'))
  } finally {
    clearActionBusy()
  }
}

async function loadSeason(month: string) {
  if (!month) return
  historyLoading.value = true
  historyFailed.value = false
  try {
    historyDetail.value = await playAPI.getTeamSeason(month, 10)
    selectedHistoryMonth.value = month
  } catch {
    historyDetail.value = null
    historyFailed.value = true
  } finally {
    historyLoading.value = false
  }
}

async function loadHistory() {
  if (historyLoaded.value || historyLoading.value) return
  historyLoading.value = true
  historyFailed.value = false
  try {
    const [seasonData, showcaseData] = await Promise.all([
      playAPI.getTeamSeasons(),
      playAPI.getTeamRewardShowcase(),
    ])
    seasons.value = seasonData
    rewardShowcase.value = showcaseData
    const settledSeason = seasonData.find(season => season.status === 'settled') ?? seasonData[0]
    if (settledSeason) {
      selectedHistoryMonth.value = settledSeason.month
      historyDetail.value = await playAPI.getTeamSeason(settledSeason.month, 10)
    }
    historyLoaded.value = true
  } catch {
    historyFailed.value = true
  } finally {
    historyLoading.value = false
  }
}

onMounted(loadCompetition)
</script>

<template>
  <AuthenticatedPlayShell>
    <div class="play-page">
      <header v-if="!authStore.isAuthenticated" class="public-page-header">
        <PublicPlayBackLink />
        <PublicPageToolbar />
      </header>

      <main class="play-main team-competition-main">
        <div class="play-workspace" :aria-busy="loading">
          <section class="play-hero-panel">
            <div class="play-hero-grid">
              <div>
                <p class="play-eyebrow">{{ t('play.agentTeam.eyebrow') }}</p>
                <h1 class="play-title">{{ t('play.agentTeam.title') }}</h1>
                <p class="play-subtitle">{{ t('play.agentTeam.subtitle') }}</p>
                <p class="play-intro team-competition-intro">{{ t('agentTeam.competitionIntro') }}</p>
              </div>

              <section v-if="ownTeam" class="play-action-panel team-hero-own-card">
                <p class="team-competition-kicker">{{ t('agentTeam.ownTeam') }}</p>
                <strong class="team-hero-own-name">{{ ownTeam.name }}</strong>
                <span v-if="ownLeaderboardRow" class="team-hero-own-rank">
                  {{ t('agentTeam.rankValue', { rank: ownLeaderboardRow.rank }) }}
                </span>
                <span v-else class="team-hero-own-rank">{{ t('agentTeam.rankPending') }}</span>
                <p>{{ t('agentTeam.monthlyWindow') }}</p>
              </section>

              <section v-else class="play-action-panel team-hero-own-card">
                <p class="team-competition-kicker">{{ t('agentTeam.publicCompetition') }}</p>
                <strong class="team-hero-own-name">{{ t(isAuthenticated ? 'agentTeam.findTeam' : 'agentTeam.guestTitle') }}</strong>
                <p>{{ t(isAuthenticated ? 'agentTeam.findTeamHint' : 'agentTeam.guestHint') }}</p>
                <router-link
                  v-if="!isAuthenticated"
                  :to="{ path: '/register', query: { redirect: '/agent-team' } }"
                  class="play-btn play-btn-primary"
                >
                  <Icon name="userPlus" size="sm" aria-hidden="true" />
                  {{ t('agentTeam.guestRegister') }}
                </router-link>
              </section>
            </div>
          </section>

          <section v-if="loading" class="play-note" aria-live="polite">{{ t('models.loading') }}</section>
          <section v-else-if="loadFailed" class="play-note team-error-state" role="alert">
            <span>{{ t('agentTeam.loadFailed') }}</span>
            <button type="button" class="play-btn play-btn-secondary" @click="loadCompetition">
              <Icon name="refresh" size="sm" aria-hidden="true" />
              {{ t('agentTeam.retry') }}
            </button>
          </section>
          <section v-else-if="!competitionEnabled" class="play-note">{{ t('agentTeam.disabled') }}</section>

          <template v-else>
            <section class="play-content-panel team-competition-panel" data-testid="team-live-leaderboard">
              <div class="team-competition-toolbar">
                <div>
                  <p class="team-competition-kicker">{{ leaderboard?.month }}</p>
                  <h2 class="play-section-title">{{ t('agentTeam.liveLeaderboard') }}</h2>
                </div>
                <span class="team-competition-pill">
                  <Icon name="chartBar" size="sm" aria-hidden="true" />
                  {{ t('agentTeam.totalTeams', { count: leaderboard?.total_teams ?? 0 }) }}
                </span>
              </div>

              <p v-if="leaderboardRows.length === 0" class="team-empty-state">{{ t('agentTeam.noLeaderboard') }}</p>
              <div v-else class="team-leaderboard-list">
                <article
                  v-for="row in leaderboardRows"
                  :key="row.team_id"
                  :data-testid="`team-leaderboard-row-${row.team_id}`"
                  class="team-leaderboard-row"
                  :class="{ 'team-leaderboard-row--mine': isOwnTeamRow(row) }"
                >
                  <strong class="team-rank">#{{ row.rank }}</strong>
                  <div class="team-leaderboard-name">
                    <strong>{{ row.team_name }}</strong>
                    <span>{{ t('agentTeam.memberCapacity', { current: row.member_count, capacity: 30 }) }}</span>
                  </div>
                  <span class="team-leaderboard-pool">
                    {{ t('agentTeam.estimatedPool', { amount: formatMoney(row.estimated_pool) }) }}
                  </span>
                  <span class="team-leaderboard-spend">${{ formatMoney(row.monthly_spend) }}</span>
                  <span v-if="isOwnTeamRow(row)" class="team-own-marker">{{ t('agentTeam.myTeamMarker') }}</span>
                </article>
              </div>
              <p class="team-privacy-note">{{ t('agentTeam.noPersonalSpend') }}</p>
            </section>

            <section v-if="ownTeam" class="play-content-panel team-own-summary" data-testid="team-own-summary">
              <div class="team-competition-toolbar">
                <div>
                  <p class="team-competition-kicker">{{ t('agentTeam.ownTeam') }}</p>
                  <h2 class="play-section-title">{{ ownTeam.name }}</h2>
                </div>
                <button type="button" class="play-btn play-btn-secondary" :disabled="Boolean(actionBusy)" @click="handleLeave">
                  <Icon name="arrowRight" size="sm" aria-hidden="true" />
                  {{ t('agentTeam.leave') }}
                </button>
              </div>
              <div class="team-own-stat-grid">
                <div>
                  <span>{{ t('agentTeam.currentRank') }}</span>
                  <strong>{{ ownLeaderboardRow ? `#${ownLeaderboardRow.rank}` : t('agentTeam.rankPending') }}</strong>
                </div>
                <div>
                  <span>{{ t('agentTeam.estimatedPoolLabel') }}</span>
                  <strong>${{ formatMoney(ownLeaderboardRow?.estimated_pool ?? ownTeam.estimated_pool) }}</strong>
                </div>
                <div>
                  <span>{{ t('agentTeam.gapToPreviousLabel') }}</span>
                  <strong>
                    {{ ownLeaderboardRow && Number(ownLeaderboardRow.gap_to_previous) > 0 ? `$${formatMoney(ownLeaderboardRow.gap_to_previous)}` : t('agentTeam.leadingOrTied') }}
                  </strong>
                </div>
                <div>
                  <span>{{ t('agentTeam.membersLabel') }}</span>
                  <strong>{{ t('agentTeam.memberCapacity', { current: ownTeam.member_count, capacity: 30 }) }}</strong>
                </div>
              </div>
              <p class="team-privacy-note">{{ t('agentTeam.ownTeamRule') }}</p>
            </section>

            <section v-if="ownSettlementRows.length" class="play-content-panel team-private-settlement-panel">
              <div class="team-competition-toolbar">
                <div>
                  <p class="team-competition-kicker">{{ t('agentTeam.privateSettlementKicker') }}</p>
                  <h2 class="play-section-title">{{ t('agentTeam.privateSettlementTitle') }}</h2>
                </div>
              </div>
              <div class="team-private-settlement-list">
                <article v-for="record in ownSettlementRows" :key="record.settlement_id" class="team-private-settlement-row">
                  <strong>{{ record.settlement_month }}</strong>
                  <span class="team-settlement-status">{{ payoutStatusLabel(record.payout_status) }}</span>
                  <time v-if="record.paid_at">{{ formatDateTime(record.paid_at) }}</time>
                </article>
              </div>
            </section>

            <section v-if="isCaptain" class="play-content-panel team-captain-panel">
              <div class="team-competition-toolbar">
                <div>
                  <p class="team-competition-kicker">{{ t('agentTeam.captainControls') }}</p>
                  <h2 class="play-section-title">{{ t('agentTeam.captainTitle') }}</h2>
                </div>
              </div>
              <div class="team-captain-actions">
                <div class="team-captain-action">
                  <span>{{ t('agentTeam.recruitingLabel') }}</span>
                  <strong>{{ recruitingKnown ? t(recruiting ? 'agentTeam.recruitingOpen' : 'agentTeam.recruitingClosed') : t('agentTeam.recruitingUnknown') }}</strong>
                  <button type="button" class="play-btn play-btn-secondary" :disabled="Boolean(actionBusy) || !recruitingKnown" @click="toggleRecruiting">
                    <Icon name="users" size="sm" aria-hidden="true" />
                    {{ t(recruiting ? 'agentTeam.pauseRecruiting' : 'agentTeam.resumeRecruiting') }}
                  </button>
                </div>
                <div class="team-captain-action">
                  <span>{{ t('agentTeam.inviteCodeLabel') }}</span>
                  <code v-if="activeInviteCode">{{ activeInviteCode }}</code>
                  <strong v-else>{{ t('agentTeam.inviteUnavailable') }}</strong>
                  <div class="team-captain-button-row">
                    <button type="button" class="play-btn play-btn-secondary" :disabled="!activeInviteCode" @click="copyInvite">
                      <Icon name="clipboard" size="sm" aria-hidden="true" />
                      {{ t('agentTeam.copyInviteCode') }}
                    </button>
                    <button type="button" class="play-btn play-btn-secondary" data-testid="team-invite-rotate" :disabled="Boolean(actionBusy)" @click="rotateInvite">
                      <Icon name="refresh" size="sm" aria-hidden="true" />
                      {{ t('agentTeam.rotateInvite') }}
                    </button>
                  </div>
                </div>
              </div>

              <div class="team-captain-applications">
                <div class="team-competition-toolbar">
                  <h3 class="team-subsection-title">{{ t('agentTeam.pendingApplications') }}</h3>
                  <span class="team-competition-pill">{{ t('agentTeam.pendingCount', { count: captainApplications.filter(application => application.status === 'pending').length }) }}</span>
                </div>
                <p v-if="captainApplications.length === 0" class="team-empty-state">{{ t('agentTeam.noPendingApplications') }}</p>
                <div v-else class="team-captain-application-list">
                  <article v-for="application in captainApplications" :key="application.id" class="team-captain-application-row">
                    <div>
                      <strong>{{ applicationStatusLabel(application.status) }}</strong>
                      <span>{{ t('agentTeam.applicationRequestedAt', { time: formatDateTime(application.requested_at) }) }}</span>
                      <p v-if="application.message">{{ application.message }}</p>
                    </div>
                    <div v-if="application.status === 'pending'" class="team-captain-button-row">
                      <button type="button" class="play-btn play-btn-primary" :data-testid="`team-application-approve-${application.id}`" :disabled="Boolean(actionBusy)" @click="decideApplication(application.id, 'approve')">
                        <Icon name="check" size="sm" aria-hidden="true" />
                        {{ t('agentTeam.approve') }}
                      </button>
                      <button type="button" class="play-btn play-btn-secondary" :disabled="Boolean(actionBusy)" @click="decideApplication(application.id, 'reject')">
                        <Icon name="x" size="sm" aria-hidden="true" />
                        {{ t('agentTeam.reject') }}
                      </button>
                    </div>
                  </article>
                </div>
              </div>
            </section>

            <section v-if="isAuthenticated && !ownTeam" class="team-no-team-workspace">
              <section v-if="pendingMyApplications.length" class="play-content-panel team-application-status-panel" aria-live="polite">
                <p class="team-competition-kicker">{{ t('agentTeam.applicationSubmitted') }}</p>
                <h2 class="play-section-title">{{ t('agentTeam.applicationPending') }}</h2>
                <p>{{ t('agentTeam.applicationSla') }}</p>
                <p>{{ t('agentTeam.applicationExpires') }}</p>
              </section>

              <section v-if="selectedApplicationTeam" class="play-content-panel team-apply-panel">
                <p class="team-competition-kicker">{{ t('agentTeam.apply') }}</p>
                <h2 class="play-section-title">{{ selectedTeamName }}</h2>
                <label class="team-form-label" for="team-application-message">{{ t('agentTeam.applicationMessage') }}</label>
                <textarea id="team-application-message" v-model="applicationMessage" data-testid="team-application-message" class="team-textarea" maxlength="300" :placeholder="t('agentTeam.applicationMessagePlaceholder')" />
                <div class="team-captain-button-row">
                  <button type="button" class="play-btn play-btn-primary" data-testid="team-application-submit" :disabled="Boolean(actionBusy)" @click="submitApplication">
                    <Icon name="userPlus" size="sm" aria-hidden="true" />
                    {{ t('agentTeam.submitApplication') }}
                  </button>
                  <button type="button" class="play-btn play-btn-secondary" :disabled="Boolean(actionBusy)" @click="selectedApplicationTeam = null">
                    {{ t('agentTeam.cancel') }}
                  </button>
                </div>
              </section>

              <div class="play-two-column-grid">
                <section class="play-content-panel">
                  <p class="team-competition-kicker">{{ t('agentTeam.createLabel') }}</p>
                  <label class="team-form-label" for="team-name">{{ t('agentTeam.createLabel') }}</label>
                  <div class="team-inline-form">
                    <input id="team-name" v-model="teamName" class="team-input" :placeholder="t('agentTeam.createPlaceholder')" maxlength="64">
                    <button type="button" class="play-btn play-btn-secondary" :disabled="Boolean(actionBusy)" @click="handleCreateTeam">
                      <Icon name="plus" size="sm" aria-hidden="true" />
                      {{ t('agentTeam.createButton') }}
                    </button>
                  </div>
                </section>
                <section class="play-content-panel">
                  <p class="team-competition-kicker">{{ t('agentTeam.inviteJoin') }}</p>
                  <label class="team-form-label" for="team-invite-code">{{ t('agentTeam.joinLabel') }}</label>
                  <div class="team-inline-form">
                    <input id="team-invite-code" v-model="inviteCode" class="team-input" :placeholder="t('agentTeam.joinPlaceholder')">
                    <button type="button" class="play-btn play-btn-secondary" :disabled="Boolean(actionBusy)" @click="handleInviteJoin">
                      <Icon name="key" size="sm" aria-hidden="true" />
                      {{ t('agentTeam.joinButton') }}
                    </button>
                  </div>
                </section>
              </div>
            </section>

            <section class="play-content-panel team-directory-panel" data-testid="team-directory">
              <div class="team-competition-toolbar">
                <div>
                  <p class="team-competition-kicker">{{ directory?.month }}</p>
                  <h2 class="play-section-title">{{ t('agentTeam.teamDirectory') }}</h2>
                </div>
                <span class="team-directory-note">{{ t('agentTeam.directoryRule') }}</span>
              </div>
              <p v-if="!directory?.rows.length" class="team-empty-state">{{ t('agentTeam.noTeams') }}</p>
              <div v-else class="team-directory-list">
                <article v-for="team in directory.rows" :key="team.team_id" class="team-directory-row">
                  <div class="team-directory-name">
                    <strong>{{ team.team_name }}</strong>
                    <span>{{ t('agentTeam.memberCapacity', { current: team.member_count, capacity: team.member_capacity }) }}</span>
                  </div>
                  <span class="team-directory-pool">{{ t('agentTeam.estimatedPool', { amount: formatMoney(team.estimated_pool) }) }}</span>
                  <span class="team-directory-spend">${{ formatMoney(team.monthly_spend) }}</span>
                  <router-link
                    v-if="!isAuthenticated && team.accepting_applications"
                    :data-testid="`team-apply-${team.team_id}`"
                    :to="{ path: '/register', query: { redirect: '/agent-team' } }"
                    class="play-btn play-btn-secondary"
                  >
                    <Icon name="userPlus" size="sm" aria-hidden="true" />
                    {{ t('agentTeam.guestRegister') }}
                  </router-link>
                  <span v-else-if="applicationFor(team.team_id)" class="team-application-state">{{ t('agentTeam.applicationPending') }}</span>
                  <button
                    v-else
                    type="button"
                    :data-testid="`team-apply-${team.team_id}`"
                    class="play-btn play-btn-secondary"
                    :disabled="!team.accepting_applications || Boolean(actionBusy) || Boolean(ownTeam)"
                    @click="selectApplicationTeam(team)"
                  >
                    <Icon name="userPlus" size="sm" aria-hidden="true" />
                    {{ team.accepting_applications ? t('agentTeam.apply') : t('agentTeam.teamFullOrClosed') }}
                  </button>
                </article>
              </div>
            </section>

            <section class="play-content-panel team-history-panel">
              <div class="team-competition-toolbar">
                <div>
                  <p class="team-competition-kicker">{{ t('agentTeam.history') }}</p>
                  <h2 class="play-section-title">{{ t('agentTeam.historyTitle') }}</h2>
                </div>
                <button type="button" class="play-btn play-btn-secondary" data-testid="team-history-load" :disabled="historyLoading" @click="loadHistory">
                  <Icon name="clock" size="sm" aria-hidden="true" />
                  {{ historyLoaded ? t('agentTeam.historyLoaded') : t('agentTeam.viewHistory') }}
                </button>
              </div>

              <p v-if="!historyLoaded && !historyLoading" class="team-history-hint">{{ t('agentTeam.historyLazyHint') }}</p>
              <p v-else-if="historyLoading" class="team-empty-state">{{ t('models.loading') }}</p>
              <div v-else-if="historyFailed" class="team-error-state" role="alert">
                <span>{{ t('agentTeam.historyFailed') }}</span>
                <button type="button" class="play-btn play-btn-secondary" @click="historyLoaded = false; loadHistory()">
                  <Icon name="refresh" size="sm" aria-hidden="true" />
                  {{ t('agentTeam.retry') }}
                </button>
              </div>
              <template v-else>
                <label v-if="seasons.length > 1" class="team-history-select-label">
                  {{ t('agentTeam.historySeason') }}
                  <select v-model="selectedHistoryMonth" class="team-history-select" @change="loadSeason(selectedHistoryMonth)">
                    <option v-for="season in seasons" :key="season.id" :value="season.month">{{ season.month }}</option>
                  </select>
                </label>
                <div class="team-history-grid">
                  <section data-testid="team-history" class="team-history-ranking">
                    <h3 class="team-subsection-title">{{ t('agentTeam.historicalTopTen') }}</h3>
                    <p v-if="historyRows.length === 0" class="team-empty-state">{{ t('agentTeam.noHistory') }}</p>
                    <div v-else class="team-history-list">
                      <article v-for="row in historyRows" :key="`${historyDetail?.season.id}-${row.team_id}`" class="team-history-row">
                        <strong>#{{ row.rank }}</strong>
                        <span>{{ row.team_name }}</span>
                        <span>{{ t('agentTeam.memberCapacity', { current: row.member_count, capacity: 30 }) }}</span>
                        <strong>${{ formatMoney(row.paid_amount) }}</strong>
                      </article>
                    </div>
                  </section>
                  <section class="team-history-proof">
                    <h3 class="team-subsection-title">{{ t('agentTeam.publicProof') }}</h3>
                    <p v-if="rewardShowcase.winners.length === 0" class="team-empty-state">{{ t('agentTeam.noPublicProof') }}</p>
                    <div v-else class="team-proof-list">
                      <article v-for="winner in rewardShowcase.winners" :key="`${winner.settlement_month}-${winner.team_name}-${winner.paid_at}`" class="team-proof-row">
                        <PlayUserAvatar :name="publicRecipientName(winner.display_name)" :avatar-url="winner.avatar_url" />
                        <span>
                          <strong>{{ publicRecipientName(winner.display_name) }}</strong>
                          <small>{{ t('agentTeam.publicRewardContext', { month: winner.settlement_month, team: winner.team_name }) }}</small>
                        </span>
                        <strong class="team-proof-amount">${{ formatMoney(winner.amount) }}</strong>
                        <time v-if="winner.paid_at">{{ formatDateTime(winner.paid_at) }}</time>
                      </article>
                    </div>
                  </section>
                </div>
              </template>
            </section>
          </template>
        </div>
      </main>

      <SupportFloatingCard v-if="!authStore.isAuthenticated" />
    </div>
  </AuthenticatedPlayShell>
</template>

<style scoped>
.team-competition-main button:focus-visible,
.team-competition-main input:focus-visible,
.team-competition-main textarea:focus-visible,
.team-competition-main select:focus-visible,
.team-competition-main a:focus-visible {
  outline: 2px solid var(--border-focus);
  outline-offset: 2px;
}

.team-competition-intro {
  margin-bottom: 0;
}

.team-hero-own-card {
  display: grid;
  align-content: start;
  gap: 8px;
}

.team-competition-kicker {
  margin: 0;
  color: var(--ink-3);
  font-family: 'IBM Plex Mono', monospace;
  font-size: 11px;
  line-height: 18px;
  text-transform: uppercase;
}

.team-hero-own-name {
  font-size: 18px;
  line-height: 26px;
}

.team-hero-own-rank,
.team-competition-pill,
.team-own-marker,
.team-application-state,
.team-settlement-status {
  display: inline-flex;
  align-items: center;
  width: fit-content;
  min-height: 28px;
  border: 1px solid var(--border-default);
  border-radius: 999px;
  background: var(--surface-sunken);
  color: var(--text-secondary);
  padding: 4px 10px;
  font-size: 12px;
  line-height: 18px;
}

.team-hero-own-rank,
.team-own-marker {
  border-color: var(--border-focus);
  background: var(--action-primary-subtle);
  color: var(--action-primary-hover);
}

.team-hero-own-card p,
.team-history-hint,
.team-directory-note,
.team-privacy-note,
.team-empty-state,
.team-application-status-panel p {
  margin: 0;
  color: var(--ink-2);
  font-size: 13px;
  line-height: 20px;
}

.team-competition-panel,
.team-own-summary,
.team-private-settlement-panel,
.team-captain-panel,
.team-directory-panel,
.team-history-panel,
.team-application-status-panel,
.team-apply-panel {
  display: grid;
  gap: 16px;
}

.team-competition-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.team-competition-toolbar .play-section-title {
  margin: 4px 0 0;
}

.team-competition-pill {
  gap: 6px;
}

.team-leaderboard-list,
.team-directory-list,
.team-private-settlement-list,
.team-captain-application-list,
.team-history-list,
.team-proof-list {
  display: grid;
}

.team-leaderboard-row {
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr) minmax(160px, auto) minmax(108px, auto) auto;
  align-items: center;
  gap: 12px;
  min-height: 52px;
  border-top: 1px solid var(--line);
  padding: 10px 0;
}

.team-leaderboard-row:first-child,
.team-directory-row:first-child,
.team-private-settlement-row:first-child,
.team-captain-application-row:first-child,
.team-history-row:first-child,
.team-proof-row:first-child {
  border-top: 0;
}

.team-leaderboard-row--mine {
  box-shadow: inset 3px 0 0 var(--play-primary);
  background: var(--play-primary-soft);
  margin: 0 -12px;
  padding: 10px 12px;
}

.team-rank,
.team-leaderboard-spend,
.team-directory-spend,
.team-own-stat-grid strong,
.team-history-row strong {
  font-family: 'JetBrains Mono', ui-monospace, monospace;
  font-variant-numeric: tabular-nums;
}

.team-rank {
  color: var(--ink-2);
}

.team-leaderboard-name,
.team-directory-name {
  display: grid;
  min-width: 0;
  gap: 2px;
}

.team-leaderboard-name strong,
.team-directory-name strong,
.team-history-row span,
.team-proof-row strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.team-leaderboard-name span,
.team-directory-name span,
.team-leaderboard-pool,
.team-directory-pool,
.team-private-settlement-row time,
.team-captain-application-row span,
.team-proof-row small {
  color: var(--ink-2);
  font-size: 12px;
  line-height: 18px;
}

.team-leaderboard-pool,
.team-leaderboard-spend,
.team-directory-pool,
.team-directory-spend {
  text-align: right;
  white-space: nowrap;
}

.team-own-stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.team-own-stat-grid div {
  display: grid;
  gap: 4px;
  min-width: 0;
  border-top: 1px solid var(--line);
  padding-top: 10px;
}

.team-own-stat-grid span,
.team-captain-action > span,
.team-form-label,
.team-history-select-label {
  color: var(--ink-2);
  font-size: 13px;
  line-height: 20px;
}

.team-own-stat-grid strong {
  min-width: 0;
  font-size: 18px;
  line-height: 26px;
}

.team-private-settlement-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 12px;
  min-height: 44px;
  border-top: 1px solid var(--line);
  padding: 8px 0;
}

.team-settlement-status {
  border-color: var(--status-success-border);
  background: var(--status-success-surface);
  color: var(--status-success-text);
}

.team-captain-actions,
.team-history-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.team-captain-action {
  display: grid;
  gap: 8px;
  border-top: 1px solid var(--line);
  padding-top: 12px;
}

.team-captain-action code {
  overflow: hidden;
  color: var(--ink);
  font-family: 'JetBrains Mono', ui-monospace, monospace;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.team-captain-button-row,
.team-inline-form {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.team-captain-applications {
  display: grid;
  gap: 12px;
  border-top: 1px solid var(--line);
  padding-top: 16px;
}

.team-subsection-title {
  margin: 0;
  font-size: 16px;
  line-height: 24px;
}

.team-captain-application-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-top: 1px solid var(--line);
  padding: 12px 0;
}

.team-captain-application-row > div:first-child {
  display: grid;
  min-width: 0;
  gap: 4px;
}

.team-captain-application-row p {
  margin: 0;
  color: var(--ink-2);
  font-size: 13px;
  line-height: 20px;
  overflow-wrap: anywhere;
}

.team-no-team-workspace {
  display: grid;
  gap: 18px;
}

.team-form-label,
.team-history-select-label {
  display: grid;
  gap: 6px;
}

.team-input,
.team-textarea,
.team-history-select {
  width: 100%;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--card);
  color: var(--ink);
  font: inherit;
}

.team-input,
.team-history-select {
  min-height: 40px;
  padding: 8px 10px;
}

.team-textarea {
  min-height: 88px;
  resize: vertical;
  padding: 10px;
}

.team-inline-form .team-input {
  flex: 1 1 180px;
}

.team-directory-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(160px, auto) minmax(108px, auto) auto;
  align-items: center;
  gap: 12px;
  min-height: 56px;
  border-top: 1px solid var(--line);
  padding: 10px 0;
}

.team-history-panel {
  min-height: 112px;
}

.team-history-ranking,
.team-history-proof {
  min-width: 0;
}

.team-history-list,
.team-proof-list {
  margin-top: 10px;
}

.team-history-row {
  display: grid;
  grid-template-columns: 38px minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 10px;
  min-height: 44px;
  border-top: 1px solid var(--line);
  padding: 8px 0;
}

.team-history-row span:nth-child(3) {
  color: var(--ink-2);
  font-size: 12px;
}

.team-proof-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 10px;
  min-height: 48px;
  border-top: 1px solid var(--line);
  padding: 8px 0;
}

.team-proof-row > span {
  display: grid;
  min-width: 0;
  gap: 2px;
}

.team-error-state {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

@media (width <= 900px) {
  .team-own-stat-grid,
  .team-captain-actions,
  .team-history-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .team-leaderboard-row {
    grid-template-columns: 36px minmax(0, 1fr) minmax(100px, auto) auto;
  }

  .team-leaderboard-pool {
    display: none;
  }

  .team-directory-row {
    grid-template-columns: minmax(0, 1fr) minmax(100px, auto) auto;
  }

  .team-directory-pool {
    display: none;
  }
}

@media (width <= 640px) {
  .team-own-stat-grid,
  .team-captain-actions,
  .team-history-grid {
    grid-template-columns: 1fr;
  }

  .team-leaderboard-row {
    grid-template-columns: 32px minmax(0, 1fr) auto;
    gap: 8px;
  }

  .team-leaderboard-spend,
  .team-own-marker {
    grid-column: 2 / -1;
    justify-self: start;
    text-align: left;
  }

  .team-directory-row {
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 8px;
  }

  .team-directory-spend,
  .team-directory-row .play-btn,
  .team-directory-row .team-application-state {
    grid-column: 1 / -1;
    justify-self: start;
    text-align: left;
  }

  .team-private-settlement-row,
  .team-history-row {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .team-proof-row {
    grid-template-columns: auto minmax(0, 1fr) auto;
  }

  .team-private-settlement-row time,
  .team-proof-row time,
  .team-history-row span:nth-child(3),
  .team-history-row strong:last-child {
    grid-column: 1 / -1;
  }
}
</style>
