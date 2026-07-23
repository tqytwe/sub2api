import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import APIOnboardingConfigPanel from '../APIOnboardingConfigPanel.vue'
import type { APIOnboardingConfig, AdminGroup } from '@/types'
import type { SubscriptionPlan } from '@/types/payment'

const getAPIOnboardingConfig = vi.hoisted(() => vi.fn())
const updateAPIOnboardingConfig = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())

vi.mock('@/api/admin/payment', () => ({
  adminPaymentAPI: {
    getAPIOnboardingConfig,
    updateAPIOnboardingConfig,
  },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
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
    name: `Plan ${id}`,
    description: '',
    price: 10,
    validity_days: 30,
    validity_unit: 'days',
    features: [],
    for_sale: true,
    sort_order: id,
    ...overrides,
  }
}

function groupFixture(id: number): AdminGroup {
  return {
    id,
    name: `Group ${id}`,
    description: null,
    platform: 'anthropic',
    rate_multiplier: 1,
    is_exclusive: false,
    status: 'active',
    subscription_type: 'token',
    daily_limit_usd: null,
    weekly_limit_usd: null,
    monthly_limit_usd: null,
    allow_image_generation: false,
    allow_batch_image_generation: false,
    image_rate_independent: false,
    image_rate_multiplier: 1,
    batch_image_discount_multiplier: 1,
    batch_image_hold_multiplier: 1,
    image_price_1k: null,
    image_price_2k: null,
    image_price_4k: null,
    video_rate_independent: false,
    video_rate_multiplier: 1,
    video_price_480p: null,
    video_price_720p: null,
    video_price_1080p: null,
    web_search_price_per_call: null,
    peak_rate_enabled: false,
    peak_start: '',
    peak_end: '',
    peak_rate_multiplier: 1,
    claude_code_only: false,
    fallback_group_id: null,
    fallback_group_id_on_invalid_request: null,
    require_oauth_only: false,
    require_privacy_set: false,
    created_at: '2026-07-23T00:00:00Z',
    updated_at: '2026-07-23T00:00:00Z',
    model_routing: null,
    model_routing_enabled: false,
    mcp_xml_inject: false,
    sort_order: id,
  }
}

function mountPanel(initial: APIOnboardingConfig = { enabled: false, title: '', subtitle: '', items: [] }) {
  getAPIOnboardingConfig.mockResolvedValue({ data: initial })
  updateAPIOnboardingConfig.mockImplementation(async (payload: APIOnboardingConfig) => ({ data: payload }))

  return mount(APIOnboardingConfigPanel, {
    props: {
      plans: [planFixture(7, { name: 'Pro Monthly', product_name: 'Pro Monthly' })],
      groups: [groupFixture(42)],
    },
    global: {
      stubs: {
        Icon: true,
      },
    },
  })
}

const getButtonByText = (wrapper: ReturnType<typeof mount>, text: string) => {
  const button = wrapper.findAll('button').find(item => item.text().includes(text))
  if (!button) {
    throw new Error(`Button not found: ${text}`)
  }
  return button
}

describe('APIOnboardingConfigPanel', () => {
  beforeEach(() => {
    getAPIOnboardingConfig.mockReset()
    updateAPIOnboardingConfig.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
  })

  it('adds a recommendation card and saves the normalized payload', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    await getButtonByText(wrapper, 'payment.admin.apiOnboarding.addCard').trigger('click')
    await wrapper.find('input[type="checkbox"]').setValue(true)
    await wrapper.find('input[placeholder="payment.admin.apiOnboarding.panelTitlePlaceholder"]').setValue('API Key 新手接入')
    await wrapper.find('input[placeholder="payment.admin.apiOnboarding.panelSubtitlePlaceholder"]').setValue('先选分组再创建 Key')

    const titleInputs = wrapper.findAll('input[placeholder="payment.admin.apiOnboarding.cardTitlePlaceholder"]')
    await titleInputs[titleInputs.length - 1].setValue('购买 Pro 月卡')

    const selects = wrapper.findAll('select')
    await selects[0].setValue('buy_plan')
    await selects[1].setValue('all_users')
    await selects[2].setValue('42')
    await selects[3].setValue('7')

    await getButtonByText(wrapper, 'common.save').trigger('click')
    await flushPromises()

    const payload = updateAPIOnboardingConfig.mock.calls[0][0] as APIOnboardingConfig
    expect(payload).toMatchObject({
      enabled: true,
      title: 'API Key 新手接入',
      subtitle: '先选分组再创建 Key',
    })
    expect(payload.items[0]).toMatchObject({
      title: '购买 Pro 月卡',
      enabled: true,
      sort_order: 1,
      group_id: 42,
      plan_id: 7,
      cta: 'buy_plan',
      audience: 'all_users',
    })
    expect(showSuccess).toHaveBeenCalledWith('payment.admin.apiOnboarding.saved')
  })

  it('blocks saving a buy-plan card without a plan', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    await getButtonByText(wrapper, 'payment.admin.apiOnboarding.addCard').trigger('click')
    await wrapper.findAll('select')[0].setValue('buy_plan')
    await getButtonByText(wrapper, 'common.save').trigger('click')
    await flushPromises()

    expect(updateAPIOnboardingConfig).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('payment.admin.apiOnboarding.validation.planRequired')
  })
})
