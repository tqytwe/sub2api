import { describe, expect, it } from 'vitest'
import {
  landOffset,
  parseValueColumns,
  spinOffset,
  syncColumnsFromValue,
  visibleDigit,
} from '@/utils/odometerColumns'

describe('odometerColumns', () => {
  it('marks last three numeric columns as tail', () => {
    const cols = parseValueColumns('12,847,360', 3)
    const digits = cols.filter((c) => c.type === 'digit')
    expect(digits).toHaveLength(8)
    expect(digits.slice(-3).every((d) => d.type === 'digit' && d.isTail)).toBe(true)
    expect(digits[0]?.type === 'digit' && digits[0].isTail).toBe(false)
  })

  it('lands on exact digit without half-step offset', () => {
    expect(visibleDigit(landOffset(7))).toBe(7)
    expect(visibleDigit(spinOffset(26, 3, 2))).toBe(3)
  })

  it('supports long formatted values', () => {
    const cols = parseValueColumns('1,234,567,890,123', 3)
    expect(cols.filter((c) => c.type === 'digit')).toHaveLength(13)
  })

  it('snaps stable digits immediately when not animating tail', () => {
    const next = syncColumnsFromValue([], '12,847,360', 3, { animateTail: false, entrance: false })
    const stable = next.find((c) => c.type === 'digit' && c.key === 'd-0')
    expect(stable?.type === 'digit' && stable.offset).toBe(landOffset(1))
  })
})
