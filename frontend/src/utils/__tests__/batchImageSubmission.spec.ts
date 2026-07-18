import { describe, expect, it, vi } from 'vitest'
import {
  resolveBatchImageSubmissionAttempt,
  batchImageAvailabilityIssue,
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
