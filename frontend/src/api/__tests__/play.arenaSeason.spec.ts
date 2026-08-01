import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock('@/api/client', () => ({ apiClient: { get } }))

import { getArenaSeasonOverview } from '@/api/play'

describe('Farm season overview API', () => {
  beforeEach(() => get.mockReset())

  it('loads only the selected period without historical aggregation by default', async () => {
    get.mockResolvedValueOnce({
      data: {
        enabled: true,
        current: { enabled: true },
        rows: [],
        reward_tiers: [],
        history: [],
      },
    })

    await getArenaSeasonOverview('monthly')

    expect(get).toHaveBeenCalledWith('/play/arena/overview', {
      params: { period: 'monthly', include_history: '0' },
    })
  })

  it('requests immutable payout proof only when the UI expands history', async () => {
    get.mockResolvedValueOnce({ data: { enabled: true, current: { enabled: true }, rows: [], reward_tiers: [], history: [] } })

    await getArenaSeasonOverview('daily', true)

    expect(get).toHaveBeenCalledWith('/play/arena/overview', {
      params: { period: 'daily', include_history: '1' },
    })
  })
})
