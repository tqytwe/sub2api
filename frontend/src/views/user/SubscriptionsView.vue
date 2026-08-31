<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
        ></div>
      </div>

      <!-- Empty State -->
      <div v-else-if="subscriptions.length === 0" class="card p-12 text-center">
        <div
          class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700"
        >
          <Icon name="creditCard" size="xl" class="text-gray-400" />
        </div>
        <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('userSubscriptions.noActiveSubscriptions') }}
        </h3>
        <p class="text-gray-500 dark:text-dark-400">
          {{ t('userSubscriptions.noActiveSubscriptionsDesc') }}
        </p>
      </div>

      <!-- Subscriptions Grid -->
      <div v-else class="grid gap-6 lg:grid-cols-2">
        <div
          v-for="subscription in subscriptions"
          :key="subscription.id"
          class="overflow-hidden rounded-2xl border bg-white dark:bg-dark-800"
          :class="platformBorderClass(subscription.group?.platform || '')"
        >
          <!-- Header -->
          <div
            class="flex items-center justify-between border-b border-gray-100 p-4 dark:border-dark-700"
          >
            <div class="flex items-center gap-3">
              <div :class="['h-1.5 w-1.5 shrink-0 rounded-full', platformAccentDotClass(subscription.group?.platform || '')]" />
              <div>
                <div class="flex items-center gap-2">
                  <h3 class="font-semibold text-gray-900 dark:text-white">
                    {{ subscription.group?.name || t('userSubscriptions.groupFallback', { id: subscription.group_id }) }}
                  </h3>
                  <span :class="['rounded-md border px-2 py-0.5 text-[11px] font-medium', platformBadgeClass(subscription.group?.platform || '')]">
                    {{ platformLabel(subscription.group?.platform || '', locale) }}
                  </span>
                </div>
                <p v-if="subscription.group?.description" class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
                  {{ subscription.group.description }}
                </p>
                <div class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-gray-400 dark:text-gray-500">
                  <span>{{ t('payment.planCard.rate') }}: ×{{ subscription.group?.rate_multiplier ?? 1 }}</span>
                  <span v-if="subscriptionHasPeakRate(subscription)" class="text-amber-700 dark:text-amber-300">
                    {{ t('payment.planCard.peakRate') }}: {{ subscriptionPeakRateLabel(subscription) }}
                  </span>
                </div>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <span
                :class="[
                  'rounded-full px-2 py-0.5 text-xs font-medium',
                  subscription.status === 'active'
                    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
                    : subscription.status === 'expired'
                      ? 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-400'
                      : 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
                ]"
              >
                {{ subscriptionStatusLabel(subscription.status) }}
              </span>
              <button
                v-if="subscription.status === 'active'"
                :class="['rounded-lg px-3 py-1.5 text-xs font-semibold text-white transition-colors', platformButtonClass(subscription.group?.platform || '')]"
                @click="router.push({ path: '/purchase', query: { tab: 'subscription', group: String(subscription.group_id) } })"
              >
                {{ t('payment.renewNow') }}
              </button>
            </div>
          </div>

          <!-- Usage Progress -->
          <div class="space-y-4 p-4">
            <!-- Expiration Info -->
            <div v-if="subscription.progress.expiresAt" class="flex items-center justify-between text-sm">
              <span class="text-gray-500 dark:text-dark-400">{{
                t('userSubscriptions.expires')
              }}</span>
              <span :class="getExpirationClass(subscription.progress.expiresAt)">
                {{ formatExpirationDate(subscription.progress.expiresAt) }}
              </span>
            </div>
            <div v-else class="flex items-center justify-between text-sm">
              <span class="text-gray-500 dark:text-dark-400">{{
                t('userSubscriptions.expires')
              }}</span>
              <span class="text-gray-700 dark:text-gray-300">{{
                t('userSubscriptions.noExpiration')
              }}</span>
            </div>

            <!-- Daily Usage -->
            <div v-if="subscription.progress.daily" class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('userSubscriptions.daily') }}
                </span>
                <span class="text-sm text-gray-500 dark:text-dark-400">
                  {{ formatUsage(subscription.progress.daily) }}
                </span>
              </div>
              <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                <div
                  class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                  :class="getProgressBarClass(subscription.progress.daily.percentage)"
                  :style="{
                    width: getProgressWidth(subscription.progress.daily.percentage)
                  }"
                ></div>
              </div>
              <p
                v-if="subscription.progress.daily.resetsInSeconds !== null"
                class="text-xs text-gray-500 dark:text-dark-400"
              >
                {{ formatDailyUsageWindow(subscription) }}
              </p>
            </div>

            <!-- Weekly Usage -->
            <div v-if="subscription.progress.weekly" class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('userSubscriptions.weekly') }}
                </span>
                <span class="text-sm text-gray-500 dark:text-dark-400">
                  {{ formatUsage(subscription.progress.weekly) }}
                </span>
              </div>
              <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                <div
                  class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                  :class="getProgressBarClass(subscription.progress.weekly.percentage)"
                  :style="{
                    width: getProgressWidth(subscription.progress.weekly.percentage)
                  }"
                ></div>
              </div>
              <p
                v-if="subscription.progress.weekly.resetsInSeconds !== null"
                class="text-xs text-gray-500 dark:text-dark-400"
              >
                {{
                  t('userSubscriptions.resetIn', {
                    time: formatResetIn(subscription.progress.weekly.resetsInSeconds)
                  })
                }}
              </p>
            </div>

            <!-- Monthly Usage -->
            <div v-if="subscription.progress.monthly" class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('userSubscriptions.monthly') }}
                </span>
                <span class="text-sm text-gray-500 dark:text-dark-400">
                  {{ formatUsage(subscription.progress.monthly) }}
                </span>
              </div>
              <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                <div
                  class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                  :class="getProgressBarClass(subscription.progress.monthly.percentage)"
                  :style="{
                    width: getProgressWidth(subscription.progress.monthly.percentage)
                  }"
                ></div>
              </div>
              <p
                v-if="subscription.progress.monthly.resetsInSeconds !== null"
                class="text-xs text-gray-500 dark:text-dark-400"
              >
                {{
                  t('userSubscriptions.resetIn', {
                    time: formatResetIn(subscription.progress.monthly.resetsInSeconds)
                  })
                }}
              </p>
            </div>

            <!-- No limits configured - Unlimited badge -->
            <div
              v-if="isUnlimitedSubscription(subscription)"
              class="flex items-center justify-center rounded-xl bg-gradient-to-r from-emerald-50 to-teal-50 py-6 dark:from-emerald-900/20 dark:to-teal-900/20"
            >
              <div class="flex items-center gap-3">
                <span class="text-4xl text-emerald-600 dark:text-emerald-400">∞</span>
                <div>
                  <p class="text-sm font-medium text-emerald-700 dark:text-emerald-300">
                    {{ t('userSubscriptions.unlimited') }}
                  </p>
                  <p class="text-xs text-emerald-600/70 dark:text-emerald-400/70">
                    {{ t('userSubscriptions.unlimitedDesc') }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import subscriptionsAPI, { createSubscriptionProgressFallback } from '@/api/subscriptions'
import type { SubscriptionProgress, SubscriptionUsageWindow, UserSubscription } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTimeToMinute } from '@/utils/format'
import { hasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import { platformBorderClass, platformBadgeClass, platformButtonClass, platformLabel } from '@/utils/platformColors'
import { localizedEnumOrUnknown } from '@/utils/localizedEnum'
import {
  getExpirationDateRelation,
  getRemainingDurationParts,
  isOneTimeDailyQuota,
  type RemainingDurationParts
} from '@/utils/subscriptionQuota'

function platformAccentDotClass(p: string): string {
  switch (p) {
    case 'anthropic': return 'bg-orange-500'
    case 'openai': return 'bg-emerald-500'
    case 'antigravity': return 'bg-purple-500'
    case 'gemini': return 'bg-blue-500'
    default: return 'bg-gray-400'
  }
}

const { t, locale } = useI18n()
const router = useRouter()
const appStore = useAppStore()

type SubscriptionProgressDisplay = UserSubscription & { progress: SubscriptionProgress }

const subscriptions = ref<SubscriptionProgressDisplay[]>([])
const loading = ref(true)

function subscriptionStatusLabel(status: string): string {
  return localizedEnumOrUnknown(t, `userSubscriptions.status.${status}`)
}

function subscriptionHasPeakRate(subscription: UserSubscription): boolean {
  return hasPeakRate(subscription.group)
}

function subscriptionPeakRateLabel(subscription: UserSubscription): string {
  return formatPeakRateWindow(subscription.group, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset))
}

