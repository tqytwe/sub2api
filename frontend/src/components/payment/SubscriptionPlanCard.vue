<template>
  <article
    data-test="subscription-plan-card"
    :class="[
      'relative flex min-h-[260px] flex-col rounded-lg border bg-white p-4 shadow-sm transition-[border-color,box-shadow,background-color] focus-within:ring-2 focus-within:ring-primary-500/20 dark:bg-dark-800',
      borderClass,
      featuredClass,
    ]"
  >
    <div
      data-test="plan-detail-trigger"
      role="button"
      tabindex="0"
      class="flex flex-1 cursor-pointer flex-col rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-dark-800"
      @click="emit('details', plan)"
      @keydown.enter.prevent="emit('details', plan)"
      @keydown.space.prevent="emit('details', plan)"
    >
      <div class="flex items-start gap-3">
        <div
          v-if="coverImageURL"
          class="h-16 w-24 shrink-0 overflow-hidden rounded-md border border-gray-100 bg-gray-50 dark:border-dark-700 dark:bg-dark-700"
        >
          <img
            data-test="plan-cover-image"
            :src="coverImageURL"
            :alt="displayName"
            class="h-full w-full object-cover"
          />
        </div>
        <div
          v-else
          data-test="plan-cover-placeholder"
          class="flex h-16 w-24 shrink-0 items-center justify-center rounded-md border border-gray-100 bg-gray-50 px-2 text-center text-sm font-semibold text-gray-500 dark:border-dark-700 dark:bg-dark-700 dark:text-dark-300"
        >
          {{ displayInitials }}
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-1.5">
            <span :class="['rounded-md border px-2 py-0.5 text-xs font-semibold', platformBadgeClass(storefrontPlatform)]">
              {{ pLabel }}
            </span>
            <span
              v-for="badge in storefrontBadges"
              :key="badge"
              data-test="plan-storefront-badge"
              class="rounded-md border border-gray-200 bg-gray-50 px-2 py-0.5 text-xs font-semibold text-gray-600 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-300"
            >
              {{ badge }}
            </span>
          </div>
          <h3 class="mt-2 line-clamp-2 text-base font-semibold leading-snug text-gray-900 dark:text-white">{{ displayName }}</h3>
          <p v-if="displayName !== plan.name" class="mt-0.5 truncate text-xs text-gray-400 dark:text-dark-400">{{ plan.name }}</p>
          <p v-if="plan.description" class="mt-1 line-clamp-2 text-sm leading-relaxed text-gray-500 dark:text-dark-400">
            {{ plan.description }}
          </p>
        </div>
      </div>

      <div class="mt-4 flex flex-wrap items-baseline gap-x-2 gap-y-1">
        <span v-if="plan.original_price" class="text-sm text-gray-400 line-through dark:text-dark-500">{{ planCurrencySymbol }}{{ plan.original_price }}<template v-if="plan.currency"> {{ plan.currency }}</template></span>
        <span class="text-xs text-gray-400 dark:text-dark-500">{{ planCurrencySymbol }}</span><span :class="['text-2xl font-bold tabular-nums', textClass]">{{ plan.price }}</span><span v-if="plan.currency" class="text-xs font-medium text-gray-400 dark:text-dark-500">{{ plan.currency }}</span>
        <span class="text-sm text-gray-500 dark:text-gray-400">/ {{ validitySuffix }}</span>
        <span v-if="plan.original_price" :class="['rounded px-1.5 py-0.5 text-xs font-semibold', discountClass]">{{ discountText }}</span>
      </div>

      <div class="mt-4 grid grid-cols-2 gap-2 text-xs">
        <div class="rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-700/60">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.rate') }}</span>
          <div class="mt-1 font-semibold text-gray-700 dark:text-gray-300">{{ rateDisplay }}</div>
        </div>
        <div v-if="hasPositiveLimit(plan.daily_limit_usd)" class="rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-700/60">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.dailyLimit') }}</span>
          <div class="mt-1 font-semibold text-gray-700 dark:text-gray-300">${{ plan.daily_limit_usd }}</div>
        </div>
        <div v-if="hasPositiveLimit(plan.weekly_limit_usd)" class="rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-700/60">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.weeklyLimit') }}</span>
          <div class="mt-1 font-semibold text-gray-700 dark:text-gray-300">${{ plan.weekly_limit_usd }}</div>
        </div>
        <div v-if="hasPositiveLimit(plan.monthly_limit_usd)" class="rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-700/60">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.monthlyLimit') }}</span>
          <div class="mt-1 font-semibold text-gray-700 dark:text-gray-300">${{ plan.monthly_limit_usd }}</div>
        </div>
        <div v-if="!hasAnyPositiveLimit" class="rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-700/60">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.quota') }}</span>
          <div class="mt-1 font-semibold text-gray-700 dark:text-gray-300">{{ t('payment.planCard.unlimited') }}</div>
        </div>
        <div v-if="hasPeakRate" class="col-span-2 rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-700/60">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.peakRate') }}</span>
          <div class="mt-1 font-semibold text-amber-700 dark:text-amber-300">{{ peakRateDisplay }}</div>
        </div>
        <div v-if="modelScopeLabels.length > 0" class="col-span-2 rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-700/60">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.models') }}</span>
          <div class="mt-2 flex flex-wrap gap-1">
            <span v-for="scope in modelScopeLabels" :key="scope"
              class="rounded bg-gray-200/80 px-1.5 py-0.5 text-[10px] font-medium text-gray-600 dark:bg-dark-600 dark:text-gray-300">
              {{ scope }}
            </span>
          </div>
        </div>
      </div>

      <div v-if="plan.features.length > 0" class="mt-4 space-y-1.5">
        <div v-for="feature in featurePreview" :key="feature" class="flex items-start gap-2">
          <Icon :class="['mt-0.5 flex-shrink-0', iconClass]" name="check" size="sm" :stroke-width="2.5" />
          <span class="line-clamp-1 text-sm text-gray-600 dark:text-gray-300">{{ feature }}</span>
        </div>
      </div>
    </div>
    <div class="mt-auto pt-4">
      <button
        data-test="plan-subscribe-button"
        type="button"
        :class="['min-h-10 w-full rounded-md px-3 py-2 text-sm font-semibold transition-colors', btnClass]"
        @click="emit('select', plan)"
      >
        {{ isRenewal ? t('payment.renewNow') : t('payment.subscribeNow') }}
      </button>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SubscriptionPlan } from '@/types/payment'
