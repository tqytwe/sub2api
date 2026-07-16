<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { extractApiErrorCode } from '@/utils/apiError'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import PublicPlayBackLink from '@/components/common/PublicPlayBackLink.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import playAPI, {
  type PlayBlindboxOpenResult,
  type PlayBlindboxPool,
  type PlayBlindboxPoolResponse,
  type PlayBlindboxRecentWin,
  type PlayBlindboxStatus,
} from '@/api/play'
import '@/styles/public-pages.css'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const loading = ref(true)
const opening = ref(false)
const statusLoadFailed = ref(false)
const status = ref<PlayBlindboxStatus | null>(null)
const publicPool = ref<PlayBlindboxPoolResponse | null>(null)
const lastResult = ref<PlayBlindboxOpenResult | null>(null)
const recentWins = ref<PlayBlindboxRecentWin[]>([])
const recentWinsFailed = ref(false)
let statusRequestID = 0

function isValidPool(pool: PlayBlindboxPool | null | undefined): pool is PlayBlindboxPool {
  if (
    !pool ||
    !pool.version.trim() ||
    !Number.isFinite(pool.cost) ||
    pool.cost <= 0 ||
    !Number.isFinite(pool.rtp_cap) ||
    pool.rtp_cap <= 0 ||
    pool.rtp_cap > 1 ||
    !Array.isArray(pool.tiers) ||
    pool.tiers.length === 0 ||
    pool.tiers.length > 32
  ) {
    return false
  }

  const totalWeight = pool.tiers.reduce((total, tier) => total + tier.weight, 0)
  const tiersValid = pool.tiers.every(
    (tier) =>
      Number.isFinite(tier.amount) &&
      tier.amount >= 0 &&
      Number.isInteger(tier.weight) &&
      tier.weight > 0,
  )
  if (!tiersValid || totalWeight !== 10_000) return false

  const expectedReward = pool.tiers.reduce(
    (total, tier) => total + tier.amount * (tier.weight / 10_000),
    0,
  )
  return expectedReward <= pool.cost * pool.rtp_cap + Number.EPSILON
}

const featureEnabled = computed(() =>
  authStore.isAuthenticated ? status.value?.enabled === true : publicPool.value?.enabled === true,
)

const prizePool = computed<PlayBlindboxPool | null>(() => {
  if (!featureEnabled.value) return null
  const pool = authStore.isAuthenticated ? status.value?.pool : publicPool.value?.pool
  return isValidPool(pool) ? pool : null
})

const canOpen = computed(
  () =>
    authStore.isAuthenticated &&
    status.value?.enabled &&
    status.value.can_open &&
    prizePool.value !== null &&
    !opening.value,
)

function formatProbability(weight: number): string {
  const percentage = weight / 100
  return `${percentage.toFixed(2).replace(/\.?0+$/, '')}%`
}

function formatPrizeAmount(amount: number): string {
  return amount.toLocaleString('en-US', {
    useGrouping: false,
    minimumFractionDigits: 2,
    maximumFractionDigits: 8,
  })
}

function formatWinWhen(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

async function loadRecentWins() {
  recentWinsFailed.value = false
  try {
    recentWins.value = await playAPI.getBlindboxRecentWins()
  } catch {
    recentWins.value = []
    recentWinsFailed.value = true
  }
}

async function loadStatus() {
  const requestID = ++statusRequestID
  const authenticated = authStore.isAuthenticated
  loading.value = true
  statusLoadFailed.value = false
  if (authenticated) {
    publicPool.value = null
  } else {
    status.value = null
  }
  try {
    if (authenticated) {
      const nextStatus = await playAPI.getBlindboxStatus()
      if (requestID !== statusRequestID || !authStore.isAuthenticated) return
      status.value = nextStatus
      publicPool.value = null
    } else {
      const nextPool = await playAPI.getBlindboxPool()
      if (requestID !== statusRequestID || authStore.isAuthenticated) return
      publicPool.value = nextPool
      status.value = null
    }
  } catch {
    if (requestID !== statusRequestID) return
    status.value = null
    publicPool.value = null
    statusLoadFailed.value = true
  } finally {
    if (requestID === statusRequestID) {
      loading.value = false
    }
  }
}

async function handleOpen() {
  if (!canOpen.value) return
  opening.value = true
  try {
    lastResult.value = await playAPI.openBlindbox(`blindbox-${Date.now()}`)
    appStore.showSuccess(
      t('blindbox.success', {
        reward: lastResult.value.reward_amount.toFixed(2),
        net: lastResult.value.net_amount.toFixed(2),
      }),
    )
    await authStore.refreshUser()
    await Promise.all([loadStatus(), loadRecentWins()])
  } catch (err: unknown) {
    const code = extractApiErrorCode(err)
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

watch(
  () => authStore.isAuthenticated,
  () => {
    void loadStatus()
  },
)
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
        <div v-else-if="statusLoadFailed" class="play-note">{{ t('blindbox.unavailable') }}</div>
        <div v-else-if="!status?.enabled" class="play-note">{{ t('blindbox.disabled') }}</div>
        <template v-else-if="!prizePool">
          <div class="play-note">{{ t('blindbox.unavailable') }}</div>
          <button type="button" class="play-btn play-btn-primary" disabled>
            {{ t('blindbox.openButton') }}
          </button>
        </template>
        <template v-else>
          <p class="play-intro">
            {{ t('blindbox.costHint', { cost: status.cost_amount.toFixed(2), opens: status.opens_today, limit: status.daily_limit }) }}
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
          v-if="!authStore.isAuthenticated && featureEnabled"
          to="/register"
          class="play-btn play-btn-primary"
        >
          {{ t('play.blindbox.ctaGuest') }}
        </router-link>
      </div>

      <div class="play-section play-prize-section">
        <h2 class="play-section-title">{{ t('blindbox.prizePoolTitle') }}</h2>
        <p class="play-note">{{ t('blindbox.prizePoolNote') }}</p>
        <p v-if="!loading && statusLoadFailed" class="play-note">{{ t('blindbox.unavailable') }}</p>
        <p v-else-if="!loading && !featureEnabled" class="play-note">{{ t('blindbox.disabled') }}</p>
        <p v-else-if="!loading && !prizePool" class="play-note">{{ t('blindbox.unavailable') }}</p>
        <ul v-else-if="prizePool" class="play-prize-grid">
          <li
            v-for="(tier, index) in prizePool.tiers"
            :key="`${prizePool.version}-${index}`"
            class="play-prize-tier"
          >
            <span class="play-prize-amount">${{ formatPrizeAmount(tier.amount) }}</span>
            <span class="play-prize-rate">{{ formatProbability(tier.weight) }}</span>
          </li>
        </ul>
      </div>

      <div class="play-section">
        <h2 class="play-section-title">{{ t('blindbox.recentWinsTitle') }}</h2>
        <p v-if="recentWinsFailed" class="play-note">{{ t('blindbox.recentWinsUnavailable') }}</p>
        <p v-else-if="recentWins.length === 0" class="play-note">{{ t('blindbox.recentWinsPlaceholder') }}</p>
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