async function loadSubscriptions() {
  try {
    loading.value = true
    const [allSubscriptions, progressEntries] = await Promise.all([
      subscriptionsAPI.getMySubscriptions(),
      subscriptionsAPI.getSubscriptionsProgress().catch((error) => {
        console.error('Failed to load subscription progress:', error)
        return []
      })
    ])
    const progressBySubscriptionID = new Map(
      progressEntries.map(({ subscription, progress }) => [subscription?.id ?? progress.id, progress])
    )

    subscriptions.value = allSubscriptions.map((subscription) => ({
      ...subscription,
      progress: progressBySubscriptionID.get(subscription.id) ??
        createSubscriptionProgressFallback(subscription)
    }))
  } catch (error) {
    console.error('Failed to load subscriptions:', error)
    appStore.showError(t('userSubscriptions.failedToLoad'))
  } finally {
    loading.value = false
  }
}

function getProgressWidth(percentage: number | null | undefined): string {
  const normalized = Math.min(Math.max(percentage ?? 0, 0), 100)
  return `${normalized}%`
}

function getProgressBarClass(percentage: number | null | undefined): string {
  if (percentage === null || percentage === undefined) return 'bg-gray-400'
  if (percentage >= 90) return 'bg-red-500'
  if (percentage >= 70) return 'bg-orange-500'
  return 'bg-green-500'
}

