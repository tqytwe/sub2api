import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { post } }))
import { getDashboardApiKeysUsage } from '@/api/usage'

describe('Key calendar consumption API', () => {
  beforeEach(() => post.mockReset().mockResolvedValue({ data: { stats: {} } }))

  it('forwards the inclusive calendar dates, timezone and cancellation', async () => {
    const signal = new AbortController().signal
    await getDashboardApiKeysUsage([7], {
      start_date: '2026-11-01', end_date: '2026-11-08', timezone: 'America/Los_Angeles', signal,
    })
    expect(post).toHaveBeenCalledWith('/usage/dashboard/api-keys-usage', {
      api_key_ids: [7], start_date: '2026-11-01', end_date: '2026-11-08', timezone: 'America/Los_Angeles',
    }, { signal })
  })

  it('keeps omitted dates compatible with existing callers', async () => {
    await getDashboardApiKeysUsage([7])
    const body = post.mock.calls[0][1]
    expect(JSON.parse(JSON.stringify(body))).toEqual({ api_key_ids: [7] })
  })
})
