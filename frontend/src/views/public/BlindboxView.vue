<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import AuthenticatedPlayShell from '@/components/layout/AuthenticatedPlayShell.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorCode } from '@/utils/apiError'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import PublicPlayBackLink from '@/components/common/PublicPlayBackLink.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import RewardCelebrationOverlay from '@/components/play/RewardCelebrationOverlay.vue'
import CouponRewardCard from '@/components/play/CouponRewardCard.vue'
import { formatCurrency, formatDateTime } from '@/utils/format'
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
const pendingOpenIdempotencyKey = ref<string | null>(null)
const pendingOpenStorageKey = ref<string | null>(null)
let statusRequestID = 0
let openRequestID = 0

const pendingOpenStoragePrefix = 'blindbox.pending-open:'
const maxIdempotencyKeyLength = 128

interface PendingBlindboxOpen {
  idempotencyKey: string
  storageKey: string | null
  userID: number | null
}

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

const couponPoolReady = computed(() => {
  const value = authStore.isAuthenticated
    ? status.value?.coupon_pool_ready
    : publicPool.value?.coupon_pool_ready
  return value !== false
})

const prizePool = computed<PlayBlindboxPool | null>(() => {
  if (!featureEnabled.value) return null
  const pool = authStore.isAuthenticated
    ? status.value?.current_pool ?? status.value?.pool
    : publicPool.value?.current_pool ?? publicPool.value?.pool
  return isValidPool(pool) ? pool : null
})

const couponPrizes = computed(() => {
  const prizes = authStore.isAuthenticated
    ? status.value?.coupon_prizes
    : publicPool.value?.coupon_prizes
  return Array.isArray(prizes) ? prizes.filter((prize) => prize.name.trim()) : []
})

const totalPrizeCount = computed(() => (prizePool.value?.tiers.length ?? 0) + couponPrizes.value.length)
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

const rewardSplit = computed(() => authStore.isAuthenticated
  ? { coupon: status.value?.coupon_weight_bp, balance: status.value?.balance_weight_bp }
  : { coupon: publicPool.value?.coupon_weight_bp, balance: publicPool.value?.balance_weight_bp })

const balanceBranchWeight = computed(() => {
  const balanceWeightBP = Number(rewardSplit.value.balance)
  return Number.isFinite(balanceWeightBP) && balanceWeightBP >= 0
    ? balanceWeightBP / 10_000
    : 0.4
})

// The configured tiers are the original balance pool. Display overall odds
// after the current published coupon/balance split so users see the real open odds.
const expectedCashReward = computed(() => currentExpectedReward.value * balanceBranchWeight.value)
const expectedCashRTPCap = computed(() => currentRTPCap.value * balanceBranchWeight.value)
const nextExpectedCashReward = computed(() => nextExpectedReward.value * balanceBranchWeight.value)

const canOpen = computed(
  () =>
    authStore.isAuthenticated &&
    status.value?.enabled &&
    couponPoolReady.value &&
    status.value.can_open &&
    prizePool.value !== null &&
    !opening.value,
)

function formatProbability(weight: number): string {
  const percentage = weight / 100
  return `${percentage.toFixed(2).replace(/\.?0+$/, '')}%`
}

function formatBalanceProbability(weight: number): string {
  return formatProbability(weight * balanceBranchWeight.value)
}

function formatPrizeAmount(amount: number): string {
  return amount.toLocaleString('en-US', {
    useGrouping: false,
    minimumFractionDigits: 2,
    maximumFractionDigits: 8,
  })
}

function couponPrizeTierLabel(tier: string): string {
  const key = `blindbox.couponTier.${tier}`
  const translated = t(key)
  return translated === key ? t('blindbox.couponTier.standard') : translated
}