function formatUsage(window: SubscriptionUsageWindow | null): string {
  if (!window) return '—'
  const limit = window.limitUsd === null ? '∞' : window.limitUsd.toFixed(2)
  return `$${window.usedUsd.toFixed(2)} / $${limit}`
}

function isUnlimitedSubscription(subscription: SubscriptionProgressDisplay): boolean {
  return (
    !subscription.progress.daily &&
    !subscription.progress.weekly &&
    !subscription.progress.monthly &&
    !subscription.group?.daily_limit_usd &&
    !subscription.group?.weekly_limit_usd &&
    !subscription.group?.monthly_limit_usd
  )
}

function formatExpirationDate(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))
  const relation = getExpirationDateRelation(expires, now)

  if (relation === null) return ''

  if (relation === 'expired') {
    return t('userSubscriptions.status.expired')
  }

  const dateStr = formatDateTimeToMinute(expires)

  if (relation === 'today') {
    return `${dateStr} (${t('common.today')})`
  }
  if (relation === 'tomorrow') {
    return `${dateStr} (${t('common.tomorrow')})`
  }

  return t('userSubscriptions.daysRemaining', { days }) + ` (${dateStr})`
}

function getExpirationClass(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))

  if (diff <= 0) return 'text-red-600 dark:text-red-400 font-medium'
  if (days <= 3) return 'text-red-600 dark:text-red-400'
  if (days <= 7) return 'text-orange-600 dark:text-orange-400'
  return 'text-gray-700 dark:text-gray-300'
}

function formatDurationParts(parts: RemainingDurationParts): string {
  if (parts.days > 0) {
    return t('userSubscriptions.duration.daysHours', { days: parts.days, hours: parts.hours })
  }

  if (parts.hours > 0) {
    return t('userSubscriptions.duration.hoursMinutes', { hours: parts.hours, minutes: parts.minutes })
  }

  return t('userSubscriptions.duration.minutes', { minutes: parts.minutes })
}

function formatDailyUsageWindow(subscription: SubscriptionProgressDisplay): string {
  if (isOneTimeDailyQuota(subscription) && subscription.progress.expiresAt) {
    const parts = getRemainingDurationParts(subscription.progress.expiresAt)
    if (!parts) return t('userSubscriptions.windowNotActive')
    return t('userSubscriptions.quotaEndsIn', { time: formatDurationParts(parts) })
  }

  return t('userSubscriptions.resetIn', {
    time: formatResetIn(subscription.progress.daily?.resetsInSeconds ?? null)
  })
}

function formatResetIn(resetsInSeconds: number | null): string {
  if (resetsInSeconds === null || resetsInSeconds <= 0) {
    return t('userSubscriptions.windowNotActive')
  }
  const totalMinutes = Math.ceil(resetsInSeconds / 60)
  return formatDurationParts({
    days: Math.floor(totalMinutes / (24 * 60)),
    hours: Math.floor((totalMinutes % (24 * 60)) / 60),
    minutes: totalMinutes % 60
  })
}

onMounted(() => {
  loadSubscriptions()
})
</script>
