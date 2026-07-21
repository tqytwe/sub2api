import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post, put },
}))

import adminWithdrawalsAPI from '@/api/admin/withdrawals'

describe('admin withdrawals API', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    put.mockReset()
  })

  it('loads queue and performs review actions through dedicated admin endpoints', async () => {
    get.mockResolvedValueOnce({ data: { items: [], total: 0, page: 1, page_size: 20, pages: 1 } })
    get.mockResolvedValueOnce({ data: { id: 12, status: 'pending_review' } })
    post.mockResolvedValueOnce({ data: { id: 12, status: 'payout_pending' } })
    post.mockResolvedValueOnce({ data: { id: 12, status: 'paid' } })

    await adminWithdrawalsAPI.list({ status: 'pending_review', page: 1, page_size: 20 })
    await adminWithdrawalsAPI.get(12)
    await adminWithdrawalsAPI.approve(12, { note: 'checked' })
    await adminWithdrawalsAPI.markPaid(12, {
      paid_amount: '12.50',
      paid_currency: 'USD',
      external_txn_id: 'offline-1',
      paid_at: '2026-07-21T12:00:00Z',
    })

    expect(get).toHaveBeenCalledWith('/admin/withdrawals', { params: { status: 'pending_review', page: 1, page_size: 20 } })
    expect(get).toHaveBeenCalledWith('/admin/withdrawals/12')
    expect(post).toHaveBeenCalledWith('/admin/withdrawals/12/approve', { note: 'checked' })
    expect(post).toHaveBeenCalledWith('/admin/withdrawals/12/mark-paid', {
      paid_amount: '12.50',
      paid_currency: 'USD',
      external_txn_id: 'offline-1',
      paid_at: '2026-07-21T12:00:00Z',
    })
  })

  it('updates global and per-user withdrawal permissions without numeric HTTP amounts', async () => {
    put.mockResolvedValueOnce({ data: { global_enabled: false } })
    put.mockResolvedValueOnce({ data: { user_id: 7, enabled: true } })
    post.mockResolvedValueOnce({ data: { affected: 2 } })

    await adminWithdrawalsAPI.updateSettings({ global_enabled: false, minimum_amount: '10.00' })
    await adminWithdrawalsAPI.updateUserSettings(7, { enabled: true, daily_limit_override: '200.00' })
    await adminWithdrawalsAPI.batchUpdateUserSettings({ user_ids: [7, 8], enabled: false })

    expect(put).toHaveBeenCalledWith('/admin/withdrawals/settings', { global_enabled: false, minimum_amount: '10.00' })
    expect(put).toHaveBeenCalledWith('/admin/withdrawals/users/7/settings', { enabled: true, daily_limit_override: '200.00' })
    expect(post).toHaveBeenCalledWith('/admin/withdrawals/user-settings/batch', { user_ids: [7, 8], enabled: false })
  })
})
