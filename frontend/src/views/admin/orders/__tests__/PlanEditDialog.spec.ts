import { describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'

import PlanEditDialog from '../PlanEditDialog.vue'
import type { SubscriptionPlan } from '@/types/payment'
import type { AdminGroup } from '@/types'

const createPlanMock = vi.hoisted(() => vi.fn())
const updatePlanMock = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      if (key === 'payment.admin.subscriptionCnyPayPreview') return `preview ${params?.amount}`
      if (key === 'payment.admin.subscriptionCnyPayPreviewWithFee') return `fee ${params?.feeRate} ${params?.total}`
      return key
    },
  }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('@/api/admin/payment', () => ({
  adminPaymentAPI: {
    createPlan: createPlanMock,
    updatePlan: updatePlanMock,
  },
}))

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: {
    show: Boolean,
    title: String,
    width: String,
  },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
})

const SelectStub = defineComponent({
  name: 'SelectStub',
  props: {
    modelValue: [String, Number],
    options: {
      type: Array,
      default: () => [],
    },
    placeholder: String,
  },
  emits: ['update:modelValue'],
  setup(_props, { emit }) {
    const onChange = (event: Event) => {
      const value = (event.target as HTMLSelectElement).value
      emit('update:modelValue', value === '' ? null : Number(value))
    }
    return { onChange }
  },
  template: `
    <select
      :value="modelValue ?? ''"
      @change="onChange"
    >
      <option value="">{{ placeholder }}</option>
      <option
        v-for="option in options"
        :key="option.value"
        :value="option.value"
        :data-platform="option.platform"
      >
        {{ option.label }}
      </option>
    </select>
  `,
})

const groupFixture = (overrides: Partial<AdminGroup>): AdminGroup => ({
  id: 1,
  name: 'OpenAI',
  description: null,
  platform: 'openai',
  rate_multiplier: 1,
  rpm_limit: 0,
  is_exclusive: false,
  status: 'active',
  subscription_type: 'subscription',
  daily_limit_usd: null,
  weekly_limit_usd: null,
  monthly_limit_usd: null,
  allow_image_generation: false,
  image_rate_independent: false,
  image_rate_multiplier: 1,
  image_price_1k: null,
  image_price_2k: null,
  image_price_4k: null,
  peak_rate_enabled: false,
  peak_start: '',
  peak_end: '',
  peak_rate_multiplier: 1,
  claude_code_only: false,
  fallback_group_id: null,
  fallback_group_id_on_invalid_request: null,
  allow_messages_dispatch: false,
  require_oauth_only: false,
  require_privacy_set: false,
  created_at: '2026-07-01T00:00:00Z',
  updated_at: '2026-07-01T00:00:00Z',
  model_routing: null,
  model_routing_enabled: false,
  mcp_xml_inject: false,
  sort_order: 0,
  ...overrides,
} as AdminGroup)

function mountDialog({
  groups = [],
  paymentConfig = null,
  plan = null,
}: {
  groups?: AdminGroup[]
  paymentConfig?: Record<string, unknown> | null
  plan?: SubscriptionPlan | null
} = {}) {
  const resolvedGroups = groups.length > 0
    ? groups
    : plan
      ? [groupFixture({
          id: plan.group_id,
          name: 'OpenAI Pro',
          platform: plan.group_platform || 'openai',
          subscription_type: 'subscription',
        })]
      : []
  return mount(PlanEditDialog, {
    props: {
      show: true,
      plan,
      groups: resolvedGroups,
      paymentConfig,
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        Select: SelectStub,
        Icon: true,
        GroupBadge: true,
        ImageUpload: {
          props: ['modelValue'],
          emits: ['update:modelValue'],
          template: '<button class="image-upload-stub" type="button" @click="$emit(\'update:modelValue\', \'data:image/png;base64,QUJD\')">upload</button>',
        },
      },
    },
  })
}

