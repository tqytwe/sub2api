import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AdminPlayBillingConfigView from '../AdminPlayBillingConfigView.vue'

const getPlayBillingConfigMock = vi.hoisted(() => vi.fn())
const updatePlayBillingConfigMock = vi.hoisted(() => vi.fn())
const getPlansMock = vi.hoisted(() => vi.fn())
const showErrorMock = vi.hoisted(() => vi.fn())
const showSuccessMock = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'payment.admin.playBilling.saved') return 'saved'
        if (key === 'payment.admin.playBilling.validation.fixBeforeSave') return 'fix errors'
        if (key === 'payment.admin.playBilling.validation.productRequired') return 'product required'
        if (key === 'payment.admin.playBilling.validation.amountRequired') return 'amount required'
        if (key === 'payment.admin.playBilling.validation.planRequired') return 'plan required'
        if (key === 'payment.admin.balanceOrder') return 'Balance'
        if (key === 'payment.admin.subscriptionOrder') return 'Subscription'
        return params ? `${key}:${JSON.stringify(params)}` : key
      },
    }),
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: showSuccessMock,
  }),
}))

vi.mock('@/api/admin/payment', () => ({
  adminPaymentAPI: {
    getPlayBillingConfig: getPlayBillingConfigMock,
    updatePlayBillingConfig: updatePlayBillingConfigMock,
    getPlans: getPlansMock,
  },
  default: {
    getPlayBillingConfig: getPlayBillingConfigMock,
    updatePlayBillingConfig: updatePlayBillingConfigMock,
    getPlans: getPlansMock,
  },
}))

function plan(id: number, name = `Plan ${id}`) {
  return {
    id,
    group_id: id,
    name,
    description: '',
    price: 10,
    currency: 'USD',
    validity_days: 30,
    validity_unit: 'days',
    features: [],
    for_sale: true,
    sort_order: id,
  }
}

function mountView() {
  return mount(AdminPlayBillingConfigView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        DataTable: {
          props: ['data'],
          template: `
            <div>
              <slot v-if="!data || data.length === 0" name="empty" />
              <div v-for="row in data" :key="row.local_id" data-test="mapping-row">
                <slot name="cell-enabled" :row="row" :value="row.enabled" />
                <slot name="cell-product_id" :row="row" :value="row.product_id" />
                <slot name="cell-product_type" :row="row" :value="row.product_type" />
                <slot name="cell-order_type" :row="row" :value="row.order_type" />
                <slot name="cell-title" :row="row" :value="row.title" />
                <slot name="cell-entitlement" :row="row" :value="row.amount" />
                <slot name="cell-pay_amount" :row="row" :value="row.pay_amount" />
                <slot name="cell-consumable" :row="row" :value="row.consumable" />
                <slot name="cell-actions" :row="row" />
              </div>
            </div>
          `,
        },
        Select: {
          props: ['modelValue', 'options'],
          emits: ['update:modelValue', 'change'],
          template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', isNaN(Number($event.target.value)) ? $event.target.value : Number($event.target.value)); $emit(\'change\', $event.target.value)"><option v-for="option in options" :key="String(option.value)" :value="option.value">{{ option.label }}</option></select>',
        },
        Toggle: {
          props: ['modelValue'],
          emits: ['update:modelValue'],
          template: '<button type="button" data-test="toggle" @click="$emit(\'update:modelValue\', !modelValue)">{{ modelValue ? "on" : "off" }}</button>',
        },
        Icon: true,
      },
    },
  })
}

describe('AdminPlayBillingConfigView', () => {
  beforeEach(() => {
    getPlayBillingConfigMock.mockReset().mockResolvedValue({
      data: {
        package_name: 'com.jisudeng.chat',
        service_account_configured: true,
        config_source: 'settings',
        product_count: 1,
        enabled_product_count: 1,
        products: [{
          product_id: 'balance_10_usd',
          product_type: 'inapp',
          order_type: 'balance',
          amount: 10,
          pay_amount: 9.99,
          currency: 'USD',
          title: 'Balance 10',
          formatted_price: 'US$9.99',
          consumable: true,
          enabled: true,
        }],
        public_products: [],
      },
    })
    updatePlayBillingConfigMock.mockReset().mockImplementation(async payload => ({
      data: {
        package_name: 'com.jisudeng.chat',
        service_account_configured: true,
        config_source: 'settings',
        product_count: payload.products.length,
        enabled_product_count: payload.products.filter((item: { enabled?: boolean }) => item.enabled !== false).length,
        products: payload.products,
        public_products: [],
      },
    }))
    getPlansMock.mockReset().mockResolvedValue({ data: [plan(7, 'Claude Pro')] })
    showErrorMock.mockReset()
    showSuccessMock.mockReset()
  })

  it('loads current mappings and saves backend-owned entitlement mapping', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('com.jisudeng.chat')
    expect((wrapper.find('[data-test="play-billing-product-id"]').element as HTMLInputElement).value).toBe('balance_10_usd')

    await wrapper.find('[data-test="save-play-billing"]').trigger('click')
    await flushPromises()

    expect(updatePlayBillingConfigMock).toHaveBeenCalledWith({
      products: [expect.objectContaining({
        product_id: 'balance_10_usd',
        product_type: 'inapp',
        order_type: 'balance',
        amount: 10,
        pay_amount: 9.99,
        currency: 'USD',
        consumable: true,
        enabled: true,
      })],
    })
    expect(showSuccessMock).toHaveBeenCalledWith('saved')
  })

  it('validates a newly added balance mapping before saving', async () => {
    getPlayBillingConfigMock.mockResolvedValueOnce({
      data: {
        package_name: 'com.jisudeng.chat',
        service_account_configured: true,
        config_source: 'settings',
        product_count: 0,
        enabled_product_count: 0,
        products: [],
        public_products: [],
      },
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-test="add-play-billing-product"]').trigger('click')
    await flushPromises()
    await wrapper.find('[data-test="save-play-billing"]').trigger('click')
    await flushPromises()

    expect(updatePlayBillingConfigMock).not.toHaveBeenCalled()
    expect(showErrorMock).toHaveBeenCalledWith('fix errors')
    expect(wrapper.text()).toContain('product required')
    expect(wrapper.text()).toContain('amount required')
  })
})
