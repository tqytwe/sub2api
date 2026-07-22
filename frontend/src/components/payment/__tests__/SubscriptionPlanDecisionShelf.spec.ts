import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { createI18n } from 'vue-i18n'
import type { SubscriptionPlan } from '@/types/payment'
import SubscriptionPlanDecisionShelf from '../SubscriptionPlanDecisionShelf.vue'

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  fallbackWarn: false,
  missingWarn: false,
  messages: {
    en: {
      common: {
        view: 'View',
      },
      payment: {
        days: 'days',
        perMonth: 'month',
        perYear: 'year',
        renewNow: 'Renew',
        subscribeNow: 'Subscribe',
        selectPlan: 'Select Plan',
        planCard: {
          dailyLimit: 'Daily',
          featured: 'Recommended',
          monthlyLimit: 'Monthly',
          quota: 'Quota',
          rate: 'Rate',
          unlimited: 'Unlimited',
          weeklyLimit: 'Weekly',
        },
        planShelf: {
          currentPreview: 'Preview',
        },
      },
    },
  },
})

function planFixture(id: number, overrides: Partial<SubscriptionPlan> = {}): SubscriptionPlan {
  return {
    id,
    group_id: id,
    group_platform: 'openai',
    group_name: 'OpenAI',
    name: `Plan ${id}`,
    description: '',
    price: 10,
    original_price: 0,
    validity_days: 30,
    validity_unit: 'day',
    features: [],
    rate_multiplier: 1,
    daily_limit_usd: null,
    weekly_limit_usd: null,
    monthly_limit_usd: null,
    supported_model_scopes: [],
    product_name: '',
    cover_image_url: '',
    detail_description: '',
    storefront_platform: '',
    storefront_category: '',
    storefront_featured: false,
    storefront_badge: '',
    for_sale: true,
    sort_order: id,
    ...overrides,
  }
}

describe('SubscriptionPlanDecisionShelf', () => {
  it('uses the configured default plan as the spotlight and renders custom labels', () => {
    const wrapper = mount(SubscriptionPlanDecisionShelf, {
      props: {
        plans: [
          planFixture(1, { name: 'Monthly 100', price: 100 }),
          planFixture(2, { name: 'Monthly 29.9', price: 29.9 }),
        ],
        defaultPlanId: 2,
        tags: [
          { id: 'best-value', label: '高性价比', tone: 'success', enabled: true, sort_order: 1, plan_ids: [2] },
        ],
      },
      global: {
        plugins: [i18n],
      },
    })

    const spotlight = wrapper.find('[data-test="plan-spotlight"]')
    expect(spotlight.text()).toContain('Monthly 29.9')
    expect(spotlight.text()).toContain('高性价比')
  })
})