describe('PlanEditDialog', () => {
  it('shows CNY channel charge using the configured subscription rate and fee', async () => {
    const wrapper = mountDialog({
      paymentConfig: {
        subscription_usd_to_cny_rate: 7.15,
        recharge_fee_rate: 2.5,
      },
    })

    await wrapper.find('input[type="number"]').setValue('9.99')

    expect(wrapper.text()).toContain('preview')
    expect(wrapper.text()).toContain('¥71.43')
    expect(wrapper.text()).toContain('fee 2.5')
    expect(wrapper.text()).toContain('¥73.22')
  })

  it('hides the preview when the subscription rate is not configured', async () => {
    const wrapper = mountDialog({
      paymentConfig: {
        subscription_usd_to_cny_rate: 0,
        recharge_fee_rate: 2.5,
      },
    })

    await wrapper.find('input[type="number"]').setValue('9.99')

    expect(wrapper.text()).not.toContain('preview')
    expect(wrapper.text()).not.toContain('¥71.43')
  })

  it('allows composite subscription groups for payment plans', () => {
    const wrapper = mountDialog({
      groups: [
        groupFixture({
          id: 10,
          name: 'OpenAI + Claude + Gemini + Grok',
          platform: 'composite',
          rate_multiplier: 1.2,
          subscription_type: 'subscription',
        }),
        groupFixture({
          id: 11,
          name: 'Standard OpenAI',
          platform: 'openai',
          subscription_type: 'standard',
        }),
      ],
    })

    const options = wrapper.findAll('option').map(option => option.text())

    expect(options).toContain('OpenAI + Claude + Gemini + Grok — composite (1.2x)')
    expect(options).not.toContain('Standard OpenAI — openai (1x)')
  })
})

