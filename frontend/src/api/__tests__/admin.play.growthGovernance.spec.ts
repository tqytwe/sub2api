import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post },
}))

import adminPlayAPI, {
  approveGrowthGovernance,
  getGrowthCohort,
  getGrowthGovernance,
  revokeGrowthGovernance,
  type AdminPlayGrowthCohort,
  type AdminPlayGrowthGovernanceApprovalInput,
} from '@/api/admin/play'

const cohort: AdminPlayGrowthCohort = {
  window_start: '2026-07-31T00:00:00Z',
  window_end: '2026-08-14T00:00:00Z',
  metrics_available: false,
  participation_users: 1248,
  real_call_7d_users: 526,
  real_call_7d_ratio: 0.421,
  real_call_30d_users: 396,
  real_call_30d_ratio: 0.317,
  first_recharge_users: 107,
  first_recharge_ratio: 0.086,
  coupons_issued: 373,
  coupons_redeemed: 53,
  coupon_redemption_ratio: 0.142,
  actual_reward_cost: 0,
  d7_retained_users: 473,
  d7_retention_ratio: 0.379,
  abnormal_redemption_users: 0,
  abnormal_redemption_ratio: null,
  appeal_count: 0,
  false_positive_appeals: 0,
  appeal_false_positive_ratio: null,
  unavailable_metrics: ['abnormal_redemption_ratio', 'appeal_false_positive_ratio'],
}

describe('admin Play growth-governance API', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    get
      .mockResolvedValueOnce({ data: cohort })
      .mockResolvedValueOnce({
        data: {
          id: 9,
          decision: 'approved',
          approved: true,
          budget_amount: 100,
          budget_spent: 12,
          budget_remaining: 88,
          rollout_percent: 10,
          cohort,
          rule_version: 'v1',
          reason: 'The two-week cohort shows controlled reward cost.',
          created_at: '2026-08-31T10:00:00Z',
        },
      })
    post.mockResolvedValue({
      data: {
        id: 9,
        decision: 'approved',
        approved: true,
        budget_amount: 100,
        budget_spent: 12,
        budget_remaining: 88,
        rollout_percent: 10,
        cohort,
        rule_version: 'v1',
        reason: 'The two-week cohort shows controlled reward cost.',
        created_at: '2026-08-31T10:00:00Z',
      },
    })
  })

  it('reads the cohort and current append-only governance decision separately', async () => {
    await expect(getGrowthCohort({ start: '2026-07-31', end: '2026-08-14' })).resolves.toEqual(cohort)
    await expect(getGrowthGovernance()).resolves.toMatchObject({ decision: 'approved', budget_remaining: 88 })

    expect(get).toHaveBeenNthCalledWith(1, '/admin/play/growth/cohort', {
      params: { start: '2026-07-31', end: '2026-08-14' },
    })
    expect(get).toHaveBeenNthCalledWith(2, '/admin/play/growth/governance')
  })

  it('leaves the default observation window to the server', async () => {
    await getGrowthCohort()

    expect(get).toHaveBeenCalledWith('/admin/play/growth/cohort', { params: {} })
  })

  it('sends the reviewed server cohort, rollout, budget, and operator reason for an approval', async () => {
    const input: AdminPlayGrowthGovernanceApprovalInput = {
      budget_amount: 100,
      rollout_percent: 10,
      cohort,
      reason: 'The two-week cohort shows controlled reward cost.',
    }

    await approveGrowthGovernance(input)

    expect(post).toHaveBeenCalledWith('/admin/play/growth/governance/approve', input)
  })

  it('sends a required audited reason when revoking and exposes all functions on the default API boundary', async () => {
    await revokeGrowthGovernance({ reason: 'Stop the rollout while the cohort evidence is reviewed.' })

    expect(post).toHaveBeenCalledWith('/admin/play/growth/governance/revoke', {
      reason: 'Stop the rollout while the cohort evidence is reviewed.',
    })
    expect(adminPlayAPI.getGrowthCohort).toBe(getGrowthCohort)
    expect(adminPlayAPI.getGrowthGovernance).toBe(getGrowthGovernance)
    expect(adminPlayAPI.approveGrowthGovernance).toBe(approveGrowthGovernance)
    expect(adminPlayAPI.revokeGrowthGovernance).toBe(revokeGrowthGovernance)
  })
})
