<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import userAPI from '@/api/user'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import PublicPlayBackLink from '@/components/common/PublicPlayBackLink.vue'
import PlayUserAvatar from '@/components/play/PlayUserAvatar.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import playAPI, {
  type PlayActivityItem,
  type PlayTeamDiscovery,
  type PlayTeamJoinRequest,
  type PlayTeamMe,
} from '@/api/play'
import { useClipboard } from '@/composables/useClipboard'
import { buildRegisterInviteLink } from '@/utils/oauthAffiliate'
import '@/styles/public-pages.css'
import { trackGrowthEvent } from '@/utils/growthAnalytics'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const submitting = ref(false)
const teamMe = ref<PlayTeamMe | null>(null)
const teamName = ref('')
const inviteCode = ref('')
const affCode = ref('')
const discoveries = ref<PlayTeamDiscovery[]>([])
const joinRequests = ref<PlayTeamJoinRequest[]>([])
const activity = ref<PlayActivityItem[]>([])

const isCaptain = computed(
  () => teamMe.value?.team && authStore.user?.id === teamMe.value.team.captain_id,
)
const combinedInviteLink = computed(() => {
  if (!affCode.value || !teamMe.value?.team?.invite_code) return ''
  return buildRegisterInviteLink(affCode.value, teamMe.value.team.invite_code)
})
const affiliateInfo = computed(() => teamMe.value?.team?.affiliate)
const weeklyPct = computed(() => {
  const weekly = teamMe.value?.team?.weekly
  if (!weekly) return 0
  const tokenPct = weekly.token_target > 0 ? weekly.token_sum / weekly.token_target : 0
  const requestPct = weekly.request_target > 0 ? weekly.request_count / weekly.request_target : 0
  return Math.min(100, Math.round(Math.min(tokenPct, requestPct) * 100))
})

async function loadTeam() {
  if (!authStore.isAuthenticated) {
    try {
      discoveries.value = await playAPI.discoverTeams(12)
      teamMe.value = { enabled: true }
    } catch {
      teamMe.value = { enabled: false }
      discoveries.value = []
    }
    loading.value = false
    return
  }
  loading.value = true
  try {
    teamMe.value = await playAPI.getTeamMe()
    if (authStore.isAuthenticated && teamMe.value?.team && authStore.user?.id === teamMe.value.team.captain_id) {
      try {
        const detail = await userAPI.getAffiliateDetail()
        affCode.value = detail.aff_code
      } catch {
        affCode.value = ''
      }
    }
    if (teamMe.value?.team) {
      activity.value = await playAPI.getTeamActivity(12)
      joinRequests.value = isCaptain.value ? await playAPI.getTeamJoinRequests() : []
      discoveries.value = []
    } else {
      discoveries.value = await playAPI.discoverTeams(12)
      activity.value = []
      joinRequests.value = []
    }
  } catch {
    teamMe.value = null
  } finally {
    loading.value = false
  }
}

async function requestJoin(teamId: number) {
  if (submitting.value) return
  submitting.value = true
  try {
    await playAPI.requestTeamJoin(teamId)
    appStore.showSuccess(t('agentTeam.requestSent'))
  } catch {
    appStore.showError(t('agentTeam.failed'))
  } finally {
    submitting.value = false
  }
}

async function reviewRequest(requestId: number, approve: boolean) {
  try {
    await playAPI.reviewTeamJoinRequest(requestId, approve)
    await loadTeam()
  } catch {
    appStore.showError(t('agentTeam.failed'))
  }
}

async function leaveCurrentTeam() {
  try {
    await playAPI.leaveTeam()
    await loadTeam()
  } catch (err: unknown) {
    const code = (err as { response?: { data?: { code?: string } } })?.response?.data?.code
    appStore.showError(code === 'PLAY_TEAM_TRANSFER_REQUIRED' ? t('agentTeam.transferRequired') : t('agentTeam.failed'))
  }
}

async function transferCaptain(userId: number) {
  try {
    await playAPI.transferTeamCaptain(userId)
    await loadTeam()
  } catch {
    appStore.showError(t('agentTeam.failed'))
  }
}

async function removeMember(userId: number) {
  try {
    await playAPI.removeTeamMember(userId)
    await loadTeam()
  } catch {
    appStore.showError(t('agentTeam.failed'))
  }
}

function activityLabel(item: PlayActivityItem) {
  return t(`agentTeam.activity.${item.event_type}`, { actor: item.actor })
}

