import { flushPromises, mount, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AdminPaymentPlansView from '../AdminPaymentPlansView.vue'

const getPlansMock = vi.hoisted(() => vi.fn())
const updatePlanMock = vi.hoisted(() => vi.fn())
const getConfigMock = vi.hoisted(() => vi.fn())
const getGroupsMock = vi.hoisted(() => vi.fn())
const showErrorMock = vi.hoisted(() => vi.fn())
const showSuccessMock = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'payment.admin.selectedPlans') return `selected ${params?.count}`
        if (key === 'payment.admin.batchUpdateSuccess') return `updated ${params?.count}`
        if (key === 'payment.planShelf.platforms.anthropic') return 'Claude'
        return key
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
    getPlans: getPlansMock,
    updatePlan: updatePlanMock,
    getConfig: getConfigMock,
    deletePlan: vi.fn(),
  },
}))

vi.mock('@/api/admin', () => ({
  default: {
    groups: {
      getAll: getGroupsMock,
    },
  },
}))

function plan(id: number, overrides: Record<string, unknown> = {}) {
  return {
    id,
    group_id: id,
    name: `Plan ${id}`,
    description: '',
    price: 10,
    original_price: 0,
    currency: '',
    validity_days: 30,
    validity_unit: 'days',
    features: [],
    product_name: '',
    cover_image_url: '',
    detail_description: '',
    storefront_platform: 'openai',
    storefront_category: 'pro',
    storefront_featured: false,
    storefront_badge: '',
    for_sale: true,
    sort_order: id,
    ...overrides,
  }
}

function mountWithStubs(dataTableStub: Record<string, unknown>) {
  return mount(AdminPaymentPlansView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        DataTable: dataTableStub,
        ConfirmDialog: true,
        GroupBadge: true,
        Icon: true,
        PlanEditDialog: true,
        Select: true,
      },
    },
  })
}

describe('AdminPaymentPlansView', () => {
  beforeEach(() => {
    getPlansMock.mockReset()
    updatePlanMock.mockReset().mockResolvedValue({})
    getConfigMock.mockReset().mockResolvedValue({ data: {} })
    getGroupsMock.mockReset().mockResolvedValue([])
    showErrorMock.mockReset()
    showSuccessMock.mockReset()
  })

  it('uses the configured currency symbol and keeps legacy prices in USD', async () => {
    getPlansMock.mockResolvedValue({
      data: [
        plan(1, { name: 'CNY plan', price: 499, original_price: 599, currency: 'CNY' }),
        plan(2, { name: 'Legacy plan', price: 10, currency: '' }),
      ],
    })

    const wrapper = mountWithStubs({
      props: ['data'],
      template: `
        <div>
          <div v-for="row in data" :key="row.id">
            <slot name="cell-price" :value="row.price" :row="row" />
          </div>
        </div>
      `,
    })
    await flushPromises()

    expect(wrapper.text()).toContain('¥499.00CNY')
    expect(wrapper.text()).toContain('¥599.00')
    expect(wrapper.text()).toContain('$10.00')
  })

  it('filters plans and sends batch storefront updates for selected rows', async () => {
    getPlansMock.mockResolvedValue({
      data: [
        plan(1, { name: 'OpenAI Daily', storefront_category: 'daily' }),
        plan(2, { name: 'Claude Pro', storefront_platform: 'anthropic' }),
      ],
    })

    const wrapper = shallowMount(AdminPaymentPlansView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DataTable: {
            props: ['data'],
            emits: ['update:selectedKeys'],
            template: '<div><div v-for="row in data" :key="row.id" data-test="row">{{ row.name }}<slot name="cell-storefront_platform" :row="row" :value="row.storefront_platform" /></div><button data-test="select" @click="$emit(\'update:selectedKeys\', [1, 2])">select</button></div>',
          },
          Select: true,
          Icon: true,
          GroupBadge: true,
          PlanEditDialog: true,
          ConfirmDialog: true,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    await wrapper.find('input[type="search"]').setValue('daily')
    const rows = wrapper.findAll('[data-test="row"]')
    expect(rows).toHaveLength(1)
    expect(rows[0].text()).toContain('OpenAI Daily')

    await wrapper.find('[data-test="select"]').trigger('click')
    await flushPromises()
    const batchButtons = wrapper.findAll('button').filter(button => button.text() === 'payment.admin.batchFeatured')
    expect(batchButtons.length).toBe(1)
    await batchButtons[0].trigger('click')
    await flushPromises()

    expect(updatePlanMock).toHaveBeenCalledWith(1, { storefront_featured: true })
    expect(updatePlanMock).toHaveBeenCalledWith(2, { storefront_featured: true })
    expect(showSuccessMock).toHaveBeenCalledWith('updated 2')
  })

  it('falls back to group platform for old plans without storefront platform', async () => {
    getPlansMock.mockResolvedValue({
      data: [
        plan(42, { name: 'Legacy Claude Plan', storefront_platform: '' }),
      ],
    })
    getGroupsMock.mockResolvedValue([
      { id: 42, name: 'Claude Group', platform: 'anthropic', rate_multiplier: 1, subscription_type: 'subscription' },
    ])

    const wrapper = shallowMount(AdminPaymentPlansView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DataTable: {
            props: ['data'],
            template: '<div><div v-for="row in data" :key="row.id" data-test="row"><slot name="cell-storefront_platform" :row="row" :value="row.storefront_platform" /></div></div>',
          },
          Select: true,
          Icon: true,
          GroupBadge: true,
          PlanEditDialog: true,
          ConfirmDialog: true,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(wrapper.find('[data-test="row"]').text()).toContain('Claude')
  })
})
