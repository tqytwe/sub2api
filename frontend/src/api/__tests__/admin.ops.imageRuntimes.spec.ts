import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: { get },
  buildGatewayUrl: vi.fn()
}))

import { getImageRuntimesHealth } from '@/api/admin/ops'

describe('admin ops image runtimes API', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('reads the dedicated image runtime health endpoint', async () => {
    const health = {
      checked_at: '2026-07-18T12:00:00Z',
      gateway_async: { enabled: true, ready: true },
      batch: { enabled: false, ready: false },
      image_studio: { enabled: true, ready: true }
    }
    get.mockResolvedValueOnce({ data: health })

    await expect(getImageRuntimesHealth()).resolves.toEqual(health)
    expect(get).toHaveBeenCalledWith('/admin/ops/image-runtimes/health')
  })
})
