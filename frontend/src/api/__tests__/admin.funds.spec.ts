import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post },
}))

import adminFundsAPI from '@/api/admin/funds'

describe('admin funds API', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
  })

  it('loads recharge return queue and performs review actions through fund endpoints', async () => {
    get.mockResolvedValueOnce({ data: { items: [], total: 0, page: 1, page_size: 20, pages: 1 } })
    get.mockResolvedValueOnce({ data: { id: 12, status: 'pending_review' } })
    post.mockResolvedValueOnce({ data: { id: 12, status: 'payout_pending' } })
    post.mockResolvedValueOnce({ data: { id: 12, status: 'rejected' } })
    post.mockResolvedValueOnce({ data: { id: 12, status: 'paid' } })
    get.mockResolvedValueOnce({ data: { account: 'alice@example.com' } })

    await adminFundsAPI.listRefundRequests({ status: 'pending_review', user_id: 7, page: 1, page_size: 20 })
    await adminFundsAPI.getRefundRequest(12)
    await adminFundsAPI.approveRefundRequest(12, { note: 'checked' })
    await adminFundsAPI.rejectRefundRequest(12, { reason: '资料不一致', note: 'manual review' })
    await adminFundsAPI.markRefundPaid(12, {
      paid_amount: '30.00',
      paid_currency: 'USD',
      payout_fx_rate: '1',
      external_txn_id: 'offline-1',
    })
    await adminFundsAPI.getRefundSensitivePayout(12)

    expect(get).toHaveBeenCalledWith('/admin/funds/refund-requests', { params: { status: 'pending_review', user_id: 7, page: 1, page_size: 20 } })
    expect(get).toHaveBeenCalledWith('/admin/funds/refund-requests/12')
    expect(post).toHaveBeenCalledWith('/admin/funds/refund-requests/12/approve', { note: 'checked' })
    expect(post).toHaveBeenCalledWith('/admin/funds/refund-requests/12/reject', { reason: '资料不一致', note: 'manual review' })
    expect(post).toHaveBeenCalledWith('/admin/funds/refund-requests/12/mark-paid', {
      paid_amount: '30',
      paid_currency: 'USD',
      payout_fx_rate: '1',
      external_txn_id: 'offline-1',
    })
    expect(get).toHaveBeenCalledWith('/admin/funds/refund-requests/12/payout-sensitive')
  })

  it('grants gift balance, records offline recharge, and runs signup gift classification', async () => {
    post.mockResolvedValueOnce({ data: { id: 21, source_type: 'ops_gift' } })
    post.mockResolvedValueOnce({ data: { id: 22, source_type: 'offline_recharge' } })
    get.mockResolvedValueOnce({ data: { mode: 'preview', candidate_count: 1, candidates: [] } })
    post.mockResolvedValueOnce({ data: { mode: 'execute', affected_count: 1, candidates: [] } })

    await adminFundsAPI.grantGift({ user_id: 7, amount: '30.00', reason: '人工赠送新用户余额' })
    await adminFundsAPI.grantOfflineRecharge({ user_id: 7, amount: '100.00', external_ref: 'wire-1001', reason: '线下充值到账' })
    await adminFundsAPI.previewSignupGift30(50)
    await adminFundsAPI.executeSignupGift30({ transaction_ids: [21], reason: '核对历史首笔 30 为赠送余额' })

    expect(post).toHaveBeenCalledWith('/admin/funds/gifts', { user_id: 7, amount: '30', reason: '人工赠送新用户余额' })
    expect(post).toHaveBeenCalledWith('/admin/funds/offline-recharges', { user_id: 7, amount: '100', external_ref: 'wire-1001', reason: '线下充值到账' })
    expect(get).toHaveBeenCalledWith('/admin/funds/classifications/signup-gift-30/preview', { params: { limit: 50 } })
    expect(post).toHaveBeenCalledWith('/admin/funds/classifications/signup-gift-30/execute', { transaction_ids: [21], reason: '核对历史首笔 30 为赠送余额' })
  })
})
