import { describe, expect, it } from 'vitest'
import {
  landOffset,
  splitStableTail,
  spinOffset,
  syncTailDigits,
  visibleDigit,
} from '@/utils/odometerColumns'

describe('odometerColumns', () => {
  it('keeps all but last three digits in stable text', () => {
    const { stable, tails } = splitStableTail('12,847,360', 3)
    expect(stable).toBe('12,847,')
    expect(tails.map((t) => t.digit)).toEqual([3, 6, 0])
  })

  it('lands on exact digit without half-step offset', () => {
    expect(visibleDigit(landOffset(7))).toBe(7)
    expect(visibleDigit(spinOffset(26, 3, 2))).toBe(3)
  })

  it('supports long formatted values', () => {
    const { stable, tails } = splitStableTail('1,234,567,890,123', 3)
    expect(stable).toBe('1,234,567,890,')
    expect(tails.map((t) => t.digit)).toEqual([1, 2, 3])
  })

  it('snaps stable text immediately when not animating', () => {
    const { stable, tails } = syncTailDigits([], '12,847,360', 3, {
      animate: false,
      entrance: false,
    })
    expect(stable).toBe('12,847,')
    expect(tails.every((t) => t.offset === landOffset(t.targetDigit))).toBe(true)
  })
})
