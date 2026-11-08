import { beforeEach, describe, expect, it, vi } from 'vitest'
const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get } }))
import { getAffiliateLeaderboard, getAffiliateOperationsSummary, listAffiliateInvitees, listAffiliateEarnings, listAffiliateCampaignHistory } from '@/api/affiliateOperations'

describe('user affiliate operations API', () => {
  beforeEach(() => get.mockReset().mockResolvedValue({ data: { items: [], total: 0 } }))
  it('uses authenticated user-scoped endpoints and preserves server pagination and filters', async () => {
    await getAffiliateOperationsSummary()
    await listAffiliateInvitees({ page: 3, page_size: 20, status: 'effective', search: 'ali' })
    await listAffiliateEarnings({ page: 2, page_size: 20, action: 'transfer' })
    await listAffiliateCampaignHistory({ page: 1, page_size: 20 })
    expect(get.mock.calls.map(call => call[0])).toEqual([
      '/user/aff/operations/summary', '/user/aff/operations/invitees',
      '/user/aff/operations/earnings', '/user/aff/operations/campaigns',
    ])
    expect(get).toHaveBeenCalledWith('/user/aff/operations/invitees', { params: { page: 3, page_size: 20, status: 'effective', search: 'ali' } })
  })
  it('passes period, activity and cancellation to the dedicated lightweight leaderboard endpoint', async () => {
    const signal = new AbortController().signal
    await getAffiliateLeaderboard({ month: '2026-10', campaign_id: 7 }, signal)
    expect(get).toHaveBeenCalledWith('/user/aff/operations/leaderboard', { params: { month: '2026-10', campaign_id: 7 }, signal })
  })
})