import type { UserSubscription } from '@/types'
import { useAppStore } from '@/stores/app'
import Icon from '@/components/icons/Icon.vue'
import { hasPeakRate as groupHasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import { currencySymbol } from '@/components/payment/currency'
import {
  platformBadgeClass,
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
const borderClass = computed(() => platformBorderClass(storefrontPlatform.value))
const textClass = computed(() => platformTextClass(storefrontPlatform.value))
const iconClass = computed(() => platformIconClass(storefrontPlatform.value))
const btnClass = computed(() => platformButtonClass(storefrontPlatform.value))
const discountClass = computed(() => platformDiscountClass(storefrontPlatform.value))
const pLabel = computed(() => platformLabel(storefrontPlatform.value))
const featuredClass = computed(() => props.plan.storefront_featured ? 'ring-2 ring-primary-400/40 dark:ring-primary-500/30' : 'hover:border-primary-300 dark:hover:border-primary-500/60')

const discountText = computed(() => {
  if (!props.plan.original_price || props.plan.original_price <= 0) return ''
  const pct = Math.round((1 - props.plan.price / props.plan.original_price) * 100)
  return pct > 0 ? `-${pct}%` : ''
})

const rateDisplay = computed(() => {
  const rate = props.plan.rate_multiplier ?? 1
  return `×${Number(rate.toPrecision(10))}`
})

function hasPositiveLimit(value: number | null | undefined): boolean {
  return typeof value === 'number' && value > 0
}

const hasAnyPositiveLimit = computed(() =>
  hasPositiveLimit(props.plan.daily_limit_usd)
  || hasPositiveLimit(props.plan.weekly_limit_usd)
  || hasPositiveLimit(props.plan.monthly_limit_usd)
)

const appStore = useAppStore()
const planCurrencySymbol = computed(() => currencySymbol(props.plan.currency || 'USD'))

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

const displayInitials = computed(() => Array.from(displayName.value.trim()).slice(0, 4).join(''))
</script>
