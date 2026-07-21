<template>
  <div
    data-test="subscription-plan-card"
    :class="[
      'group relative flex min-h-[420px] flex-col overflow-hidden rounded-lg border transition-all',
      'hover:-translate-y-0.5 hover:shadow-lg',
      borderClass,
      featuredClass,
      'bg-white dark:bg-dark-800',
    ]"
  >
    <div
      data-test="plan-detail-trigger"
      role="button"
      tabindex="0"
      class="flex flex-1 cursor-pointer flex-col focus:outline-none focus:ring-2 focus:ring-inset focus:ring-primary-500"
      @click="emit('details', plan)"
      @keydown.enter.prevent="emit('details', plan)"
      @keydown.space.prevent="emit('details', plan)"
    >
      <div class="relative aspect-[16/9] overflow-hidden bg-gray-100 dark:bg-dark-700">
        <img
          v-if="coverImageURL"
          data-test="plan-cover-image"
          :src="coverImageURL"
          :alt="displayName"
          class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-[1.03]"
        />
        <div
          v-if="coverImageURL"
          class="pointer-events-none absolute inset-0 -translate-x-full bg-gradient-to-r from-transparent via-white/20 to-transparent opacity-0 transition duration-700 group-hover:translate-x-full group-hover:opacity-100"
        />
        <div
          v-else
          data-test="plan-cover-placeholder"
          :class="['flex h-full w-full items-center justify-center px-5 text-center text-lg font-bold text-white', accentClass]"
        >
          {{ displayName }}
        </div>
        <div class="absolute left-3 top-3">
          <span class="rounded-md bg-white/85 px-2 py-1 text-xs font-semibold text-gray-700 shadow-sm ring-1 ring-black/5 backdrop-blur dark:bg-dark-900/75 dark:text-gray-100 dark:ring-white/10">
            {{ pLabel }}
          </span>
        </div>
        <div v-if="storefrontBadges.length > 0" class="absolute right-3 top-3 flex max-w-[58%] flex-wrap justify-end gap-1">
          <span
            v-for="badge in storefrontBadges"
            :key="badge"
            data-test="plan-storefront-badge"
            class="rounded-md bg-gray-900/80 px-2 py-1 text-xs font-semibold text-white shadow-sm ring-1 ring-white/20 backdrop-blur"
          >
            {{ badge }}
          </span>
        </div>
      </div>

      <div class="flex flex-1 flex-col p-4 pb-3">
        <!-- Header: name + badge + price -->
        <div class="mb-3 flex items-start justify-between gap-3">
          <div class="min-w-0 flex-1">
            <h3 class="line-clamp-2 text-base font-bold leading-snug text-gray-900 dark:text-white">{{ displayName }}</h3>
            <p v-if="displayName !== plan.name" class="mt-0.5 truncate text-xs text-gray-400 dark:text-dark-400">{{ plan.name }}</p>
            <p v-if="plan.description" class="mt-0.5 text-xs leading-relaxed text-gray-500 dark:text-dark-400 line-clamp-2">
              {{ plan.description }}
            </p>
          </div>
          <div class="shrink-0 text-right">
            <div class="flex items-baseline gap-1">
              <span class="text-xs text-gray-400 dark:text-dark-500">$</span>
              <span :class="['text-2xl font-extrabold tracking-tight', textClass]">{{ plan.price }}</span>
              <span v-if="plan.currency" class="text-xs font-medium text-gray-400 dark:text-dark-500">{{ plan.currency }}</span>
            </div>
            <span class="text-[11px] text-gray-400 dark:text-dark-500">/ {{ validitySuffix }}</span>
            <div v-if="plan.original_price" class="mt-0.5 flex items-center justify-end gap-1.5">
              <span class="text-xs text-gray-400 line-through dark:text-dark-500">${{ plan.original_price }}<template v-if="plan.currency"> {{ plan.currency }}</template></span>
              <span :class="['rounded px-1 py-0.5 text-[10px] font-semibold', discountClass]">{{ discountText }}</span>
            </div>
          </div>
        </div>

        <!-- Group quota info (compact) -->
        <div class="mb-3 grid grid-cols-2 gap-x-3 gap-y-1 rounded-lg bg-gray-50 px-3 py-2 text-xs dark:bg-dark-700/50">
          <div class="flex items-center justify-between">
            <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.rate') }}</span>
            <span class="font-medium text-gray-700 dark:text-gray-300">{{ rateDisplay }}</span>
          </div>
          <div v-if="hasPeakRate" class="col-span-2 flex items-center justify-between gap-2">
            <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.peakRate') }}</span>
            <span class="text-right font-medium text-amber-700 dark:text-amber-300">{{ peakRateDisplay }}</span>
          </div>
          <div v-if="plan.daily_limit_usd != null" class="flex items-center justify-between">
            <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.dailyLimit') }}</span>
            <span class="font-medium text-gray-700 dark:text-gray-300">${{ plan.daily_limit_usd }}</span>
          </div>
          <div v-if="plan.weekly_limit_usd != null" class="flex items-center justify-between">
            <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.weeklyLimit') }}</span>
            <span class="font-medium text-gray-700 dark:text-gray-300">${{ plan.weekly_limit_usd }}</span>
          </div>
          <div v-if="plan.monthly_limit_usd != null" class="flex items-center justify-between">
            <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.monthlyLimit') }}</span>
            <span class="font-medium text-gray-700 dark:text-gray-300">${{ plan.monthly_limit_usd }}</span>
          </div>
          <div v-if="plan.daily_limit_usd == null && plan.weekly_limit_usd == null && plan.monthly_limit_usd == null" class="flex items-center justify-between">
            <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.quota') }}</span>
            <span class="font-medium text-gray-700 dark:text-gray-300">{{ t('payment.planCard.unlimited') }}</span>
          </div>
          <div v-if="modelScopeLabels.length > 0" class="col-span-2 flex items-center justify-between">
            <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.models') }}</span>
            <div class="flex flex-wrap justify-end gap-1">
              <span v-for="scope in modelScopeLabels" :key="scope"
                class="rounded bg-gray-200/80 px-1.5 py-0.5 text-[10px] font-medium text-gray-600 dark:bg-dark-600 dark:text-gray-300">
                {{ scope }}
              </span>
            </div>
          </div>
        </div>

        <!-- Features list (compact) -->
        <div v-if="plan.features.length > 0" class="mb-3 space-y-1">
          <div v-for="feature in featurePreview" :key="feature" class="flex items-start gap-1.5">
            <svg :class="['mt-0.5 h-3.5 w-3.5 flex-shrink-0', iconClass]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5" />
            </svg>
            <span class="text-xs text-gray-600 dark:text-gray-300">{{ feature }}</span>
          </div>
        </div>

        <div class="flex-1" />
      </div>
    </div>
    <div class="p-4 pt-0">
      <button
        data-test="plan-subscribe-button"
        type="button"
        :class="['w-full rounded-lg py-2.5 text-sm font-semibold transition-all active:scale-[0.98]', btnClass]"
        @click="emit('select', plan)"
      >
        {{ isRenewal ? t('payment.renewNow') : t('payment.subscribeNow') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SubscriptionPlan } from '@/types/payment'
import type { UserSubscription } from '@/types'
import { useAppStore } from '@/stores/app'
import { hasPeakRate as groupHasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import {
  platformAccentBarClass,
  platformBorderClass,
  platformTextClass,
  platformIconClass,
  platformButtonClass,
  platformDiscountClass,
  platformLabel,
} from '@/utils/platformColors'

const props = defineProps<{ plan: SubscriptionPlan; activeSubscriptions?: UserSubscription[] }>()
const emit = defineEmits<{ select: [plan: SubscriptionPlan]; details: [plan: SubscriptionPlan] }>()
const { t } = useI18n()

const platform = computed(() => props.plan.group_platform || '')
const storefrontPlatform = computed(() => props.plan.storefront_platform?.trim() || props.plan.group_platform || '')
const displayName = computed(() => props.plan.product_name?.trim() || props.plan.name)
const coverImageURL = computed(() => props.plan.cover_image_url?.trim() || '')
const storefrontBadges = computed(() => {
  const badges: string[] = []
  if (props.plan.storefront_featured) badges.push(t('payment.planCard.featured'))
  const badge = props.plan.storefront_badge?.trim()
  if (badge && !badges.includes(badge)) badges.push(badge)
  return badges
})
const isRenewal = computed(() =>
  props.activeSubscriptions?.some(s => s.group_id === props.plan.group_id && s.status === 'active') ?? false
)

// Derived color classes from central config
const accentClass = computed(() => platformAccentBarClass(storefrontPlatform.value))
const borderClass = computed(() => platformBorderClass(storefrontPlatform.value))
const textClass = computed(() => platformTextClass(storefrontPlatform.value))
const iconClass = computed(() => platformIconClass(storefrontPlatform.value))
const btnClass = computed(() => platformButtonClass(storefrontPlatform.value))
const discountClass = computed(() => platformDiscountClass(storefrontPlatform.value))
const pLabel = computed(() => platformLabel(storefrontPlatform.value))
const featuredClass = computed(() => props.plan.storefront_featured ? 'shadow-md ring-2 ring-primary-400/50 dark:ring-primary-500/40' : '')

const discountText = computed(() => {
  if (!props.plan.original_price || props.plan.original_price <= 0) return ''
  const pct = Math.round((1 - props.plan.price / props.plan.original_price) * 100)
  return pct > 0 ? `-${pct}%` : ''
})

const rateDisplay = computed(() => {
  const rate = props.plan.rate_multiplier ?? 1
  return `×${Number(rate.toPrecision(10))}`
})

const appStore = useAppStore()

const hasPeakRate = computed(() => groupHasPeakRate(props.plan))

const peakRateDisplay = computed(() => {
  return formatPeakRateWindow(props.plan, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset))
})

const MODEL_SCOPE_LABELS: Record<string, string> = {
  claude: 'Claude',
  gemini_text: 'Gemini',
  gemini_image: 'Imagen',
}

const modelScopeLabels = computed(() => {
  if (platform.value !== 'antigravity') return []
  const scopes = props.plan.supported_model_scopes
  if (!scopes || scopes.length === 0) return []
  return scopes.map(s => MODEL_SCOPE_LABELS[s] || s)
})

const featurePreview = computed(() => props.plan.features.slice(0, 3))

const validitySuffix = computed(() => {
  const u = props.plan.validity_unit || 'day'
  if (u === 'month') return t('payment.perMonth')
  if (u === 'year') return t('payment.perYear')
  return `${props.plan.validity_days}${t('payment.days')}`
})
</script>
