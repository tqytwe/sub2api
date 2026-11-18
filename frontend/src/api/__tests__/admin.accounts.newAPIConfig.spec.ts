import { describe, expect, it, vi } from 'vitest'
import { apiClient } from '../client'
import {
  deleteNewAPIUpstreamConfig,
  getNewAPIUpstreamConfig,
  previewNewAPIUpstreamConfig,
  saveNewAPIUpstreamConfig
} from '../admin/accounts'

vi.mock('../client', () => ({ apiClient: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() } }))

describe('New API account authorization API', () => {
  it('uses the account-scoped preview, save and unlink endpoints', async () => {
    vi.mocked(apiClient.get).mockResolvedValue({ data: { configured: false } })
    vi.mocked(apiClient.post).mockResolvedValue({ data: { accounts: [] } })
    vi.mocked(apiClient.put).mockResolvedValue({ data: { accounts: [] } })
    vi.mocked(apiClient.delete).mockResolvedValue({ data: { account_id: 7, configured: false } })
    const request = { user_id: 42, access_token: 'secret', account_ids: [7] }

    await getNewAPIUpstreamConfig(7)
    await previewNewAPIUpstreamConfig(7, request)
    await saveNewAPIUpstreamConfig(7, request)
    await deleteNewAPIUpstreamConfig(7)

    expect(apiClient.get).toHaveBeenCalledWith('/admin/accounts/7/upstream-billing-probe/config')
    expect(apiClient.post).toHaveBeenCalledWith('/admin/accounts/7/upstream-billing-probe/config/preview', request)
    expect(apiClient.put).toHaveBeenCalledWith('/admin/accounts/7/upstream-billing-probe/config', request)
    expect(apiClient.delete).toHaveBeenCalledWith('/admin/accounts/7/upstream-billing-probe/config')
  })
})
