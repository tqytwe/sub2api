<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import playAPI, { type PlayCheckinResult, type PlayCheckinStatus, type PlayGrowthEligibility } from '@/api/play'
import authAPI from '@/api/auth'
import { trackQuestCompleteOnce } from '@/utils/growthAnalytics'
import { extractApiErrorCode } from '@/utils/apiError'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import '@/styles/growth-world.css'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()

const loading = ref(true)
const submitting = ref(false)
const makingUp = ref(false)
const status = ref<PlayCheckinStatus | null>(null)
const balance = ref<number | null>(null)
const loadError = ref('')
let loadInFlight = false
let reloadAfterLoad = false
let disposed = false

// The API field is an absolute streak target, while this label shows days left.
const remainingMilestoneDays = computed(() =>
  Math.max(0, (status.value?.next_milestone_days || 0) - (status.value?.streak_count || 0)),
)

const growthEligibility = computed(() => status.value?.growth_eligibility)
const growthEnergyMode = computed(
  () => status.value?.growth_energy_enabled || growthEligibility.value?.reward_mode === 'energy',
)
const governanceBlocked = computed(
  () => !growthEnergyMode.value && status.value?.growth_governance_available === false,
)
const canCheckIn = computed(
  () =>
    status.value?.enabled &&
    !status.value.checked_in_today &&
    !governanceBlocked.value &&
    !loading.value && !submitting.value && !makingUp.value,
)
const canMakeup = computed(
  () => status.value?.enabled && status.value.can_makeup && status.value.redeemable_reward_eligible && !loading.value && !makingUp.value && !submitting.value,
)

async function loadStatus() {
  if (loadInFlight || disposed) return
  loadInFlight = true
  const userID = authStore.user?.id
  const token = authStore.token
  const sameSession = () => userID === authStore.user?.id && token === authStore.token
  loading.value = true
  loadError.value = ''
  status.value = null
  balance.value = null
  try {
    const nextStatus = await playAPI.getCheckinStatus()
    if (disposed || !sameSession()) return
    // Keep this read local: a late profile response from an unmounted page
    // must not overwrite the shared account or a newly opened page's balance.
    const { data: profile } = await authAPI.getCurrentUser()
    if (!disposed && sameSession() && !reloadAfterLoad) {
      status.value = nextStatus
      balance.value = profile.balance
    }
  } catch (err: unknown) {
    if (!disposed && !reloadAfterLoad) {
      const httpStatus = (err as { status?: number })?.status
      loadError.value = httpStatus === 403 ? 'checkin.loadForbidden'
        : httpStatus === 0 ? 'checkin.loadNetworkFailed' : 'checkin.loadFailed'
    }
  } finally {
    loadInFlight = false
    if (!disposed) {
      if (reloadAfterLoad || !sameSession()) {
        reloadAfterLoad = false
        void loadStatus()
      } else loading.value = false
    }
  }
}

function refreshStatus() {
  if (!submitting.value && !makingUp.value) void loadStatus()
}

async function reloadStatusAfterMutation() {
  // A token renewal may have started a read before the reward committed.
  // Discard that response and follow it with a fresh read of the committed state.
  if (loadInFlight) reloadAfterLoad = true
  else await loadStatus()
}

function successMessage(result: PlayCheckinResult) {
  if (result.growth_energy && result.growth_energy > 0) {
    return t('checkin.energySuccess', { amount: result.growth_energy })
  }
  if (result.reward_type === 'coupon' && result.coupon) {
    return t('checkin.couponSuccess', { name: result.coupon.name })
  }
  if (result.reward_type === 'redeem_code' && result.redeem_code) {
    return t('checkin.redeemSuccess', { code: result.redeem_code.code })
  }
  let msg = t('checkin.success', { amount: result.balance_added.toFixed(2) })
  if (result.streak_count && result.streak_count > 1) {
    msg += ` · ${t('checkin.streak', { days: result.streak_count })}`
  }
  if (result.milestone_bonus && result.milestone_bonus > 0) {
    msg += ` · ${t('checkin.milestoneBonus', { amount: result.milestone_bonus.toFixed(2) })}`
  }
  return msg
}

function growthReasonMessage(reason?: string) {
  const key = `checkin.growthReasons.${reason || 'no_recent_activity'}`
  const label = t(key)
  return label === key ? t('checkin.growthReasons.no_recent_activity') : label
}