function formatRecentWinReward(win: PlayBlindboxRecentWin): string {
  if (win.reward_type === 'coupon' && win.coupon_name) {
    return t('blindbox.recentCouponWin', { name: win.coupon_name })
  }
  return t('blindbox.recentBalanceWin', { amount: win.reward.toFixed(2) })
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

const hasCouponResult = computed(() => lastResult.value?.reward_type === 'coupon' && !!lastResult.value.coupon)
const couponResultPendingActivation = computed(() => {
  const validFrom = lastResult.value?.coupon?.valid_from
  const timestamp = validFrom ? Date.parse(validFrom) : Number.NaN
  return Number.isFinite(timestamp) && timestamp > Date.now()
})
const celebrationTitle = computed(() => hasCouponResult.value ? t('coupon.reward.blindboxTitle') : t('blindbox.celebrationTitle'))
const celebrationAmount = computed(() => hasCouponResult.value
  ? lastResult.value?.coupon?.name || ''
  : `$${formatMoney(lastResult.value?.reward_amount)}`)
const celebrationSubtitle = computed(() => hasCouponResult.value
  ? t('coupon.reward.issuedToWallet')
  : t('blindbox.celebrationSubtitle'))

const celebrationDetails = computed(() => {
  if (!lastResult.value) return []
  if (lastResult.value.reward_type === 'coupon' && lastResult.value.coupon) {
    const coupon = lastResult.value.coupon
    const details = [
      t('coupon.reward.expiresAt', { time: formatDateTime(coupon.expires_at) }),
      t('coupon.reward.minimum', { amount: formatCurrency(coupon.minimum_order_amount, coupon.currency) }),
      lastResult.value.coupon_pool_version || lastResult.value.pool_version,
    ]
    if (couponResultPendingActivation.value) {
      details.unshift(t('coupon.reward.availableAt', { time: formatDateTime(coupon.valid_from) }))
    }
    return details
  }
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

function currentBlindboxUserID(): number | null {
  const userID = authStore.user?.id
  if (typeof userID !== 'number' || !Number.isInteger(userID) || userID <= 0) return null
  return userID
}

function pendingOpenStorageKeyForCurrentUser(): string | null {
  const userID = currentBlindboxUserID()
  if (!userID) return null
  return `${pendingOpenStoragePrefix}${userID}`
}

function loadPersistedOpenIdempotencyKey(storageKey: string): string | null {
  if (typeof window === 'undefined') return null
  try {
    const idempotencyKey = window.sessionStorage.getItem(storageKey)?.trim() ?? ''
    if (
      idempotencyKey &&
      idempotencyKey.length <= maxIdempotencyKeyLength &&
      [...idempotencyKey].every((character) => {
        const code = character.charCodeAt(0)
        return code >= 33 && code <= 126
      })
    ) {
      return idempotencyKey
    }
    if (idempotencyKey) window.sessionStorage.removeItem(storageKey)
  } catch {
    // Storage can be unavailable in privacy-restricted browser contexts.
  }
  return null
}

function persistOpenIdempotencyKey(storageKey: string | null, idempotencyKey: string) {
  if (!storageKey || typeof window === 'undefined') return
  try {
    window.sessionStorage.setItem(storageKey, idempotencyKey)
  } catch {
    // The in-memory key still protects retries in the current page.
  }
}

function clearPendingOpenIdempotencyKey(open: PendingBlindboxOpen) {
  if (open.storageKey && typeof window !== 'undefined') {
    try {
      window.sessionStorage.removeItem(open.storageKey)
    } catch {
      // Nothing else is needed once the settled response has been received.
    }
  }
  if (
    pendingOpenIdempotencyKey.value === open.idempotencyKey &&
    pendingOpenStorageKey.value === open.storageKey
  ) {
    pendingOpenIdempotencyKey.value = null
    pendingOpenStorageKey.value = null
  }
}

function currentOpenIdempotencyKey(): PendingBlindboxOpen {
  const userID = currentBlindboxUserID()
  const storageKey = pendingOpenStorageKeyForCurrentUser()
  if (
    pendingOpenIdempotencyKey.value &&
    pendingOpenStorageKey.value === storageKey
  ) {
    return {
      idempotencyKey: pendingOpenIdempotencyKey.value,
      storageKey,
      userID,
    }
  }

  pendingOpenIdempotencyKey.value = null
  pendingOpenStorageKey.value = null

  const persistedKey = storageKey ? loadPersistedOpenIdempotencyKey(storageKey) : null
  if (persistedKey) {
    pendingOpenIdempotencyKey.value = persistedKey
    pendingOpenStorageKey.value = storageKey
    return { idempotencyKey: persistedKey, storageKey, userID }
  }

  const uuid = globalThis.crypto?.randomUUID?.()
  pendingOpenIdempotencyKey.value = uuid
    ? `blindbox-${uuid}`
    : `blindbox-${Date.now()}-${Math.random().toString(36).slice(2)}`
  pendingOpenStorageKey.value = storageKey
  persistOpenIdempotencyKey(storageKey, pendingOpenIdempotencyKey.value)
  return {
    idempotencyKey: pendingOpenIdempotencyKey.value,
    storageKey,
    userID,
  }
}

function isCurrentOpenRequest(requestID: number, open: PendingBlindboxOpen): boolean {
  return (
    requestID === openRequestID &&
    authStore.isAuthenticated &&
    open.userID === currentBlindboxUserID()
  )
}

async function handleOpen() {
  if (!canOpen.value) return
  const requestID = ++openRequestID
  const open = currentOpenIdempotencyKey()
  opening.value = true
  try {
    let result: PlayBlindboxOpenResult
    try {
      result = await playAPI.openBlindbox(open.idempotencyKey)
    } catch (err: unknown) {
      if (!isCurrentOpenRequest(requestID, open)) return
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
      if (code === 'COUPON_REWARD_POOL_UNAVAILABLE') {
        appStore.showInfo(t('blindbox.couponPoolUnavailable'))
        await loadStatus()
        return
      }
      appStore.showError(t('blindbox.failed'))
      return
    }

    clearPendingOpenIdempotencyKey(open)
    if (!isCurrentOpenRequest(requestID, open)) return
    lastResult.value = result
    celebrationOpen.value = true
    try {
      await authStore.refreshUser()
    } catch {
      // The draw is already settled; refresh the view below without reporting it as a failed open.
    }
    await Promise.all([loadStatus(), loadRecentWins()])
  } finally {
    if (requestID === openRequestID) opening.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadStatus(), loadRecentWins()])
})

watch(
  () => [authStore.isAuthenticated, authStore.user?.id] as const,
  () => {
    openRequestID += 1
    opening.value = false
    const storageKey = pendingOpenStorageKeyForCurrentUser()
    if (pendingOpenStorageKey.value !== storageKey) {
      pendingOpenIdempotencyKey.value = null
      pendingOpenStorageKey.value = null
      lastResult.value = null
      celebrationOpen.value = false
    }
    void loadStatus()
  },
)
</script>

<template>
  <AuthenticatedPlayShell>
    <div class="play-page">
    <header v-if="!authStore.isAuthenticated" class="public-page-header">
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
                <template v-else-if="!couponPoolReady">
                  <div class="play-note">{{ t('blindbox.couponPoolUnavailable') }}</div>
                  <button type="button" class="play-btn play-btn-primary" disabled>
                    {{ t('blindbox.openButton') }}
                  </button>
                </template>
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
                    <p>{{ t('blindbox.expectedReward', { amount: formatMoney(expectedCashReward), rtp: Math.round(expectedCashRTPCap * 100) }) }}</p>
                    <p v-if="nextPool && vipPool.amount_to_next">
                      {{ t('blindbox.nextPoolHint', {
                        amount: formatMoney(vipPool.amount_to_next),
                        label: vipPool.next_label ?? `V${vipPool.next_tier}`,
                        pool: nextPool.version,
                        reward: formatMoney(nextExpectedCashReward),
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
                  <p v-if="lastResult && !hasCouponResult" class="play-note">
                    {{ t('blindbox.lastResult', { reward: lastResult.reward_amount.toFixed(2), net: lastResult.net_amount.toFixed(2) }) }}
                  </p>
                  <CouponRewardCard v-if="hasCouponResult && lastResult?.coupon" :coupon="lastResult.coupon" />
                </template>
              </div>

              <div v-else class="play-actions">
                <p v-if="!couponPoolReady" class="play-note">{{ t('blindbox.couponPoolUnavailable') }}</p>
                <router-link
                  v-else-if="featureEnabled"
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
            <span class="play-mini-value">{{ totalPrizeCount }}</span>
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
            <p class="play-note">{{ t('blindbox.rewardSplit') }}</p>
            <p v-if="!loading && statusLoadFailed" class="play-note">{{ t('blindbox.unavailable') }}</p>
            <p v-else-if="!loading && !featureEnabled" class="play-note">{{ t('blindbox.disabled') }}</p>
            <p v-else-if="!loading && !couponPoolReady" class="play-note">{{ t('blindbox.couponPoolUnavailable') }}</p>
            <p v-else-if="!loading && !prizePool" class="play-note">{{ t('blindbox.unavailable') }}</p>
            <template v-else-if="prizePool">
              <div class="blindbox-prize-block">
                <h3 class="blindbox-prize-heading">{{ t('blindbox.couponPrizeTitle') }}</h3>
                <p v-if="couponPrizes.length === 0" class="play-note">{{ t('blindbox.couponPrizeEmpty') }}</p>
                <ul v-else class="play-prize-grid">
                  <li
                    v-for="prize in couponPrizes"
                    :key="`coupon-${prize.template_id}`"
                    class="play-prize-tier blindbox-coupon-prize"
                  >
                    <span class="play-prize-amount">{{ prize.name }}</span>
                    <span class="play-prize-rate">{{ couponPrizeTierLabel(prize.tier) }}</span>
                  </li>
                </ul>
              </div>
              <div class="blindbox-prize-block">
                <h3 class="blindbox-prize-heading">{{ t('blindbox.balancePrizeTitle') }}</h3>
                <ul class="play-prize-grid">
                  <li
                    v-for="(tier, index) in prizePool.tiers"
                    :key="`${prizePool.version}-${index}`"
                    class="play-prize-tier"
                  >
                    <span class="play-prize-amount">${{ formatPrizeAmount(tier.amount) }}</span>
                    <span class="play-prize-rate">{{ formatBalanceProbability(tier.weight) }}</span>
                  </li>
                </ul>
              </div>
            </template>
          </section>

          <section class="play-content-panel">
            <h2 class="play-section-title">{{ t('blindbox.recentWinsTitle') }}</h2>
            <p v-if="recentWinsFailed" class="play-note">{{ t('blindbox.recentWinsUnavailable') }}</p>
            <p v-else-if="recentWins.length === 0" class="play-note">{{ t('blindbox.recentWinsPlaceholder') }}</p>
            <ul v-else class="play-wins-list">
              <li v-for="(win, idx) in recentWins" :key="idx" class="play-win-item">
                <span class="play-win-user">{{ win.user }}</span>
                <span class="play-win-reward" :class="{ 'blindbox-coupon-win': win.reward_type === 'coupon' }">{{ formatRecentWinReward(win) }}</span>
                <span class="play-win-when">{{ formatWinWhen(win.when) }}</span>
              </li>
            </ul>
          </section>
        </div>
      </div>
    </main>

    <RewardCelebrationOverlay
      :open="celebrationOpen && !!lastResult"
      :title="celebrationTitle"
      :amount="celebrationAmount"
      :subtitle="celebrationSubtitle"
      :details="celebrationDetails"
      :vip-label="lastResult?.vip_tier?.label ?? vipPool?.label ?? ''"
      :color-key="lastResult?.vip_tier?.color_key ?? vipPool?.color_key ?? 'neutral'"
      :variant="celebrationVariant"
      :primary-label="t('blindbox.openAgain')"
      :secondary-label="t('blindbox.viewReward')"
      @close="celebrationOpen = false"
      @primary="() => { celebrationOpen = false; void handleOpen() }"
      @secondary="celebrationOpen = false"
    />
    <SupportFloatingCard v-if="!authStore.isAuthenticated" />
    </div>
  </AuthenticatedPlayShell>
</template>

<style scoped>
.blindbox-vip-pool {
  display: grid;
  gap: 7px;
  border: 1px solid var(--border-focus);
  border-radius: 8px;
  background: var(--action-primary-subtle);
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
  background: var(--play-primary);
  color: var(--text-inverse);
  padding: 2px 9px;
  font-size: 12px;
  font-weight: 800;
}

.play-prize-tier:last-child {
  border-color: var(--status-warning-border);
  background: var(--status-warning-surface);
}

.play-prize-tier:last-child .play-prize-amount,
.play-prize-tier:last-child .play-prize-rate {
  color: var(--status-warning-text);
}

.blindbox-opening {
  animation: blindbox-button-shake 0.42s ease-in-out infinite;
}

.blindbox-prize-block + .blindbox-prize-block {
  margin-top: 18px;
}

.blindbox-prize-heading {
  margin: 0 0 10px;
  font-size: 14px;
  font-weight: 600;
  color: var(--ink);
}

.blindbox-coupon-prize {
  @apply border-emerald-200 bg-emerald-50/70 dark:border-emerald-500/30 dark:bg-emerald-900/10;
}

.blindbox-coupon-prize .play-prize-amount {
  font-family: 'Noto Sans SC', system-ui, sans-serif;
  font-size: 15px;
  line-height: 1.35;
}

.blindbox-coupon-win {
  @apply text-emerald-700 dark:text-emerald-300;
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