async function handleCreate() {
  if (!teamName.value.trim() || submitting.value) return
  submitting.value = true
  try {
    await playAPI.createTeam(teamName.value.trim())
	trackGrowthEvent('team_created')
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
	trackGrowthEvent('team_joined', { source: 'invite_code' })
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
  if (!combinedInviteLink.value) return
  await copyToClipboard(combinedInviteLink.value, t('agentTeam.linkCopied'))
}

onMounted(loadTeam)
</script>

<template>
  <div class="play-page">
    <header class="public-page-header">
      <PublicPlayBackLink />
      <PublicPageToolbar />
    </header>

    <main class="play-main">
      <p class="play-eyebrow">{{ t('play.agentTeam.eyebrow') }}</p>
      <h1 class="play-title">{{ t('play.agentTeam.title') }}</h1>
      <p class="play-subtitle">{{ t('play.agentTeam.subtitle') }}</p>
      <p class="play-intro">{{ t('play.agentTeam.intro') }}</p>

      <div v-if="loading" class="play-note">{{ t('models.loading') }}</div>
      <div v-else-if="!teamMe?.enabled" class="play-note">{{ t('agentTeam.disabled') }}</div>
      <template v-else-if="!authStore.isAuthenticated">
        <div class="play-actions">
          <router-link to="/register" class="play-btn play-btn-primary">{{ t('play.agentTeam.ctaGuest') }}</router-link>
        </div>
        <section v-if="discoveries.length" class="play-section space-y-3">
          <h2 class="play-section-title">{{ t('agentTeam.discoverTitle') }}</h2>
          <div v-for="team in discoveries" :key="team.id" class="flex items-center justify-between gap-3 border-b border-[var(--play-line)] py-3">
            <div>
              <div class="font-medium">{{ team.name }} · Lv.{{ team.level }}</div>
              <div class="text-sm opacity-60">{{ t('agentTeam.discoveryStats', { members: team.member_count, max: team.max_members, requests: team.request_count.toLocaleString() }) }}</div>
            </div>
          </div>
        </section>
      </template>
      <div v-else-if="teamMe.team" class="play-section space-y-4">
        <h2 class="play-section-title">{{ teamMe.team.name }}</h2>
        <p class="play-intro">{{ t('agentTeam.inviteCode', { code: teamMe.team.invite_code }) }}</p>
        <p class="play-intro">
          {{ t('agentTeam.stats', { members: teamMe.team.member_count, tokens: teamMe.team.token_sum.toLocaleString() }) }}
        </p>
        <div class="grid border-y border-[var(--play-line)] sm:grid-cols-3">
          <div class="py-3 sm:border-r sm:border-[var(--play-line)] sm:pr-3">
            <div class="text-xs opacity-60">{{ t('agentTeam.levelLabel') }}</div>
            <div class="mt-1 font-semibold">Lv.{{ teamMe.team.level }}</div>
          </div>
          <div class="py-3 sm:border-r sm:border-[var(--play-line)] sm:px-3">
            <div class="text-xs opacity-60">{{ t('agentTeam.requestsLabel') }}</div>
            <div class="mt-1 font-semibold tabular-nums">{{ teamMe.team.request_count.toLocaleString() }}</div>
          </div>
          <div class="py-3 sm:pl-3">
            <div class="text-xs opacity-60">{{ t('agentTeam.activeDaysLabel') }}</div>
            <div class="mt-1 font-semibold tabular-nums">{{ teamMe.team.active_days }}</div>
          </div>
        </div>
        <section v-if="teamMe.team.weekly" class="space-y-2 border-y border-[var(--play-line)] py-4">
          <div class="flex items-center justify-between gap-3">
            <h3 class="font-medium">{{ t('agentTeam.weeklyMission') }}</h3>
            <span class="text-sm tabular-nums">{{ weeklyPct }}%</span>
          </div>
          <div class="h-2 overflow-hidden bg-black/10 dark:bg-white/10">
            <div class="h-full bg-[var(--play-ink)]" :style="{ width: `${weeklyPct}%` }" />
          </div>
          <p class="text-sm opacity-70">
            {{ t('agentTeam.weeklyProgress', {
              tokens: teamMe.team.weekly.token_sum.toLocaleString(),
              tokenTarget: teamMe.team.weekly.token_target.toLocaleString(),
              requests: teamMe.team.weekly.request_count.toLocaleString(),
              requestTarget: teamMe.team.weekly.request_target.toLocaleString(),
            }) }}
          </p>
        </section>
        <p v-if="affiliateInfo?.enabled" class="play-intro">
          {{ t('agentTeam.affiliateProgress', {
            tokens: teamMe.team.token_sum.toLocaleString(),
            threshold: affiliateInfo.token_threshold.toLocaleString(),
          }) }}
        </p>
        <p v-if="affiliateInfo?.enabled && !affiliateInfo.milestone_reached" class="play-intro">
          {{ t('agentTeam.affiliateRemaining', {
            remaining: (affiliateInfo.tokens_to_milestone ?? 0).toLocaleString(),
            bonus: (affiliateInfo.captain_bonus ?? 0).toFixed(2),
          }) }}
        </p>
        <p v-else-if="affiliateInfo?.milestone_reached" class="play-intro">
          {{ t('agentTeam.affiliateReached', { bonus: (affiliateInfo.captain_bonus ?? 0).toFixed(2) }) }}
        </p>
        <div v-if="isCaptain && combinedInviteLink" class="play-section space-y-2">
          <p class="text-sm font-medium">{{ t('agentTeam.combinedInviteLink') }}</p>
          <div class="flex flex-wrap gap-2">
            <code class="flex-1 truncate rounded-lg border px-3 py-2 text-sm">{{ combinedInviteLink }}</code>
            <button type="button" class="play-btn play-btn-secondary" @click="copyCombinedInviteLink">
              {{ t('agentTeam.copyInviteLink') }}
            </button>
          </div>
        </div>
        <ul class="play-rules space-y-3">
          <li class="text-sm font-medium">{{ t('agentTeam.contributionsTitle') }}</li>
          <li v-if="teamMe.team.token_sum <= 0" class="play-intro text-sm">
            {{ t('agentTeam.memberUsageEmpty') }}
          </li>
          <li
            v-for="member in teamMe.team.members"
            :key="member.user_id"
            class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-[var(--play-line)] px-3 py-2"
          >
            <div class="flex min-w-0 items-center gap-2">
              <PlayUserAvatar :name="member.display_name" :avatar-url="member.avatar_url" />
              <span v-if="member.user_id === teamMe.team.captain_id" class="text-xs opacity-70">
                {{ t('agentTeam.captainBadge') }}
              </span>
            </div>
              <div class="text-right text-sm">
              <div class="font-medium tabular-nums">
                {{ t('agentTeam.memberUsage', {
                  tokens: (member.token_sum ?? 0).toLocaleString(),
                  pct: member.token_pct ?? 0,
                }) }}
              </div>
              <div
                v-if="teamMe.team.token_sum > 0"
                class="mt-1 h-1.5 w-28 overflow-hidden rounded-full bg-black/10 dark:bg-white/10"
              >
                <div
                  class="h-full rounded-full bg-[var(--play-ink)]"
                  :style="{ width: `${member.token_pct ?? 0}%` }"
                />
              </div>
              <div v-if="isCaptain && member.user_id !== teamMe.team.captain_id" class="flex gap-2">
                <button type="button" class="text-xs underline" @click="transferCaptain(member.user_id)">{{ t('agentTeam.transferCaptain') }}</button>
                <button type="button" class="text-xs underline" @click="removeMember(member.user_id)">{{ t('agentTeam.removeMember') }}</button>
              </div>
            </div>
          </li>
        </ul>
        <section v-if="isCaptain && joinRequests.length" class="space-y-3 border-t border-[var(--play-line)] pt-4">
          <h3 class="font-medium">{{ t('agentTeam.joinRequestsTitle') }}</h3>
          <div v-for="request in joinRequests" :key="request.id" class="flex items-center justify-between gap-3 text-sm">
            <span>{{ request.display_name }}</span>
            <div class="flex gap-2">
              <button type="button" class="play-btn play-btn-primary" @click="reviewRequest(request.id, true)">{{ t('agentTeam.approve') }}</button>
              <button type="button" class="play-btn play-btn-secondary" @click="reviewRequest(request.id, false)">{{ t('agentTeam.reject') }}</button>
            </div>
          </div>
        </section>
        <section class="space-y-2 border-t border-[var(--play-line)] pt-4">
          <h3 class="font-medium">{{ t('agentTeam.activityTitle') }}</h3>
          <p v-if="!activity.length" class="text-sm opacity-60">{{ t('agentTeam.activityEmpty') }}</p>
          <ul v-else class="space-y-2 text-sm">
            <li v-for="item in activity" :key="item.id">{{ activityLabel(item) }}</li>
          </ul>
        </section>
        <button type="button" class="play-btn play-btn-secondary" @click="leaveCurrentTeam">{{ t('agentTeam.leaveTeam') }}</button>
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
        <section v-if="discoveries.length" class="space-y-3 border-t border-[var(--play-line)] pt-5">
          <h2 class="play-section-title">{{ t('agentTeam.discoverTitle') }}</h2>
          <div v-for="team in discoveries" :key="team.id" class="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--play-line)] py-3">
            <div>
              <div class="font-medium">{{ team.name }} · Lv.{{ team.level }}</div>
              <div class="text-sm opacity-60">{{ t('agentTeam.discoveryStats', { members: team.member_count, max: team.max_members, requests: team.request_count.toLocaleString() }) }}</div>
            </div>
            <button type="button" class="play-btn play-btn-secondary" :disabled="submitting" @click="requestJoin(team.id)">{{ t('agentTeam.requestJoin') }}</button>
          </div>
        </section>
      </div>
    </main>

    <SupportFloatingCard />
  </div>
</template>
