import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock('@/api/client', () => ({ apiClient: { get } }))

import { getPrioritySnapshot } from '@/api/admin/priorityScheduling'

describe('admin priority scheduling snapshot API', () => {
  beforeEach(() => get.mockReset())

  it('treats a cold scheduler envelope as no snapshot', async () => {
    get.mockResolvedValue({
      data: {
        snapshot: {
          history: null,
          history_status: 'unavailable',
          candidates: null,
        },
      },
    })

    await expect(getPrioritySnapshot()).resolves.toBeNull()
  })

  it('unwraps the latest observation from the server envelope', async () => {
    const history = {
      at: '2026-11-06T00:00:00Z',
      model: 'public-gpt-test',
      group_id: 3,
      history_status: 'unavailable',
      candidates: [],
    }
    get.mockResolvedValue({ data: { snapshot: { history, candidates: null } } })

    await expect(getPrioritySnapshot()).resolves.toEqual(history)
  })

  it('keeps direct snapshot responses compatible and normalizes a missing candidate list', async () => {
    get.mockResolvedValue({
      data: {
        snapshot: {
          at: '2026-11-06T00:00:00Z',
          model: 'public-gpt-test',
          group_id: null,
          candidates: null,
        },
      },
    })

    await expect(getPrioritySnapshot()).resolves.toMatchObject({ candidates: [] })
  })
})
