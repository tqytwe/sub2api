import { describe, expect, it } from 'vitest'
import { formatAffiliatePct } from '@/utils/formatAffiliatePct'

describe('formatAffiliatePct', () => {
  it('formats fractional percent rates', () => {
    expect(formatAffiliatePct(0.1)).toBe('0.1')
    expect(formatAffiliatePct(0.5)).toBe('0.5')
  })

  it('rounds whole percent rates', () => {
    expect(formatAffiliatePct(20)).toBe('20')
    expect(formatAffiliatePct(19.6)).toBe('20')
  })
})
