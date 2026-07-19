<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { extractApiErrorCode } from '@/utils/apiError'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import PublicPlayBackLink from '@/components/common/PublicPlayBackLink.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import RewardCelebrationOverlay from '@/components/play/RewardCelebrationOverlay.vue'
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
const celebrationOpen = ref(false)
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
  const pool = authStore.isAuthenticated
    ? status.value?.current_pool ?? status.value?.pool
    : publicPool.value?.current_pool ?? publicPool.value?.pool
  return isValidPool(pool) ? pool : null
})

const vipPool = computed(() => status.value?.vip_tier ?? publicPool.value?.vip_tier ?? null)
const nextPool = computed(() => status.value?.next_pool ?? publicPool.value?.next_pool ?? null)
const currentExpectedReward = computed(() =>
  status.value?.expected_reward ??
  publicPool.value?.expected_reward ??
  (prizePool.value ? expectedReward(prizePool.value) : 0),
)
const poolVersion = computed(() => status.value?.pool_version ?? publicPool.value?.pool_version ?? prizePool.value?.version ?? '')
const currentRTPCap = computed(() => status.value?.rtp_cap ?? publicPool.value?.rtp_cap ?? prizePool.value?.rtp_cap ?? 0)
const nextExpectedReward = computed(() => status.value?.next_expected_reward ?? publicPool.value?.next_expected_reward ?? (nextPool.value ? expectedReward(nextPool.value) : 0))

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

function formatMoney(amount: number | undefined): string {
  return (amount ?? 0).toFixed(2)
}

function expectedReward(pool: PlayBlindboxPool): number {
  return pool.tiers.reduce((total, tier) => total + tier.amount * (tier.weight / 10_000), 0)
}

