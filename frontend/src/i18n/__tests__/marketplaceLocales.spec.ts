import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

function collectLeaves(node: unknown, path = ''): Map<string, string> {
  const leaves = new Map<string, string>()
  if (!node || typeof node !== 'object' || Array.isArray(node)) {
    return leaves
  }
  for (const [key, value] of Object.entries(node as Record<string, unknown>)) {
    const nextPath = path ? `${path}.${key}` : key
    if (typeof value === 'string') {
      leaves.set(nextPath, value)
      continue
    }
    for (const [leafPath, leafValue] of collectLeaves(value, nextPath)) {
      leaves.set(leafPath, leafValue)
    }
  }
  return leaves
}

describe('marketplace locale resources', () => {
  it('keeps marketplace locale trees recursively symmetric and non-empty', () => {
    expect(zh.marketplace).toBeDefined()
    expect(en.marketplace).toBeDefined()

    const zhLeaves = collectLeaves(zh.marketplace)
    const enLeaves = collectLeaves(en.marketplace)

    expect([...zhLeaves.keys()].sort()).toEqual([...enLeaves.keys()].sort())
    expect(zhLeaves.size).toBeGreaterThan(0)
    for (const value of [...zhLeaves.values(), ...enLeaves.values()]) {
      expect(value.trim()).not.toBe('')
    }
  })
})
