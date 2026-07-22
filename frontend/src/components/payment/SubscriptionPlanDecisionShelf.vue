<template>
  <section
    v-if="plans.length > 0"
    data-test="subscription-decision-shelf"
    class="space-y-3"
  >
    <div
      data-test="plan-grid"
      class="grid gap-4 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4"
    >
      <article
        v-for="plan in plans"
        :key="plan.id"
        data-test="plan-grid-card"
        :class="planCardClass(plan)"
      >
        <div class="flex items-start gap-3">
          <div
            v-if="coverImageURL(plan)"
            data-test="plan-cover-thumbnail"
            class="h-16 w-24 shrink-0 overflow-hidden rounded-md border border-gray-100 bg-gray-50 dark:border-dark-700 dark:bg-dark-700"
          >
            <img
              :src="coverImageURL(plan)"
              :alt="displayName(plan)"
              class="h-full w-full object-cover"
            />
          </div>
          <div
            v-else
            data-test="plan-cover-placeholder"
            class="flex h-16 w-24 shrink-0 items-center justify-center rounded-md border border-gray-100 bg-gray-50 px-2 text-center text-sm font-semibold text-gray-500 dark:border-dark-700 dark:bg-dark-700 dark:text-dark-300"
          >
            {{ displayInitials(plan) }}
          </div>

          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-1.5">
              <span :class="['rounded-md border px-2 py-0.5 text-xs font-semibold', platformBadgeClass(displayPlatform(plan))]">
                {{ platformLabel(displayPlatform(plan)) }}
              </span>
              <span
                v-if="isDefaultPlan(plan)"
                data-test="plan-default-badge"
                class="rounded-md border border-primary-200 bg-primary-50 px-2 py-0.5 text-xs font-semibold text-primary-700 dark:border-primary-500/30 dark:bg-primary-500/10 dark:text-primary-300"
              >
                {{ t('payment.planCard.featured') }}
              </span>
              <span
                v-for="badge in secondaryBadges(plan)"
                :key="badge"
                data-test="plan-storefront-badge"
                class="rounded-md border border-gray-200 bg-gray-50 px-2 py-0.5 text-xs font-semibold text-gray-600 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-300"
              >
                {{ badge }}
              </span>
            </div>
            <h4 class="mt-2 line-clamp-2 text-base font-semibold leading-snug text-gray-900 dark:text-white">
              {{ displayName(plan) }}
            </h4>
            <p v-if="displayName(plan) !== plan.name" class="mt-0.5 truncate text-xs text-gray-400 dark:text-dark-400">
              {{ plan.name }}
            </p>
          </div>
        </div>

        <p v-if="planSummary(plan)" class="mt-3 line-clamp-2 text-sm leading-relaxed text-gray-500 dark:text-gray-400">
          {{ planSummary(plan) }}
        </p>

        <div class="mt-4 flex flex-wrap items-baseline gap-x-2 gap-y-1">
          <span v-if="plan.original_price" class="text-sm text-gray-400 line-through dark:text-dark-500">
            {{ priceLabel(plan, plan.original_price) }}
          </span>
          <span :class="['text-2xl font-bold tabular-nums', platformTextClass(displayPlatform(plan))]">
            {{ priceLabel(plan, plan.price) }}
          </span>
          <span class="text-sm text-gray-500 dark:text-gray-400">/ {{ planValidityLabel(plan) }}</span>
          <span v-if="discountText(plan)" :class="['rounded px-1.5 py-0.5 text-xs font-semibold', platformDiscountClass(displayPlatform(plan))]">
            {{ discountText(plan) }}
          </span>
        </div>

        <div class="mt-4 grid grid-cols-2 gap-2">
          <div
            v-for="metric in planMetrics(plan)"
            :key="metric.label"
            data-test="plan-metric"
            class="rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-700/60"
          >
            <p class="text-xs text-gray-400 dark:text-dark-400">{{ metric.label }}</p>
            <p :class="['mt-1 truncate text-sm font-semibold tabular-nums', metric.emphasis ? platformTextClass(displayPlatform(plan)) : 'text-gray-900 dark:text-white']">
              {{ metric.value }}
            </p>
          </div>
        </div>

        <ul v-if="featurePreview(plan).length > 0" class="mt-4 space-y-1.5">
          <li
            v-for="feature in featurePreview(plan)"
            :key="feature"
            class="flex items-start gap-2 text-sm text-gray-600 dark:text-gray-300"
          >
            <span class="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-gray-300 dark:bg-dark-500" />
            <span class="line-clamp-1">{{ feature }}</span>
          </li>
        </ul>

        <div class="mt-auto flex gap-2 pt-4">
          <button
            type="button"
            data-test="plan-grid-subscribe"
            :class="['min-h-10 flex-1 rounded-md px-3 py-2 text-sm font-semibold transition-colors', platformButtonClass(displayPlatform(plan))]"
            @click="emit('select', plan)"
          >
            {{ isRenewal(plan) ? t('payment.renewNow') : t('payment.subscribeNow') }}
          </button>
          <button
            type="button"
            data-test="plan-grid-details"
            class="min-h-10 rounded-md border border-gray-200 px-3 py-2 text-sm font-semibold text-gray-700 transition-colors hover:border-primary-300 hover:text-primary-600 dark:border-dark-600 dark:text-gray-200 dark:hover:border-primary-500/60 dark:hover:text-primary-300"
            @click="emit('details', plan)"
          >
            {{ t('common.view') }}
          </button>
        </div>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PaymentStorefrontTag, SubscriptionPlan } from '@/types/payment'