function growthProgressMessage(eligibility?: PlayGrowthEligibility) {
  const progress = eligibility?.progress
  if (!progress) return t('checkin.energyProgress')
  return t('checkin.energyProgress', {
    accountAge: progress.account_age_days,
    minimumAge: progress.minimum_account_age_days,
    recharge: progress.net_balance_recharge_30d.toFixed(2),
    minimumRecharge: progress.minimum_recharge_cny.toFixed(2),
  })
}

function governanceMessage(reason?: string) {
  if (reason === 'rollout_excluded') return t('checkin.governanceRolloutExcluded')
  if (reason === 'budget_exhausted') return t('checkin.governanceBudgetExhausted')
  if (reason === 'unavailable') return t('checkin.governanceUnavailable')
  return t('checkin.governanceNotApproved')
}

async function handleCheckin() {
  if (!canCheckIn.value) return
  submitting.value = true
  try {
    const result = await playAPI.checkin()
    appStore.showSuccess(successMessage(result))
    trackQuestCompleteOnce('checkin')
    await authStore.refreshUser().catch(() => undefined)
    await reloadStatusAfterMutation()
  } catch (err: unknown) {
    const code = extractApiErrorCode(err)
    if (code === 'PLAY_CHECKIN_ALREADY_DONE') {
      appStore.showInfo(t('checkin.alreadyDone'))
      await reloadStatusAfterMutation()
      return
    }
    if (code === 'PLAY_FEATURE_DISABLED') {
      appStore.showError(t('checkin.disabled'))
      return
    }
    if (code === 'PLAY_GROWTH_REWARD_INELIGIBLE') {
      appStore.showInfo(growthReasonMessage(status.value?.growth_eligibility?.primary_reason))
      return
    }
    if (code === 'PLAY_GROWTH_GOVERNANCE_NOT_APPROVED') {
      appStore.showInfo(t('checkin.governanceNotApproved'))
      return
    }
    if (code === 'PLAY_GROWTH_GOVERNANCE_ROLLOUT_EXCLUDED') {
      appStore.showInfo(t('checkin.governanceRolloutExcluded'))
      return
    }
    if (code === 'PLAY_GROWTH_GOVERNANCE_BUDGET_EXHAUSTED') {
      appStore.showInfo(t('checkin.governanceBudgetExhausted'))
      return
    }
    if (code === 'PLAY_GROWTH_GOVERNANCE_UNAVAILABLE') {
      appStore.showError(t('checkin.governanceUnavailable'))
      return
    }
    if (code === 'COUPON_REWARD_POOL_UNAVAILABLE') {
      appStore.showInfo(t('checkin.poolUnavailable'))
      await reloadStatusAfterMutation()
      return
    }
    if (code === 'PLAY_GROWTH_QUALIFICATION_UNAVAILABLE') {
      appStore.showError(t('checkin.qualificationUnavailable'))
      return
    }
    appStore.showError(t('checkin.failed'))
  } finally {
    submitting.value = false
  }
}

async function handleMakeup() {
  if (!canMakeup.value) return
  makingUp.value = true
  try {
    const result = await playAPI.checkinMakeup()
    appStore.showSuccess(t('checkin.makeupSuccess', { amount: result.balance_added.toFixed(2) }))
    await authStore.refreshUser().catch(() => undefined)
    await reloadStatusAfterMutation()
  } catch (err: unknown) {
    const code = extractApiErrorCode(err)
    if (code === 'PLAY_CHECKIN_MAKEUP_UNAVAILABLE' || code === 'PLAY_CHECKIN_MAKEUP_ALREADY_DONE') {
      appStore.showError(t('checkin.makeupUnavailable'))
      await reloadStatusAfterMutation()
      return
    }
    appStore.showError(t('checkin.makeupFailed'))
  } finally {
    makingUp.value = false
  }
}

function goPurchase() {
  router.push('/purchase')
}

onMounted(loadStatus)
onBeforeUnmount(() => { disposed = true })
watch([() => authStore.user?.id, () => authStore.token], () => { void loadStatus() })
</script>

