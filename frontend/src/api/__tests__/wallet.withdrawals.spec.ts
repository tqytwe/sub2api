import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post, put },
}))

import {
  cancelWithdrawal,
  createWithdrawal,
  getWithdrawalAccount,
  getWithdrawals,
  updateWithdrawalAccount,
} from '@/api/wallet'

describe('wallet withdrawal API', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    put.mockReset()
  })

  it('loads and updates the current payout account through user wallet endpoints', async () => {
    get.mockResolvedValueOnce({ data: null })
    put.mockResolvedValueOnce({ data: { id: 7, method: 'alipay', currency: 'CNY', account_mask: 'ali***@example.com' } })

    await getWithdrawalAccount()
    await updateWithdrawalAccount({
      method: 'alipay',
      currency: 'CNY',
      recipient_name: 'Alice',
      details: { account: 'alice@example.com' },
    })

    expect(get).toHaveBeenCalledWith('/user/wallet/withdrawal-account')
    expect(put).toHaveBeenCalledWith('/user/wallet/withdrawal-account', {
      method: 'alipay',
      currency: 'CNY',
      recipient_name: 'Alice',
      details: { account: 'alice@example.com' },
    })
  })

  it('creates, lists, and cancels withdrawal requests using whole string amounts', async () => {
    get.mockResolvedValueOnce({ data: { items: [], total: 0, page: 1, page_size: 20, pages: 1 } })
    post.mockResolvedValueOnce({ data: { id: 9, status: 'pending_review', amount: '12' } })
    post.mockResolvedValueOnce({ data: { id: 9, status: 'canceled', amount: '12' } })

    await getWithdrawals({ page: 1, page_size: 20 })
    await createWithdrawal({ amount: '12.00' })
    await cancelWithdrawal(9)

    expect(get).toHaveBeenCalledWith('/user/wallet/withdrawals', { params: { page: 1, page_size: 20 } })
    expect(post).toHaveBeenCalledWith('/user/wallet/withdrawals', { amount: '12' })
    expect(post).toHaveBeenCalledWith('/user/wallet/withdrawals/9/cancel')
  })
})
