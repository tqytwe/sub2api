import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('../client', () => ({ apiClient: { get } }))

import { readProfitOperations, type OperationsQuery } from '../admin/profitOperations'

const query: OperationsQuery = {
  from: '2026-11-25T00:00:00.000Z',
  until: '2026-11-25T12:00:00.000Z',
  upstream: '',
  account_id: 0,
  group_id: 0,
  model: '',
  model_scope: 'public',
  dimension: 'upstream',
  sort: 'requests',
  direction: 'desc',
  page: 1,
  page_size: 20,
}

describe('profit operations report refresh', () => {
  beforeEach(() => get.mockReset().mockResolvedValue({ data: {} }))

  it('uses the shared cached report for ordinary page reads', async () => {
    const signal = new AbortController().signal
    await readProfitOperations(query, signal)
    expect(get).toHaveBeenCalledWith('/admin/profit-operations/snapshot', expect.objectContaining({
      params: query,
      signal,
      timeout: 330_000,
    }))
  })

  it('bypasses the report cache when the administrator explicitly refreshes', async () => {
    const signal = new AbortController().signal
    await readProfitOperations(query, signal, true)
    expect(get).toHaveBeenCalledWith('/admin/profit-operations/snapshot', expect.objectContaining({
      params: { ...query, refresh_report: '1' },
      signal,
      timeout: 330_000,
    }))
  })
})