import type { UserSubscription } from '@/types'
import { currencySymbol } from '@/components/payment/currency'
import {
  platformBadgeClass,
  platformBorderClass,
  platformButtonClass,
  platformDiscountClass,
  platformLabel,
  platformTextClass,
} from '@/utils/platformColors'

const props = defineProps<{
  plans: SubscriptionPlan[]
  activeSubscriptions?: UserSubscription[]
  tags?: PaymentStorefrontTag[]
  defaultPlanId?: number | null
}>()
const emit = defineEmits<{ select: [plan: SubscriptionPlan]; details: [plan: SubscriptionPlan] }>()
const { t } = useI18n()

const recommendedPlan = computed(() => {
  if (props.defaultPlanId != null) {
    const configuredPlan = props.plans.find(plan => plan.id === props.defaultPlanId)
    if (configuredPlan) return configuredPlan
  }
  const monthlyPlans = props.plans
    .filter(plan => !isDailyPlan(plan))
    .sort((a, b) => a.price - b.price || (a.sort_order || 0) - (b.sort_order || 0) || a.id - b.id)
  return monthlyPlans[0] || props.plans[0] || null
})

function displayName(plan: SubscriptionPlan): string {
  return plan.product_name?.trim() || plan.name
}

function displayPlatform(plan: SubscriptionPlan): string {
  return plan.storefront_platform?.trim() || plan.group_platform || ''
}

function coverImageURL(plan: SubscriptionPlan): string {
  return plan.cover_image_url?.trim() || ''
}

function storefrontBadges(plan: SubscriptionPlan): string[] {
  const badges: string[] = []
  if (plan.storefront_featured) badges.push(t('payment.planCard.featured'))
  const badge = plan.storefront_badge?.trim()
  if (badge && !badges.includes(badge)) badges.push(badge)
  for (const tag of props.tags || []) {
    if (!tag.enabled || !tag.plan_ids?.includes(plan.id)) continue
    const label = tag.label?.trim()
    if (label && !badges.includes(label)) badges.push(label)
  }
  return badges
}

function secondaryBadges(plan: SubscriptionPlan): string[] {
  const defaultLabel = t('payment.planCard.featured')
  return storefrontBadges(plan)
    .filter(badge => !isDefaultPlan(plan) || badge !== defaultLabel)
    .slice(0, 3)
}

