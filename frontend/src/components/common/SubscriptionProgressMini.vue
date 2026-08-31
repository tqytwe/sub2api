<template>
  <div v-if="hasActiveSubscriptions" class="relative" ref="containerRef">
    <!-- Mini Progress Display -->
    <button
      @click="toggleTooltip"
      class="flex cursor-pointer items-center gap-2 rounded-xl bg-purple-50 px-3 py-1.5 transition-colors hover:bg-purple-100 dark:bg-purple-900/20 dark:hover:bg-purple-900/30"
      :title="t('subscriptionProgress.viewDetails')"
    >
      <Icon name="creditCard" size="sm" class="text-purple-600 dark:text-purple-400" />
      <div class="flex items-center gap-1.5">
        <!-- Combined progress indicator -->
        <div class="flex items-center gap-0.5">
          <div
            v-for="(sub, index) in displaySubscriptions.slice(0, 3)"
            :key="index"
            class="h-2 w-2 rounded-full"
            :class="getProgressDotClass(sub)"
          ></div>
        </div>
        <span class="text-xs font-medium text-purple-700 dark:text-purple-300">
          {{ displaySubscriptions.length }}
        </span>
      </div>
    </button>

    <!-- Hover/Click Tooltip -->
    <transition name="dropdown">
      <div
        v-if="tooltipOpen"
        class="absolute right-0 z-50 mt-2 w-[340px] overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xl dark:border-dark-700 dark:bg-dark-800"
      >
        <div class="border-b border-gray-100 p-3 dark:border-dark-700">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('subscriptionProgress.title') }}
          </h3>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
            {{ t('subscriptionProgress.activeCount', { count: displaySubscriptions.length }) }}
          </p>
        </div>

        <div class="max-h-64 overflow-y-auto">
          <div
            v-for="subscription in displaySubscriptions"
            :key="subscription.id"
            class="border-b border-gray-50 p-3 last:border-b-0 dark:border-dark-700/50"
          >
            <div class="mb-2 flex items-center justify-between">
              <span class="text-sm font-medium text-gray-900 dark:text-white">
                {{
                  subscription.group?.name ||
                  t('subscriptionProgress.groupFallback', { id: subscription.group_id })
                }}
              </span>
              <span
                v-if="subscription.progress.expiresAt"
                class="text-xs"
                :class="getDaysRemainingClass(subscription.progress.expiresAt)"
              >
                {{ formatDaysRemaining(subscription.progress.expiresAt) }}
              </span>
            </div>

            <!-- Progress bars or Unlimited badge -->
            <div class="space-y-1.5">
              <!-- Unlimited subscription badge -->
              <div
                v-if="isUnlimited(subscription)"
                class="flex items-center gap-2 rounded-lg bg-gradient-to-r from-emerald-50 to-teal-50 px-2.5 py-1.5 dark:from-emerald-900/20 dark:to-teal-900/20"
              >
                <span class="text-lg text-emerald-600 dark:text-emerald-400">∞</span>
                <span class="text-xs font-medium text-emerald-700 dark:text-emerald-300">
                  {{ t('subscriptionProgress.unlimited') }}
                </span>
              </div>

              <!-- Progress bars for limited subscriptions -->
              <template v-else>
                <div v-if="subscription.progress.daily" class="flex items-center gap-2">
                  <span class="w-8 flex-shrink-0 text-[10px] text-gray-500">{{
                    t('subscriptionProgress.daily')
                  }}</span>
                  <div class="h-1.5 min-w-0 flex-1 rounded-full bg-gray-200 dark:bg-dark-600">
                    <div
                      class="h-1.5 rounded-full transition-all"
                      :class="getProgressBarClass(subscription.progress.daily.percentage)"
                      :style="{
                        width: getProgressWidth(subscription.progress.daily.percentage)
                      }"
                    ></div>
                  </div>
                  <span class="w-24 flex-shrink-0 text-right text-[10px] text-gray-500">
                    {{ formatUsage(subscription.progress.daily) }}
                  </span>
                </div>

                <div v-if="subscription.progress.weekly" class="flex items-center gap-2">
                  <span class="w-8 flex-shrink-0 text-[10px] text-gray-500">{{
                    t('subscriptionProgress.weekly')
                  }}</span>
                  <div class="h-1.5 min-w-0 flex-1 rounded-full bg-gray-200 dark:bg-dark-600">
                    <div
                      class="h-1.5 rounded-full transition-all"
                      :class="getProgressBarClass(subscription.progress.weekly.percentage)"
                      :style="{
                        width: getProgressWidth(subscription.progress.weekly.percentage)
                      }"
                    ></div>
                  </div>
                  <span class="w-24 flex-shrink-0 text-right text-[10px] text-gray-500">
                    {{ formatUsage(subscription.progress.weekly) }}
                  </span>
                </div>

                <div v-if="subscription.progress.monthly" class="flex items-center gap-2">
                  <span class="w-8 flex-shrink-0 text-[10px] text-gray-500">{{
                    t('subscriptionProgress.monthly')
                  }}</span>
                  <div class="h-1.5 min-w-0 flex-1 rounded-full bg-gray-200 dark:bg-dark-600">
                    <div
                      class="h-1.5 rounded-full transition-all"
                      :class="getProgressBarClass(subscription.progress.monthly.percentage)"
                      :style="{
                        width: getProgressWidth(subscription.progress.monthly.percentage)
                      }"
                    ></div>
                  </div>
                  <span class="w-24 flex-shrink-0 text-right text-[10px] text-gray-500">
                    {{ formatUsage(subscription.progress.monthly) }}
                  </span>
                </div>
              </template>
            </div>
          </div>
        </div>

        <div class="border-t border-gray-100 p-2 dark:border-dark-700">
          <router-link
            to="/subscriptions"
            @click="closeTooltip"
            class="block w-full py-1 text-center text-xs text-primary-600 hover:underline dark:text-primary-400"
          >
            {{ t('subscriptionProgress.viewAll') }}
          </router-link>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useSubscriptionStore } from '@/stores'
