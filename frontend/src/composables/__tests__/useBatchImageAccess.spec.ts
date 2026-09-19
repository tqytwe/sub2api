import { describe, expect, it } from 'vitest'
import { keyAllowsBatchImage } from '../useBatchImageAccess'

const key = (overrides: Record<string, unknown> = {}) => ({
  status: 'active',
  group: {
    platform: 'gemini',
    allow_image_generation: true,
    allow_batch_image_generation: true,
  },
  ...overrides,
}) as any

describe('keyAllowsBatchImage', () => {
  it('requires both regular and batch image capabilities', () => {
    expect(keyAllowsBatchImage(key())).toBe(true)
    expect(keyAllowsBatchImage(key({ group: { platform: 'gemini', allow_image_generation: false, allow_batch_image_generation: true } }))).toBe(false)
    expect(keyAllowsBatchImage(key({ group: { platform: 'gemini', allow_image_generation: true, allow_batch_image_generation: false } }))).toBe(false)
  })

  it('rejects inactive or non-Gemini keys', () => {
    expect(keyAllowsBatchImage(key({ status: 'inactive' }))).toBe(false)
    expect(keyAllowsBatchImage(key({ group: { platform: 'openai', allow_image_generation: true, allow_batch_image_generation: true } }))).toBe(false)
  })
})
