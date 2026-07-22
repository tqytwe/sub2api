<template>
  <section
    v-if="spotlightPlan"
    data-test="subscription-decision-shelf"
    class="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(380px,560px)]"
  >
    <article
      data-test="plan-spotlight"
      :class="[
        'overflow-hidden rounded-lg border bg-white shadow-sm dark:bg-dark-800',
        platformBorderClass(displayPlatform(spotlightPlan)),
      ]"
    >
      <div class="relative aspect-[16/9] overflow-hidden bg-gray-100 dark:bg-dark-700">
        <img
          v-if="coverImageURL(spotlightPlan)"
          :src="coverImageURL(spotlightPlan)"
          :alt="displayName(spotlightPlan)"
          class="h-full w-full object-cover"
        />
        <div
          v-else
          :class="[
            'flex h-full w-full items-center justify-center px-8 text-center text-3xl font-bold text-white',
            platformAccentBarClass(displayPlatform(spotlightPlan)),
          ]"
        >
          {{ displayName(spotlightPlan) }}
        </div>
        <div class="absolute left-4 top-4 flex flex-wrap gap-2">
          <span :class="['rounded-md border px-2 py-1 text-xs font-semibold shadow-sm backdrop-blur', platformBadgeClass(displayPlatform(spotlightPlan))]">
            {{ platformLabel(displayPlatform(spotlightPlan)) }}
          </span>
          <span
            v-for="badge in storefrontBadges(spotlightPlan)"
            :key="badge"
            class="rounded-md bg-gray-900/80 px-2 py-1 text-xs font-semibold text-white shadow-sm ring-1 ring-white/20 backdrop-blur dark:bg-white/90 dark:text-gray-900"
          >
            {{ badge }}
          </span>
        </div>
      </div>

      <div class="space-y-5 p-5">
        <div>
          <h3 class="text-2xl font-bold leading-tight text-gray-900 dark:text-white">{{ displayName(spotlightPlan) }}</h3>
          <p v-if="displayName(spotlightPlan) !== spotlightPlan.name" class="mt-1 text-sm text-gray-400 dark:text-dark-400">{{ spotlightPlan.name }}</p>
          <p v-if="planSummary(spotlightPlan)" class="mt-2 line-clamp-2 text-sm leading-relaxed text-gray-500 dark:text-gray-400">
            {{ planSummary(spotlightPlan) }}
          </p>
        </div>

        <div class="flex flex-wrap items-baseline gap-x-2 gap-y-1">
          <span v-if="spotlightPlan.original_price" class="text-sm text-gray-400 line-through dark:text-dark-500">
            {{ priceLabel(spotlightPlan, spotlightPlan.original_price) }}
          </span>
          <span :class="['text-4xl font-extrabold', platformTextClass(displayPlatform(spotlightPlan))]">
            {{ priceLabel(spotlightPlan, spotlightPlan.price) }}
          </span>
          <span class="text-sm text-gray-500 dark:text-gray-400">/ {{ planValidityLabel(spotlightPlan) }}</span>
          <span v-if="discountText(spotlightPlan)" :class="['rounded px-1.5 py-0.5 text-xs font-semibold', platformDiscountClass(displayPlatform(spotlightPlan))]">
            {{ discountText(spotlightPlan) }}
          </span>
        </div>

        <div class="grid gap-3 sm:grid-cols-2">
          <div
            v-for="metric in planMetrics(spotlightPlan)"
            :key="metric.label"
            class="rounded-lg bg-gray-50 p-3 dark:bg-dark-700/50"
          >
            <p class="text-xs text-gray-400 dark:text-dark-400">{{ metric.label }}</p>
            <p :class="['mt-1 text-2xl font-bold', metric.emphasis ? platformTextClass(displayPlatform(spotlightPlan)) : 'text-gray-900 dark:text-white']">
              {{ metric.value }}
            </p>
          </div>
        </div>

        <div v-if="featurePreview(spotlightPlan).length > 0" class="space-y-2">
          <div
            v-for="feature in featurePreview(spotlightPlan)"
            :key="feature"
            class="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-300"
          >
            <span :class="['mt-1.5 h-2 w-2 shrink-0 rounded-full', platformAccentBarClass(displayPlatform(spotlightPlan))]" />
            <span>{{ feature }}</span>
          </div>
        </div>

        <div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto]">
          <button
            type="button"
            data-test="plan-spotlight-subscribe"
            :class="['rounded-lg px-4 py-3 text-sm font-semibold transition-colors active:scale-[0.98]', platformButtonClass(displayPlatform(spotlightPlan))]"
            @click="emit('select', spotlightPlan)"
          >
            {{ isRenewal(spotlightPlan) ? t('payment.renewNow') : t('payment.subscribeNow') }}
          </button>
          <button
            type="button"
            data-test="plan-spotlight-details"
            class="rounded-lg border border-gray-200 px-4 py-3 text-sm font-semibold text-gray-700 transition-colors hover:border-primary-300 hover:text-primary-600 dark:border-dark-600 dark:text-gray-200 dark:hover:border-primary-500/60 dark:hover:text-primary-300"
            @click="emit('details', spotlightPlan)"
          >
            {{ t('common.view') }}
          </button>
        </div>
      </div>
    </article>

    <div class="space-y-3">
      <div class="flex items-center justify-between gap-3 px-1">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('payment.selectPlan') }}</h3>
        <span class="rounded-full bg-gray-100 px-2 py-1 text-xs font-medium text-gray-500 dark:bg-dark-700 dark:text-gray-300">{{ plans.length }}</span>
      </div>
      <div class="space-y-3 xl:max-h-[720px] xl:overflow-y-auto xl:pr-1">
        <button
          v-for="plan in plans"
          :key="plan.id"
          type="button"
          data-test="plan-list-item"
          :aria-pressed="plan.id === spotlightPlan.id"
          :class="[
            'grid w-full grid-cols-[88px_minmax(0,1fr)] gap-3 rounded-lg border bg-white p-3 text-left shadow-sm transition-colors dark:bg-dark-800 sm:grid-cols-[112px_minmax(0,1fr)_auto]',
            plan.id === spotlightPlan.id
              ? 'border-primary-400 ring-2 ring-primary-200 dark:border-primary-500/70 dark:ring-primary-500/20'
              : 'border-gray-200 hover:border-primary-300 dark:border-dark-700 dark:hover:border-primary-500/60',
          ]"
          @click="spotlightPlanId = plan.id"
        >
          <div class="relative h-20 overflow-hidden rounded-md bg-gray-100 dark:bg-dark-700">
            <img
              v-if="coverImageURL(plan)"
              :src="coverImageURL(plan)"
              :alt="displayName(plan)"
              class="h-full w-full object-cover"
            />
            <div
              v-else
              :class="['flex h-full w-full items-center justify-center px-2 text-center text-xs font-bold text-white', platformAccentBarClass(displayPlatform(plan))]"
            >
              {{ displayName(plan) }}
            </div>
          </div>

          <div class="min-w-0">
            <div class="mb-1 flex flex-wrap items-center gap-1.5">
              <span :class="['rounded border px-1.5 py-0.5 text-[10px] font-semibold', platformBadgeClass(displayPlatform(plan))]">
                {{ platformLabel(displayPlatform(plan)) }}
              </span>
              <span
                v-for="badge in storefrontBadges(plan)"
                :key="badge"
                class="rounded bg-gray-900 px-1.5 py-0.5 text-[10px] font-semibold text-white dark:bg-white dark:text-gray-900"
              >
                {{ badge }}
              </span>
            </div>
            <h4 class="truncate text-base font-bold text-gray-900 dark:text-white">{{ displayName(plan) }}</h4>
            <p v-if="planSummary(plan)" class="mt-1 line-clamp-2 text-xs leading-relaxed text-gray-500 dark:text-gray-400">{{ planSummary(plan) }}</p>
            <div class="mt-2 flex flex-wrap gap-1.5">
              <span
                v-for="chip in planChips(plan)"
                :key="chip"
                class="rounded-full bg-gray-100 px-2 py-1 text-[11px] font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300"
              >
                {{ chip }}
              </span>
            </div>
          </div>

          <div class="col-span-2 flex items-center justify-between gap-3 border-t border-gray-100 pt-3 dark:border-dark-700 sm:col-span-1 sm:block sm:border-l sm:border-t-0 sm:pl-4 sm:pt-0 sm:text-right">
            <div>
              <div :class="['text-xl font-extrabold', platformTextClass(displayPlatform(plan))]">{{ priceLabel(plan, plan.price) }}</div>
              <div class="text-xs text-gray-400 dark:text-dark-400">/ {{ planValidityLabel(plan) }}</div>
            </div>
            <span
              :class="[
                'inline-flex h-8 items-center rounded-full px-3 text-xs font-semibold sm:mt-3',
                plan.id === spotlightPlan.id
                  ? platformButtonClass(displayPlatform(plan))
                  : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300',
              ]"
            >
              {{ plan.id === spotlightPlan.id ? t('payment.planShelf.currentPreview') : t('common.view') }}
            </span>
          </div>
        </button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PaymentStorefrontTag, SubscriptionPlan } from '@/types/payment'
