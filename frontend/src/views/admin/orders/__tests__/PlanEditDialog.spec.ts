import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PlanEditDialog from '../PlanEditDialog.vue'
import type { SubscriptionPlan } from '@/types/payment'

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

function mountDialog(paymentConfig: Record<string, unknown> | null, plan: SubscriptionPlan | null = null) {
  return mount(PlanEditDialog, {
    props: {
      show: true,
      plan,
      groups: plan
        ? [{
            id: plan.group_id,
            name: 'OpenAI Pro',
            platform: 'openai',
            rate_multiplier: 1,
            subscription_type: 'subscription',
          }]
        : [],
      paymentConfig,
    },
    global: {
      stubs: {
        BaseDialog: {
          props: ['show'],
          template: '<div v-if="show"><slot /><slot name="footer" /></div>',
        },
        Select: true,
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

describe('PlanEditDialog subscription CNY payment preview', () => {
  it('shows CNY channel charge using the configured subscription rate and fee', async () => {
    const wrapper = mountDialog({
      subscription_usd_to_cny_rate: 7.15,
      recharge_fee_rate: 2.5,
    })

    await wrapper.find('input[type="number"]').setValue('9.99')

    expect(wrapper.text()).toContain('preview')
    expect(wrapper.text()).toContain('¥71.43')
    expect(wrapper.text()).toContain('fee 2.5')
    expect(wrapper.text()).toContain('¥73.22')
  })

  it('hides the preview when the subscription rate is not configured', async () => {
    const wrapper = mountDialog({
      subscription_usd_to_cny_rate: 0,
      recharge_fee_rate: 2.5,
    })

    await wrapper.find('input[type="number"]').setValue('9.99')

    expect(wrapper.text()).not.toContain('preview')
    expect(wrapper.text()).not.toContain('¥71.43')
  })
})

describe('PlanEditDialog product display fields', () => {
  it('saves product name, cover image URL, uploaded cover image, and detail description', async () => {
    updatePlanMock.mockReset().mockResolvedValue({})
    const wrapper = mountDialog(null, {
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
    })

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
    const wrapper = mountDialog(null, {
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
    })

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
    const wrapper = mountDialog(null, {
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
    })

    await wrapper.find('form').trigger('submit')

    expect(updatePlanMock).toHaveBeenCalledWith(9, expect.objectContaining({
      storefront_platform: 'openai',
      storefront_category: 'pro',
    }))
  })
})
