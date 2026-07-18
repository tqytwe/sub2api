import { beforeEach, describe, expect, it, vi } from 'vitest'
import { recoverFromChunkLoadError } from '@/router/chunkRecovery'

function makeStorage(): Pick<Storage, 'getItem' | 'setItem'> {
  const values = new Map<string, string>()
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => {
      values.set(key, value)
    },
  }
}

describe('chunk load recovery', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('replaces the browser location with the failed target fullPath once', () => {
    const storage = makeStorage()
    const replace = vi.fn()

    const recovered = recoverFromChunkLoadError(
      new Error('Failed to fetch dynamically imported module'),
      '/keys?tab=usage#daily',
      {
        storage,
        location: { replace },
        now: () => 123,
      }
    )

    expect(recovered).toBe(true)
    expect(replace).toHaveBeenCalledOnce()
    expect(replace).toHaveBeenCalledWith('/keys?tab=usage#daily')
  })

  it('does not retry the same target more than once', () => {
    const storage = makeStorage()
    const replace = vi.fn()

    recoverFromChunkLoadError(new Error('Loading chunk 42 failed'), '/dashboard', {
      storage,
      location: { replace },
      now: () => 123,
    })
    const recovered = recoverFromChunkLoadError(new Error('Loading chunk 42 failed'), '/dashboard', {
      storage,
      location: { replace },
      now: () => 456,
    })

    expect(recovered).toBe(true)
    expect(replace).toHaveBeenCalledTimes(1)
  })

  it('allows a different target to retry independently', () => {
    const storage = makeStorage()
    const replace = vi.fn()

    recoverFromChunkLoadError({ name: 'ChunkLoadError' }, '/dashboard', {
      storage,
      location: { replace },
      now: () => 123,
    })
    recoverFromChunkLoadError({ name: 'ChunkLoadError' }, '/keys', {
      storage,
      location: { replace },
      now: () => 456,
    })

    expect(replace).toHaveBeenCalledTimes(2)
    expect(replace).toHaveBeenLastCalledWith('/keys')
  })

  it('ignores non chunk errors', () => {
    const replace = vi.fn()

    const recovered = recoverFromChunkLoadError(new Error('auth failed'), '/keys', {
      storage: makeStorage(),
      location: { replace },
      now: () => 123,
    })

    expect(recovered).toBe(false)
    expect(replace).not.toHaveBeenCalled()
  })
})
