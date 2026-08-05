import { describe, expect, it } from 'vitest'

import { normalizeVipTier } from '@/utils/vipTier'

describe('normalizeVipTier', () => {
  it('normalizes every configured tier value from controls and storage', () => {
    expect([0, 1, 2, 3, 4, 5, 6].map(normalizeVipTier)).toEqual([0, 1, 2, 3, 4, 5, 6])
    expect(['0', '1', '2', '3', '4', '5', '6'].map(normalizeVipTier)).toEqual([0, 1, 2, 3, 4, 5, 6])
  })

  it('does not turn invalid values into a broad user filter', () => {
    expect(normalizeVipTier('')).toBeUndefined()
    expect(normalizeVipTier('not-a-tier')).toBeUndefined()
    expect(normalizeVipTier(1.5)).toBeUndefined()
    expect(normalizeVipTier(-1)).toBeUndefined()
    expect(normalizeVipTier(100)).toBeUndefined()
  })
})
