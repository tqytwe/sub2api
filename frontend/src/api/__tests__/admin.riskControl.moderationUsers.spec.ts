import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))

vi.mock('../client', () => ({ apiClient: { get, post } }))

import { getModerationUser, listModerationUsers, manageModerationUser } from '../admin/riskControl'

describe('admin moderation user API', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
  })

  it('uses the server list filters and returns the paginated projection', async () => {
    const result = { items: [], total: 0, page: 1, page_size: 20, pages: 0 }
    get.mockResolvedValue({ data: result })
    const params = { page: 1, page_size: 20, state: 'all' as const, user_id: 42 }
    await expect(listModerationUsers(params)).resolves.toEqual(result)
    expect(get).toHaveBeenCalledWith('/admin/risk-control/users', { params })
  })

  it('uses the detail projection and posts versioned management actions', async () => {
    const state = { user_id: 42, version: 9, owned_disable: true, status: 'disabled' }
    get.mockResolvedValue({ data: state })
    await expect(getModerationUser(42)).resolves.toEqual(state)
    expect(get).toHaveBeenCalledWith('/admin/risk-control/users/42')

    const payload = { action: 'unban' as const, version: 9, reason: 'Reviewed appeal', confirm: true }
    post.mockResolvedValue({ data: state })
    await expect(manageModerationUser(42, payload)).resolves.toEqual(state)
    expect(post).toHaveBeenCalledWith('/admin/risk-control/users/42/actions', payload)
  })
})
