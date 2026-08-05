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
      storefront_platform: 'image',
      storefront_category: 'image',
      storefront_featured: true,
      storefront_badge: 'Best Value',
    }))
  })

  it('falls back to group platform when editing old plans without storefront platform', async () => {
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

    expect(updatePlanMock).toHaveBeenCalledWith(9, expect.objectContaining({
      storefront_platform: 'openai',
      storefront_category: 'pro',
    }))
  })
})
