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

    <main class="play-main">
      <p class="play-eyebrow">{{ t('play.agentTeam.eyebrow') }}</p>
      <h1 class="play-title">{{ t('play.agentTeam.title') }}</h1>
      <p class="play-subtitle">{{ t('play.agentTeam.subtitle') }}</p>
      <p class="play-intro">{{ t('play.agentTeam.intro') }}</p>

      <div v-if="loading" class="play-note">{{ t('models.loading') }}</div>
      <div v-else-if="!teamMe?.enabled" class="play-note">{{ t('agentTeam.disabled') }}</div>
      <div v-else-if="!authStore.isAuthenticated" class="play-actions">
        <router-link to="/register" class="play-btn play-btn-primary">{{ t('play.agentTeam.ctaGuest') }}</router-link>
      </div>
      <div v-else-if="teamMe.team" class="play-section space-y-4">
        <h2 class="play-section-title">{{ teamMe.team.name }}</h2>
        <p class="play-intro">{{ t('agentTeam.inviteCode', { code: teamMe.team.invite_code }) }}</p>
        <p class="play-intro">{{ t('agentTeam.spendStats', { members: teamMe.team.member_count, spend: teamSpend.toFixed(2) }) }}</p>
        <p class="play-intro">
          {{ rewardRatePct > 0
            ? t('agentTeam.reachedTier', { rate: rewardRatePct.toFixed(0), pool: Number(teamMe.team.estimated_pool).toFixed(2) })
            : t('agentTeam.noTier') }}
        </p>
        <p v-if="nextThreshold > 0" class="play-intro">
          {{ t('agentTeam.nextTier', { amount: Math.max(0, nextThreshold - teamSpend).toFixed(2), threshold: nextThreshold.toFixed(2) }) }}
        </p>
        <p class="play-intro">{{ t('agentTeam.rewardRule', { cap: Number(teamMe.team.reward_cap).toFixed(2) }) }}</p>
        <div v-if="isCaptain" class="play-section space-y-2">
          <p class="text-sm font-medium">{{ t('agentTeam.inviteCodeLabel') }}</p>
          <div class="flex flex-wrap gap-2">
            <code class="flex-1 truncate rounded border px-3 py-2 text-sm">{{ teamMe.team.invite_code }}</code>
            <button type="button" class="play-btn play-btn-secondary" @click="copyCombinedInviteLink">
              {{ t('agentTeam.copyInviteCode') }}
            </button>
          </div>
        </div>
        <ul class="play-rules space-y-3">
          <li class="text-sm font-medium">{{ t('agentTeam.contributionsTitle') }}</li>
          <li v-if="teamSpend <= 0" class="play-intro text-sm">
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
                {{ t('agentTeam.memberSpend', {
                  spend: Number(member.spend).toFixed(2),
                  pct: member.spend_pct ?? 0,
                }) }}
              </div>
              <div class="text-xs opacity-70">{{ t('agentTeam.memberTokens', { tokens: member.token_sum.toLocaleString() }) }}</div>
              <div class="text-xs opacity-70">
                {{ t('agentTeam.memberEstimatedReward', { reward: Number(member.estimated_reward || 0).toFixed(2) }) }}
              </div>
              <div
                v-if="teamSpend > 0"
                class="mt-1 h-1.5 w-28 overflow-hidden rounded bg-black/10 dark:bg-white/10"
              >
                <div
                  class="h-full rounded bg-[var(--play-ink)]"
                  :style="{ width: `${member.spend_pct ?? 0}%` }"
                />
              </div>
              <div v-if="isCaptain && member.user_id !== authStore.user?.id" class="mt-2 flex justify-end gap-2">
                <button type="button" class="play-btn play-btn-secondary" :disabled="submitting" @click="handleTransfer(member.user_id)">
                  {{ t('agentTeam.transfer') }}
                </button>
                <button type="button" class="play-btn play-btn-secondary" :disabled="submitting" @click="handleRemove(member.user_id)">
                  {{ t('agentTeam.remove') }}
                </button>
              </div>
            </div>
          </li>
        </ul>
        <div class="flex justify-end">
          <button type="button" class="play-btn play-btn-secondary" :disabled="submitting" @click="handleLeave">
            {{ t('agentTeam.leave') }}
          </button>
        </div>
        <section class="space-y-3">
          <h3 class="play-section-title">{{ t('agentTeam.settlementHistory') }}</h3>
          <p v-if="settlements.length === 0" class="play-intro">{{ t('agentTeam.noSettlements') }}</p>
          <div v-for="record in settlements" :key="record.settlement.id" class="rounded border border-[var(--play-line)] p-3">
            <div class="flex flex-wrap justify-between gap-2 text-sm">
              <strong>{{ record.settlement.period_start.slice(0, 7) }}</strong>
              <span>{{ t('agentTeam.poolStatus', { pool: Number(record.settlement.pool_amount).toFixed(2), status: record.settlement.status }) }}</span>
            </div>
            <ul class="mt-2 space-y-1 text-sm">
              <li v-for="allocation in record.allocations" :key="allocation.id" class="flex justify-between gap-3">
                <span class="min-w-0 truncate">{{ allocation.display_name || `#${allocation.user_id}` }}</span>
                <span class="text-right">
                  {{ t('agentTeam.allocationLine', {
                    contribution: Number(allocation.contribution).toFixed(2),
                    ratio: (Number(allocation.ratio) * 100).toFixed(1),
                    reward: Number(allocation.reward_amount).toFixed(2),
                    status: allocation.payout_status,
                  }) }}
                </span>
              </li>
            </ul>
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
