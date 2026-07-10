import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AxiosInstance } from 'axios'

vi.mock('@/i18n', () => ({
  getLocale: () => 'en',
}))

describe('publicVipTiers API', () => {
  let apiClient: AxiosInstance

  beforeEach(async () => {
    vi.resetModules()
    const mod = await import('@/api/client')
    apiClient = mod.apiClient
  })

  it('unwraps VIP tiers from standard API envelope', async () => {
    const payload = {
      enabled: true,
      tiers: [{ tier: 0, label: 'V0', min_recharge: 0 }],
    }

    apiClient.defaults.adapter = vi.fn().mockResolvedValue({
      status: 200,
      data: { code: 0, message: 'success', data: payload },
      headers: {},
      config: {},
      statusText: 'OK',
    })

    const { fetchPublicVIPTiers } = await import('@/api/publicVipTiers')
    await expect(fetchPublicVIPTiers()).resolves.toEqual(payload)
  })

  it('returns empty tiers when response body is missing', async () => {
    apiClient.defaults.adapter = vi.fn().mockResolvedValue({
      status: 200,
      data: { code: 0, message: 'success', data: null },
      headers: {},
      config: {},
      statusText: 'OK',
    })

    const { fetchPublicVIPTiers } = await import('@/api/publicVipTiers')
    await expect(fetchPublicVIPTiers()).resolves.toEqual({ enabled: false, tiers: [] })
  })
})