import type { UserSubscription } from '@/types'
import { currencySymbol } from '@/components/payment/currency'
import {
  platformAccentBarClass,
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

const spotlightPlanId = ref<number | null>(null)

const defaultSpotlightPlan = computed(() => {
  if (props.defaultPlanId) {
    const configuredPlan = props.plans.find(plan => plan.id === props.defaultPlanId)
    if (configuredPlan) return configuredPlan
  }
  const monthlyPlans = props.plans
    .filter(plan => !isDailyPlan(plan))
    .sort((a, b) => a.price - b.price || (a.sort_order || 0) - (b.sort_order || 0) || a.id - b.id)
  return monthlyPlans[0] || props.plans[0] || null
})

const spotlightPlan = computed(() =>
  props.plans.find(plan => plan.id === spotlightPlanId.value) || defaultSpotlightPlan.value
)

watch(
  () => `${props.defaultPlanId ?? ''}:${props.plans.map(plan => plan.id).join(',')}`,
  () => {
    if (!props.plans.some(plan => plan.id === spotlightPlanId.value)) {
      spotlightPlanId.value = defaultSpotlightPlan.value?.id ?? null
    }
  },
  { immediate: true },
)

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

function planChips(plan: SubscriptionPlan): string[] {
  const chips: string[] = []
  if (hasPositiveLimit(plan.daily_limit_usd)) chips.push(`${t('payment.planCard.dailyLimit')} ${planLimitLabel(plan.daily_limit_usd)}`)
  if (hasPositiveLimit(plan.weekly_limit_usd)) chips.push(`${t('payment.planCard.weeklyLimit')} ${planLimitLabel(plan.weekly_limit_usd)}`)
  if (hasPositiveLimit(plan.monthly_limit_usd)) chips.push(`${t('payment.planCard.monthlyLimit')} ${planLimitLabel(plan.monthly_limit_usd)}`)
  if (chips.length === 0) chips.push(t('payment.planCard.unlimited'))
  chips.push(`x${Number((plan.rate_multiplier ?? 1).toPrecision(10))}`)
  return chips.slice(0, 3)
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
</script>
