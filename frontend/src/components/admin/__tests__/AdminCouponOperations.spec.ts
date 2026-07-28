import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AdminCouponOperations from '../AdminCouponOperations.vue'

const listTemplates = vi.hoisted(() => vi.fn())
const listUserCoupons = vi.hoisted(() => vi.fn())
const listPools = vi.hoisted(() => vi.fn())
const issueBatch = vi.hoisted(() => vi.fn())
const voidUserCoupon = vi.hoisted(() => vi.fn())
const createTemplate = vi.hoisted(() => vi.fn())
const createPool = vi.hoisted(() => vi.fn())
const deletePool = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())

vi.mock('@/api/admin', () => ({
  adminAPI: {
    coupon: {
      listTemplates,
      listUserCoupons,
      listPools,
      issueBatch,
      voidUserCoupon,
      createTemplate,
      createPool,
      deletePool,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

function mountIssuePanel() {
  return mount(AdminCouponOperations, {
    props: { activeTab: 'issue' },
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        ConfirmDialog: true,
        DataTable: {
          props: ['data'],
          template: '<div><template v-for="row in data" :key="row.id"><slot name="cell-actions" :row="row" /></template></div>',
        },
        Pagination: {
          name: 'Pagination',
          emits: ['update:page', 'update:pageSize'],
          template: '<div data-test="coupon-pagination" />',
        },
        Icon: true,
      },
    },
  })
}

function mountTemplatePanel() {
  return mount(AdminCouponOperations, {
    props: { activeTab: 'templates' },
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        ConfirmDialog: true,
        DataTable: { template: '<div><slot /></div>' },
        Pagination: {
          name: 'Pagination',
          emits: ['update:page', 'update:pageSize'],
          template: '<div data-test="template-pagination" />',
        },
        Icon: true,
      },
    },
  })
}

function mountPoolPanel(activeTab: 'blindbox' | 'quiz' = 'blindbox') {
  return mount(AdminCouponOperations, {
    props: { activeTab },
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        ConfirmDialog: {
          props: ['show'],
          emits: ['confirm', 'cancel'],
          template: '<div v-if="show"><button type="button" data-test="confirm-delete-pool" @click="$emit(\'confirm\')" /></div>',
        },
        DataTable: {
          props: ['data'],
          template: '<div><template v-for="row in data" :key="row.id"><slot name="cell-actions" :row="row" /></template></div>',
        },
        Icon: true,
      },
    },
  })
}

