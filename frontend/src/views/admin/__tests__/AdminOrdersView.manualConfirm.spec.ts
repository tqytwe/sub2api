import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AdminOrdersView from '../orders/AdminOrdersView.vue'

const {
  getOrders,
  manualConfirm,
  showError,
  showSuccess,
  runStepUp,
  stepUpCancelled,
} = vi.hoisted(() => ({
  getOrders: vi.fn(),
  manualConfirm: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  runStepUp: vi.fn(),
  stepUpCancelled: new Error('step-up cancelled'),
}))

vi.mock('@/api/admin/payment', () => {
  const paymentAPI = {
    getOrders,
    getOrder: vi.fn(),
    cancelOrder: vi.fn(),
    retryRecharge: vi.fn(),
    manualConfirm,
    refundOrder: vi.fn(),
    queryRefund: vi.fn(),
  }
  return { adminPaymentAPI: paymentAPI, default: paymentAPI }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

vi.mock('@/composables/useStepUp', () => ({
  useStepUp: () => ({ run: runStepUp, visible: { value: false } }),
  isStepUpCancelled: (error: unknown) => error === stepUpCancelled,
  isStepUpBlocked: () => false,
  stepUpBlockReason: () => '',
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const pendingOrder = {
  id: 91,
  user_id: 42,
  amount: 50.5,
  pay_amount: 50.5,
  currency: 'CNY',
  payment_currency: 'CNY',
  status: 'PENDING',
  payment_type: 'bepusdt',
  out_trade_no: 'sub2_pending',
  created_at: '2026-09-08T00:00:00Z',
  expires_at: '2026-09-08T01:00:00Z',
  refund_amount: 0,
}

function mountView() {
  return mount(AdminOrdersView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        Pagination: true,
        Select: true,
        OrderStatusBadge: true,
        AdminRefundDialog: true,
        TotpStepUpDialog: true,
        BaseDialog: {
          props: ['show', 'title'],
          template: '<section v-if="show"><h2>{{ title }}</h2><slot /><slot name="footer" /></section>',
        },
        OrderTable: {
          props: ['orders'],
          template: '<div><slot v-if="orders && orders.length" name="actions" :row="orders[0]" /></div>',
        },
      },
    },
  })
}

describe('AdminOrdersView manual payment confirmation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getOrders.mockResolvedValue({ data: { items: [pendingOrder], total: 1 } })
  })

  it('retries only the original confirmation after STEP_UP_REQUIRED', async () => {
    manualConfirm
      .mockRejectedValueOnce({ code: 'STEP_UP_REQUIRED' })
      .mockResolvedValueOnce({ data: { fulfillment_pending: false } })
    runStepUp.mockImplementation(async (action: () => Promise<unknown>) => {
      try {
        return await action()
      } catch (error) {
        expect(error).toEqual({ code: 'STEP_UP_REQUIRED' })
        return action()
      }
    })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="manual-payment-confirm-action"]').trigger('click')
    await wrapper.get('#manual-payment-reference').setValue('gateway-trade-091')
    await wrapper.get('[data-testid="manual-payment-confirm-submit"]').trigger('click')
    await flushPromises()

    expect(runStepUp).toHaveBeenCalledTimes(1)
    expect(manualConfirm).toHaveBeenCalledTimes(2)
    expect(manualConfirm).toHaveBeenNthCalledWith(1, 91, {
      gateway_transaction_reference: 'gateway-trade-091',
    })
    expect(showSuccess).toHaveBeenCalledWith('payment.admin.manualConfirm.success')
    expect(wrapper.find('#manual-payment-reference').exists()).toBe(false)
  })

  it('keeps the confirmation form and avoids an error toast when TOTP is cancelled', async () => {
    runStepUp.mockRejectedValueOnce(stepUpCancelled)

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="manual-payment-confirm-action"]').trigger('click')
    await wrapper.get('#manual-payment-reference').setValue('gateway-trade-092')
    await wrapper.get('[data-testid="manual-payment-confirm-submit"]').trigger('click')
    await flushPromises()

    expect(manualConfirm).not.toHaveBeenCalled()
    expect(showError).not.toHaveBeenCalled()
    expect((wrapper.get('#manual-payment-reference').element as HTMLInputElement).value).toBe('gateway-trade-092')
  })

  it('does not expose manual confirmation for an order with any refund marker', async () => {
    getOrders.mockResolvedValueOnce({
      data: { items: [{ ...pendingOrder, force_refund: true }], total: 1 },
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="manual-payment-confirm-action"]').exists()).toBe(false)
  })
})
