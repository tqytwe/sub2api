<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import playAPI, { type PlayCheckinStatus } from '@/api/play'
import { trackQuestCompleteOnce } from '@/utils/growthAnalytics'
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

const user = computed(() => authStore.user)
const canCheckIn = computed(
  () => status.value?.enabled && !status.value.checked_in_today && !submitting.value,
)
const canMakeup = computed(
  () => status.value?.can_makeup && !makingUp.value && !submitting.value,
)

async function loadStatus() {
  loading.value = true
  try {
    status.value = await playAPI.getCheckinStatus()
  } catch {
    status.value = null
  } finally {
    loading.value = false
  }
}

function successMessage(result: { balance_added: number; streak_count?: number; milestone_bonus?: number }) {
  let msg = t('checkin.success', { amount: result.balance_added.toFixed(2) })
  if (result.streak_count && result.streak_count > 1) {
    msg += ` · ${t('checkin.streak', { days: result.streak_count })}`
  }
  if (result.milestone_bonus && result.milestone_bonus > 0) {
    msg += ` · ${t('checkin.milestoneBonus', { amount: result.milestone_bonus.toFixed(2) })}`
  }
  return msg
}

async function handleCheckin() {
  if (!canCheckIn.value) return
  submitting.value = true
  try {
    const result = await playAPI.checkin()
    appStore.showSuccess(successMessage(result))
    trackQuestCompleteOnce('checkin')
    await authStore.refreshUser()
    await loadStatus()
  } catch (err: unknown) {
    const code = (err as { response?: { data?: { code?: string } } })?.response?.data?.code
    if (code === 'PLAY_CHECKIN_ALREADY_DONE') {
      appStore.showInfo(t('checkin.alreadyDone'))
      await loadStatus()
      return
    }
    if (code === 'PLAY_FEATURE_DISABLED') {
      appStore.showError(t('checkin.disabled'))
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
    await authStore.refreshUser()
    await loadStatus()
  } catch (err: unknown) {
    const code = (err as { response?: { data?: { code?: string } } })?.response?.data?.code
    if (code === 'PLAY_CHECKIN_MAKEUP_UNAVAILABLE') {
      appStore.showError(t('checkin.makeupUnavailable'))
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
</script>

<template>
  <AppLayout>
    <div class="gw-page gw-workspace pb-10">
      <section class="gw-hero-panel">
        <div class="gw-hero-grid">
          <div>
            <p class="gw-eyebrow">{{ t('checkin.eyebrow') }}</p>
            <h1 class="gw-title">{{ t('checkin.title') }}</h1>
            <p v-if="status?.enabled" class="gw-subtitle">
              {{ t('checkin.rewardHint', { amount: status.reward_amount.toFixed(2) }) }}
            </p>
          </div>

          <div class="gw-hub-balance">
            <p class="gw-balance-label">{{ t('checkin.balanceLabel') }}</p>
            <p class="gw-balance-value">${{ user?.balance?.toFixed(2) || '0.00' }}</p>
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
          <div v-if="status?.next_milestone_days" class="gw-quest-banner">
            {{ t('checkin.nextMilestone', { days: status.next_milestone_days, bonus: (status.next_milestone_bonus || 0).toFixed(2) }) }}
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