import type {
  SubscriptionProgress,
  SubscriptionUsageWindow,
  UserSubscription
} from '@/types'

const { t } = useI18n()
const subscriptionStore = useSubscriptionStore()

const containerRef = ref<HTMLElement | null>(null)
const tooltipOpen = ref(false)

type SubscriptionProgressDisplay = UserSubscription & { progress: SubscriptionProgress }

const displaySubscriptions = computed<SubscriptionProgressDisplay[]>(() => {
  return subscriptionStore.activeSubscriptionProgress
    .flatMap(({ subscription, progress }) => subscription ? [{ ...subscription, progress }] : [])
    .sort((a, b) => {
      const aMax = getMaxUsagePercentage(a)
      const bMax = getMaxUsagePercentage(b)
      return bMax - aMax
    })
})

const hasActiveSubscriptions = computed(() => displaySubscriptions.value.length > 0)

function getMaxUsagePercentage(sub: SubscriptionProgressDisplay): number {
  const percentages = [sub.progress.daily, sub.progress.weekly, sub.progress.monthly]
    .flatMap((window) => window ? [window.percentage] : [])
  return percentages.length > 0 ? Math.max(...percentages) : 0
}

function isUnlimited(sub: SubscriptionProgressDisplay): boolean {
  return (
    !sub.progress.daily &&
    !sub.progress.weekly &&
    !sub.progress.monthly &&
    !sub.group?.daily_limit_usd &&
    !sub.group?.weekly_limit_usd &&
    !sub.group?.monthly_limit_usd
  )
}

function getProgressDotClass(sub: SubscriptionProgressDisplay): string {
  // Unlimited subscriptions get a special color
  if (isUnlimited(sub)) {
    return 'bg-emerald-500'
  }
  const maxPercentage = getMaxUsagePercentage(sub)
  if (maxPercentage >= 90) return 'bg-red-500'
  if (maxPercentage >= 70) return 'bg-orange-500'
  return 'bg-green-500'
}

function getProgressBarClass(percentage: number | null | undefined): string {
  if (percentage === null || percentage === undefined) return 'bg-gray-400'
  if (percentage >= 90) return 'bg-red-500'
  if (percentage >= 70) return 'bg-orange-500'
  return 'bg-green-500'
}

function getProgressWidth(percentage: number | null | undefined): string {
  const normalized = Math.min(Math.max(percentage ?? 0, 0), 100)
  return `${normalized}%`
}

function formatUsage(window: SubscriptionUsageWindow | null): string {
  if (!window) return '—'
  const limit = window.limitUsd === null ? '∞' : window.limitUsd.toFixed(2)
  return `$${window.usedUsd.toFixed(2)}/$${limit}`
}

function formatDaysRemaining(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  if (diff < 0) return t('subscriptionProgress.expired')
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))
  if (days === 0) return t('subscriptionProgress.expiresToday')
  if (days === 1) return t('subscriptionProgress.expiresTomorrow')
  return t('subscriptionProgress.daysRemaining', { days })
}

function getDaysRemainingClass(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))
  if (days <= 3) return 'text-red-600 dark:text-red-400'
  if (days <= 7) return 'text-orange-600 dark:text-orange-400'
  return 'text-gray-500 dark:text-dark-400'
}

function toggleTooltip() {
  tooltipOpen.value = !tooltipOpen.value
}

function closeTooltip() {
  tooltipOpen.value = false
}

function handleClickOutside(event: MouseEvent) {
  if (containerRef.value && !containerRef.value.contains(event.target as Node)) {
    closeTooltip()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  subscriptionStore.fetchSubscriptionProgress().catch((error) => {
    console.error('Failed to load subscriptions in SubscriptionProgressMini:', error)
  })
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.2s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: scale(0.95) translateY(-4px);
}
</style>
