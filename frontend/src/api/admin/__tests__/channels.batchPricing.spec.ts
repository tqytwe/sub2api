import { beforeEach, describe, expect, it, vi } from 'vitest'
import { batchUpdatePricing } from '../channels'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('../../client', () => ({ apiClient: { post } }))

describe('admin channel batch pricing API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    post.mockResolvedValue({ data: { updated: 2 } })
  })

  it('sends the full selected channel set to the atomic pricing endpoint', async () => {
    const items = [
      { channel_id: 8, model_pricing: [{ platform: 'openai', models: ['gpt-x'] }] },
      { channel_id: 9, model_pricing: [{ platform: 'anthropic', models: ['claude-x'] }] },
    ] as never
    await expect(batchUpdatePricing(items)).resolves.toEqual({ updated: 2 })
    expect(post).toHaveBeenCalledWith('/admin/channels/pricing/batch', { items })
  })
})
