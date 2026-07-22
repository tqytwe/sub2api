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
  it('renders configured plans as a storefront spotlight with a selectable plan list', async () => {
    const wrapper = mount(SubscriptionPlanDecisionShelf, {
      props: {
        plans: [
          planFixture(1, { name: 'Monthly 100', price: 100, cover_image_url: '/assets/plans/100.webp' }),
          planFixture(2, { name: 'Monthly 29.9', price: 29.9, cover_image_url: '/assets/plans/29.webp' }),
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

    expect(wrapper.find('[data-test="plan-spotlight"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="plan-list-item"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="plan-grid"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="plan-grid-card"]').exists()).toBe(false)
    expect(wrapper.html()).not.toContain('aspect-[16/9]')
    expect(wrapper.html()).toContain('2xl:h-80')

    const listItems = wrapper.findAll('[data-test="plan-list-item"]')
    expect(listItems).toHaveLength(2)
    expect(listItems.map(item => item.text())).toEqual(
      expect.arrayContaining([
        expect.stringContaining('Monthly 100'),
        expect.stringContaining('Monthly 29.9'),
      ]),
    )

    expect(wrapper.find('[data-test="plan-spotlight"]').text()).toContain('Monthly 29.9')
    expect(wrapper.find('[data-test="plan-spotlight"]').text()).toContain('高性价比')
    expect(listItems[1].attributes('aria-pressed')).toBe('true')

    await listItems[0].trigger('click')
    expect(wrapper.find('[data-test="plan-spotlight"]').text()).toContain('Monthly 100')

    await wrapper.find('[data-test="plan-spotlight-details"]').trigger('click')
    await wrapper.find('[data-test="plan-spotlight-subscribe"]').trigger('click')
    expect(wrapper.emitted('details')?.[0]).toBeTruthy()
    expect(wrapper.emitted('select')?.[0]).toBeTruthy()
  })
})
