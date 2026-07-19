import { describe, expect, it, vi } from 'vitest'
import {
  batchImageAvailabilityIssue,
  buildBatchImageSubmitItems,
  normalizeBatchImageOutputCount,
  resolveBatchImageSubmissionAttempt,
} from '../batchImageSubmission'

describe('batch image submission idempotency', () => {
  it('reuses the key when retrying the same payload', () => {
    const createKey = vi.fn(() => 'batch-key-1')
    const payload = { model: 'gemini-image', items: [{ custom_id: 'img_001', prompt: 'one' }] }

    const first = resolveBatchImageSubmissionAttempt(null, payload, createKey)
    const retry = resolveBatchImageSubmissionAttempt(first, payload, createKey)

    expect(retry.key).toBe(first.key)
    expect(createKey).toHaveBeenCalledTimes(1)
  })

  it('creates a new key when the payload changes', () => {
    const createKey = vi.fn()
      .mockReturnValueOnce('batch-key-1')
      .mockReturnValueOnce('batch-key-2')
    const first = resolveBatchImageSubmissionAttempt(null, { model: 'a', items: [] }, createKey)

    const changed = resolveBatchImageSubmissionAttempt(first, { model: 'b', items: [] }, createKey)

    expect(changed.key).toBe('batch-key-2')
  })
})

describe('batch image availability errors', () => {
  it('distinguishes runtime readiness from an unfinished job', () => {
    expect(batchImageAvailabilityIssue({ code: 'BATCH_IMAGE_NOT_READY', status: 503 })).toBe('runtimeNotReady')
    expect(batchImageAvailabilityIssue({ code: 'BATCH_IMAGE_NOT_READY', status: 409 })).toBeNull()
  })

  it('distinguishes account, model, pricing, group, and global disablement', () => {
    expect(batchImageAvailabilityIssue({ code: 'BATCH_IMAGE_DISABLED' })).toBe('disabled')
    expect(batchImageAvailabilityIssue({ code: 'BATCH_IMAGE_GROUP_DISABLED' })).toBe('groupDisabled')
    expect(batchImageAvailabilityIssue({ code: 'BATCH_IMAGE_NO_ACCOUNT_AVAILABLE' })).toBe('noCompatibleAccount')
    expect(batchImageAvailabilityIssue({ code: 'BATCH_IMAGE_NO_MODEL_AVAILABLE' })).toBe('noModelsHint')
    expect(batchImageAvailabilityIssue({ code: 'BATCH_IMAGE_PRICING_NOT_READY' })).toBe('pricingMissing')
  })
})

describe('batch image item construction', () => {
  it.each([5, 10])('builds %i requested images as independent items', (count) => {
    const items = buildBatchImageSubmitItems(
      Array.from({ length: count }, (_, index) => ({
        custom_id: `image_${index + 1}`,
        prompt: `prompt ${index + 1}`,
      })),
    )

    expect(items).toHaveLength(count)
    expect(new Set(items.map(item => item.custom_id)).size).toBe(count)
    expect(items.every(item => item.output_count === undefined)).toBe(true)
  })

  it('normalizes per-item output count and duplicate custom ids', () => {
    const items = buildBatchImageSubmitItems([
      { custom_id: 'same id', prompt: 'one', output_count: 10 },
      { custom_id: 'same id', prompt: 'two', output_count: 0 },
      { custom_id: 'ignored', prompt: '  ' },
    ])

    expect(items).toEqual([
      { custom_id: 'same_id', prompt: 'one', output_count: 4 },
      { custom_id: 'same_id_2', prompt: 'two' },
    ])
    expect(normalizeBatchImageOutputCount(Number.NaN)).toBe(1)
  })
})