function formatWinWhen(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

const celebrationVariant = computed(() => {
  if (!lastResult.value) return 'standard'
  return lastResult.value.reward_amount >= Math.max(lastResult.value.cost_amount * 2, 3)
    ? 'jackpot'
    : 'standard'
})

const celebrationDetails = computed(() => {
  if (!lastResult.value) return []
  return [
    lastResult.value.pool_version,
    t('blindbox.celebrationNet', {
      cost: formatMoney(lastResult.value.cost_amount),
      net: formatMoney(lastResult.value.net_amount),
    }),
    t('blindbox.celebrationPool', {
      pool: lastResult.value.pool_version,
      opens: lastResult.value.opens_today,
      limit: status.value?.effective_limit ?? status.value?.daily_limit ?? 0,
    }),
  ]
})

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
    celebrationOpen.value = true
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
      <div class="play-workspace">
        <section class="play-hero-panel">
          <div class="play-hero-grid">
            <div>
              <p class="play-eyebrow">{{ t('play.blindbox.eyebrow') }}</p>
              <h1 class="play-title">{{ t('play.blindbox.title') }}</h1>
              <p class="play-subtitle">{{ t('play.blindbox.subtitle') }}</p>
              <p class="play-intro">{{ t('play.blindbox.intro') }}</p>
            </div>

            <div class="play-action-panel">
              <h2 class="play-section-title">{{ t('blindbox.prizePoolTitle') }}</h2>
              <div v-if="authStore.isAuthenticated" class="space-y-4">
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
                  <div v-if="vipPool && prizePool" class="blindbox-vip-pool">
                    <div>
                      <span class="blindbox-vip-label">{{ vipPool.label }}</span>
                      <strong>{{ t('blindbox.vipPoolTitle') }}</strong>
                    </div>
                    <p>{{ t('blindbox.currentPool', { pool: poolVersion }) }}</p>
                    <code>{{ poolVersion }}</code>
                    <p>{{ t('blindbox.expectedReward', { amount: formatMoney(currentExpectedReward), rtp: Math.round(currentRTPCap * 100) }) }}</p>
                    <p v-if="nextPool && vipPool.amount_to_next">
                      {{ t('blindbox.nextPoolHint', {
                        amount: formatMoney(vipPool.amount_to_next),
                        label: vipPool.next_label ?? `V${vipPool.next_tier}`,
                        pool: nextPool.version,
                        reward: formatMoney(nextExpectedReward),
                      }) }}
                    </p>
                  </div>
                  <button
                    type="button"
                    class="play-btn play-btn-primary w-full"
                    :class="{ 'blindbox-opening': opening }"
                    :disabled="!canOpen"
                    @click="handleOpen"
                  >
                    {{ opening ? t('blindbox.opening') : t('blindbox.openButton') }}
                  </button>
                  <p v-if="lastResult" class="play-note">
                    {{ t('blindbox.lastResult', { reward: lastResult.reward_amount.toFixed(2), net: lastResult.net_amount.toFixed(2) }) }}
                  </p>
                </template>
              </div>

              <div v-else class="play-actions">
                <router-link
                  v-if="featureEnabled"
                  to="/register"
                  class="play-btn play-btn-primary"
                >
                  {{ t('play.blindbox.ctaGuest') }}
                </router-link>
              </div>
            </div>
          </div>
        </section>

        <section class="play-four-stat-grid" aria-label="blindbox status">
          <div class="play-mini-stat">
            <span class="play-mini-label">{{ t('blindbox.prizePoolTitle') }}</span>
            <span class="play-mini-value">{{ prizePool?.tiers.length ?? 0 }}</span>
          </div>
          <div class="play-mini-stat">
            <span class="play-mini-label">{{ t('blindbox.openButton') }}</span>
            <span class="play-mini-value">{{ status?.opens_today ?? 0 }}/{{ status?.daily_limit ?? 0 }}</span>
          </div>
          <div class="play-mini-stat">
            <span class="play-mini-label">{{ t('blindbox.recentWinsTitle') }}</span>
            <span class="play-mini-value">{{ recentWins.length }}</span>
          </div>
        </section>

        <div class="play-two-column-grid">
          <section class="play-content-panel play-prize-section">
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
          </section>

          <section class="play-content-panel">
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
          </section>
        </div>
      </div>
    </main>

    <RewardCelebrationOverlay
      :open="celebrationOpen && !!lastResult"
      :title="t('blindbox.celebrationTitle')"
      :amount="`$${formatMoney(lastResult?.reward_amount)}`"
      :subtitle="t('blindbox.celebrationSubtitle')"
      :details="celebrationDetails"
      :vip-label="lastResult?.vip_tier?.label ?? vipPool?.label ?? ''"
      :color-key="lastResult?.vip_tier?.color_key ?? vipPool?.color_key ?? 'neutral'"
      :variant="celebrationVariant"
      :primary-label="t('blindbox.openAgain')"
      :secondary-label="t('blindbox.viewPool')"
      @close="celebrationOpen = false"
      @primary="() => { celebrationOpen = false; void handleOpen() }"
      @secondary="celebrationOpen = false"
    />
    <SupportFloatingCard />
  </div>
</template>

<style scoped>
.blindbox-vip-pool {
  display: grid;
  gap: 7px;
  border: 1px solid rgba(31, 122, 91, 0.22);
  border-radius: 8px;
  background: rgba(31, 122, 91, 0.08);
  padding: 12px;
  color: var(--text);
}

.blindbox-vip-pool div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.blindbox-vip-pool p {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
}

.blindbox-vip-label {
  display: inline-flex;
  min-height: 24px;
  align-items: center;
  border-radius: 999px;
  background: #1f7a5b;
  color: #fff;
  padding: 2px 9px;
  font-size: 12px;
  font-weight: 800;
}

.blindbox-opening {
  animation: blindbox-button-shake 0.42s ease-in-out infinite;
}

@keyframes blindbox-button-shake {
  0%, 100% {
    transform: translateX(0);
  }
  25% {
    transform: translateX(-2px);
  }
  75% {
    transform: translateX(2px);
  }
}

@media (prefers-reduced-motion: reduce) {
  .blindbox-opening {
    animation: none;
  }
}
</style>
