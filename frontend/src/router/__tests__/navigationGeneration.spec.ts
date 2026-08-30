import { describe, expect, it } from 'vitest'

import { createNavigationGeneration } from '../navigationGeneration'

describe('navigation generation', () => {
  it('makes an older asynchronous guard stale once a later navigation begins', async () => {
    const generation = createNavigationGeneration()
    const older = generation.begin()
    let releaseOlderWork: (() => void) | undefined
    const olderWork = new Promise<void>((resolve) => {
      releaseOlderWork = resolve
    })

    const olderGuard = (async () => {
      await olderWork
      return generation.isCurrent(older)
    })()

    const newer = generation.begin()
    expect(generation.isCurrent(newer)).toBe(true)

    releaseOlderWork?.()
    await expect(olderGuard).resolves.toBe(false)
  })
})