<template>
  <AppLayout>
    <div class="gw-page gw-workspace pb-10" :aria-busy="loading">
      <section class="gw-hero-panel">
        <div class="gw-hero-grid">
          <div>
            <p class="gw-eyebrow">{{ t('checkin.eyebrow') }}</p>
            <h1 class="gw-title">{{ t('checkin.title') }}</h1>
            <p v-if="status?.enabled" class="gw-subtitle">
              {{ growthEnergyMode ? t('checkin.energyHint') : t('checkin.randomRewardHint') }}
            </p>
            <button type="button" data-testid="refresh-status" class="gw-btn gw-btn-secondary mt-3" :disabled="loading || submitting || makingUp" @click="refreshStatus">
              {{ t('common.refresh') }}
            </button>
          </div>

          <div class="gw-hub-balance">
            <p class="gw-balance-label">{{ t('checkin.balanceLabel') }}</p>
            <p class="gw-balance-value">{{ balance !== null ? `$${balance.toFixed(2)}` : '—' }}</p>
            <p v-if="status?.streak_count" class="gw-subtitle mt-2">
              {{ t('checkin.streak', { days: status.streak_count }) }}
            </p>
            <p v-if="status?.recharge_boost_active" class="gw-buff mt-2 inline-flex">
              {{ t('checkin.boostActive', { mult: status.boost_checkin_multiplier || 2 }) }}
            </p>
          </div>
        </div>
      </section>

      <div class="gw-detail-grid">
        <div class="gw-panel">
          <div v-if="loading" class="gw-polling text-center py-6">
            {{ t('models.loading') }}
          </div>
          <div v-else-if="loadError" class="gw-subtitle" role="alert">
            <p>{{ t(loadError) }}</p>
            <button type="button" class="gw-btn gw-btn-secondary mt-3" @click="loadStatus">{{ t('common.retry') }}</button>
          </div>
          <div v-else-if="!status?.enabled" class="gw-subtitle text-center py-6">
            {{ t('checkin.disabled') }}
          </div>
          <div v-else class="space-y-4">
            <p class="gw-subtitle">
              {{ t('checkin.serverDate', { date: status.server_date }) }}
            </p>
            <p v-if="status.checked_in_today" class="text-sm font-medium" style="color: var(--gw-ok)">
              {{ t('checkin.alreadyDone') }}
            </p>
            <div v-else-if="growthEnergyMode" class="gw-quest-banner gw-quest-banner--warn" role="status">
              <p class="gw-quest-banner-title">{{ t('checkin.energyTitle') }}</p>
              <p class="gw-subtitle">{{ growthReasonMessage(growthEligibility?.primary_reason) }}</p>
              <p class="gw-subtitle">{{ growthProgressMessage(growthEligibility) }}</p>
            </div>
            <div v-else-if="governanceBlocked" class="gw-quest-banner gw-quest-banner--warn" role="status">
              <p class="gw-quest-banner-title">{{ governanceMessage(status?.growth_governance_reason) }}</p>
            </div>
            <button
              type="button"
              class="gw-btn gw-btn-primary w-full"
              :disabled="!canCheckIn"
              @click="handleCheckin"
            >
              {{
                submitting
                  ? t('checkin.submitting')
                  : status.checked_in_today
                    ? t('checkin.doneButton')
                    : t('checkin.button')
              }}
            </button>
          </div>
        </div>

        <aside class="gw-workspace">
          <div v-if="status?.redeemable_reward_eligible" class="gw-quest-banner">
            <p class="gw-quest-banner-title">{{ t('checkin.redeemableTitle') }}</p>
            <p class="gw-subtitle">{{ t('checkin.redeemableHint') }}</p>
          </div>
          <div v-if="remainingMilestoneDays" class="gw-quest-banner">
            {{ t('checkin.nextMilestone', { days: remainingMilestoneDays, bonus: (status?.next_milestone_bonus || 0).toFixed(2) }) }}
          </div>

          <div v-if="status?.can_makeup" class="gw-panel">
            <p class="gw-subtitle">{{ t('checkin.makeupHint', { date: status.makeup_date }) }}</p>
            <div class="mt-3 flex flex-wrap gap-2">
              <button type="button" class="gw-btn gw-btn-primary" :disabled="!canMakeup" @click="handleMakeup">
                {{ makingUp ? t('checkin.makeupSubmitting') : t('checkin.makeupButton') }}
              </button>
              <button type="button" class="gw-btn gw-btn-secondary" @click="goPurchase">{{ t('checkin.rechargeCta') }}</button>
            </div>
          </div>
        </aside>
      </div>
    </div>
  </AppLayout>
</template>
