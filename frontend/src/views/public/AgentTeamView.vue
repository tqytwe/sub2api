<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import userAPI from '@/api/user'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import playAPI, { type PlayTeamMe } from '@/api/play'
import { useClipboard } from '@/composables/useClipboard'
import { buildRegisterInviteLink } from '@/utils/oauthAffiliate'
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
const affCode = ref('')

const isCaptain = computed(
  () => teamMe.value?.team && authStore.user?.id === teamMe.value.team.captain_id,
)
const combinedInviteLink = computed(() => {
  if (!affCode.value || !teamMe.value?.team?.invite_code) return ''
  return buildRegisterInviteLink(affCode.value, teamMe.value.team.invite_code)
})
const affiliateInfo = computed(() => teamMe.value?.team?.affiliate)

async function loadTeam() {
  if (!authStore.isAuthenticated) {
    teamMe.value = { enabled: false }
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
  if (!combinedInviteLink.value) return
  await copyToClipboard(combinedInviteLink.value, t('agentTeam.linkCopied'))
}

onMounted(loadTeam)
</script>

<template>
  <div class="play-page">
    <header class="public-page-header">
      <router-link to="/home" class="back-link">{{ t('play.backHome') }}</router-link>
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
        <router-link to="/login" class="play-btn play-btn-primary">{{ t('play.agentTeam.ctaGuest') }}</router-link>
      </div>
      <div v-else-if="teamMe.team" class="play-section space-y-4">
        <h2 class="play-section-title">{{ teamMe.team.name }}</h2>
        <p class="play-intro">{{ t('agentTeam.inviteCode', { code: teamMe.team.invite_code }) }}</p>
        <p class="play-intro">
          {{ t('agentTeam.stats', { members: teamMe.team.member_count, tokens: teamMe.team.token_sum.toLocaleString() }) }}
        </p>
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
        <ul class="play-rules">
          <li v-for="member in teamMe.team.members" :key="member.user_id">
            {{ member.display_name }}
          </li>
        </ul>
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