describe('AdminCouponOperations', () => {
  beforeEach(() => {
    listTemplates.mockReset().mockResolvedValue({
      data: {
        items: [
          { id: 3, name: 'Recharge 5', status: 'active' },
          { id: 4, name: 'Recharge 10', status: 'active' },
        ],
        total: 2,
      },
    })
    listUserCoupons.mockReset().mockResolvedValue({ data: { items: [], total: 0 } })
    listPools.mockReset().mockResolvedValue({ data: [] })
    issueBatch.mockReset().mockResolvedValue({ data: { id: 11 } })
    voidUserCoupon.mockReset().mockResolvedValue({ data: { id: 31, status: 'voided' } })
    createTemplate.mockReset().mockResolvedValue({ data: { id: 41 } })
    createPool.mockReset().mockResolvedValue({ data: { id: 91 } })
    deletePool.mockReset().mockResolvedValue({})
    showSuccess.mockReset()
    showError.mockReset()
    vi.spyOn(globalThis.crypto, 'randomUUID').mockReturnValue('11111111-1111-4111-8111-111111111111')
  })

  it('sends a stable idempotency key required by the batch issue endpoint', async () => {
    const wrapper = mountIssuePanel()
    await flushPromises()

    await wrapper.find('select').setValue('3')
    await wrapper.find('input').setValue('7, 8, 7')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(issueBatch).toHaveBeenCalledWith({
      template_id: 3,
      user_ids: [7, 8],
      source: 'admin_batch',
      idempotency_key: 'coupon-batch-11111111-1111-4111-8111-111111111111',
    })
  })

  it('uses CNY as the default currency for a new coupon template', async () => {
    const wrapper = mountTemplatePanel()
    await flushPromises()

    expect((wrapper.get('[data-test="coupon-template-currency"]').element as HTMLInputElement).value).toBe('CNY')
  })

  it('clears relative-days fields when saving an end-of-day template', async () => {
    const wrapper = mountTemplatePanel()
    await flushPromises()

    const inputs = wrapper.findAll('#coupon-template-form input')
    await inputs[0].setValue('same-day-recharge')
    await inputs[1].setValue('Same-day recharge')
    await wrapper.get('[data-test="coupon-template-validity-mode"]').setValue('end_of_day')
    await wrapper.get('#coupon-template-form').trigger('submit')
    await flushPromises()

    expect(createTemplate).toHaveBeenCalledWith(expect.objectContaining({
      validity_mode: 'end_of_day',
      validity_days: 0,
      fixed_expires_at: null,
    }))
  })

  it('searches and paginates coupon templates through the backend', async () => {
    listTemplates.mockResolvedValue({
      data: {
        items: [{ id: 3, name: 'Recharge 5', status: 'active' }],
        total: 101,
        page: 1,
        page_size: 50,
        pages: 3,
      },
    })
    const wrapper = mountTemplatePanel()
    await flushPromises()

    expect(listTemplates).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 50,
      search: undefined,
    })

    await wrapper.get('[data-test="template-search"]').setValue('summer')
    await wrapper.get('[data-test="template-search-form"]').trigger('submit')
    await flushPromises()

    expect(listTemplates).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 50,
      search: 'summer',
    })

    wrapper.findComponent({ name: 'Pagination' }).vm.$emit('update:page', 2)
    await flushPromises()
    expect(listTemplates).toHaveBeenLastCalledWith({
      page: 2,
      page_size: 50,
      search: 'summer',
    })
  })

  it('loads every template page for batch issuing and issued-coupon filters', async () => {
    listTemplates.mockImplementation(({ page, page_size }) => {
      expect(page_size).toBe(1000)
      if (page === 1) {
        return Promise.resolve({
          data: {
            items: [{ id: 3, name: 'First template', status: 'active' }],
            total: 1001,
            page: 1,
            page_size: 1000,
            pages: 2,
          },
        })
      }
      return Promise.resolve({
        data: {
          items: [{ id: 1001, name: 'Later template', status: 'active' }],
          total: 1001,
          page: 2,
          page_size: 1000,
          pages: 2,
        },
      })
    })
    const wrapper = mountIssuePanel()
    await flushPromises()

    expect(listTemplates).toHaveBeenNthCalledWith(1, { page: 1, page_size: 1000 })
    expect(listTemplates).toHaveBeenNthCalledWith(2, { page: 2, page_size: 1000 })
    expect(wrapper.get('form select').find('option[value="1001"]').text()).toBe('Later template')
    expect(wrapper.get('[data-test="coupon-template-filter"]').find('option[value="1001"]').text()).toBe('Later template')
  })

  it.each(['blindbox', 'quiz'] as const)('loads every active template page for the %s pool selector', async (activeTab) => {
    listTemplates.mockImplementation(({ page, page_size }) => {
      expect(page_size).toBe(1000)
      if (page === 1) {
        return Promise.resolve({
          data: {
            items: [{ id: 3, name: 'First template', status: 'active' }],
            total: 1001,
            page: 1,
            page_size: 1000,
            pages: 2,
          },
        })
      }
      return Promise.resolve({
        data: {
          items: [{ id: 1001, name: 'Later template', status: 'active' }],
          total: 1001,
          page: 2,
          page_size: 1000,
          pages: 2,
        },
      })
    })
    const wrapper = mountPoolPanel(activeTab)
    await flushPromises()

    await wrapper.get('button.btn-primary').trigger('click')
    expect(wrapper.get('[data-test="pool-fallback-template"]').find('option[value="1001"]').text()).toBe('Later template')
  })

  it('voids an available issued coupon with an audit reason', async () => {
    listUserCoupons.mockResolvedValue({
      data: {
        items: [{
          id: 31,
          template_id: 3,
          template_name: 'Recharge 5',
          user_id: 9,
          status: 'available',
          source: 'admin_batch',
          issued_at: '2026-07-27T08:00:00.000Z',
          valid_from: '2026-07-27T08:00:00.000Z',
          expires_at: '2026-07-30T08:00:00.000Z',
          terms_snapshot: { name: 'Recharge 5' },
        }],
        total: 1,
      },
    })
    const wrapper = mountIssuePanel()
    await flushPromises()

    await wrapper.get('[data-test="void-user-coupon"]').trigger('click')
    await wrapper.get('[data-test="void-coupon-reason"]').setValue('Manual correction')
    await wrapper.get('#void-user-coupon-form').trigger('submit')
    await flushPromises()

    expect(voidUserCoupon).toHaveBeenCalledWith(31, 'Manual correction')
    expect(showSuccess).toHaveBeenCalledWith('coupon.admin.voided')
  })

  it('filters and paginates issued coupons through the existing admin API', async () => {
    listUserCoupons.mockResolvedValue({
      data: {
        items: [],
        total: 101,
        page: 1,
        page_size: 50,
        pages: 3,
      },
    })
    const wrapper = mountIssuePanel()
    await flushPromises()

    await wrapper.get('[data-test="coupon-user-filter"]').setValue('9')
    await wrapper.get('[data-test="coupon-template-filter"]').setValue('3')
    await wrapper.get('[data-test="coupon-status-filter"]').setValue('locked')
    await wrapper.findAll('form')[1].trigger('submit')
    await flushPromises()

    expect(listUserCoupons).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 50,
      user: '9',
      template_id: 3,
      source: undefined,
      status: 'locked',
      issued_from: undefined,
      issued_to: undefined,
    })

    wrapper.findComponent({ name: 'Pagination' }).vm.$emit('update:page', 2)
    await flushPromises()
    expect(listUserCoupons).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2 }))
  })

  it('keeps fallback outside ordinary weights and forces its fixed value when saving a copied draft', async () => {
    listPools.mockResolvedValue({
      data: [{
        id: 8,
        activity: 'blindbox',
        version: 'blindbox-july',
        status: 'published',
        coupon_weight_bp: 6000,
        balance_weight_bp: 4000,
        fallback_template_id: 3,
        entries: [
          {
            id: 70,
            template_id: 4,
            weight_bp: 10000,
            enabled: true,
            starts_at: null,
            ends_at: null,
            issued_count: 18,
            sort_order: 1,
          },
          {
            id: 71,
            template_id: 3,
            weight_bp: 2500,
            enabled: true,
            starts_at: '2026-07-27T08:00:00.000Z',
            ends_at: '2026-07-30T08:00:00.000Z',
            issued_count: 18,
            sort_order: 2,
          },
        ],
      }],
    })
    const wrapper = mountPoolPanel()
    await flushPromises()

    await wrapper.get('[data-test="copy-immutable-pool"]').trigger('click')

    const versionInput = wrapper.get('#coupon-pool-form input')
    expect((versionInput.element as HTMLInputElement).value).toBe('blindbox-july-draft-1')
    const startInput = wrapper.findAll('[data-test="pool-entry-start"]')[1].element as HTMLInputElement
    const endInput = wrapper.findAll('[data-test="pool-entry-end"]')[1].element as HTMLInputElement
    expect(startInput.value).toBe('')
    expect(endInput.value).toBe('')
    expect(startInput.disabled).toBe(true)
    expect(endInput.disabled).toBe(true)
    expect((wrapper.findAll('[data-test="pool-entry-stock-cap"]')[1].element as HTMLInputElement).disabled).toBe(true)
    expect((wrapper.findAll('[data-test="pool-entry-per-user-limit"]')[1].element as HTMLInputElement).disabled).toBe(true)

    const ordinaryWeight = wrapper.findAll('[data-test="pool-entry-weight"]')[0]
    const fallbackWeight = wrapper.findAll('[data-test="pool-entry-weight"]')[1]
    expect((ordinaryWeight.element as HTMLInputElement).value).toBe('10000')
    expect(ordinaryWeight.attributes('disabled')).toBeUndefined()
    expect((fallbackWeight.element as HTMLInputElement).value).toBe('1')
    expect(fallbackWeight.attributes('disabled')).toBeDefined()
    expect(fallbackWeight.classes()).toContain('w-28')

    const saveButton = wrapper.get('button[form="coupon-pool-form"]')
    expect(saveButton.attributes('disabled')).toBeUndefined()
    await wrapper.get('#coupon-pool-form').trigger('submit')
    await flushPromises()
    expect(createPool).toHaveBeenCalledWith(expect.objectContaining({
      fallback_template_id: 3,
      entries: expect.arrayContaining([
        expect.objectContaining({ template_id: 4, weight_bp: 10000 }),
        expect.objectContaining({ template_id: 3, weight_bp: 1 }),
      ]),
    }))
    expect(wrapper.text()).toContain('coupon.admin.fallbackHint')
    expect(wrapper.text()).toContain('coupon.admin.weightTotal')
    expect(wrapper.text()).not.toContain('coupon.admin.editPool')
  })

  it('creates a protected fallback entry as soon as an operator selects its template', async () => {
    const wrapper = mountPoolPanel()
    await flushPromises()

    await wrapper.get('button.btn-primary').trigger('click')
    await wrapper.get('[data-test="pool-fallback-template"]').setValue('3')
    await flushPromises()

    const entries = wrapper.findAll('#coupon-pool-form tbody tr')
    expect(entries).toHaveLength(1)
    expect((entries[0].find('select').element as HTMLSelectElement).value).toBe('3')
    const weight = entries[0].get('[data-test="pool-entry-weight"]')
    expect((weight.element as HTMLInputElement).value).toBe('1')
    expect(weight.attributes('disabled')).toBeDefined()
  })

  it('only exposes deletion for drafts and deletes a draft after confirmation', async () => {
    listPools.mockResolvedValue({
      data: [
        { id: 21, activity: 'blindbox', version: 'draft-pool', status: 'draft', coupon_weight_bp: 6000, balance_weight_bp: 4000, fallback_template_id: 3, entries: [] },
        { id: 22, activity: 'blindbox', version: 'published-pool', status: 'published', coupon_weight_bp: 6000, balance_weight_bp: 4000, fallback_template_id: 3, entries: [] },
        { id: 23, activity: 'blindbox', version: 'retired-pool', status: 'retired', coupon_weight_bp: 6000, balance_weight_bp: 4000, fallback_template_id: 3, entries: [] },
      ],
    })
    const wrapper = mountPoolPanel()
    await flushPromises()

    const deleteButtons = wrapper.findAll('[data-test="delete-draft-pool"]')
    expect(deleteButtons).toHaveLength(1)
    expect(wrapper.findAll('[data-test="edit-draft-pool"]')).toHaveLength(1)
    expect(wrapper.findAll('[data-test="publish-draft-pool"]')).toHaveLength(1)
    expect(wrapper.findAll('[data-test="copy-immutable-pool"]')).toHaveLength(2)
    expect(deletePool).not.toHaveBeenCalled()

    await deleteButtons[0].trigger('click')
    expect(deletePool).not.toHaveBeenCalled()
    await wrapper.get('[data-test="confirm-delete-pool"]').trigger('click')
    await flushPromises()

    expect(deletePool).toHaveBeenCalledWith(21)
    expect(showSuccess).toHaveBeenCalledWith('coupon.admin.poolDeleted')
    expect(listPools).toHaveBeenCalledTimes(2)
  })
})
