import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get } }))

import operations from '@/api/admin/affiliateOperations'
import { affiliatesAPI } from '@/api/admin/affiliates'

describe('administrator affiliate operations API', () => {
  beforeEach(() => get.mockReset())

  it('keeps exact user filters and the selected cohort month in separate admin endpoints', async () => {
    get.mockResolvedValue({ data: { items: [], total: 0 } })
    await operations.getOverview({ month: '2026-11' })
    await operations.getLeaderboard({ month: '2026-11' })
    await operations.listPromoters({ user_id: 17, page: 1 })
    await operations.listInvitees({ user_id: 17, invitee_id: 19, status: 'pending_evidence' })
    await operations.listAudits({ user_id: 17 })
    expect(get.mock.calls.map(call => call[0])).toEqual([
      '/admin/affiliates/operations/overview', '/admin/affiliates/operations/leaderboard',
      '/admin/affiliates/operations/promoters', '/admin/affiliates/operations/invitees',
      '/admin/affiliates/operations/audits',
    ])
    expect(get).toHaveBeenCalledWith('/admin/affiliates/operations/invitees', {
      params: { user_id: 17, invitee_id: 19, status: 'pending_evidence' },
    })
  })

  it('exports the complete server-filtered result rather than the current table page', async () => {
    const blob = new Blob(['csv'], { type: 'text/csv' })
    get.mockResolvedValue({ data: blob })
    const params = { user_id: 17, action: 'withdraw' as const, page: 4, page_size: 20 }
    expect(await affiliatesAPI.exportRecords('transfers', params)).toBe(blob)
    expect(get).toHaveBeenCalledWith('/admin/affiliates/transfers/export', {
      params: { user_id: 17, action: 'withdraw' }, responseType: 'blob',
    })
    get.mockRejectedValueOnce({ response: { status: 422 } })
    await expect(affiliatesAPI.exportRecords('transfers', params)).rejects.toMatchObject({ response: { status: 422 } })
  })

  it('requests summaries with precisely the same filters and no pagination', async () => {
    get.mockResolvedValue({ data: { record_count: 14, total_amount: 3, currency: 'USD' } })
    await affiliatesAPI.getRecordsSummary('rebates', { inviter_id: 17, source_order_id: 81, page: 3 })
    expect(get).toHaveBeenCalledWith('/admin/affiliates/rebates/summary', {
      params: { inviter_id: 17, source_order_id: 81 },
    })
  })
})
