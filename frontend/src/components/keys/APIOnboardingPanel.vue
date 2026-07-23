<template>
  <section v-if="visibleItems.length > 0" data-test="api-onboarding-panel" class="w-full rounded-lg border border-gray-200 bg-white p-4 text-left dark:border-dark-700 dark:bg-dark-900">
    <div class="mb-4 flex items-start gap-3">
      <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary-50 text-primary-600 dark:bg-primary-500/10 dark:text-primary-300">
        <Icon name="key" size="md" />
      </div>
      <div class="min-w-0">
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ configTitle }}</h3>
        <p class="mt-1 text-sm leading-6 text-gray-500 dark:text-dark-400">{{ configSubtitle }}</p>
      </div>
    </div>

    <div class="grid gap-3 lg:grid-cols-3">
      <article
        v-for="item in visibleItems"
        :key="item.id"
        class="flex min-h-[188px] flex-col rounded-lg border border-gray-200 bg-white p-4 transition-colors hover:border-primary-200 dark:border-dark-700 dark:bg-dark-900 dark:hover:border-primary-500/40"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="flex items-center gap-2">
            <span class="flex h-8 w-8 items-center justify-center rounded-lg bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-dark-300">
              <Icon :name="item.icon" size="sm" />
            </span>
            <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ item.title }}</h4>
          </div>
          <span v-if="item.badge" class="shrink-0 rounded-md border border-primary-100 bg-primary-50 px-2 py-0.5 text-xs font-medium text-primary-700 dark:border-primary-500/30 dark:bg-primary-500/10 dark:text-primary-300">
            {{ item.badge }}
          </span>
        </div>

        <p class="mt-3 line-clamp-3 text-sm leading-6 text-gray-500 dark:text-dark-400">{{ item.description || defaultDescription(item.cta) }}</p>

        <div class="mt-4 space-y-2 text-xs text-gray-500 dark:text-dark-400">
          <div v-if="item.group" class="flex items-center gap-2">
            <span>{{ t('keys.apiOnboarding.recommendedGroup') }}</span>
            <GroupBadge
              :name="item.group.name"
              :platform="item.group.platform"
              :subscription-type="item.group.subscription_type"
              :rate-multiplier="item.group.rate_multiplier"
            />
          </div>
          <div v-if="item.plan" class="flex items-center justify-between gap-3 rounded-md bg-gray-50 px-2 py-1.5 dark:bg-dark-800">
            <span class="min-w-0 truncate">{{ planDisplayName(item.plan) }}</span>
            <span class="shrink-0 font-medium text-gray-700 dark:text-dark-200">{{ planPrice(item.plan) }}</span>
          </div>
          <p v-if="balanceHint(item)" class="rounded-md bg-amber-50 px-2 py-1.5 text-amber-700 dark:bg-amber-500/10 dark:text-amber-300">
            {{ balanceHint(item) }}
          </p>
        </div>

        <div class="mt-auto pt-4">
          <button type="button" class="btn w-full" :class="item.primary ? 'btn-primary' : 'btn-secondary'" @click="runAction(item)">
            <Icon :name="item.actionIcon" size="sm" />
            {{ item.actionLabel }}
          </button>
        </div>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { APIOnboardingConfig, APIOnboardingItem, Group } from '@/types'
import type { SubscriptionPlan } from '@/types/payment'
import Icon from '@/components/icons/Icon.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import { currencySymbol } from '@/components/payment/currency'

type IconName = 'key' | 'creditCard' | 'dollar' | 'book' | 'arrowRight'

interface VisibleItem extends APIOnboardingItem {
  icon: IconName
  actionIcon: IconName
  actionLabel: string
  primary: boolean
  group?: Group
  plan?: SubscriptionPlan
}

const props = withDefaults(defineProps<{
  config?: APIOnboardingConfig | null
  groups: Group[]
  plans: SubscriptionPlan[]
  balance: number
  docUrl?: string
  isNewUser?: boolean
}>(), {
  config: null,
  docUrl: '',
  isNewUser: true,
})

const emit = defineEmits<{
  createKey: [groupId: number | null]
  recharge: []
  buyPlan: [plan: SubscriptionPlan]
  openDocs: []
}>()

const { t } = useI18n()

const configTitle = computed(() => props.config?.title?.trim() || t('keys.apiOnboarding.title'))
const configSubtitle = computed(() => props.config?.subtitle?.trim() || t('keys.apiOnboarding.subtitle'))