describe('PlanEditDialog product display fields', () => {
	it('saves the three independent package limits', async () => {
		updatePlanMock.mockReset().mockResolvedValue({})
		const wrapper = mountDialog({ plan: {
			id: 6, group_id: 3, name: 'Package', description: 'Short copy', price: 39,
			validity_days: 30, validity_unit: 'days', features: [], for_sale: true, sort_order: 1,
			request_limit: 10000, amount_limit_usd: 700, token_limit: 100000000,
		} })

		await wrapper.find('[data-test="plan-request-limit"]').setValue('12000')
		await wrapper.find('[data-test="plan-amount-limit"]').setValue('750')
		await wrapper.find('[data-test="plan-token-limit"]').setValue('120000000')
		await wrapper.find('form').trigger('submit')

		expect(updatePlanMock).toHaveBeenCalledWith(6, {
			request_limit: 12000,
			amount_limit_usd: 750,
			token_limit: 120000000,
		})
	})

	it('explicitly clears removed package limits', async () => {
		updatePlanMock.mockReset().mockResolvedValue({})
		const wrapper = mountDialog({ plan: {
			id: 8, group_id: 3, name: 'Package', description: '', price: 39,
			validity_days: 30, validity_unit: 'days', features: [], for_sale: true, sort_order: 1,
			request_limit: 10000, amount_limit_usd: 700, token_limit: 100000000,
		} })

		await wrapper.find('[data-test="plan-request-limit"]').setValue('')
		await wrapper.find('[data-test="plan-amount-limit"]').setValue('')
		await wrapper.find('[data-test="plan-token-limit"]').setValue('')
		await wrapper.find('form').trigger('submit')

		expect(updatePlanMock).toHaveBeenCalledWith(8, expect.objectContaining({
			request_limit: null,
			amount_limit_usd: null,
			token_limit: null,
			clear_request_limit: true,
			clear_amount_limit_usd: true,
			clear_token_limit: true,
		}))
	})

	it('saves product name, cover image URL, uploaded cover image, and detail description', async () => {
    updatePlanMock.mockReset().mockResolvedValue({})
    const wrapper = mountDialog({ plan: {
      id: 7,
      group_id: 3,
      name: 'Starter',
      description: 'Short copy',
      price: 9.99,
      original_price: 0,
      currency: '',
      validity_days: 30,
      validity_unit: 'days',
      features: ['Priority models'],
      product_name: '',
      cover_image_url: '',
      detail_description: '',
      for_sale: true,
      sort_order: 1,
    } })

    await wrapper.find('[data-test="plan-product-name"]').setValue('GPT Pro Workbench')
    await wrapper.find('[data-test="plan-cover-image-url"]').setValue('/assets/plans/pro.webp')
    await wrapper.find('.image-upload-stub').trigger('click')
    await wrapper.find('[data-test="plan-detail-description"]').setValue('Line one\nLine two')
    await wrapper.find('form').trigger('submit')

    expect(updatePlanMock).toHaveBeenCalledWith(7, expect.objectContaining({
      product_name: 'GPT Pro Workbench',
      cover_image_url: 'data:image/png;base64,QUJD',
      detail_description: 'Line one\nLine two',
    }))
  })

  it('saves storefront shelf fields with the plan payload', async () => {
    updatePlanMock.mockReset().mockResolvedValue({})
    const wrapper = mountDialog({ plan: {
      id: 8,
      group_id: 3,
      name: 'Image Day Pass',
      description: 'Short copy',
      price: 4.99,
      original_price: 0,
      currency: '',
      validity_days: 1,
      validity_unit: 'days',
      features: [],
      product_name: '',
      cover_image_url: '',
      detail_description: '',
      storefront_platform: 'image',
      storefront_category: 'image',
      storefront_featured: true,
      storefront_badge: 'Hot',
      for_sale: true,
      sort_order: 1,
    } })

    await wrapper.find('[data-test="plan-storefront-badge"]').setValue('Best Value')
    await wrapper.find('form').trigger('submit')

    expect(updatePlanMock).toHaveBeenCalledWith(8, expect.objectContaining({
      storefront_badge: 'Best Value',
    }))
  })

  it('does not persist inferred storefront defaults when editing a blank legacy plan', async () => {
    updatePlanMock.mockReset().mockResolvedValue({})
    const wrapper = mountDialog({ plan: {
      id: 9,
      group_id: 3,
      name: 'Legacy OpenAI Pro',
      description: 'Short copy',
      price: 9.99,
      original_price: 0,
      currency: '',
      validity_days: 30,
      validity_unit: 'days',
      features: [],
      product_name: '',
      cover_image_url: '',
      detail_description: '',
      storefront_platform: '',
      storefront_category: '',
      storefront_featured: false,
      storefront_badge: '',
      for_sale: true,
      sort_order: 1,
    } })

    await wrapper.find('form').trigger('submit')

    expect(updatePlanMock).toHaveBeenCalledWith(9, {})
  })

  it.each([false, true])('submits only edits with missing legacy fields: %s', async (legacy) => {
    updatePlanMock.mockReset().mockResolvedValue({})
    const plan: SubscriptionPlan = {
      id: 20, group_id: 3, name: 'Existing plan', description: 'Original',
      price: 19.99, validity_days: 30, validity_unit: 'days',
      features: [' Feature with spaces '], for_sale: true, sort_order: 4,
      ...(!legacy ? {
        cover_image_url: '/kept.webp', detail_description: ' Detail with spaces ',
        storefront_platform: 'image' as const, storefront_category: 'enterprise' as const,
        storefront_featured: true, storefront_badge: 'Badge', currency: 'USD',
        original_price: 39, request_limit: 123, amount_limit_usd: 4.56, token_limit: 789,
      } : {}),
    }
    const wrapper = mountDialog({ plan })
    await wrapper.find('textarea').setValue('Changed description')
    await wrapper.find('form').trigger('submit')
    expect(updatePlanMock).toHaveBeenCalledWith(20, { description: 'Changed description' })
    expect(plan.description).toBe('Original')
  })

  it('preserves blank storefront values through reopen and cancels without writes', async () => {
    updatePlanMock.mockReset().mockResolvedValue({})
    const plan: SubscriptionPlan = {
      id: 21, group_id: 3, name: 'Existing plan', description: 'Original', price: 19.99,
      validity_days: 30, validity_unit: 'days', features: [], for_sale: false, sort_order: 0,
      storefront_platform: '', storefront_category: '', storefront_featured: false,
    }
    const wrapper = mountDialog({ plan })
    await wrapper.find('textarea').setValue('Cancelled')
    await wrapper.find('.btn-secondary').trigger('click')
    expect(updatePlanMock).not.toHaveBeenCalled()
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe('Original')
    await wrapper.find('form').trigger('submit')
    expect(updatePlanMock).toHaveBeenCalledWith(21, {})
  })

  it('sends explicit empty strings, false and zero while omitting untouched fields', async () => {
    updatePlanMock.mockReset().mockResolvedValue({})
    const wrapper = mountDialog({ plan: {
      id: 22, group_id: 3, name: 'Existing plan', description: 'Original', price: 19.99,
      validity_days: 30, validity_unit: 'days', features: [], for_sale: true, sort_order: 4,
      cover_image_url: '/kept.webp', detail_description: 'Details', storefront_badge: 'Badge',
      storefront_featured: true, original_price: 39,
    } })
    await wrapper.find('[data-test="plan-cover-image-url"]').setValue('')
    await wrapper.find('[data-test="plan-detail-description"]').setValue('')
    await wrapper.find('[data-test="plan-storefront-badge"]').setValue('')
    const toggles = wrapper.findAll('button').filter(button => button.classes().includes('rounded-full'))
    await toggles[0].trigger('click')
    await toggles[1].trigger('click')
    const numbers = wrapper.findAll('input[type="number"]')
    await numbers[1].setValue('0')
    await numbers[numbers.length - 1].setValue('0')
    await wrapper.find('form').trigger('submit')
    expect(updatePlanMock).toHaveBeenCalledWith(22, {
      cover_image_url: '', detail_description: '', storefront_badge: '', storefront_featured: false,
      for_sale: false, original_price: 0, sort_order: 0,
    })
  })

  it('rejects an invalid price without writing', async () => {
    updatePlanMock.mockReset()
    const wrapper = mountDialog({ plan: {
      id: 23, group_id: 3, name: 'Existing plan', description: 'Original', price: 19.99,
      validity_days: 30, validity_unit: 'days', features: [], for_sale: true, sort_order: 4,
    } })
    await wrapper.find('input[type="number"]').setValue('-1')
    await wrapper.find('form').trigger('submit')
    expect(updatePlanMock).not.toHaveBeenCalled()
  })

  it('resets the edit baseline when reopening a different group with blank shelves', async () => {
    updatePlanMock.mockReset().mockResolvedValue({})
    const first: SubscriptionPlan = {
      id: 24, group_id: 3, name: 'First plan', description: 'First', price: 19.99,
      validity_days: 30, validity_unit: 'days', features: [], for_sale: true, sort_order: 4,
      storefront_platform: 'openai', storefront_category: 'pro',
    }
    const wrapper = mountDialog({ plan: first, groups: [
      groupFixture({ id: 3, platform: 'openai' }),
      groupFixture({ id: 4, platform: 'gemini' }),
    ] })
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true, plan: {
      ...first, id: 25, group_id: 4, description: 'Second',
      storefront_platform: '', storefront_category: '',
    } })
    const selects = wrapper.findAllComponents(SelectStub)
    expect(selects[1].props('modelValue')).toBe('')
    expect(selects[2].props('modelValue')).toBe('')
    await wrapper.find('textarea').setValue('Changed second description')
    await wrapper.find('form').trigger('submit')
    expect(updatePlanMock).toHaveBeenCalledWith(25, { description: 'Changed second description' })
  })
})
