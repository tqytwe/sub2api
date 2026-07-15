<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import PublicPlayBackLink from '@/components/common/PublicPlayBackLink.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import playAPI, { type PlayBlindboxOpenResult, type PlayBlindboxRecentWin, type PlayBlindboxStatus } from '@/api/play'
import '@/styles/public-pages.css'
import { trackGrowthEvent } from '@/utils/growthAnalytics'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const loading = ref(true)
const opening = ref(false)
const status = ref<PlayBlindboxStatus | null>(null)
const lastResult = ref<PlayBlindboxOpenResult | null>(null)
const recentWins = ref<PlayBlindboxRecentWin[]>([])

const canOpen = computed(
  () =>
    authStore.isAuthenticated &&
    status.value?.enabled &&
    status.value.can_open &&
    !opening.value,
)

const prizeTiers = computed(() => (status.value?.pool?.tiers || []).map((tier) => ({
  amount: tier.amount.toFixed(2),
  rate: `${(tier.weight / 100).toFixed(tier.weight % 100 === 0 ? 0 : 2)}%`,
})))

function formatWinWhen(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

async function loadRecentWins() {
  try {
    recentWins.value = await playAPI.getBlindboxRecentWins()
  } catch {
    recentWins.value = []
  }
}

async function loadStatus() {
  if (!authStore.isAuthenticated) {
    try {
      status.value = await playAPI.getBlindboxPool()
    } catch {
      status.value = null
    }
    loading.value = false
    return
  }
  loading.value = true
  try {
    status.value = await playAPI.getBlindboxStatus()
  } catch {
    status.value = null
  } finally {
    loading.value = false
  }
}

async function handleOpen() {
  if (!canOpen.value) return
  opening.value = true
  try {
    lastResult.value = await playAPI.openBlindbox(`blindbox-${Date.now()}`)
	trackGrowthEvent('blindbox_opened', { pool_version: lastResult.value.pool_version, source: lastResult.value.open_source, reward: lastResult.value.reward_amount })
    appStore.showSuccess(
      t('blindbox.success', {
        reward: lastResult.value.reward_amount.toFixed(2),
        net: lastResult.value.net_amount.toFixed(2),
      }),
    )
    await authStore.refreshUser()
    await Promise.all([loadStatus(), loadRecentWins()])
  } catch (err: unknown) {
    const code = (err as { response?: { data?: { code?: string } } })?.response?.data?.code
    if (code === 'INSUFFICIENT_BALANCE') {
      appStore.showError(t('blindbox.insufficientBalance'))
      return
    }
    if (code === 'PLAY_BLINDBOX_DAILY_LIMIT') {
      appStore.showInfo(t('blindbox.dailyLimit'))
      await loadStatus()
      return
    }
    appStore.showError(t('blindbox.failed'))
  } finally {
    opening.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadStatus(), loadRecentWins()])
})
</script>

<template>
  <div class="play-page">
    <header class="public-page-header">
      <PublicPlayBackLink />
      <PublicPageToolbar />
    </header>

    <main class="play-main">
      <p class="play-eyebrow">{{ t('play.blindbox.eyebrow') }}</p>
      <h1 class="play-title">{{ t('play.blindbox.title') }}</h1>
      <p class="play-subtitle">{{ t('play.blindbox.subtitle') }}</p>
      <p class="play-intro">{{ t('play.blindbox.intro') }}</p>

      <div v-if="authStore.isAuthenticated" class="play-section space-y-4">
        <div v-if="loading" class="play-note">{{ t('models.loading') }}</div>
        <div v-else-if="!status?.enabled" class="play-note">{{ t('blindbox.disabled') }}</div>
        <template v-else>
          <p class="play-intro">
            {{ t('blindbox.costHint', { cost: status.cost_amount.toFixed(2), opens: status.opens_today, limit: status.effective_limit || status.daily_limit }) }}
          </p>
          <p v-if="status.ticket_balance > 0" class="play-note">
            {{ t('blindbox.ticketHint', { count: status.ticket_balance }) }}
          </p>
          <p v-if="!status.paid_enabled || !status.region_enabled" class="play-note">
            {{ t('blindbox.paidDisabled') }}
          </p>
          <button type="button" class="play-btn play-btn-primary" :disabled="!canOpen" @click="handleOpen">
            {{ opening ? t('blindbox.opening') : t('blindbox.openButton') }}
          </button>
          <p v-if="lastResult" class="play-note">
            {{ t('blindbox.lastResult', { reward: lastResult.reward_amount.toFixed(2), net: lastResult.net_amount.toFixed(2) }) }}
          </p>
        </template>
      </div>

      <div class="play-actions">
        <router-link
          v-if="!authStore.isAuthenticated"
          to="/register"
          class="play-btn play-btn-primary"
        >
          {{ t('play.blindbox.ctaGuest') }}
        </router-link>
      </div>

      <div class="play-section play-prize-section">
        <h2 class="play-section-title">{{ t('blindbox.prizePoolTitle') }}</h2>
        <p class="play-note">{{ t('blindbox.prizePoolNote') }}</p>
        <ul class="play-prize-grid">
          <li v-for="tier in prizeTiers" :key="tier.amount" class="play-prize-tier">
            <span class="play-prize-amount">${{ tier.amount }}</span>
            <span class="play-prize-rate">{{ tier.rate }}</span>
          </li>
        </ul>
      </div>

      <div class="play-section">
        <h2 class="play-section-title">{{ t('blindbox.recentWinsTitle') }}</h2>
        <p v-if="recentWins.length === 0" class="play-note">{{ t('blindbox.recentWinsPlaceholder') }}</p>
        <ul v-else class="play-wins-list">
          <li v-for="(win, idx) in recentWins" :key="idx" class="play-win-item">
            <span class="play-win-user">{{ win.user }}</span>
            <span class="play-win-reward">+${{ win.reward.toFixed(2) }}</span>
            <span class="play-win-when">{{ formatWinWhen(win.when) }}</span>
          </li>
        </ul>
      </div>
    </main>

    <SupportFloatingCard />
  </div>
</template>