const visibleItems = computed<VisibleItem[]>(() => {
  if (!props.config?.enabled) return []
  const groupById = new Map(props.groups.map(group => [group.id, group]))
  const planById = new Map(props.plans.map(plan => [plan.id, plan]))
  return (props.config.items || [])
    .filter(item => item.enabled !== false)
    .filter(item => item.audience !== 'new_users' || props.isNewUser)
    .map((item, index) => resolveItem(item, index, groupById, planById))
    .filter((item): item is VisibleItem => item !== null)
    .sort((a, b) => (a.sort_order || 0) - (b.sort_order || 0))
})

function resolveItem(
  item: APIOnboardingItem,
  index: number,
  groupById: Map<number, Group>,
  planById: Map<number, SubscriptionPlan>,
): VisibleItem | null {
  const cta = normalizeCTA(item.cta)
  const groupId = Number(item.group_id || 0)
  const planId = Number(item.plan_id || 0)
  const group = groupId > 0 ? groupById.get(groupId) : undefined
  const plan = planId > 0 ? planById.get(planId) : undefined

  if (cta === 'create_key' && groupId > 0 && !group) return null
  if (cta === 'buy_plan' && !plan) return null
  if (cta === 'open_docs' && !props.docUrl) return null

  return {
    ...item,
    cta,
    id: item.id || `api-onboarding-${index + 1}`,
    title: item.title || defaultTitle(cta),
    description: item.description || '',
    badge: item.badge || '',
    sort_order: item.sort_order || index + 1,
    group_id: group?.id ?? null,
    plan_id: plan?.id ?? null,
    min_balance: Math.max(0, Number(item.min_balance || 0)),
    audience: item.audience === 'all_users' ? 'all_users' : 'new_users',
    icon: iconForCTA(cta),
    actionIcon: actionIconForCTA(cta),
    actionLabel: actionLabel(cta),
    primary: index === 0 || cta === 'create_key',
    group,
    plan,
  }
}

function normalizeCTA(value: string): APIOnboardingItem['cta'] {
  return ['create_key', 'recharge', 'buy_plan', 'open_docs'].includes(value) ? value : 'create_key'
}

function iconForCTA(cta: string): IconName {
  switch (cta) {
    case 'buy_plan':
      return 'creditCard'
    case 'recharge':
      return 'dollar'
    case 'open_docs':
      return 'book'
    default:
      return 'key'
  }
}

function actionIconForCTA(cta: string): IconName {
  return cta === 'create_key' ? 'key' : 'arrowRight'
}

function defaultTitle(cta: string): string {
  switch (cta) {
    case 'buy_plan':
      return t('keys.apiOnboarding.defaultBuyPlanTitle')
    case 'recharge':
      return t('keys.apiOnboarding.defaultRechargeTitle')
    case 'open_docs':
      return t('keys.apiOnboarding.defaultDocsTitle')
    default:
      return t('keys.apiOnboarding.defaultCreateTitle')
  }
}

function defaultDescription(cta: string): string {
  switch (cta) {
    case 'buy_plan':
      return t('keys.apiOnboarding.defaultBuyPlanDescription')
    case 'recharge':
      return t('keys.apiOnboarding.defaultRechargeDescription')
    case 'open_docs':
      return t('keys.apiOnboarding.defaultDocsDescription')
    default:
      return t('keys.apiOnboarding.defaultCreateDescription')
  }
}

function actionLabel(cta: string): string {
  switch (cta) {
    case 'buy_plan':
      return t('keys.apiOnboarding.buyPlan')
    case 'recharge':
      return t('keys.apiOnboarding.recharge')
    case 'open_docs':
      return t('keys.apiOnboarding.openDocs')
    default:
      return t('keys.apiOnboarding.createKey')
  }
}

function balanceHint(item: VisibleItem): string {
  const required = Number(item.min_balance || 0)
  if (required <= 0 || props.balance >= required) return ''
  return t('keys.apiOnboarding.balanceHint', {
    balance: props.balance.toFixed(2),
    required: required.toFixed(2),
  })
}

function planDisplayName(plan: SubscriptionPlan): string {
  return plan.product_name?.trim() || plan.name
}

function planPrice(plan: SubscriptionPlan): string {
  const symbol = currencySymbol(plan.currency || 'USD')
  return `${symbol}${Number(plan.price || 0).toFixed(2)}`
}

function runAction(item: VisibleItem) {
  switch (item.cta) {
    case 'buy_plan':
      if (item.plan) emit('buyPlan', item.plan)
      break
    case 'recharge':
      emit('recharge')
      break
    case 'open_docs':
      emit('openDocs')
      break
    default:
      emit('createKey', item.group?.id ?? null)
  }
}
</script>
