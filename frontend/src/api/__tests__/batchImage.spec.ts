import { beforeEach, describe, expect, it, vi } from 'vitest'
import { submitBatchImageJob } from '../batchImage'

describe('batch image API', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('accepts 202 and forwards the stable idempotency key', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      id: 'imgbatch_accepted',
      object: 'image.batch',
      status: 'queued',
    }), {
      status: 202,
      headers: { 'Content-Type': 'application/json' },
    }))

    const result = await submitBatchImageJob('secret-key', {
      model: 'gemini-image',
      items: [{ custom_id: 'img_001', prompt: 'one' }],
    }, 'stable-idempotency-key')

    expect(result.id).toBe('imgbatch_accepted')
    expect(fetchMock).toHaveBeenCalledWith(expect.stringContaining('/v1/images/batches'), expect.objectContaining({
      method: 'POST',
      headers: expect.objectContaining({
        'Idempotency-Key': 'stable-idempotency-key',
      }),
    }))
  })
})
