<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PlayCouponReward } from '@/api/play'
import { formatCurrency, formatDateTime } from '@/utils/format'

const props = defineProps<{
  coupon: PlayCouponReward
}>()

const { t } = useI18n()

const benefitLabel = computed(() => {
  if (props.coupon.benefit_type === 'percentage') {
    return props.coupon.max_discount_amount
      ? t('coupon.reward.percentCapped', {
          percent: props.coupon.benefit_value,
          cap: formatCurrency(props.coupon.max_discount_amount, props.coupon.currency),
        })
      : t('coupon.reward.percent', { percent: props.coupon.benefit_value })
  }
  return t('coupon.reward.fixed', { amount: formatCurrency(props.coupon.benefit_value, props.coupon.currency) })
})

const hasRechargeScope = computed(() => props.coupon.applicable_scopes.includes('balance'))
const hasSubscriptionScope = computed(() => props.coupon.applicable_scopes.includes('subscription'))
const scopeLabel = computed(() => {
  if (hasRechargeScope.value && hasSubscriptionScope.value) return t('coupon.reward.scopeBoth')
  return hasSubscriptionScope.value ? t('coupon.reward.scopeSubscription') : t('coupon.reward.scopeRecharge')
})
const isPendingActivation = computed(() => {
  const validFrom = Date.parse(props.coupon.valid_from)
  return Number.isFinite(validFrom) && validFrom > Date.now()
})
</script>

<template>
  <section class="mt-4 border-l-2 border-primary-500 bg-primary-50/60 p-4 text-sm dark:bg-primary-500/10" aria-live="polite">
    <p class="font-semibold text-gray-900 dark:text-white">{{ coupon.name }}</p>
    <p class="mt-1 font-medium text-primary-700 dark:text-primary-300">{{ benefitLabel }}</p>
    <dl class="mt-3 grid gap-1 text-xs text-gray-600 dark:text-gray-300">
      <div class="flex flex-wrap justify-between gap-2"><dt>{{ t('coupon.reward.scope') }}</dt><dd>{{ scopeLabel }}</dd></div>
      <div class="flex flex-wrap justify-between gap-2"><dt>{{ t('coupon.reward.minimum') }}</dt><dd>{{ formatCurrency(coupon.minimum_order_amount, coupon.currency) }}</dd></div>
      <div v-if="isPendingActivation" class="flex flex-wrap justify-between gap-2"><dt>{{ t('coupon.reward.availableAt') }}</dt><dd>{{ formatDateTime(coupon.valid_from) }}</dd></div>
      <div class="flex flex-wrap justify-between gap-2"><dt>{{ t('coupon.reward.expiresAt') }}</dt><dd>{{ formatDateTime(coupon.expires_at) }}</dd></div>
    </dl>
    <div class="mt-4 flex flex-wrap gap-2">
      <p v-if="isPendingActivation" class="w-full text-xs text-amber-700 dark:text-amber-200">{{ t('coupon.reward.pendingHint') }}</p>
      <router-link v-if="hasRechargeScope && !isPendingActivation" :to="{ path: '/purchase', query: { tab: 'recharge' } }" class="play-btn play-btn-secondary">
        {{ t('coupon.reward.goRecharge') }}
      </router-link>
      <router-link v-if="hasSubscriptionScope && !isPendingActivation" :to="{ path: '/purchase', query: { tab: 'subscription' } }" class="play-btn play-btn-primary">
        {{ t('coupon.reward.goSubscription') }}
      </router-link>
    </div>
  </section>
</template>
