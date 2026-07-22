import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import PlanStorefrontConfigPanel from '../PlanStorefrontConfigPanel.vue'
import type { PaymentStorefrontConfig, SubscriptionPlan } from '@/types/payment'

const getStorefrontConfig = vi.hoisted(() => vi.fn())
const updateStorefrontConfig = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())

vi.mock('@/api/admin/payment', () => ({
  adminPaymentAPI: {
    getStorefrontConfig,
    updateStorefrontConfig,
  },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (!params) return key
        return `${key}:${JSON.stringify(params)}`
      },
    }),
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

function planFixture(id: number, overrides: Partial<SubscriptionPlan> = {}): SubscriptionPlan {
  return {
    id,
    group_id: id,
    group_platform: 'openai',
    group_name: 'OpenAI',
    name: `Plan ${id}`,
    description: '',
    price: id * 10,
    original_price: 0,
    validity_days: 30,
    validity_unit: 'days',
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

describe('PlanStorefrontConfigPanel', () => {
  beforeEach(() => {
    getStorefrontConfig.mockReset()
    updateStorefrontConfig.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
  })

  it('adds shelves and labels, assigns plans, and saves the normalized payload', async () => {
    const initial: PaymentStorefrontConfig = {
      shelves: [],
      tags: [],
    }
    getStorefrontConfig.mockResolvedValue({ data: initial })
    updateStorefrontConfig.mockImplementation(async (payload: PaymentStorefrontConfig) => ({ data: payload }))

    const wrapper = mount(PlanStorefrontConfigPanel, {
      props: {
        plans: [
          planFixture(1, { name: 'Monthly 100', price: 100 }),
          planFixture(2, { name: 'Monthly 29.9', price: 29.9 }),
        ],
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.findAll('button').find(button => button.text().includes('payment.admin.addShelf'))!.trigger('click')
    await wrapper.find('input[placeholder="payment.admin.shelfLabelPlaceholder"]').setValue('月卡')
    await wrapper.findAll('input[type="checkbox"]')[2].setValue(true)
    await wrapper.find('select').setValue(2)

    await wrapper.findAll('button').find(button => button.text().includes('payment.admin.addTag'))!.trigger('click')
    const tagInputs = wrapper.findAll('input[placeholder="payment.admin.tagLabelPlaceholder"]')
    await tagInputs[0].setValue('高性价比')
    const allCheckboxes = wrapper.findAll('input[type="checkbox"]')
    await allCheckboxes[allCheckboxes.length - 1].setValue(true)

    await wrapper.findAll('button').find(button => button.text().includes('common.save'))!.trigger('click')
    await flushPromises()

    const payload = updateStorefrontConfig.mock.calls[0][0] as PaymentStorefrontConfig
    expect(payload.shelves[0]).toMatchObject({
      label: '月卡',
      enabled: true,
      plan_ids: [2],
      default_plan_id: 2,
      sort_order: 1,
    })
    expect(payload.tags[0]).toMatchObject({
      label: '高性价比',
      enabled: true,
      plan_ids: [2],
      sort_order: 1,
    })
    expect(showSuccess).toHaveBeenCalledWith('payment.admin.storefrontConfigSaved')
  })
})