function planSummary(plan: SubscriptionPlan): string {
  return plan.description?.trim() || plan.detail_description?.trim().split('\n')[0] || ''
}

function priceLabel(plan: SubscriptionPlan, value: number): string {
  const code = plan.currency?.trim()
  return `${currencySymbol(code || 'USD')}${value}${code ? ` ${code}` : ''}`
}

function planValidityLabel(plan: SubscriptionPlan): string {
  const unit = plan.validity_unit || 'day'
  if (unit === 'month') return t('payment.perMonth')
  if (unit === 'year') return t('payment.perYear')
  return `${plan.validity_days}${t('payment.days')}`
}

function hasPositiveLimit(value: number | null | undefined): boolean {
  return typeof value === 'number' && value > 0
}

function planLimitLabel(value: number | null | undefined): string {
  return hasPositiveLimit(value) ? `$${value}` : t('payment.planCard.unlimited')
}

function planMetrics(plan: SubscriptionPlan): { label: string; value: string; emphasis?: boolean }[] {
  const items: { label: string; value: string; emphasis?: boolean }[] = []
  if (hasPositiveLimit(plan.daily_limit_usd)) {
    items.push({ label: t('payment.planCard.dailyLimit'), value: planLimitLabel(plan.daily_limit_usd), emphasis: true })
  }
  if (hasPositiveLimit(plan.weekly_limit_usd)) {
    items.push({ label: t('payment.planCard.weeklyLimit'), value: planLimitLabel(plan.weekly_limit_usd), emphasis: true })
  }
  if (hasPositiveLimit(plan.monthly_limit_usd)) {
    items.push({ label: t('payment.planCard.monthlyLimit'), value: planLimitLabel(plan.monthly_limit_usd), emphasis: true })
  }
  if (items.length === 0) {
    items.push({ label: t('payment.planCard.quota'), value: t('payment.planCard.unlimited') })
  }
  items.push({ label: t('payment.planCard.rate'), value: `x${Number((plan.rate_multiplier ?? 1).toPrecision(10))}` })
  return items.slice(0, 4)
}

function featurePreview(plan: SubscriptionPlan): string[] {
  const detailLines = plan.detail_description?.split('\n').map(line => line.trim()).filter(Boolean) || []
  return [...detailLines, ...(plan.features || [])].filter(Boolean).slice(0, 3)
}

function discountText(plan: SubscriptionPlan): string {
  if (!plan.original_price || plan.original_price <= 0) return ''
  const pct = Math.round((1 - plan.price / plan.original_price) * 100)
  return pct > 0 ? `-${pct}%` : ''
}

function isDailyPlan(plan: SubscriptionPlan): boolean {
  const category = plan.storefront_category?.trim().toLowerCase()
  const name = `${plan.product_name || ''} ${plan.name || ''}`.toLowerCase()
  return category === 'daily' || plan.validity_days === 1 || name.includes('日卡') || name.includes('daily')
}

function isRenewal(plan: SubscriptionPlan): boolean {
  return props.activeSubscriptions?.some(sub => sub.group_id === plan.group_id && sub.status === 'active') ?? false
}

function isDefaultPlan(plan: SubscriptionPlan): boolean {
  return recommendedPlan.value?.id === plan.id
}

function displayInitials(plan: SubscriptionPlan): string {
  return Array.from(displayName(plan).trim()).slice(0, 4).join('')
}

function planCardClass(plan: SubscriptionPlan): string[] {
  return [
    'flex min-h-[300px] flex-col rounded-lg border bg-white p-4 shadow-sm transition-[border-color,box-shadow,background-color] focus-within:ring-2 focus-within:ring-primary-500/20 dark:bg-dark-800',
    platformBorderClass(displayPlatform(plan)),
    isDefaultPlan(plan)
      ? 'ring-2 ring-primary-200 dark:ring-primary-500/20'
      : 'hover:border-primary-300 dark:hover:border-primary-500/60',
  ]
}
</script>
